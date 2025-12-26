package marketdata

import (
	"time"

	"github.com/google/uuid"
)

// Candle represents an OHLCV record for a specific interval (docs/marketdata_doc.md).
type Candle struct {
	ID              uuid.UUID
	InstrumentUID   uuid.UUID
	IntervalSeconds int64
	PeriodStart     time.Time
	Open            float64
	High            float64
	Low             float64
	Close           float64
	VolumeLots      int64
	VolumeBuyLots   *int64
	VolumeSellLots  *int64
	LastTradeAt     *time.Time
	Metadata        map[string]any
}
