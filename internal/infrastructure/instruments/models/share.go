package models

type ShareModel struct {
	BaseModel
}

func (ShareModel) TableName() string {
	return "shares"
}

func (s ShareModel) GetFigi() string {
	return s.Figi
}

func (s ShareModel) GetTicker() string {
	return s.Ticker
}

func (s ShareModel) GetLots() int32 {
	return s.Lot
}

func (s ShareModel) GetMinPriceIncrement() float64 {
	return 0
}

func (s ShareModel) GetMinPriceIncrementAmount() float64 {
	return 0
}

func (s ShareModel) GetAssetType() AssetType {
	return AssetTypeSecurity
}

func (s ShareModel) GetPrice(points float64) float64 {
	return float64(s.GetLots()) * points
}
