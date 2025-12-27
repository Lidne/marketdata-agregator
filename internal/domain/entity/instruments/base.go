package instruments

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type InstrumentType string

const (
	ShareType    InstrumentType = "share"
	FutureType   InstrumentType = "future"
	CurrencyType InstrumentType = "currency"
	BondType     InstrumentType = "bond"
	EtfType      InstrumentType = "etf"
	AllType      InstrumentType = "*"
)

type AssetType string

const (
	AssetTypeIndex     AssetType = "TYPE_INDEX"
	AssetTypeCommodity AssetType = "TYPE_COMMODITY"
	AssetTypeSecurity  AssetType = "TYPE_SECURITY"
	AssetTypeCurrency  AssetType = "TYPE_CURRENCY"
)

func (at AssetType) String() string {
	return string(at)
}

func (at AssetType) IsValid() bool {
	switch at {
	case AssetTypeIndex, AssetTypeCommodity, AssetTypeSecurity, AssetTypeCurrency:
		return true
	default:
		return false
	}
}

func NewAssetType(s string) (AssetType, error) {
	at := AssetType(s)
	if !at.IsValid() {
		return "", fmt.Errorf("invalid asset type: %s", s)
	}
	return at, nil
}

type InstrumentModel interface {
	GetUID() uuid.UUID
	GetPrice(points float64) float64
	GetFigi() string
	GetTicker() string
	GetLots() int32
	GetMinPriceIncrement() float64
	GetMinPriceIncrementAmount() float64
	GetAssetType() AssetType
}

// Instrument corresponds to the base table `instruments` from docs/db_doc.md.
// Specific instrument tables (shares/bonds/futures/...) reference this row by UID (1:1).
type Instrument struct {
	UID       uuid.UUID
	Figi      string
	Ticker    string
	Lot       int32
	ClassCode string
	LogoURL   string
	BrandUID  uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (i Instrument) GetUID() uuid.UUID { return i.UID }
func (i Instrument) GetFigi() string   { return i.Figi }
func (i Instrument) GetTicker() string { return i.Ticker }
func (i Instrument) GetLots() int32    { return i.Lot }
