package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	httpdelivery "github.com/hjyoon/ogame-opensource/backend/internal/delivery/http"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/filesystem"
	infraruntime "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/runtime"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           buildHandler(cfg, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting ogame go server", "addr", cfg.Addr, "env", cfg.Environment)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("ogame go server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func buildHandler(cfg config.Config, logger *slog.Logger) http.Handler {
	health := appsystem.NewHealthService(appsystem.HealthConfig{
		Environment:    cfg.Environment,
		StaticDir:      cfg.StaticDir,
		LegacyAssetDir: cfg.LegacyAssetDir,
		LegacyBaseURL:  cfg.LegacyBaseURL,
		GoTarget:       config.GoTarget,
		BunTarget:      config.BunTarget,
		ReactTarget:    config.ReactTarget,
	}, filesystem.Probe{}, infraruntime.GoRuntime{})

	return httpdelivery.New(httpdelivery.Dependencies{
		Health:       health,
		Frontend:     filesystem.StaticDir{Root: cfg.StaticDir},
		LegacyAssets: filesystem.NewNoListingFS(cfg.LegacyAssetDir),
		Logger:       logger,
	})
}

func newLogger(levelName string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(levelName) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
