package main

import (
	"context"
	"database/sql"
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
	infrahttpclient "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/httpclient"
	inframail "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mail"
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
	activation := registrationActivation(cfg, logger)
	directEntry := apppublicsite.NewDirectEntryService(infrahttpclient.NewExternalImageFetcher())
	passwordRecovery := passwordRecoveryService(cfg, logger)
	loginDrafts := loginValidator(cfg, logger)
	login := loginAuthenticator(cfg, logger)
	gameSessions := gameSessionLookup(cfg, logger)
	logout := logoutService(cfg, logger)
	gameOverview := gameOverviewService(cfg, logger, gameSessions)
	gameBuildings := gameBuildingsService(cfg, logger, gameSessions)
	gameEmpire := gameEmpireService(cfg, logger, gameSessions)
	gameResources := gameResourcesService(cfg, logger, gameSessions)
	gameMerchant := gameMerchantService(cfg, logger, gameSessions)
	gameOfficers := gameOfficersService(cfg, logger, gameSessions)
	gameAlliance := gameAllianceService(cfg, logger, gameSessions)
	gameAdmin := gameAdminService(cfg, logger, gameSessions)
	gameResearch := gameResearchService(cfg, logger, gameSessions)
	gameShipyard := gameShipyardService(cfg, logger, gameSessions)
	gameFleet := gameFleetService(cfg, logger, gameSessions)
	gameGalaxy := gameGalaxyService(cfg, logger, gameSessions)
	gameDefense := gameDefenseService(cfg, logger, gameSessions)
	gameTechnology := gameTechnologyService(cfg, logger, gameSessions)
	gameStatistics := gameStatisticsService(cfg, logger, gameSessions)
	gameSearch := gameSearchService(cfg, logger, gameSessions)
	gameBuddy := gameBuddyService(cfg, logger, gameSessions)
	gameNotes := gameNotesService(cfg, logger, gameSessions)
	gameMessages := gameMessagesService(cfg, logger, gameSessions)
	gameReport := gameReportService(cfg, logger, gameSessions)
	gamePhalanx := gamePhalanxService(cfg, logger, gameSessions)
	gameFeed := gameFeedService(cfg, logger)
	gameOptions := gameOptionsService(cfg, logger, gameSessions)
	gamePayment := gamePaymentService(cfg, logger, gameSessions)

	return httpdelivery.New(httpdelivery.Dependencies{
		Health:             health,
		Universes:          universes,
		RegistrationDrafts: registrationDrafts,
		Registration:       registration,
		Activation:         activation,
		DirectEntry:        directEntry,
		PasswordRecovery:   passwordRecovery,
		LoginDrafts:        loginDrafts,
		Login:              login,
		GameSessions:       gameSessions,
		Logout:             logout,
		GameOverview:       gameOverview,
		GameBuildings:      gameBuildings,
		GameEmpire:         gameEmpire,
		GameResources:      gameResources,
		GameMerchant:       gameMerchant,
		GameOfficers:       gameOfficers,
		GameAlliance:       gameAlliance,
		GameAdmin:          gameAdmin,
		GameResearch:       gameResearch,
		GameShipyard:       gameShipyard,
		GameFleet:          gameFleet,
		GameGalaxy:         gameGalaxy,
		GameDefense:        gameDefense,
		GameTechnology:     gameTechnology,
		GameStatistics:     gameStatistics,
		GameSearch:         gameSearch,
		GameBuddy:          gameBuddy,
		GameNotes:          gameNotes,
		GameMessages:       gameMessages,
		GameReport:         gameReport,
		GamePhalanx:        gamePhalanx,
		GameFeed:           gameFeed,
		GameOptions:        gameOptions,
		GamePayment:        gamePayment,
		Frontend:           filesystem.StaticDir{Root: cfg.StaticDir},
		LegacyAssets:       filesystem.NewNoListingFS(cfg.LegacyAssetDir),
		Logger:             logger,
	})
}

func registrationActivation(cfg config.Config, logger *slog.Logger) apppublicsite.RegistrationActivationService {
	if !cfg.UniDBEnabled {
		return apppublicsite.RegistrationActivationService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB registration activation disabled", "error", err)
		return apppublicsite.RegistrationActivationService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB registration activation disabled", "error", err)
		_ = db.Close()
		return apppublicsite.RegistrationActivationService{}
	}

	logger.Info("universe DB registration activation enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return apppublicsite.NewRegistrationActivationService(
		mysqlregistration.NewAccountActivator(db, cfg.UniDBPrefix),
		mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix),
		infrasession.TokenGenerator{},
		cfg.UniNumber,
	)
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
	return apppublicsite.NewRegistrationRegistrarWithMailer(
		mysqlregistration.NewAvailabilityChecker(db, cfg.UniDBPrefix),
		mysqlregistration.NewAccountCreator(db, cfg.UniDBPrefix, cfg.UniDBSecret),
		mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix),
		infrasession.TokenGenerator{},
		cfg.UniNumber,
		registrationWelcomeMailer(cfg, logger),
	)
}

