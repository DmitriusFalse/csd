//go:generate windres -i bridge.rc -o bridge.syso -O coff
// Build with: go build -ldflags="-H=windowsgui" -o csd-bridge.exe .\cmd\bridge\
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/DmitriusFalse/csd/internal/api"
	"github.com/DmitriusFalse/csd/internal/config"
	"github.com/DmitriusFalse/csd/internal/crypto"
	"github.com/DmitriusFalse/csd/internal/downloader"
	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/server"
	"github.com/DmitriusFalse/csd/internal/tray"
	"github.com/energye/systray"
	"go.uber.org/zap"
)

const appVersion = "8.1.0"

func main() {
	headless := os.Getenv("CSD_HEADLESS") == "1" || os.Getenv("CSD_HEADLESS") == "true"

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
		zap.Bool("headless", headless),
	)

	if cfg.APIKey != "" {
		if len(cfg.APIKey) > 20 && cfg.APIKey[:7] == "aes256:" {
			decrypted, err := crypto.Decrypt(cfg.APIKey[7:])
			if err != nil {
				logger.Log.Warn("Failed to decrypt API key, may need re-entry", zap.Error(err))
			} else {
				cfg.APIKey = decrypted
			}
		}
	}

	civitaiClient := api.NewClient()
	manager := downloader.NewManager(cfg, civitaiClient)

	srv := server.New(cfg.Server.Host, cfg.Server.Port, manager, cfgPath, cfg.Logging.File, civitaiClient)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Server runs in background goroutine
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		logger.Log.Info("Server starting", zap.String("addr", addr))
		if err := srv.Start(); err != nil {
			logger.Log.Error("Server error", zap.Error(err))
		}
	}()

	if headless {
		logger.Log.Info("Running in headless mode (no system tray)")
		<-ctx.Done()
		logger.Log.Info("Shutting down by signal...")
	} else {
		var shutdownOnce sync.Once
		shutdownCh := make(chan struct{})

		// Signal handler in background goroutine
		go func() {
			<-ctx.Done()
			logger.Log.Info("Shutting down by signal...")
			shutdownOnce.Do(func() { close(shutdownCh) })
			systray.Quit()
		}()

		// Tray MUST run on main goroutine for Windows message pump
		tray.Run(manager, cfg.RootPath, appVersion, func() {
			shutdownOnce.Do(func() { close(shutdownCh) })
		})

		<-shutdownCh
		logger.Log.Info("Shutting down by user request...")
	}

	manager.Shutdown()
	_ = srv.Shutdown()
	logger.Log.Info("Shutdown complete")
}
