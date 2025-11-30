package models

type FutureModel struct {
	BaseModel
	MinPriceIncrement       float64   `gorm:"column:min_price_increment;type:decimal(10,6)"`
	MinPriceIncrementAmount float64   `gorm:"column:min_price_increment_amount;type:decimal(10,6)"`
	AssetType               AssetType `gorm:"column:asset_type;type:varchar(20);not null"`
}

func (FutureModel) TableName() string {
	return "futures"
}

func (f FutureModel) GetFigi() string {
	return f.Figi
}

func (f FutureModel) GetTicker() string {
	return f.Ticker
}

func (f FutureModel) GetLots() int32 {
	return f.Lot
}

func (f FutureModel) GetMinPriceIncrement() float64 {
	return f.MinPriceIncrement
}

func (f FutureModel) GetMinPriceIncrementAmount() float64 {
	return f.MinPriceIncrementAmount
}

func (f FutureModel) GetAssetType() AssetType {
	return f.AssetType
}

func (f FutureModel) GetPrice(points float64) float64 {
	return (points / f.GetMinPriceIncrement()) * f.GetMinPriceIncrementAmount()
}
