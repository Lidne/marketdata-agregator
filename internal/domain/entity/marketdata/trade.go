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
	ID            uuid.UUID
	InstrumentUID uuid.UUID
	Side          TradeSide
	Price         float64
	QuantityLots  int64
	TradedAt      time.Time
	Metadata      map[string]any
}
