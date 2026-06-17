package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	httpdelivery "github.com/hjyoon/ogame-opensource/backend/internal/delivery/http"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/catalogrepo"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/configcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/filesystem"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlcatalog"
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
	universes := apppublicsite.NewUniverseCatalogService(universeRepository(cfg, logger))

	return httpdelivery.New(httpdelivery.Dependencies{
		Health:       health,
		Universes:    universes,
		Frontend:     filesystem.StaticDir{Root: cfg.StaticDir},
		LegacyAssets: filesystem.NewNoListingFS(cfg.LegacyAssetDir),
		Logger:       logger,
	})
}

func universeRepository(cfg config.Config, logger *slog.Logger) apppublicsite.UniverseRepository {
	fallback := configcatalog.UniverseCatalog{
		RawJSON:       cfg.PublicUniverses,
		LegacyBaseURL: cfg.LegacyBaseURL,
	}

	if strings.TrimSpace(cfg.PublicUniverses) != "" || !cfg.MasterDBEnabled {
		return fallback
	}

	db, err := mysqlcatalog.Open(mysqlcatalog.MasterDBConfig{
		Host:     cfg.MasterDBHost,
		User:     cfg.MasterDBUser,
		Password: cfg.MasterDBPassword,
		Name:     cfg.MasterDBName,
	})
	if err != nil {
		logger.Warn("master DB universe catalog disabled", "error", err)
		return fallback
	}

	logger.Info("master DB universe catalog enabled", "host", cfg.MasterDBHost, "database", cfg.MasterDBName)
	return catalogrepo.FallbackUniverseCatalog{
		Primary:  mysqlcatalog.NewMasterUniverseCatalog(db),
		Fallback: fallback,
	}
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
