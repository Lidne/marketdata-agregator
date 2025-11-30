package models

type CurrencyModel struct {
	BaseModel
}

func (CurrencyModel) TableName() string {
	return "currencies"
}

func (c CurrencyModel) GetFigi() string {
	return c.Figi
}

func (c CurrencyModel) GetTicker() string {
	return c.Ticker
}

func (c CurrencyModel) GetLots() int32 {
	return c.Lot
}

func (c CurrencyModel) GetMinPriceIncrement() float64 {
	return 0
}

func (c CurrencyModel) GetMinPriceIncrementAmount() float64 {
	return 0
}

func (c CurrencyModel) GetAssetType() AssetType {
	return AssetTypeCurrency
}

func (c CurrencyModel) GetPrice(points float64) float64 {
	return float64(c.GetLots()) * points
}
