package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultEnv             = "development"
	defaultHTTPHost        = "0.0.0.0"
	defaultHTTPPort        = 8080
	defaultRedisAddr       = "localhost:6379"
	defaultRedisDB         = 0
	defaultCacheTTLSeconds = 30
)

// Config keeps the runtime configuration for the service.
type Config struct {
	Env      string
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Cache    CacheConfig
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

// Load builds Config from environment variables.
func Load() (*Config, error) {
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
