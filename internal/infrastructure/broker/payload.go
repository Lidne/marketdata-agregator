package broker

import domain "main/internal/domain/entity/marketdata"

type BaseMessage struct {
	Trade             *domain.Trade             `json:"trade,omitempty"`
	Candle            *domain.Candle            `json:"candle,omitempty"`
	OrderBookSnapshot *domain.OrderBookSnapshot `json:"order_book_snapshot,omitempty"`
}
