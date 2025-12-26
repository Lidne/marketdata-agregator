package instruments

// Bond is a joined domain entity: base `instruments` row + `bonds` row.
type Bond struct {
	Instrument
	Nominal  float64
	AciValue float64
}

func (b Bond) GetMinPriceIncrement() float64       { return 0 }
func (b Bond) GetMinPriceIncrementAmount() float64 { return 0 }
func (b Bond) GetAssetType() AssetType             { return AssetTypeSecurity }
func (b Bond) GetPrice(points float64) float64     { return (points / 100) * b.Nominal }
