package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	investgo "github.com/russianinvestments/invest-api-go-sdk/investgo"
	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	domain "main/internal/domain/entity/marketdata"
)

const (
	defaultInvestEndpoint     = "https://invest-public-api.tinkoff.ru:443"
	defaultAppName            = "marketdata-producer"
	defaultRabbitURL          = "amqp://guest:guest@localhost:5672/"
	defaultInstrumentsFile    = "cmd/producer/instruments.json"
	defaultTradesExchange     = "marketdata.trades"
	defaultCandlesExchange    = "marketdata.candles"
	defaultOrderBooksExchange = "marketdata.orderbooks"
)

type producerConfig struct {
	Token              string
	Endpoint           string
	AppName            string
	SkipTLSVerify      bool
	RabbitURL          string
	Exchanges          exchangeSet
	Instruments        []string
	CandleInterval     pb.SubscriptionInterval
	CandleWaitingClose bool
	OrderBookDepth     int32
	TradeSource        pb.TradeSourceType
}

type exchangeSet struct {
	Trades     string
	Candles    string
	OrderBooks string
}

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := loadConfig()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rabbitConn, err := amqp.Dial(cfg.RabbitURL)
	if err != nil {
		logger.Fatalf("connect rabbitmq: %v", err)
	}
	defer rabbitConn.Close()

	pub, err := newPublisher(rabbitConn, cfg.Exchanges, logger)
	if err != nil {
		logger.Fatalf("init publisher: %v", err)
	}
	defer pub.Close()

	investCfg := investgo.Config{
		EndPoint:           cfg.Endpoint,
		Token:              cfg.Token,
		AppName:            cfg.AppName,
		InsecureSkipVerify: cfg.SkipTLSVerify,
	}

	client, err := investgo.NewClient(ctx, investCfg, logger)
	if err != nil {
		logger.Fatalf("create invest api client: %v", err)
	}
	defer func() {
		if stopErr := client.Stop(); stopErr != nil {
			logger.Errorf("stop invest api client: %v", stopErr)
		}
	}()

	mdClient := client.NewMarketDataStreamClient()
	stream, err := mdClient.MarketDataStream()
	if err != nil {
		logger.Fatalf("create market data stream: %v", err)
	}
	defer stream.Stop()

	candleChan, err := stream.SubscribeCandle(cfg.Instruments, cfg.CandleInterval, cfg.CandleWaitingClose, nil)
	if err != nil {
		logger.Fatalf("subscribe candles: %v", err)
	}

	tradeChan, err := stream.SubscribeTrade(cfg.Instruments, cfg.TradeSource, false)
	if err != nil {
		logger.Fatalf("subscribe trades: %v", err)
	}

	orderBookChan, err := stream.SubscribeOrderBook(cfg.Instruments, cfg.OrderBookDepth)
	if err != nil {
		logger.Fatalf("subscribe order books: %v", err)
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return stream.Listen()
	})
	g.Go(func() error {
		return pumpCandles(gctx, candleChan, pub, logger)
	})
	g.Go(func() error {
		return pumpTrades(gctx, tradeChan, pub, logger)
	})
	g.Go(func() error {
		return pumpOrderBooks(gctx, orderBookChan, pub, logger)
	})

	logger.WithFields(logrus.Fields{
		"instruments":  len(cfg.Instruments),
		"trades_ex":    cfg.Exchanges.Trades,
		"candles_ex":   cfg.Exchanges.Candles,
		"orderbook_ex": cfg.Exchanges.OrderBooks,
	}).Info("producer started")

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("producer stopped with error: %v", err)
	}

	logger.Info("producer stopped")
}

