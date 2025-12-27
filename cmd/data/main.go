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
	investgo "github.com/russianinvestments/invest-api-go-sdk/investgo"
	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
	"github.com/sirupsen/logrus"

	domain "main/internal/domain/entity/instruments"
)

const (
	defaultInvestEndpoint = "https://invest-public-api.tinkoff.ru:443"
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
		if len(code) != 2 {
			continue
		}
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
		countryCode := strings.ToUpper(strings.TrimSpace(brand.GetCountryOfRisk()))
		if len(countryCode) != 2 {
			logger.WithField("brand_uid", brand.GetUid()).Warn("skip brand without country code")
			continue
		}
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
