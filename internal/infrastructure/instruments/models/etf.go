package models

type EtfModel struct {
	BaseModel
	MinPriceIncrement float64 `gorm:"column:min_price_increment;type:decimal(10,6)"`
}

func (e EtfModel) TableName() string {
	return "etfs"
}

func (e EtfModel) GetFigi() string {
	return e.Figi
}

func (e EtfModel) GetTicker() string {
	return e.Ticker
}

func (e EtfModel) GetLots() int32 {
	return e.Lot
}

func (e EtfModel) GetMinPriceIncrement() float64 {
	return e.MinPriceIncrement
}

func (e EtfModel) GetMinPriceIncrementAmount() float64 {
	return 0
}

func (e EtfModel) GetAssetType() AssetType {
	return AssetTypeSecurity
}

func (e EtfModel) GetPrice(points float64) float64 {
	return float64(e.GetLots()) * points
}
