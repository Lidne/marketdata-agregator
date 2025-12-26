package instruments

// Future is a joined domain entity: base `instruments` row + `futures` row.
type Future struct {
	Instrument
	MinPriceIncrement       float64
	MinPriceIncrementAmount float64
	AssetType               AssetType
}

func (f Future) GetMinPriceIncrement() float64       { return f.MinPriceIncrement }
func (f Future) GetMinPriceIncrementAmount() float64 { return f.MinPriceIncrementAmount }
func (f Future) GetAssetType() AssetType             { return f.AssetType }
func (f Future) GetPrice(points float64) float64 {
	return (points / f.GetMinPriceIncrement()) * f.GetMinPriceIncrementAmount()
}
