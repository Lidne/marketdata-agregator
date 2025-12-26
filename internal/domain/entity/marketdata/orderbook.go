package marketdata

import (
	"time"

	"github.com/google/uuid"
)

// OrderBookLevel holds price/quantity pair for bids/asks within a snapshot.
type OrderBookLevel struct {
	Price    float64
	Quantity int64
}

// OrderBookSnapshot represents a captured order book at a specific time/depth (docs/marketdata_doc.md).
type OrderBookSnapshot struct {
	ID            uuid.UUID
	InstrumentUID uuid.UUID
	SnapshotAt    time.Time
	Depth         int32
	Bids          []OrderBookLevel
	Asks          []OrderBookLevel
	Metadata      map[string]any
}
