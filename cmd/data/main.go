package main

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	investgo "github.com/russianinvestments/invest-api-go-sdk/investgo"
	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
	"github.com/sirupsen/logrus"

	domain "main/internal/domain/entity/instruments"
)

const (
	defaultInvestEndpoint = "invest-public-api.tinkoff.ru:443"
	defaultAppName        = "marketdata-data-loader"
)

type dataConfig struct {
	Token         string
	Endpoint      string
	AppName       string
	SkipTLSVerify bool
	DatabaseDSN   string
}

func main() {
	godotenv.Load(".env")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := loadConfig()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		logger.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	investCfg := investgo.Config{
		EndPoint:           cfg.Endpoint,
		Token:              cfg.Token,
		AppName:            cfg.AppName,
		InsecureSkipVerify: cfg.SkipTLSVerify,
	}

	client, err := investgo.NewClient(ctx, investCfg, logger)
	if err != nil {
		logger.Fatalf("create invest api client: %v", err)
	}
	defer func() {
		if stopErr := client.Stop(); stopErr != nil {
			logger.Errorf("stop invest api client: %v", stopErr)
		}
	}()

	instrumentClient := client.NewInstrumentsServiceClient()

	countries, err := fetchCountries(instrumentClient)
	if err != nil {
		logger.Fatalf("fetch countries: %v", err)
	}
	if err := upsertCountries(ctx, pool, countries); err != nil {
		logger.Fatalf("save countries: %v", err)
	}
	logger.WithField("countries", len(countries)).Info("countries synced")

	brands, err := fetchBrands(instrumentClient)
	if err != nil {
		logger.Fatalf("fetch brands: %v", err)
	}
	brandEntities, companies, sectors := prepareBrandData(brands, countries, logger)

	if err := upsertCompanies(ctx, pool, companies); err != nil {
		logger.Fatalf("save companies: %v", err)
	}
	logger.WithField("companies", len(companies)).Info("companies synced")

	if err := upsertSectors(ctx, pool, sectors); err != nil {
		logger.Fatalf("save sectors: %v", err)
	}
	logger.WithField("sectors", len(sectors)).Info("sectors synced")

	if err := upsertBrands(ctx, pool, brandEntities); err != nil {
		logger.Fatalf("save brands: %v", err)
	}
	logger.WithField("brands", len(brandEntities)).Info("brands synced")

	shares, err := fetchShares(instrumentClient)
	if err != nil {
		logger.Fatalf("fetch shares: %v", err)
	}
	bonds, err := fetchBonds(instrumentClient)
	if err != nil {
		logger.Fatalf("fetch bonds: %v", err)
	}
	futures, err := fetchFutures(instrumentClient)
	if err != nil {
		logger.Fatalf("fetch futures: %v", err)
	}
	etfs, err := fetchEtfs(instrumentClient)
	if err != nil {
		logger.Fatalf("fetch etfs: %v", err)
	}
	currencies, err := fetchCurrencies(instrumentClient)
	if err != nil {
		logger.Fatalf("fetch currencies: %v", err)
	}

	shareRows := prepareShareInstruments(shares, logger)
	bondRows := prepareBondInstruments(bonds, logger)
	futureRows := prepareFutureInstruments(futures, logger)
	etfRows := prepareEtfInstruments(etfs, logger)
	currencyRows := prepareCurrencyInstruments(currencies, logger)

	if err := upsertInstruments(ctx, pool, shareRows, "shares"); err != nil {
		logger.Fatalf("save shares: %v", err)
	}
	logger.WithField("shares", len(shareRows)).Info("shares synced")

	if err := upsertInstruments(ctx, pool, bondRows, "bonds"); err != nil {
		logger.Fatalf("save bonds: %v", err)
	}
	logger.WithField("bonds", len(bondRows)).Info("bonds synced")

	if err := upsertInstruments(ctx, pool, futureRows, "futures"); err != nil {
		logger.Fatalf("save futures: %v", err)
	}
	logger.WithField("futures", len(futureRows)).Info("futures synced")

	if err := upsertInstruments(ctx, pool, etfRows, "etfs"); err != nil {
		logger.Fatalf("save etfs: %v", err)
	}
	logger.WithField("etfs", len(etfRows)).Info("etfs synced")

	if err := upsertInstruments(ctx, pool, currencyRows, "currencies"); err != nil {
		logger.Fatalf("save currencies: %v", err)
	}
	logger.WithField("currencies", len(currencyRows)).Info("currencies synced")

	logger.Info("reference data sync finished")
}

