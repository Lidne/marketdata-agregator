package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	appmarketdata "main/internal/application/service/marketdata"
	"main/internal/config"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Consumer subscribes to RabbitMQ fanout exchanges and forwards messages
// into the market data service via buffered batch writers.
type Consumer struct {
	cfg     config.RabbitMQConfig
	service *appmarketdata.Service
	logger  *logrus.Logger

	conn     *amqp.Connection
	channels []*amqp.Channel
	wg       sync.WaitGroup
	batcher  *BatchWriter
}

// NewConsumer prepares a consumer for the given configuration.
func NewConsumer(cfg config.RabbitMQConfig, service *appmarketdata.Service, logger *logrus.Logger) (*Consumer, error) {
	if cfg.URL == "" {
		return nil, errors.New("rabbitmq url is required")
	}
	batchCfg := BatchConfig{
		Size:    cfg.BatchSize,
		Timeout: cfg.BatchTimeout,
	}
	consumer := &Consumer{
		cfg:     cfg,
		service: service,
		logger:  logger,
		batcher: NewBatchWriter(batchCfg, service, logger),
	}
	return consumer, nil
}

// Start establishes the AMQP connection and begins consuming fanout exchanges.
func (c *Consumer) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	conn, err := amqp.Dial(c.cfg.URL)
	if err != nil {
		return fmt.Errorf("connect to rabbitmq: %w", err)
	}
	c.conn = conn
	c.batcher.Run(ctx)

	if err := c.startStream(ctx, streamTrade, c.cfg.TradesExchange); err != nil {
		c.Close(ctx)
		return err
	}
	if err := c.startStream(ctx, streamCandle, c.cfg.CandlesExchange); err != nil {
		c.Close(ctx)
		return err
	}
	if err := c.startStream(ctx, streamOrderBook, c.cfg.OrderBooksExchange); err != nil {
		c.Close(ctx)
		return err
	}

	c.logger.Infof("rabbitmq consumer started: exchanges=%s,%s,%s", c.cfg.TradesExchange, c.cfg.CandlesExchange, c.cfg.OrderBooksExchange)
	return nil
}

// Close stops consumption, flushes pending batches, and releases resources.
func (c *Consumer) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for _, ch := range c.channels {
		_ = ch.Close()
	}
	c.channels = nil
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.wg.Wait()
	if c.batcher == nil {
		return nil
	}
	return c.batcher.Stop(ctx)
}

func (c *Consumer) startStream(ctx context.Context, stream streamType, exchange string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel for %s: %w", stream, err)
	}
	if err := ch.ExchangeDeclare(exchange, "fanout", true, false, false, false, nil); err != nil {
		ch.Close()
		return fmt.Errorf("declare exchange %s: %w", exchange, err)
	}
	queue, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		ch.Close()
		return fmt.Errorf("declare queue for %s: %w", stream, err)
	}
	if err := ch.QueueBind(queue.Name, "", exchange, false, nil); err != nil {
		ch.Close()
		return fmt.Errorf("bind queue %s to %s: %w", queue.Name, exchange, err)
	}
	prefetch := c.cfg.Prefetch
	if prefetch <= 0 {
		prefetch = 1
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		ch.Close()
		return fmt.Errorf("set qos for %s: %w", stream, err)
	}
	deliveries, err := ch.Consume(queue.Name, "", false, true, false, false, nil)
	if err != nil {
		ch.Close()
		return fmt.Errorf("start consume for %s: %w", stream, err)
	}
	c.channels = append(c.channels, ch)
	c.wg.Add(1)
	go c.consumeLoop(ctx, stream, deliveries)
	return nil
}

func (c *Consumer) consumeLoop(ctx context.Context, stream streamType, deliveries <-chan amqp.Delivery) {
	defer c.wg.Done()
	log := c.logger.WithField("stream", string(stream))
	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-deliveries:
			if !ok {
				return
			}
			if err := c.handleDelivery(stream, &delivery); err != nil {
				log.WithError(err).Warn("failed to process message")
				_ = delivery.Nack(false, true)
				continue
			}
			if err := delivery.Ack(false); err != nil {
				log.WithError(err).Warn("failed to ack delivery")
			}
		}
	}
}

func (c *Consumer) handleDelivery(stream streamType, delivery *amqp.Delivery) error {
	var payload BaseMessage
	if err := json.Unmarshal(delivery.Body, &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	switch stream {
	case streamTrade:
		if payload.Trade == nil {
			return errors.New("trade payload is nil")
		}
		return c.batcher.AddTrade(payload.Trade)
	case streamCandle:
		if payload.Candle == nil {
			return errors.New("candle payload is nil")
		}
		return c.batcher.AddCandle(payload.Candle)
	case streamOrderBook:
		if payload.OrderBookSnapshot == nil {
			return errors.New("order book payload is nil")
		}
		return c.batcher.AddOrderBook(payload.OrderBookSnapshot)
	default:
		return fmt.Errorf("unsupported stream: %s", stream)
	}
}

type streamType string

func (s streamType) String() string {
	return string(s)
}

const (
	streamTrade     streamType = "trades"
	streamCandle    streamType = "candles"
	streamOrderBook streamType = "orderbooks"
)
