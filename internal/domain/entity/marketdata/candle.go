package marketdata

import (
	"time"

	"github.com/google/uuid"
)

// Candle represents an OHLCV record for a specific interval (docs/marketdata_doc.md).
type Candle struct {
	ID              uuid.UUID      `json:"id"`
	InstrumentUID   uuid.UUID      `json:"instrument_uid"`
	IntervalSeconds int64          `json:"interval_seconds"`
	PeriodStart     time.Time      `json:"period_start"`
	Open            float64        `json:"open"`
	High            float64        `json:"high"`
	Low             float64        `json:"low"`
	Close           float64        `json:"close"`
	VolumeLots      int64          `json:"volume_lots"`
	VolumeBuyLots   *int64         `json:"volume_buy_lots,omitempty"`
	VolumeSellLots  *int64         `json:"volume_sell_lots,omitempty"`
	LastTradeAt     *time.Time     `json:"last_trade_at,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}
