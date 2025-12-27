package marketdata

import (
	"time"

	"github.com/google/uuid"
)

// OrderBookLevel holds price/quantity pair for bids/asks within a snapshot.
type OrderBookLevel struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
}

// OrderBookSnapshot represents a captured order book at a specific time/depth (docs/marketdata_doc.md).
type OrderBookSnapshot struct {
	ID            uuid.UUID        `json:"id"`
	InstrumentUID uuid.UUID        `json:"instrument_uid"`
	SnapshotAt    time.Time        `json:"snapshot_at"`
	Depth         int32            `json:"depth"`
	Bids          []OrderBookLevel `json:"bids"`
	Asks          []OrderBookLevel `json:"asks"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
}
