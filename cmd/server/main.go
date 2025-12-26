package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	docs "main/docs"
	appinstruments "main/internal/application/service/instruments"
	appmarketdata "main/internal/application/service/marketdata"
	"main/internal/config"
	infrainstruments "main/internal/infrastructure/instruments"
	inframarketdata "main/internal/infrastructure/marketdata"
	infrahttp "main/internal/interfaces/http"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("failed to load config: %v", err)
	}

	docs.SwaggerInfo.BasePath = "/api/v1"
	docs.SwaggerInfo.Host = cfg.HTTP.Addr()

	instrumentRepo, err := infrainstruments.NewRepository(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Fatalf("failed to init instruments repo: %v", err)
	}
	defer instrumentRepo.Close()

	marketdataRepo, err := inframarketdata.NewRepository(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Fatalf("failed to init marketdata repo: %v", err)
	}
	defer marketdataRepo.Close()

	var redisClient *redis.Client
	if cfg.Redis.Addr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Fatalf("failed to connect to redis: %v", err)
		}
		defer redisClient.Close()
	}

	instrumentService := appinstruments.NewService(instrumentRepo)
	marketdataService := appmarketdata.NewService(marketdataRepo)

	cacheTTL := time.Duration(cfg.Cache.TTLSeconds) * time.Second
	handler := infrahttp.NewHandler(instrumentService, marketdataService, redisClient, cacheTTL)

	server := &http.Server{
		Addr:    cfg.HTTP.Addr(),
		Handler: handler,
	}

	go func() {
		logger.Infof("HTTP server listening on %s", cfg.HTTP.Addr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Infof("shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("server shutdown error: %v", err)
	}
	logger.Info("server stopped")
}
