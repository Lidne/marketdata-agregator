package instruments

import (
	"main/internal/infrastructure/instruments/models"
)

type Instrument interface {
	GetFigi() string
	GetTicker() string
	GetType() models.InstrumentType
}

type InstrumentWrapper struct {
	// share          *ShareModel
	// future         *FutureModel
	// currency       *CurrencyModel
	instrument     models.InstrumentModel
	instrumentType models.InstrumentType
}

func NewInstrumentsWrapper(instrument models.InstrumentModel, instrType models.InstrumentType) *InstrumentWrapper {
	return &InstrumentWrapper{instrument: instrument, instrumentType: instrType}
}

func (instrWrapper *InstrumentWrapper) GetType() models.InstrumentType {
	return instrWrapper.instrumentType
}

func (instrWrapper *InstrumentWrapper) GetFigi() string {
	if instrWrapper.instrument != nil {
		return instrWrapper.instrument.GetFigi()
	}
	return ""
}

func (instrWrapper *InstrumentWrapper) GetTicker() string {
	if instrWrapper.instrument != nil {
		return instrWrapper.instrument.GetTicker()
	}
	return ""
}

func (instrWrapper *InstrumentWrapper) GetInstrumentType() string {
	return string(instrWrapper.instrumentType)
}

func (instrWrapper *InstrumentWrapper) GetLots() int32 {
	if instrWrapper.instrument != nil {
		return instrWrapper.instrument.GetLots()
	}
	return 1
}

// GetPrice - Возвращает стоимость одного лота
func (instrWrapper *InstrumentWrapper) GetPrice(points float64) float64 {
	return instrWrapper.instrument.GetPrice(points)
}