func loadConfig() (*dataConfig, error) {
	token := strings.TrimSpace(os.Getenv("INVEST_TOKEN"))
	if token == "" {
		return nil, errors.New("INVEST_TOKEN is required")
	}

	dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	if dsn == "" {
		return nil, errors.New("DATABASE_DSN is required")
	}

	return &dataConfig{
		Token:         token,
		Endpoint:      envOrDefault("INVEST_ENDPOINT", defaultInvestEndpoint),
		AppName:       envOrDefault("INVEST_APP_NAME", defaultAppName),
		SkipTLSVerify: boolEnv("INVEST_INSECURE_SKIP_VERIFY", true),
		DatabaseDSN:   dsn,
	}, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "t", "true", "yes", "y":
		return true
	case "0", "f", "false", "no", "n":
		return false
	default:
		return fallback
	}
}

func fetchCountries(client *investgo.InstrumentsServiceClient) (map[string]*domain.Country, error) {
	resp, err := client.GetCountries()
	if err != nil {
		return nil, fmt.Errorf("get countries: %w", err)
	}

	result := make(map[string]*domain.Country, len(resp.GetCountries()))
	for _, item := range resp.GetCountries() {
		if item == nil {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(item.GetAlfaTwo()))
		alfaThree := strings.ToUpper(strings.TrimSpace(item.GetAlfaThree()))
		if len(alfaThree) != 3 {
			continue
		}
		name := strings.TrimSpace(item.GetName())
		if name == "" {
			name = code
		}
		result[code] = &domain.Country{
			AlfaTwo:   code,
			AlfaThree: alfaThree,
			Name:      name,
			NameBrief: strings.TrimSpace(item.GetNameBrief()),
		}
	}
	return result, nil
}

func fetchBrands(client *investgo.InstrumentsServiceClient) ([]*pb.Brand, error) {
	resp, err := client.GetBrands()
	if err != nil {
		return nil, fmt.Errorf("get brands: %w", err)
	}
	return resp.GetBrands(), nil
}

func prepareBrandData(brands []*pb.Brand, countries map[string]*domain.Country, logger *logrus.Logger) ([]*domain.Brand, map[string]domain.Company, map[string]*domain.Sector) {
	brandEntities := make([]*domain.Brand, 0, len(brands))
	companies := make(map[string]domain.Company)
	sectors := make(map[string]*domain.Sector)

	for _, brand := range brands {
		if brand == nil {
			continue
		}
		countryOfRisk := brand.GetCountryOfRisk()
		if countryOfRisk == "" {
			continue
		}
		countryCode := strings.ToUpper(strings.TrimSpace(countryOfRisk[0:2]))
		if _, ok := countries[countryCode]; !ok {
			logger.WithFields(logrus.Fields{
				"brand_uid": brand.GetUid(),
				"country":   countryCode,
			}).Warn("skip brand with unknown country")
			continue
		}

		name := strings.TrimSpace(brand.GetName())
		if name == "" {
			logger.WithField("brand_uid", brand.GetUid()).Warn("skip brand without name")
			continue
		}

		companyName := strings.TrimSpace(brand.GetCompany())
		if companyName == "" {
			companyName = name
		}
		companyKey := strings.ToLower(companyName)
		if _, ok := companies[companyKey]; !ok {
			companies[companyKey] = domain.Company{
				UID:  stableUUID(uuid.NameSpaceDNS, "company:"+companyKey),
				Name: companyName,
			}
		}

		sectorName := strings.TrimSpace(brand.GetSector())
		if sectorName == "" {
			sectorName = "Other"
		}
		sectorKey := strings.ToLower(sectorName)
		if _, ok := sectors[sectorKey]; !ok {
			sectors[sectorKey] = &domain.Sector{
				UID:        stableUUID(uuid.NameSpaceOID, "sector:"+sectorKey),
				Name:       sectorName,
				Volatility: pseudoVolatility(sectorName),
			}
		}

		brandEntities = append(brandEntities, &domain.Brand{
			UID:         parseBrandUID(brand.GetUid(), name),
			Name:        name,
			Description: strings.TrimSpace(brand.GetDescription()),
			Info:        strings.TrimSpace(brand.GetInfo()),
			CompanyUID:  companies[companyKey].UID,
			SectorUID:   sectors[sectorKey].UID,
			CountryCode: countryCode,
		})
	}

	return brandEntities, companies, sectors
}

