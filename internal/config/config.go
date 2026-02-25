package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

const (
	defaultEnv                = "development"
	defaultHTTPHost           = "0.0.0.0"
	defaultHTTPPort           = 8080
	defaultRedisAddr          = "localhost:6379"
	defaultRedisDB            = 0
	defaultCacheTTLSeconds    = 30
	defaultRabbitURL          = "amqp://guest:guest@localhost:5672/"
	defaultTradesExchange     = "marketdata.trades"
	defaultCandlesExchange    = "marketdata.candles"
	defaultOrderBooksExchange = "marketdata.orderbooks"
	defaultRabbitPrefetch     = 500
	defaultBatchSize          = 2000
	defaultBatchTimeoutMS     = 200
)

// Config keeps the runtime configuration for the service.
type Config struct {
	Env      string
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Cache    CacheConfig
	RabbitMQ RabbitMQConfig
}

// HTTPConfig holds HTTP server related settings.
type HTTPConfig struct {
	Host string
	Port int
}

// Addr renders the listen address in host:port form.
func (h HTTPConfig) Addr() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

// PostgresConfig stores database connection parameters.
type PostgresConfig struct {
	DSN string
}

// RedisConfig stores Redis connection parameters.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// CacheConfig stores cache behavior.
type CacheConfig struct {
	TTLSeconds int
}

// RabbitMQConfig stores broker connection and batching settings.
type RabbitMQConfig struct {
	URL                string
	TradesExchange     string
	CandlesExchange    string
	OrderBooksExchange string
	Prefetch           int
	BatchSize          int
	BatchTimeout       time.Duration
}

// Load builds Config from environment variables.
// It first attempts to load a .env file if present (non-fatal if missing).
func Load() (*Config, error) {
	_ = godotenv.Load()

	host := getString("HTTP_HOST", defaultHTTPHost)
	port, err := getInt("HTTP_PORT", defaultHTTPPort)
	if err != nil {
		return nil, fmt.Errorf("parse HTTP_PORT: %w", err)
	}

	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		return nil, errors.New("DATABASE_DSN is required")
	}

	redisDB, err := getInt("REDIS_DB", defaultRedisDB)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_DB: %w", err)
	}

	cacheTTL, err := getInt("CACHE_TTL_SECONDS", defaultCacheTTLSeconds)
	if err != nil {
		return nil, fmt.Errorf("parse CACHE_TTL_SECONDS: %w", err)
	}

	prefetch, err := getInt("RABBITMQ_PREFETCH", defaultRabbitPrefetch)
	if err != nil {
		return nil, fmt.Errorf("parse RABBITMQ_PREFETCH: %w", err)
	}
	batchSize, err := getInt("RABBITMQ_BATCH_SIZE", defaultBatchSize)
	if err != nil {
		return nil, fmt.Errorf("parse RABBITMQ_BATCH_SIZE: %w", err)
	}
	timeoutMS, err := getInt("RABBITMQ_BATCH_TIMEOUT_MS", defaultBatchTimeoutMS)
	if err != nil {
		return nil, fmt.Errorf("parse RABBITMQ_BATCH_TIMEOUT_MS: %w", err)
	}

	return &Config{
		Env:  getString("APP_ENV", defaultEnv),
		HTTP: HTTPConfig{Host: host, Port: port},
		Postgres: PostgresConfig{
			DSN: dsn,
		},
		Redis: RedisConfig{
			Addr:     getString("REDIS_ADDR", defaultRedisAddr),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		},
		Cache: CacheConfig{
			TTLSeconds: cacheTTL,
		},
		RabbitMQ: RabbitMQConfig{
			URL:                getString("RABBITMQ_URL", defaultRabbitURL),
			TradesExchange:     getString("RABBITMQ_TRADES_EXCHANGE", defaultTradesExchange),
			CandlesExchange:    getString("RABBITMQ_CANDLES_EXCHANGE", defaultCandlesExchange),
			OrderBooksExchange: getString("RABBITMQ_ORDERBOOKS_EXCHANGE", defaultOrderBooksExchange),
			Prefetch:           prefetch,
			BatchSize:          batchSize,
			BatchTimeout:       time.Duration(timeoutMS) * time.Millisecond,
		},
	}, nil
}

// StreamerConfig holds parameters for the historical replay streamer.
type StreamerConfig struct {
	From           time.Time
	To             time.Time
	Trades         bool
	Candles        bool
	Orders         bool
	CandleInterval int64 // interval in seconds used when querying candles
	OrderDepth     int32 // depth used when querying order book snapshots
	Instruments    []string
}

// yamlStreamerConfig is the raw YAML representation of streamer config.
type yamlStreamerConfig struct {
	From           string   `yaml:"from"`
	To             string   `yaml:"to"`
	Trades         bool     `yaml:"trades"`
	Candles        bool     `yaml:"candles"`
	Orders         bool     `yaml:"orders"`
	CandleInterval int64    `yaml:"candle_interval"`
	OrderDepth     int32    `yaml:"order_depth"`
	Instruments    []string `yaml:"instruments"`
}

type yamlRoot struct {
	Streamer yamlStreamerConfig `yaml:"streamer"`
}

// LoadStreamerConfig reads streamer configuration from a YAML file.
// Timestamps must be in RFC3339 format, e.g. "2024-01-01T00:00:00Z".
func LoadStreamerConfig(path string) (*StreamerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var root yamlRoot
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	raw := root.Streamer

	from, err := time.Parse(time.RFC3339, raw.From)
	if err != nil {
		return nil, fmt.Errorf("parse streamer.from %q: %w", raw.From, err)
	}
	to, err := time.Parse(time.RFC3339, raw.To)
	if err != nil {
		return nil, fmt.Errorf("parse streamer.to %q: %w", raw.To, err)
	}
	if from.After(to) {
		from, to = to, from
	}

	if raw.CandleInterval <= 0 {
		raw.CandleInterval = 60
	}
	if raw.OrderDepth <= 0 {
		raw.OrderDepth = 20
	}

	return &StreamerConfig{
		From:           from,
		To:             to,
		Trades:         raw.Trades,
		Candles:        raw.Candles,
		Orders:         raw.Orders,
		CandleInterval: raw.CandleInterval,
		OrderDepth:     raw.OrderDepth,
		Instruments:    raw.Instruments,
	}, nil
}

func getString(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func getInt(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("convert %s value %q to int: %w", key, value, err)
	}
	return parsed, nil
}
