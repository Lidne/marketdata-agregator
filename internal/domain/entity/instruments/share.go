package instruments

// Share is a joined domain entity: base `instruments` row + `shares` row (which has no extra fields).
type Share struct {
	Instrument
}

func (s Share) GetMinPriceIncrement() float64       { return 0 }
func (s Share) GetMinPriceIncrementAmount() float64 { return 0 }
func (s Share) GetAssetType() AssetType             { return AssetTypeSecurity }
func (s Share) GetPrice(points float64) float64     { return float64(s.GetLots()) * points }
