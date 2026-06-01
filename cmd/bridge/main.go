package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/DmitriusFalse/csd/internal/api"
	"github.com/DmitriusFalse/csd/internal/config"
	"github.com/DmitriusFalse/csd/internal/crypto"
	"github.com/DmitriusFalse/csd/internal/downloader"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/server"
	"github.com/DmitriusFalse/csd/internal/tray"
	"go.uber.org/zap"
)

const appVersion = "0.1.0"

func main() {
	cfgPath := "config.yaml"
	if envPath := os.Getenv("CSD_CONFIG"); envPath != "" {
		cfgPath = envPath
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	if err := logger.Init(cfg.Logging.Level, cfg.Logging.File, cfg.Logging.MaxSizeMB, cfg.Logging.MaxBackups, cfg.Logging.Compress); err != nil {
		panic("Failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Log.Info("Starting Civitai Smart Downloader",
		zap.String("version", appVersion),
		zap.Int("port", cfg.Server.Port),
	)

	if cfg.APIKey != "" {
		decrypted, err := crypto.Decrypt(cfg.APIKey)
		if err != nil {
			logger.Log.Warn("Failed to decrypt API key, may need re-entry", zap.Error(err))
		} else {
			cfg.APIKey = decrypted
		}
	}

	civitaiClient := api.NewClient()
	manager := downloader.NewManager(cfg, civitaiClient)

	srv := server.New(cfg.Server.Host, cfg.Server.Port, manager)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownCh := make(chan struct{})

	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		logger.Log.Info("Server starting", zap.String("addr", addr))
		if err := srv.Start(); err != nil {
			logger.Log.Error("Server error", zap.Error(err))
		}
	}()

	go tray.Run(manager, cfg.RootPath, func() {
		close(shutdownCh)
	})

	select {
	case <-ctx.Done():
		logger.Log.Info("Shutting down by signal...")
	case <-shutdownCh:
		logger.Log.Info("Shutting down by user request...")
	}

	manager.Shutdown()
	_ = srv.Shutdown()
	logger.Log.Info("Shutdown complete")
}