func registrationWelcomeMailer(cfg config.Config, logger *slog.Logger) apppublicsite.RegistrationWelcomeMailer {
	if !cfg.SMTPEnabled {
		return nil
	}
	logger.Info("registration welcome SMTP enabled", "addr", cfg.SMTPAddr, "publicBaseURL", cfg.PublicBaseURL)
	return inframail.NewRegistrationWelcomeMailer(inframail.SMTPConfig{
		Addr:          cfg.SMTPAddr,
		From:          cfg.SMTPFrom,
		PublicBaseURL: cfg.PublicBaseURL,
	})
}

func passwordRecoveryService(cfg config.Config, logger *slog.Logger) apppublicsite.PasswordRecoveryService {
	if !cfg.UniDBEnabled {
		return apppublicsite.PasswordRecoveryService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB password recovery disabled", "error", err)
		return apppublicsite.PasswordRecoveryService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB password recovery disabled", "error", err)
		_ = db.Close()
		return apppublicsite.PasswordRecoveryService{}
	}

	logger.Info("universe DB password recovery enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return apppublicsite.NewPasswordRecoveryService(
		mysqlregistration.NewPasswordRecoveryRepository(db, cfg.UniDBPrefix, cfg.UniDBSecret),
		passwordRecoveryMailer(cfg, logger),
		cfg.UniNumber,
		cfg.PublicBaseURL,
	)
}

func passwordRecoveryMailer(cfg config.Config, logger *slog.Logger) apppublicsite.PasswordRecoveryMailer {
	if !cfg.SMTPEnabled {
		return nil
	}
	logger.Info("password recovery SMTP enabled", "addr", cfg.SMTPAddr, "publicBaseURL", cfg.PublicBaseURL)
	return inframail.NewPasswordRecoveryMailer(inframail.SMTPConfig{
		Addr:          cfg.SMTPAddr,
		From:          cfg.SMTPFrom,
		PublicBaseURL: cfg.PublicBaseURL,
	})
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
	store := mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix)
	return apppublicsite.NewGameSessionLookupWithActivity(store, store, cfg.UniNumber)
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