func upsertCountries(ctx context.Context, pool *pgxpool.Pool, countries map[string]*domain.Country) error {
	batch := &pgx.Batch{}
	for _, country := range countries {
		batch.Queue(`
			INSERT INTO countries (alfa_two, alfa_three, name, name_brief)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (alfa_two) DO UPDATE
			SET alfa_three = EXCLUDED.alfa_three,
			    name = EXCLUDED.name,
			    name_brief = EXCLUDED.name_brief`,
			country.AlfaTwo,
			country.AlfaThree,
			country.Name,
			country.NameBrief,
		)
	}
	return execBatch(ctx, pool, batch)
}

func upsertCompanies(ctx context.Context, pool *pgxpool.Pool, companies map[string]domain.Company) error {
	batch := &pgx.Batch{}
	for _, company := range companies {
		batch.Queue(`
			INSERT INTO companies (uid, name)
			VALUES ($1, $2)
			ON CONFLICT (uid) DO UPDATE
			SET name = EXCLUDED.name`,
			company.UID,
			company.Name,
		)
	}
	return execBatch(ctx, pool, batch)
}

func upsertSectors(ctx context.Context, pool *pgxpool.Pool, sectors map[string]*domain.Sector) error {
	batch := &pgx.Batch{}
	for _, sector := range sectors {
		batch.Queue(`
			INSERT INTO sectors (uid, name, volatility)
			VALUES ($1, $2, $3)
			ON CONFLICT (uid) DO UPDATE
			SET name = EXCLUDED.name,
			    volatility = EXCLUDED.volatility`,
			sector.UID,
			sector.Name,
			sector.Volatility,
		)
	}
	return execBatch(ctx, pool, batch)
}

func upsertBrands(ctx context.Context, pool *pgxpool.Pool, brands []*domain.Brand) error {
	batch := &pgx.Batch{}
	for _, brand := range brands {
		batch.Queue(`
			INSERT INTO brands (uid, name, description, info, company_uid, sector_uid, country_code)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (uid) DO UPDATE
			SET name = EXCLUDED.name,
			    description = EXCLUDED.description,
			    info = EXCLUDED.info,
			    company_uid = EXCLUDED.company_uid,
			    sector_uid = EXCLUDED.sector_uid,
			    country_code = EXCLUDED.country_code`,
			brand.UID,
			brand.Name,
			brand.Description,
			brand.Info,
			brand.CompanyUID,
			brand.SectorUID,
			brand.CountryCode,
		)
	}
	return execBatch(ctx, pool, batch)
}

func execBatch(ctx context.Context, pool *pgxpool.Pool, batch *pgx.Batch) error {
	if batch.Len() == 0 {
		return nil
	}
	results := pool.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return err
		}
	}
	return results.Close()
}

func stableUUID(namespace uuid.UUID, value string) uuid.UUID {
	if value == "" {
		return uuid.New()
	}
	return uuid.NewSHA1(namespace, []byte(value))
}

func parseBrandUID(rawID, fallback string) uuid.UUID {
	if id, err := uuid.Parse(strings.TrimSpace(rawID)); err == nil {
		return id
	}
	key := strings.TrimSpace(rawID)
	if key == "" {
		key = strings.TrimSpace(fallback)
	}
	if key == "" {
		return uuid.New()
	}
	return stableUUID(uuid.NameSpaceURL, "brand:"+strings.ToLower(key))
}

func pseudoVolatility(name string) int32 {
	sum := crc32.ChecksumIEEE([]byte(strings.ToLower(strings.TrimSpace(name))))
	return int32(sum % 100)
}

type instrumentRow struct {
	UID       uuid.UUID
	Figi      string
	Ticker    string
	Lot       int32
	ClassCode string
	LogoURL   string
	BrandUID  *uuid.UUID
}

type bondInstrumentRow struct {
	instrumentRow
	Nominal  *float64
	AciValue *float64
}

