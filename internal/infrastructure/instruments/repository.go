package instruments

import (
	"context"
	"main/internal/infrastructure/instruments/models"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/tinkoff/invest-api-go-sdk/investgo"
	"gorm.io/gorm"
)

type InstrumentsRepository struct {
	ctx               context.Context
	db                *gorm.DB
	instruments       map[string]*InstrumentWrapper
	instrumentsClient *investgo.InstrumentsServiceClient
	logger            *logrus.Logger
	mu                sync.RWMutex
}

func NewInstrumentsRepository(ctx context.Context, db *gorm.DB, instrumentsClient *investgo.InstrumentsServiceClient, logger *logrus.Logger) *InstrumentsRepository {
	return &InstrumentsRepository{
		ctx:               ctx,
		db:                db,
		instruments:       make(map[string]*InstrumentWrapper),
		instrumentsClient: instrumentsClient,
		logger:            logger,
	}
}

func (instRepo *InstrumentsRepository) LoadShares() error {
	instRepo.mu.Lock()
	defer instRepo.mu.Unlock()

	var shares []models.ShareModel
	if err := instRepo.db.Find(&shares).Error; err != nil {
		instRepo.logger.Errorf("Failed to get shares IDs: %v", err)
	}

	for _, share := range shares {
		instRepo.instruments[share.Figi] = NewInstrumentsWrapper(&share, models.ShareType)
	}
	instRepo.logger.Infof("Loaded %d shares", len(shares))

	return nil
}

func (instRepo *InstrumentsRepository) LoadFutures() error {
	instRepo.mu.Lock()
	defer instRepo.mu.Unlock()

	var futures []models.FutureModel
	if err := instRepo.db.Find(&futures).Error; err != nil {
		instRepo.logger.Errorf("Failed to get shares IDs: %v", err)
	}

	for _, future := range futures {
		instRepo.instruments[future.Figi] = NewInstrumentsWrapper(&future, future.InstrumentGroup)
	}
	instRepo.logger.Infof("Loaded %d futures", len(futures))

	return nil
}

func (instRepo *InstrumentsRepository) LoadCurrencies() error {
	instRepo.mu.Lock()
	defer instRepo.mu.Unlock()

	var currencies []models.CurrencyModel
	if err := instRepo.db.Find(&currencies).Error; err != nil {
		instRepo.logger.Errorf("Failed to get currencies IDs: %v", err)
	}
	for _, currency := range currencies {
		instRepo.instruments[currency.Figi] = NewInstrumentsWrapper(&currency, currency.InstrumentGroup)
	}
	instRepo.logger.Infof("Loaded %d currencies", len(currencies))

	return nil
}
func (instRepo *InstrumentsRepository) LoadEtfs() error {
	instRepo.mu.Lock()
	defer instRepo.mu.Unlock()

	var etfs []models.EtfModel
	if err := instRepo.db.Find(&etfs).Error; err != nil {
		instRepo.logger.Errorf("Failed to get etfs IDs: %v", err)
	}
	for _, etf := range etfs {
		instRepo.instruments[etf.Figi] = NewInstrumentsWrapper(&etf, models.EtfType)
	}
	instRepo.logger.Infof("Loaded %d etfs", len(etfs))

	return nil
}

func (instRepo *InstrumentsRepository) LoadBonds() error {
	instRepo.mu.Lock()
	defer instRepo.mu.Unlock()

	var bonds []models.BondModel
	if err := instRepo.db.Find(&bonds).Error; err != nil {
		instRepo.logger.Errorf("Failed to get bonds IDs: %v", err)
	}
	for _, bond := range bonds {
		instRepo.instruments[bond.Figi] = NewInstrumentsWrapper(&bond, bond.InstrumentGroup)
	}
	instRepo.logger.Infof("Loaded %d bonds", len(bonds))

	return nil
}

func (instRepo *InstrumentsRepository) LoadAll() error {
	err := instRepo.LoadShares()
	if err != nil {
		return err
	}
	err = instRepo.LoadFutures()
	if err != nil {
		return err
	}
	err = instRepo.LoadCurrencies()
	if err != nil {
		return err
	}
	err = instRepo.LoadEtfs()
	if err != nil {
		return err
	}
	err = instRepo.LoadBonds()
	if err != nil {
		return err
	}
	return nil
}

func (instRepo *InstrumentsRepository) GetIds() []string {
	instRepo.mu.RLock()
	defer instRepo.mu.RUnlock()

	instrumentsIds := make([]string, 0, len(instRepo.instruments))

	for _, instrument := range instRepo.instruments {
		instrumentsIds = append(instrumentsIds, instrument.GetFigi())
	}

	return instrumentsIds
}

func (instRepo *InstrumentsRepository) GetIdsByType(instrumentType models.InstrumentType) []string {
	instRepo.mu.RLock()
	defer instRepo.mu.RUnlock()

	instrumentsIds := make([]string, 0, len(instRepo.instruments))

	for _, instrument := range instRepo.instruments {
		if instrument.GetType() == instrumentType {
			instrumentsIds = append(instrumentsIds, instrument.GetFigi())
		}
	}

	return instrumentsIds
}

func (instRepo *InstrumentsRepository) Find(figi string) *InstrumentWrapper {
	instRepo.mu.RLock()
	defer instRepo.mu.RUnlock()

	instrument, ok := instRepo.instruments[figi]
	if ok {
		return instrument
	}

	return nil
}
