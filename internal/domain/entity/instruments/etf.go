package instruments

// Etf is a joined domain entity: base `instruments` row + `etfs` row.
type Etf struct {
	Instrument
	MinPriceIncrement float64
}

func (e Etf) GetMinPriceIncrement() float64       { return e.MinPriceIncrement }
func (e Etf) GetMinPriceIncrementAmount() float64 { return 0 }
func (e Etf) GetAssetType() AssetType             { return AssetTypeSecurity }
func (e Etf) GetPrice(points float64) float64     { return float64(e.GetLots()) * points }
