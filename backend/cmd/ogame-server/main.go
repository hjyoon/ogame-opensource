package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	httpdelivery "github.com/hjyoon/ogame-opensource/backend/internal/delivery/http"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/catalogrepo"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/configcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/filesystem"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlgame"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlregistration"
	infraruntime "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/runtime"
	infrasession "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/session"
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
	registrationDrafts := registrationValidator(cfg, logger)
	registration := registrationRegistrar(cfg, logger)
	loginDrafts := loginValidator(cfg, logger)
	login := loginAuthenticator(cfg, logger)
	gameSessions := gameSessionLookup(cfg, logger)
	logout := logoutService(cfg, logger)
	gameOverview := gameOverviewService(cfg, logger, gameSessions)
	gameBuildings := gameBuildingsService(cfg, logger, gameSessions)
	gameResources := gameResourcesService(cfg, logger, gameSessions)
	gameResearch := gameResearchService(cfg, logger, gameSessions)
	gameShipyard := gameShipyardService(cfg, logger, gameSessions)
	gameFleet := gameFleetService(cfg, logger, gameSessions)
	gameGalaxy := gameGalaxyService(cfg, logger, gameSessions)
	gameDefense := gameDefenseService(cfg, logger, gameSessions)
	gameTechnology := gameTechnologyService(cfg, logger, gameSessions)
	gameStatistics := gameStatisticsService(cfg, logger, gameSessions)
	gameSearch := gameSearchService(cfg, logger, gameSessions)
	gameNotes := gameNotesService(cfg, logger, gameSessions)

	return httpdelivery.New(httpdelivery.Dependencies{
		Health:             health,
		Universes:          universes,
		RegistrationDrafts: registrationDrafts,
		Registration:       registration,
		LoginDrafts:        loginDrafts,
		Login:              login,
		GameSessions:       gameSessions,
		Logout:             logout,
		GameOverview:       gameOverview,
		GameBuildings:      gameBuildings,
		GameResources:      gameResources,
		GameResearch:       gameResearch,
		GameShipyard:       gameShipyard,
		GameFleet:          gameFleet,
		GameGalaxy:         gameGalaxy,
		GameDefense:        gameDefense,
		GameTechnology:     gameTechnology,
		GameStatistics:     gameStatistics,
		GameSearch:         gameSearch,
		GameNotes:          gameNotes,
		Frontend:           filesystem.StaticDir{Root: cfg.StaticDir},
		LegacyAssets:       filesystem.NewNoListingFS(cfg.LegacyAssetDir),
		Logger:             logger,
	})
}