func loadConfig() (*producerConfig, error) {
	token := strings.TrimSpace(os.Getenv("INVEST_TOKEN"))
	if token == "" {
		return nil, errors.New("INVEST_TOKEN is required")
	}

	endpoint := envOrDefault("INVEST_ENDPOINT", defaultInvestEndpoint)
	appName := envOrDefault("INVEST_APP_NAME", defaultAppName)
	rabbitURL := envOrDefault("RABBITMQ_URL", defaultRabbitURL)

	instrumentsFile := envOrDefault("INSTRUMENTS_FILE", defaultInstrumentsFile)
	instruments, err := readInstruments(instrumentsFile)
	if err != nil {
		return nil, err
	}
	if len(instruments) == 0 {
		return nil, errors.New("instruments list is empty")
	}

	exchanges := exchangeSet{
		Trades:     envOrDefault("RABBITMQ_TRADES_EXCHANGE", defaultTradesExchange),
		Candles:    envOrDefault("RABBITMQ_CANDLES_EXCHANGE", defaultCandlesExchange),
		OrderBooks: envOrDefault("RABBITMQ_ORDERBOOKS_EXCHANGE", defaultOrderBooksExchange),
	}

	orderBookDepth := intEnv("ORDERBOOK_DEPTH", 10)
	if orderBookDepth <= 0 {
		orderBookDepth = 10
	}

	waitingClose := boolEnv("CANDLE_WAITING_CLOSE", true)
	skipVerify := boolEnv("INVEST_INSECURE_SKIP_VERIFY", true)

	return &producerConfig{
		Token:              token,
		Endpoint:           endpoint,
		AppName:            appName,
		SkipTLSVerify:      skipVerify,
		RabbitURL:          rabbitURL,
		Exchanges:          exchanges,
		Instruments:        instruments,
		CandleInterval:     pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE,
		CandleWaitingClose: waitingClose,
		OrderBookDepth:     int32(orderBookDepth),
		TradeSource:        pb.TradeSourceType_TRADE_SOURCE_EXCHANGE,
	}, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func readInstruments(path string) ([]string, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read instruments file: %w", err)
	}
	var payload struct {
		Instruments []string `json:"instruments"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse instruments file: %w", err)
	}
	instruments := make([]string, 0, len(payload.Instruments))
	for _, figi := range payload.Instruments {
		figi = strings.TrimSpace(figi)
		if figi != "" {
			instruments = append(instruments, figi)
		}
	}
	return instruments, nil
}

type publisher struct {
	channel   *amqp.Channel
	exchanges exchangeSet
	logger    *logrus.Logger
	mu        sync.Mutex
}

func newPublisher(conn *amqp.Connection, exchanges exchangeSet, logger *logrus.Logger) (*publisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}

	declared := map[string]struct{}{}
	for _, name := range []string{exchanges.Trades, exchanges.Candles, exchanges.OrderBooks} {
		if name == "" {
			ch.Close()
			return nil, errors.New("exchange name cannot be empty")
		}
		if _, ok := declared[name]; ok {
			continue
		}
		if err := ch.ExchangeDeclare(name, "fanout", true, false, false, false, nil); err != nil {
			ch.Close()
			return nil, fmt.Errorf("declare exchange %s: %w", name, err)
		}
		declared[name] = struct{}{}
	}

	return &publisher{
		channel:   ch,
		exchanges: exchanges,
		logger:    logger,
	}, nil
}

func (p *publisher) Close() {
	if p == nil {
		return
	}
	if err := p.channel.Close(); err != nil {
		p.logger.Errorf("close rabbitmq channel: %v", err)
	}
}

func (p *publisher) PublishCandle(ctx context.Context, candle *domain.Candle) error {
	return p.publish(ctx, p.exchanges.Candles, candle)
}

func (p *publisher) PublishTrade(ctx context.Context, trade *domain.Trade) error {
	return p.publish(ctx, p.exchanges.Trades, trade)
}

func (p *publisher) PublishOrderBook(ctx context.Context, snapshot *domain.OrderBookSnapshot) error {
	return p.publish(ctx, p.exchanges.OrderBooks, snapshot)
}

func (p *publisher) publish(ctx context.Context, exchange string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	return p.channel.PublishWithContext(ctx, exchange, "", false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
}

func pumpCandles(ctx context.Context, stream <-chan *pb.Candle, pub *publisher, logger *logrus.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case candle, ok := <-stream:
			if !ok {
				return nil
			}
			entity, err := convertCandle(candle)
			if err != nil {
				logger.WithError(err).Warn("skip candle")
				continue
			}
			if candle == nil {
				continue
			}
			if err := pub.PublishCandle(ctx, entity); err != nil {
				return fmt.Errorf("publish candle: %w", err)
			}
		}
	}
}

func pumpTrades(ctx context.Context, stream <-chan *pb.Trade, pub *publisher, logger *logrus.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case trade, ok := <-stream:
			if !ok {
				return nil
			}
			entity, err := convertTrade(trade)
			if err != nil {
				logger.WithError(err).Warn("skip trade")
				continue
			}
			if err := pub.PublishTrade(ctx, entity); err != nil {
				return fmt.Errorf("publish trade: %w", err)
			}
		}
	}
}

func pumpOrderBooks(ctx context.Context, stream <-chan *pb.OrderBook, pub *publisher, logger *logrus.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case snapshot, ok := <-stream:
			if !ok {
				return nil
			}
			entity, err := convertOrderBook(snapshot)
			if err != nil {
				logger.WithError(err).Warn("skip order book")
				continue
			}
			if err := pub.PublishOrderBook(ctx, entity); err != nil {
				return fmt.Errorf("publish order book: %w", err)
			}
		}
	}
}

func convertCandle(msg *pb.Candle) (*domain.Candle, error) {
	if msg == nil {
		return nil, errors.New("candle payload is nil")
	}

	instrumentID, err := parseInstrumentUID(msg.GetInstrumentUid())
	if err != nil {
		return nil, err
	}

	intervalSeconds, err := candleIntervalToSeconds(msg.GetInterval())
	if err != nil {
		return nil, err
	}
	if intervalSeconds == 0 {
		return nil, nil
	}

	periodStart := time.Time{}
	if ts := msg.GetTime(); ts != nil {
		periodStart = ts.AsTime().UTC()
	}

	var lastTradeAt *time.Time
	if ts := msg.GetLastTradeTs(); ts != nil {
		t := ts.AsTime().UTC()
		lastTradeAt = &t
	}

	metadata := map[string]any{}
	if figi := strings.TrimSpace(msg.GetFigi()); figi != "" {
		metadata["figi"] = figi
	}
	metadata["interval"] = msg.GetInterval().String()
	metadata["source"] = msg.GetCandleSourceType().String()

	if len(metadata) == 0 {
		metadata = nil
	}

	return &domain.Candle{
		ID:              uuid.New(),
		InstrumentUID:   instrumentID,
		IntervalSeconds: intervalSeconds,
		PeriodStart:     periodStart,
		Open:            quotationToFloat(msg.GetOpen()),
		High:            quotationToFloat(msg.GetHigh()),
		Low:             quotationToFloat(msg.GetLow()),
		Close:           quotationToFloat(msg.GetClose()),
		VolumeLots:      msg.GetVolume(),
		LastTradeAt:     lastTradeAt,
		Metadata:        metadata,
	}, nil
}

func convertTrade(msg *pb.Trade) (*domain.Trade, error) {
	if msg == nil {
		return nil, errors.New("trade payload is nil")
	}

	instrumentID, err := parseInstrumentUID(msg.GetInstrumentUid())
	if err != nil {
		return nil, err
	}

	side, err := mapTradeSide(msg.GetDirection())
	if err != nil {
		return nil, err
	}

	tradedAt := time.Time{}
	if ts := msg.GetTime(); ts != nil {
		tradedAt = ts.AsTime().UTC()
	}

	metadata := map[string]any{}
	if figi := strings.TrimSpace(msg.GetFigi()); figi != "" {
		metadata["figi"] = figi
	}
	metadata["trade_source"] = msg.GetTradeSource().String()
	if len(metadata) == 0 {
		metadata = nil
	}

	return &domain.Trade{
		ID:            uuid.New(),
		InstrumentUID: instrumentID,
		Side:          side,
		Price:         quotationToFloat(msg.GetPrice()),
		QuantityLots:  msg.GetQuantity(),
		TradedAt:      tradedAt,
		Metadata:      metadata,
	}, nil
}

func convertOrderBook(msg *pb.OrderBook) (*domain.OrderBookSnapshot, error) {
	if msg == nil {
		return nil, errors.New("order book payload is nil")
	}

	instrumentID, err := parseInstrumentUID(msg.GetInstrumentUid())
	if err != nil {
		return nil, err
	}

	snapshotAt := time.Time{}
	if ts := msg.GetTime(); ts != nil {
		snapshotAt = ts.AsTime().UTC()
	}

	bids := make([]domain.OrderBookLevel, 0, len(msg.GetBids()))
	for _, level := range msg.GetBids() {
		bids = append(bids, domain.OrderBookLevel{
			Price:    quotationToFloat(level.GetPrice()),
			Quantity: level.GetQuantity(),
		})
	}

	asks := make([]domain.OrderBookLevel, 0, len(msg.GetAsks()))
	for _, level := range msg.GetAsks() {
		asks = append(asks, domain.OrderBookLevel{
			Price:    quotationToFloat(level.GetPrice()),
			Quantity: level.GetQuantity(),
		})
	}

	metadata := map[string]any{}
	if figi := strings.TrimSpace(msg.GetFigi()); figi != "" {
		metadata["figi"] = figi
	}
	metadata["is_consistent"] = msg.GetIsConsistent()
	metadata["order_book_type"] = msg.GetOrderBookType().String()
	if len(metadata) == 0 {
		metadata = nil
	}

	return &domain.OrderBookSnapshot{
		ID:            uuid.New(),
		InstrumentUID: instrumentID,
		SnapshotAt:    snapshotAt,
		Depth:         msg.GetDepth(),
		Bids:          bids,
		Asks:          asks,
		Metadata:      metadata,
	}, nil
}

func quotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return q.ToFloat()
}

func parseInstrumentUID(raw string) (uuid.UUID, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return uuid.Nil, errors.New("instrument uid is empty")
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse instrument uid: %w", err)
	}
	return id, nil
}

func mapTradeSide(direction pb.TradeDirection) (domain.TradeSide, error) {
	switch direction {
	case pb.TradeDirection_TRADE_DIRECTION_BUY:
		return domain.TradeSideBuy, nil
	case pb.TradeDirection_TRADE_DIRECTION_SELL:
		return domain.TradeSideSell, nil
	default:
		return "", fmt.Errorf("unsupported trade direction: %s", direction.String())
	}
}

func candleIntervalToSeconds(interval pb.SubscriptionInterval) (int64, error) {
	switch interval {
	case pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE:
		return 60, nil
	default:
		return 0, nil
	}
}
