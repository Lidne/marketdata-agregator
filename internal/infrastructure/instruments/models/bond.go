package models

type BondModel struct {
	BaseModel
	Nominal  float64 `gorm:"column:nominal;type:decimal(10,2)"`
	AciValue float64 `gorm:"column:aci_value;type:decimal(10,2)"`
}

func (b BondModel) TableName() string {
	return "bonds"
}

func (b BondModel) GetFigi() string {
	return b.Figi
}

func (b BondModel) GetTicker() string {
	return b.Ticker
}

func (b BondModel) GetLots() int32 {
	return b.Lot
}

func (b BondModel) GetMinPriceIncrement() float64 {
	return 0
}

func (b BondModel) GetMinPriceIncrementAmount() float64 {
	return 0
}

func (b BondModel) GetAssetType() AssetType {
	return AssetTypeSecurity
}

func (b BondModel) GetPrice(points float64) float64 {
	return (points / 100) * b.Nominal
}
