package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
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
	GetPrice(points float64) float64
	GetFigi() string
	GetTicker() string
	GetLots() int32
	GetMinPriceIncrement() float64
	GetMinPriceIncrementAmount() float64
	GetAssetType() AssetType
}

type BaseModel struct {
	Figi            string         `gorm:"primaryKey;column:figi;type:varchar(255);not null"`
	Ticker          string         `gorm:"column:ticker;type:varchar(50);not null;index"`
	Lot             int32          `gorm:"column:lot;type:integer;not null"`
	ClassCode       string         `gorm:"column:class_code;type:varchar(50)"`
	LogoUrl         string         `gorm:"column:logo_url;type:varchar"`
	InstrumentGroup InstrumentType `gorm:"column:instrument_group;type:varchar(50)"`
	CreatedAt       time.Time      `gorm:"column:created_at;type:timestamp;default:CURRENT_TIMESTAMP"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;type:timestamp;default:CURRENT_TIMESTAMP"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;type:timestamp;index"`
}