type futureInstrumentRow struct {
	instrumentRow
	MinPriceIncrement       *float64
	MinPriceIncrementAmount *float64
	AssetType               string
}

type etfInstrumentRow struct {
	instrumentRow
	MinPriceIncrement *float64
}

func fetchShares(client *investgo.InstrumentsServiceClient) ([]*pb.Share, error) {
	resp, err := client.Shares(pb.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("get shares: %w", err)
	}
	return resp.GetInstruments(), nil
}

func fetchBonds(client *investgo.InstrumentsServiceClient) ([]*pb.Bond, error) {
	resp, err := client.Bonds(pb.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("get bonds: %w", err)
	}
	return resp.GetInstruments(), nil
}

func fetchFutures(client *investgo.InstrumentsServiceClient) ([]*pb.Future, error) {
	resp, err := client.Futures(pb.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("get futures: %w", err)
	}
	return resp.GetInstruments(), nil
}

func fetchEtfs(client *investgo.InstrumentsServiceClient) ([]*pb.Etf, error) {
	resp, err := client.Etfs(pb.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("get etfs: %w", err)
	}
	return resp.GetInstruments(), nil
}

func fetchCurrencies(client *investgo.InstrumentsServiceClient) ([]*pb.Currency, error) {
	resp, err := client.Currencies(pb.InstrumentStatus_INSTRUMENT_STATUS_BASE)
	if err != nil {
		return nil, fmt.Errorf("get currencies: %w", err)
	}
	return resp.GetInstruments(), nil
}

func parseInstrumentUID(raw string, figi string) uuid.UUID {
	if id, err := uuid.Parse(strings.TrimSpace(raw)); err == nil && id != uuid.Nil {
		return id
	}
	return stableUUID(uuid.NameSpaceURL, "instrument:"+strings.ToLower(strings.TrimSpace(figi)))
}

func parseQuotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.GetUnits()) + float64(q.GetNano())/1e9
}

func prepareShareInstruments(shares []*pb.Share, _ *logrus.Logger) []instrumentRow {
	rows := make([]instrumentRow, 0, len(shares))
	for _, s := range shares {
		if s == nil {
			continue
		}
		figi := strings.TrimSpace(s.GetFigi())
		ticker := strings.TrimSpace(s.GetTicker())
		if figi == "" || ticker == "" {
			continue
		}
		rows = append(rows, instrumentRow{
			UID:       parseInstrumentUID(s.GetUid(), figi),
			Figi:      figi,
			Ticker:    ticker,
			Lot:       s.GetLot(),
			ClassCode: strings.TrimSpace(s.GetClassCode()),
			LogoURL:   "",
			BrandUID:  nil,
		})
	}
	return rows
}

func prepareBondInstruments(bonds []*pb.Bond, _ *logrus.Logger) []bondInstrumentRow {
	rows := make([]bondInstrumentRow, 0, len(bonds))
	for _, b := range bonds {
		if b == nil {
			continue
		}
		figi := strings.TrimSpace(b.GetFigi())
		ticker := strings.TrimSpace(b.GetTicker())
		if figi == "" || ticker == "" {
			continue
		}
		var nominal, aciValue *float64
		if n := b.GetNominal(); n != nil {
			v := float64(n.GetUnits()) + float64(n.GetNano())/1e9
			nominal = &v
		}
		if a := b.GetAciValue(); a != nil {
			v := float64(a.GetUnits()) + float64(a.GetNano())/1e9
			aciValue = &v
		}
		rows = append(rows, bondInstrumentRow{
			instrumentRow: instrumentRow{
				UID:       parseInstrumentUID(b.GetUid(), figi),
				Figi:      figi,
				Ticker:    ticker,
				Lot:       b.GetLot(),
				ClassCode: strings.TrimSpace(b.GetClassCode()),
				LogoURL:   "",
				BrandUID:  nil,
			},
			Nominal:  nominal,
			AciValue: aciValue,
		})
	}
	return rows
}

