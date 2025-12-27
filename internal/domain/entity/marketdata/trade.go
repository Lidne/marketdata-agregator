package marketdata

import (
	"time"

	"github.com/google/uuid"
)

// TradeSide represents BUY/SELL direction derived from the incoming stream.
type TradeSide string

const (
	TradeSideBuy  TradeSide = "BUY"
	TradeSideSell TradeSide = "SELL"
)

// Trade models a single executed trade (see docs/marketdata_doc.md).
type Trade struct {
	ID            uuid.UUID      `json:"id"`
	InstrumentUID uuid.UUID      `json:"instrument_uid"`
	Side          TradeSide      `json:"side"`
	Price         float64        `json:"price"`
	QuantityLots  int64          `json:"quantity_lots"`
	TradedAt      time.Time      `json:"traded_at"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}