func registrationRegistrar(cfg config.Config, logger *slog.Logger) apppublicsite.RegistrationRegistrar {
	if !cfg.UniDBEnabled {
		return apppublicsite.RegistrationRegistrar{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB registration creation disabled", "error", err)
		return apppublicsite.RegistrationRegistrar{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB registration creation disabled", "error", err)
		_ = db.Close()
		return apppublicsite.RegistrationRegistrar{}
	}

	logger.Info("universe DB registration creation enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return apppublicsite.NewRegistrationRegistrar(
		mysqlregistration.NewAvailabilityChecker(db, cfg.UniDBPrefix),
		mysqlregistration.NewAccountCreator(db, cfg.UniDBPrefix, cfg.UniDBSecret),
		mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix),
		infrasession.TokenGenerator{},
		cfg.UniNumber,
	)
}

func loginValidator(cfg config.Config, logger *slog.Logger) apppublicsite.LoginDraftValidator {
	if !cfg.UniDBEnabled {
		return apppublicsite.NewLoginDraftValidator()
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB login credentials disabled", "error", err)
		return apppublicsite.NewLoginDraftValidator()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB login credentials disabled", "error", err)
		_ = db.Close()
		return apppublicsite.NewLoginDraftValidator()
	}

	logger.Info("universe DB login credentials enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return apppublicsite.NewLoginDraftValidatorWithCredentials(mysqlregistration.NewCredentialChecker(db, cfg.UniDBPrefix, cfg.UniDBSecret))
}

func loginAuthenticator(cfg config.Config, logger *slog.Logger) apppublicsite.LoginAuthenticator {
	if !cfg.UniDBEnabled {
		return apppublicsite.LoginAuthenticator{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB login sessions disabled", "error", err)
		return apppublicsite.LoginAuthenticator{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB login sessions disabled", "error", err)
		_ = db.Close()
		return apppublicsite.LoginAuthenticator{}
	}

	logger.Info("universe DB login sessions enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return apppublicsite.NewLoginAuthenticator(
		mysqlregistration.NewCredentialChecker(db, cfg.UniDBPrefix, cfg.UniDBSecret),
		mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix),
		infrasession.TokenGenerator{},
		cfg.UniNumber,
	)
}

func gameSessionLookup(cfg config.Config, logger *slog.Logger) apppublicsite.GameSessionLookup {
	if !cfg.UniDBEnabled {
		return apppublicsite.GameSessionLookup{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game session lookup disabled", "error", err)
		return apppublicsite.GameSessionLookup{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game session lookup disabled", "error", err)
		_ = db.Close()
		return apppublicsite.GameSessionLookup{}
	}

	logger.Info("universe DB game session lookup enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return apppublicsite.NewGameSessionLookup(mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix), cfg.UniNumber)
}

func logoutService(cfg config.Config, logger *slog.Logger) apppublicsite.LogoutService {
	if !cfg.UniDBEnabled {
		return apppublicsite.LogoutService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB logout disabled", "error", err)
		return apppublicsite.LogoutService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB logout disabled", "error", err)
		_ = db.Close()
		return apppublicsite.LogoutService{}
	}

	logger.Info("universe DB logout enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return apppublicsite.NewLogoutService(mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix), cfg.UniNumber)
}

func gameOverviewService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.OverviewService {
	if !cfg.UniDBEnabled {
		return appgame.OverviewService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game overview disabled", "error", err)
		return appgame.OverviewService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game overview disabled", "error", err)
		_ = db.Close()
		return appgame.OverviewService{}
	}

	logger.Info("universe DB game overview enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewOverviewService(sessions, mysqlgame.NewOverviewRepositoryWithSecret(db, cfg.UniDBPrefix, cfg.UniDBSecret))
}

func gameBuildingsService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.BuildingsService {
	if !cfg.UniDBEnabled {
		return appgame.BuildingsService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game buildings disabled", "error", err)
		return appgame.BuildingsService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game buildings disabled", "error", err)
		_ = db.Close()
		return appgame.BuildingsService{}
	}

	logger.Info("universe DB game buildings enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewBuildingsService(sessions, mysqlgame.NewBuildingsRepository(db, cfg.UniDBPrefix))
}

func gameResourcesService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.ResourcesService {
	if !cfg.UniDBEnabled {
		return appgame.ResourcesService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game resources disabled", "error", err)
		return appgame.ResourcesService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game resources disabled", "error", err)
		_ = db.Close()
		return appgame.ResourcesService{}
	}

	logger.Info("universe DB game resources enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewResourcesService(sessions, mysqlgame.NewResourcesRepository(db, cfg.UniDBPrefix))
}

func gameResearchService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.ResearchService {
	if !cfg.UniDBEnabled {
		return appgame.ResearchService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game research disabled", "error", err)
		return appgame.ResearchService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game research disabled", "error", err)
		_ = db.Close()
		return appgame.ResearchService{}
	}

	logger.Info("universe DB game research enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewResearchService(sessions, mysqlgame.NewResearchRepository(db, cfg.UniDBPrefix))
}

func gameShipyardService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.ShipyardService {
	if !cfg.UniDBEnabled {
		return appgame.ShipyardService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game shipyard disabled", "error", err)
		return appgame.ShipyardService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game shipyard disabled", "error", err)
		_ = db.Close()
		return appgame.ShipyardService{}
	}

	logger.Info("universe DB game shipyard enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewShipyardService(sessions, mysqlgame.NewShipyardRepository(db, cfg.UniDBPrefix))
}

func gameDefenseService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.DefenseService {
	if !cfg.UniDBEnabled {
		return appgame.DefenseService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game defense disabled", "error", err)
		return appgame.DefenseService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game defense disabled", "error", err)
		_ = db.Close()
		return appgame.DefenseService{}
	}

	logger.Info("universe DB game defense enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewDefenseService(sessions, mysqlgame.NewDefenseRepository(db, cfg.UniDBPrefix))
}

func gameFleetService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.FleetService {
	if !cfg.UniDBEnabled {
		return appgame.FleetService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game fleet disabled", "error", err)
		return appgame.FleetService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game fleet disabled", "error", err)
		_ = db.Close()
		return appgame.FleetService{}
	}

	logger.Info("universe DB game fleet enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewFleetService(sessions, mysqlgame.NewFleetRepository(db, cfg.UniDBPrefix))
}

func gameGalaxyService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.GalaxyService {
	if !cfg.UniDBEnabled {
		return appgame.GalaxyService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game galaxy disabled", "error", err)
		return appgame.GalaxyService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game galaxy disabled", "error", err)
		_ = db.Close()
		return appgame.GalaxyService{}
	}

	logger.Info("universe DB game galaxy enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewGalaxyService(sessions, mysqlgame.NewGalaxyRepository(db, cfg.UniDBPrefix))
}

func gameTechnologyService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.TechnologyService {
	if !cfg.UniDBEnabled {
		return appgame.TechnologyService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game technology disabled", "error", err)
		return appgame.TechnologyService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game technology disabled", "error", err)
		_ = db.Close()
		return appgame.TechnologyService{}
	}

	logger.Info("universe DB game technology enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewTechnologyService(sessions, mysqlgame.NewTechnologyRepository(db, cfg.UniDBPrefix))
}

func gameStatisticsService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.StatisticsService {
	if !cfg.UniDBEnabled {
		return appgame.StatisticsService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game statistics disabled", "error", err)
		return appgame.StatisticsService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game statistics disabled", "error", err)
		_ = db.Close()
		return appgame.StatisticsService{}
	}

	logger.Info("universe DB game statistics enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewStatisticsService(sessions, mysqlgame.NewStatisticsRepository(db, cfg.UniDBPrefix))
}

func gameSearchService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.SearchService {
	if !cfg.UniDBEnabled {
		return appgame.SearchService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game search disabled", "error", err)
		return appgame.SearchService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game search disabled", "error", err)
		_ = db.Close()
		return appgame.SearchService{}
	}

	logger.Info("universe DB game search enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewSearchService(sessions, mysqlgame.NewSearchRepository(db, cfg.UniDBPrefix))
}

func gameNotesService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.NotesService {
	if !cfg.UniDBEnabled {
		return appgame.NotesService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game notes disabled", "error", err)
		return appgame.NotesService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game notes disabled", "error", err)
		_ = db.Close()
		return appgame.NotesService{}
	}

	logger.Info("universe DB game notes enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewNotesService(sessions, mysqlgame.NewNotesRepository(db, cfg.UniDBPrefix))
}

func registrationValidator(cfg config.Config, logger *slog.Logger) apppublicsite.RegistrationDraftValidator {
	if !cfg.UniDBEnabled {
		return apppublicsite.NewRegistrationDraftValidator()
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB registration availability disabled", "error", err)
		return apppublicsite.NewRegistrationDraftValidator()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB registration availability disabled", "error", err)
		_ = db.Close()
		return apppublicsite.NewRegistrationDraftValidator()
	}

	logger.Info("universe DB registration availability enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return apppublicsite.NewRegistrationDraftValidatorWithAvailability(mysqlregistration.NewAvailabilityChecker(db, cfg.UniDBPrefix))
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