func prepareFutureInstruments(futures []*pb.Future, _ *logrus.Logger) []futureInstrumentRow {
	rows := make([]futureInstrumentRow, 0, len(futures))
	for _, f := range futures {
		if f == nil {
			continue
		}
		figi := strings.TrimSpace(f.GetFigi())
		ticker := strings.TrimSpace(f.GetTicker())
		if figi == "" || ticker == "" {
			continue
		}
		var minPriceInc, minPriceIncAmount *float64
		if q := f.GetMinPriceIncrement(); q != nil {
			v := parseQuotationToFloat(q)
			minPriceInc = &v
		}
		if q := f.GetBasicAssetSize(); q != nil {
			v := parseQuotationToFloat(q)
			minPriceIncAmount = &v
		}
		assetType := strings.TrimSpace(f.GetAssetType())
		if assetType == "" {
			assetType = "TYPE_SECURITY"
		}
		rows = append(rows, futureInstrumentRow{
			instrumentRow: instrumentRow{
				UID:       parseInstrumentUID(f.GetUid(), figi),
				Figi:      figi,
				Ticker:    ticker,
				Lot:       f.GetLot(),
				ClassCode: strings.TrimSpace(f.GetClassCode()),
				LogoURL:   "",
				BrandUID:  nil,
			},
			MinPriceIncrement:       minPriceInc,
			MinPriceIncrementAmount: minPriceIncAmount,
			AssetType:               assetType,
		})
	}
	return rows
}

func prepareEtfInstruments(etfs []*pb.Etf, _ *logrus.Logger) []etfInstrumentRow {
	rows := make([]etfInstrumentRow, 0, len(etfs))
	for _, e := range etfs {
		if e == nil {
			continue
		}
		figi := strings.TrimSpace(e.GetFigi())
		ticker := strings.TrimSpace(e.GetTicker())
		if figi == "" || ticker == "" {
			continue
		}
		var minPriceInc *float64
		if q := e.GetMinPriceIncrement(); q != nil {
			v := parseQuotationToFloat(q)
			minPriceInc = &v
		}
		rows = append(rows, etfInstrumentRow{
			instrumentRow: instrumentRow{
				UID:       parseInstrumentUID(e.GetUid(), figi),
				Figi:      figi,
				Ticker:    ticker,
				Lot:       e.GetLot(),
				ClassCode: strings.TrimSpace(e.GetClassCode()),
				LogoURL:   "",
				BrandUID:  nil,
			},
			MinPriceIncrement: minPriceInc,
		})
	}
	return rows
}

func prepareCurrencyInstruments(currencies []*pb.Currency, _ *logrus.Logger) []instrumentRow {
	rows := make([]instrumentRow, 0, len(currencies))
	for _, c := range currencies {
		if c == nil {
			continue
		}
		figi := strings.TrimSpace(c.GetFigi())
		ticker := strings.TrimSpace(c.GetTicker())
		if figi == "" || ticker == "" {
			continue
		}
		rows = append(rows, instrumentRow{
			UID:       parseInstrumentUID(c.GetUid(), figi),
			Figi:      figi,
			Ticker:    ticker,
			Lot:       c.GetLot(),
			ClassCode: strings.TrimSpace(c.GetClassCode()),
			LogoURL:   "",
			BrandUID:  nil,
		})
	}
	return rows
}

func upsertInstruments(ctx context.Context, pool *pgxpool.Pool, rows interface{}, typedTable string) error {
	switch r := rows.(type) {
	case []instrumentRow:
		return upsertInstrumentRows(ctx, pool, r, nil, typedTable)
	case []bondInstrumentRow:
		return upsertBondInstrumentRows(ctx, pool, r)
	case []futureInstrumentRow:
		return upsertFutureInstrumentRows(ctx, pool, r)
	case []etfInstrumentRow:
		return upsertEtfInstrumentRows(ctx, pool, r)
	default:
		return fmt.Errorf("unsupported instrument type: %T", rows)
	}
}

func upsertInstrumentRows(ctx context.Context, pool *pgxpool.Pool, rows []instrumentRow, extras interface{}, typedTable string) error {
	batch := &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`
			INSERT INTO instruments (uid, figi, ticker, lot, class_code, logo_url, brand_uid)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (figi) DO UPDATE
			SET ticker = EXCLUDED.ticker,
			    lot = EXCLUDED.lot,
			    class_code = EXCLUDED.class_code,
			    logo_url = EXCLUDED.logo_url,
			    brand_uid = EXCLUDED.brand_uid,
			    updated_at = NOW()`,
			row.UID,
			row.Figi,
			row.Ticker,
			row.Lot,
			nullIfEmpty(row.ClassCode),
			nullIfEmpty(row.LogoURL),
			row.BrandUID,
		)
	}
	if err := execBatch(ctx, pool, batch); err != nil {
		return err
	}
	batch = &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`INSERT INTO `+typedTable+` (uid) VALUES ($1) ON CONFLICT (uid) DO NOTHING`, row.UID)
	}
	return execBatch(ctx, pool, batch)
}

