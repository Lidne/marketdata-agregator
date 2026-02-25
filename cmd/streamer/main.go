package main

import (
	"context"
	"os/signal"
	"syscall"

	appstreamer "main/internal/application/service/streamer"
	"main/internal/config"
	infrainstruments "main/internal/infrastructure/instruments"
	inframarketdata "main/internal/infrastructure/marketdata"
	infrastreamer "main/internal/infrastructure/streamer"

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

	streamerCfg, err := config.LoadStreamerConfig("configs/config.yaml")
	if err != nil {
		logger.Fatalf("failed to load streamer config: %v", err)
	}

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

	publisher, err := infrastreamer.NewPublisher(cfg.RabbitMQ)
	if err != nil {
		logger.Fatalf("failed to init rabbitmq publisher: %v", err)
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			logger.Errorf("publisher close error: %v", err)
		}
	}()

	service := appstreamer.NewService(marketdataRepo, instrumentRepo, publisher, logger)

	logger.WithFields(logrus.Fields{
		"from":        streamerCfg.From,
		"to":          streamerCfg.To,
		"trades":      streamerCfg.Trades,
		"candles":     streamerCfg.Candles,
		"orders":      streamerCfg.Orders,
		"instruments": streamerCfg.Instruments,
	}).Info("starting streamer")

	if err := service.Stream(ctx, *streamerCfg); err != nil {
		logger.Fatalf("streamer error: %v", err)
	}

	logger.Info("streamer finished")
}