func gameEmpireService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.EmpireService {
	if !cfg.UniDBEnabled {
		return appgame.EmpireService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game empire disabled", "error", err)
		return appgame.EmpireService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game empire disabled", "error", err)
		_ = db.Close()
		return appgame.EmpireService{}
	}

	logger.Info("universe DB game empire enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewEmpireService(sessions, mysqlgame.NewEmpireRepository(db, cfg.UniDBPrefix))
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

func gameMerchantService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.MerchantService {
	if !cfg.UniDBEnabled {
		return appgame.MerchantService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game merchant disabled", "error", err)
		return appgame.MerchantService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game merchant disabled", "error", err)
		_ = db.Close()
		return appgame.MerchantService{}
	}

	logger.Info("universe DB game merchant enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewMerchantService(sessions, mysqlgame.NewMerchantRepository(db, cfg.UniDBPrefix))
}

func gameOfficersService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.OfficersService {
	if !cfg.UniDBEnabled {
		return appgame.OfficersService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game officers disabled", "error", err)
		return appgame.OfficersService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game officers disabled", "error", err)
		_ = db.Close()
		return appgame.OfficersService{}
	}

	logger.Info("universe DB game officers enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewOfficersService(sessions, mysqlgame.NewOfficersRepository(db, cfg.UniDBPrefix))
}

func gameAllianceService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.AllianceService {
	if !cfg.UniDBEnabled {
		return appgame.AllianceService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game alliance disabled", "error", err)
		return appgame.AllianceService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game alliance disabled", "error", err)
		_ = db.Close()
		return appgame.AllianceService{}
	}

	logger.Info("universe DB game alliance enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewAllianceService(sessions, mysqlgame.NewAllianceRepository(db, cfg.UniDBPrefix))
}

func gameAdminService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.AdminService {
	if !cfg.UniDBEnabled {
		return appgame.AdminService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game admin disabled", "error", err)
		return appgame.AdminService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game admin disabled", "error", err)
		_ = db.Close()
		return appgame.AdminService{}
	}

	logger.Info("universe DB game admin enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	repository := mysqlgame.NewAdminRepository(db, cfg.UniDBPrefix).WithLegacyGameDir(cfg.LegacyGameDir)
	if masterDB := openMasterDBForGame(cfg, logger, "admin coupons"); masterDB != nil {
		masterRunner := mysqlgame.SQLQueryer{DB: masterDB}
		repository = repository.WithMasterRunner(masterRunner, masterRunner).WithUniverseNumber(cfg.UniNumber)
	}
	return appgame.NewAdminService(sessions, repository)
}

func gamePaymentService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.PaymentService {
	if !cfg.UniDBEnabled || !cfg.MasterDBEnabled {
		return appgame.PaymentService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game payment disabled", "error", err)
		return appgame.PaymentService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game payment disabled", "error", err)
		_ = db.Close()
		return appgame.PaymentService{}
	}
	masterDB := openMasterDBForGame(cfg, logger, "payment coupons")
	if masterDB == nil {
		_ = db.Close()
		return appgame.PaymentService{}
	}

	logger.Info("universe DB game payment enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return appgame.NewPaymentService(sessions, mysqlgame.NewPaymentRepository(db, masterDB, cfg.UniDBPrefix, cfg.UniNumber))
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

func gameBuddyService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.BuddyService {
	if !cfg.UniDBEnabled {
		return appgame.BuddyService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game buddy disabled", "error", err)
		return appgame.BuddyService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game buddy disabled", "error", err)
		_ = db.Close()
		return appgame.BuddyService{}
	}

	logger.Info("universe DB game buddy enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewBuddyService(sessions, mysqlgame.NewBuddyRepository(db, cfg.UniDBPrefix))
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

func gameMessagesService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.MessagesService {
	if !cfg.UniDBEnabled {
		return appgame.MessagesService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game messages disabled", "error", err)
		return appgame.MessagesService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game messages disabled", "error", err)
		_ = db.Close()
		return appgame.MessagesService{}
	}

	logger.Info("universe DB game messages enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewMessagesService(sessions, mysqlgame.NewMessagesRepository(db, cfg.UniDBPrefix))
}

func gameReportService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.ReportService {
	if !cfg.UniDBEnabled {
		return appgame.ReportService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game report disabled", "error", err)
		return appgame.ReportService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game report disabled", "error", err)
		_ = db.Close()
		return appgame.ReportService{}
	}

	logger.Info("universe DB game report enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewReportService(sessions, mysqlgame.NewReportRepository(db, cfg.UniDBPrefix))
}

func gamePhalanxService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.PhalanxService {
	if !cfg.UniDBEnabled {
		return appgame.PhalanxService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game phalanx disabled", "error", err)
		return appgame.PhalanxService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game phalanx disabled", "error", err)
		_ = db.Close()
		return appgame.PhalanxService{}
	}

	logger.Info("universe DB game phalanx enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewPhalanxService(sessions, mysqlgame.NewPhalanxRepository(db, cfg.UniDBPrefix))
}

func gameFeedService(cfg config.Config, logger *slog.Logger) appgame.FeedService {
	if !cfg.UniDBEnabled {
		return appgame.FeedService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game feed disabled", "error", err)
		return appgame.FeedService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game feed disabled", "error", err)
		_ = db.Close()
		return appgame.FeedService{}
	}

	logger.Info("universe DB game feed enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewFeedService(mysqlgame.NewFeedRepository(db, cfg.UniDBPrefix))
}

func gameOptionsService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup) appgame.OptionsService {
	if !cfg.UniDBEnabled {
		return appgame.OptionsService{}
	}

	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{
		Host:     cfg.UniDBHost,
		User:     cfg.UniDBUser,
		Password: cfg.UniDBPassword,
		Name:     cfg.UniDBName,
	})
	if err != nil {
		logger.Warn("universe DB game options disabled", "error", err)
		return appgame.OptionsService{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("universe DB game options disabled", "error", err)
		_ = db.Close()
		return appgame.OptionsService{}
	}

	logger.Info("universe DB game options enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewOptionsService(sessions, mysqlgame.NewOptionsRepositoryWithSecret(db, cfg.UniDBPrefix, cfg.UniDBSecret))
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

func openMasterDBForGame(cfg config.Config, logger *slog.Logger, label string) *sql.DB {
	if !cfg.MasterDBEnabled {
		return nil
	}
	db, err := mysqlcatalog.Open(mysqlcatalog.MasterDBConfig{
		Host:     cfg.MasterDBHost,
		User:     cfg.MasterDBUser,
		Password: cfg.MasterDBPassword,
		Name:     cfg.MasterDBName,
	})
	if err != nil {
		logger.Warn("master DB "+label+" disabled", "error", err)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Warn("master DB "+label+" disabled", "error", err)
		_ = db.Close()
		return nil
	}
	logger.Info("master DB "+label+" enabled", "host", cfg.MasterDBHost, "database", cfg.MasterDBName)
	return db
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