func upsertBondInstrumentRows(ctx context.Context, pool *pgxpool.Pool, rows []bondInstrumentRow) error {
	batch := &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`
			INSERT INTO instruments (uid, figi, ticker, lot, class_code, logo_url, brand_uid)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (figi) DO UPDATE
			SET ticker = EXCLUDED.ticker,
			    lot = EXCLUDED.lot,
			    class_code = EXCLUDED.class_code,
			    logo_url = EXCLUDED.logo_url,
			    brand_uid = EXCLUDED.brand_uid,
			    updated_at = NOW()`,
			row.UID,
			row.Figi,
			row.Ticker,
			row.Lot,
			nullIfEmpty(row.ClassCode),
			nullIfEmpty(row.LogoURL),
			row.BrandUID,
		)
	}
	if err := execBatch(ctx, pool, batch); err != nil {
		return err
	}
	batch = &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`
			INSERT INTO bonds (uid, nominal, aci_value)
			VALUES ($1, $2, $3)
			ON CONFLICT (uid) DO UPDATE
			SET nominal = EXCLUDED.nominal,
			    aci_value = EXCLUDED.aci_value`,
			row.UID,
			row.Nominal,
			row.AciValue,
		)
	}
	return execBatch(ctx, pool, batch)
}

func upsertFutureInstrumentRows(ctx context.Context, pool *pgxpool.Pool, rows []futureInstrumentRow) error {
	batch := &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`
			INSERT INTO instruments (uid, figi, ticker, lot, class_code, logo_url, brand_uid)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (figi) DO UPDATE
			SET ticker = EXCLUDED.ticker,
			    lot = EXCLUDED.lot,
			    class_code = EXCLUDED.class_code,
			    logo_url = EXCLUDED.logo_url,
			    brand_uid = EXCLUDED.brand_uid,
			    updated_at = NOW()`,
			row.UID,
			row.Figi,
			row.Ticker,
			row.Lot,
			nullIfEmpty(row.ClassCode),
			nullIfEmpty(row.LogoURL),
			row.BrandUID,
		)
	}
	if err := execBatch(ctx, pool, batch); err != nil {
		return err
	}
	batch = &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`
			INSERT INTO futures (uid, min_price_increment, min_price_increment_amount, asset_type)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (uid) DO UPDATE
			SET min_price_increment = EXCLUDED.min_price_increment,
			    min_price_increment_amount = EXCLUDED.min_price_increment_amount,
			    asset_type = EXCLUDED.asset_type`,
			row.UID,
			row.MinPriceIncrement,
			row.MinPriceIncrementAmount,
			row.AssetType,
		)
	}
	return execBatch(ctx, pool, batch)
}

func upsertEtfInstrumentRows(ctx context.Context, pool *pgxpool.Pool, rows []etfInstrumentRow) error {
	batch := &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`
			INSERT INTO instruments (uid, figi, ticker, lot, class_code, logo_url, brand_uid)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (figi) DO UPDATE
			SET ticker = EXCLUDED.ticker,
			    lot = EXCLUDED.lot,
			    class_code = EXCLUDED.class_code,
			    logo_url = EXCLUDED.logo_url,
			    brand_uid = EXCLUDED.brand_uid,
			    updated_at = NOW()`,
			row.UID,
			row.Figi,
			row.Ticker,
			row.Lot,
			nullIfEmpty(row.ClassCode),
			nullIfEmpty(row.LogoURL),
			row.BrandUID,
		)
	}
	if err := execBatch(ctx, pool, batch); err != nil {
		return err
	}
	batch = &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`
			INSERT INTO etfs (uid, min_price_increment)
			VALUES ($1, $2)
			ON CONFLICT (uid) DO UPDATE
			SET min_price_increment = EXCLUDED.min_price_increment`,
			row.UID,
			row.MinPriceIncrement,
		)
	}
	return execBatch(ctx, pool, batch)
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
