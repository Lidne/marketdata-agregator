package streamer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"main/internal/config"
	domain "main/internal/domain/entity/marketdata"

	amqp "github.com/rabbitmq/amqp091-go"
)

// message is the envelope published to each fanout exchange,
// matching the format consumed by the broker.Consumer.
type message struct {
	Trade     *domain.Trade             `json:"trade,omitempty"`
	Candle    *domain.Candle            `json:"candle,omitempty"`
	OrderBook *domain.OrderBookSnapshot `json:"order_book_snapshot,omitempty"`
}

// Publisher establishes a single AMQP connection and publishes
// market data entities to the configured fanout exchanges.
type Publisher struct {
	cfg  config.RabbitMQConfig
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewPublisher connects to RabbitMQ and declares all three fanout exchanges.
func NewPublisher(cfg config.RabbitMQConfig) (*Publisher, error) {
	if cfg.URL == "" {
		return nil, errors.New("rabbitmq url is required")
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	for _, exchange := range []string{cfg.TradesExchange, cfg.CandlesExchange, cfg.OrderBooksExchange} {
		if err := ch.ExchangeDeclare(exchange, "fanout", true, false, false, false, nil); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("declare exchange %q: %w", exchange, err)
		}
	}

	return &Publisher{cfg: cfg, conn: conn, ch: ch}, nil
}

// PublishTrade publishes a single trade to the trades fanout exchange.
func (p *Publisher) PublishTrade(_ context.Context, trade *domain.Trade) error {
	if trade == nil {
		return errors.New("trade is nil")
	}
	return p.publish(p.cfg.TradesExchange, message{Trade: trade})
}

// PublishCandle publishes a single candle to the candles fanout exchange.
func (p *Publisher) PublishCandle(_ context.Context, candle *domain.Candle) error {
	if candle == nil {
		return errors.New("candle is nil")
	}
	return p.publish(p.cfg.CandlesExchange, message{Candle: candle})
}

// PublishOrderBook publishes a single order book snapshot to the order books fanout exchange.
func (p *Publisher) PublishOrderBook(_ context.Context, snapshot *domain.OrderBookSnapshot) error {
	if snapshot == nil {
		return errors.New("order book snapshot is nil")
	}
	return p.publish(p.cfg.OrderBooksExchange, message{OrderBook: snapshot})
}

// Close releases the AMQP channel and connection.
func (p *Publisher) Close() error {
	var errs []error
	if p.ch != nil {
		if err := p.ch.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close channel: %w", err))
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close connection: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (p *Publisher) publish(exchange string, msg message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return p.ch.Publish(exchange, "", false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}
