package instruments

// Currency is a joined domain entity: base `instruments` row + `currencies` row (which has no extra fields).
type Currency struct {
	Instrument
}

func (c Currency) GetMinPriceIncrement() float64       { return 0 }
func (c Currency) GetMinPriceIncrementAmount() float64 { return 0 }
func (c Currency) GetAssetType() AssetType             { return AssetTypeCurrency }
func (c Currency) GetPrice(points float64) float64     { return float64(c.GetLots()) * points }
