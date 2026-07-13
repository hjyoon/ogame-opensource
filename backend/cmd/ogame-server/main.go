package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	appmcp "github.com/hjyoon/ogame-opensource/backend/internal/application/mcp"
	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	httpdelivery "github.com/hjyoon/ogame-opensource/backend/internal/delivery/http"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/catalogrepo"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/configcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/filesystem"
	infrahttpclient "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/httpclient"
	inframail "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mail"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mcpaudit"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mcpauth"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mcpoidc"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlgame"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlregistration"
	infraruntime "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/runtime"
	infrasession "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/session"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	pools := openDatabasePools(cfg, logger)
	defer pools.Close(logger)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           buildHandler(cfg, logger, pools),
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverError := make(chan error, 1)
	go func() {
		logger.Info("starting ogame go server", "addr", cfg.Addr, "env", cfg.Environment)
		serverError <- server.ListenAndServe()
	}()

	select {
	case err := <-serverError:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("ogame go server stopped unexpectedly", "error", err)
			return
		}
	case <-shutdownSignal.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("ogame go server graceful shutdown failed", "error", err)
		}
	}
}

func buildHandler(cfg config.Config, logger *slog.Logger, pools databasePools) http.Handler {
	masterDBProbe, universeDBProbe, modRuntimeProbe := pools.readinessProbes(cfg.UniDBPrefix)
	health := appsystem.NewHealthService(appsystem.HealthConfig{
		Environment:        cfg.Environment,
		StaticDir:          cfg.StaticDir,
		LegacyAssetDir:     cfg.LegacyAssetDir,
		LegacyBaseURL:      cfg.LegacyBaseURL,
		GoTarget:           config.GoTarget,
		BunTarget:          config.BunTarget,
		ReactTarget:        config.ReactTarget,
		MasterDBRequired:   cfg.MasterDBEnabled,
		UniverseDBRequired: cfg.UniDBEnabled,
		ModRuntimeRequired: cfg.UniDBEnabled,
	}, filesystem.Probe{}, infraruntime.GoRuntime{}, masterDBProbe, universeDBProbe, modRuntimeProbe)
	universes := apppublicsite.NewUniverseCatalogService(universeRepository(cfg, logger, pools))
	registrationDrafts := registrationValidator(cfg, logger, pools)
	registration := registrationRegistrar(cfg, logger, pools)
	activation := registrationActivation(cfg, logger, pools)
	directEntry := apppublicsite.NewDirectEntryService(infrahttpclient.NewExternalImageFetcher())
	passwordRecovery := passwordRecoveryService(cfg, logger, pools)
	loginDrafts := loginValidator(cfg, logger, pools)
	login := loginAuthenticator(cfg, logger, pools)
	gameSessions := gameSessionLookup(cfg, logger, pools)
	mcp := mcpService(cfg, logger, health, gameSessions, pools)
	logout := logoutService(cfg, logger, pools)
	gameOverview := gameOverviewService(cfg, logger, gameSessions, pools)
	gameBuildings := gameBuildingsService(cfg, logger, gameSessions, pools)
	gameEmpire := gameEmpireService(cfg, logger, gameSessions, pools)
	gameResources := gameResourcesService(cfg, logger, gameSessions, pools)
	gameMerchant := gameMerchantService(cfg, logger, gameSessions, pools)
	gameOfficers := gameOfficersService(cfg, logger, gameSessions, pools)
	gameAlliance := gameAllianceService(cfg, logger, gameSessions, pools)
	gameAdmin := gameAdminService(cfg, logger, gameSessions, pools)
	gameResearch := gameResearchService(cfg, logger, gameSessions, pools)
	gameShipyard := gameShipyardService(cfg, logger, gameSessions, pools)
	gameFleet := gameFleetService(cfg, logger, gameSessions, pools)
	gameGalaxy := gameGalaxyService(cfg, logger, gameSessions, pools)
	gameDefense := gameDefenseService(cfg, logger, gameSessions, pools)
	gameTechnology := gameTechnologyService(cfg, logger, gameSessions, pools)
	gameStatistics := gameStatisticsService(cfg, logger, gameSessions, pools)
	gameSearch := gameSearchService(cfg, logger, gameSessions, pools)
	gameBuddy := gameBuddyService(cfg, logger, gameSessions, pools)
	gameNotes := gameNotesService(cfg, logger, gameSessions, pools)
	gameMessages := gameMessagesService(cfg, logger, gameSessions, pools)
	gameReport := gameReportService(cfg, logger, gameSessions, pools)
	gamePhalanx := gamePhalanxService(cfg, logger, gameSessions, pools)
	gameJumpGate := gameJumpGateService(cfg, logger, gameSessions, pools)
	gamePranger := gamePrangerService(cfg, logger, pools)
	gameMaintenance := gameMaintenanceService(cfg, logger, pools)
	gameFeed := gameFeedService(cfg, logger, pools)
	gameOptions := gameOptionsService(cfg, logger, gameSessions, pools)
	gamePayment := gamePaymentService(cfg, logger, gameSessions, pools)

	return httpdelivery.New(httpdelivery.Dependencies{
		Health:                health,
		MCP:                   mcp,
		MCPTokens:             mcp,
		MCPOAuth:              mcp,
		UniverseNumber:        cfg.UniNumber,
		MaintenanceStartPage:  "/",
		Universes:             universes,
		RegistrationDrafts:    registrationDrafts,
		Registration:          registration,
		Activation:            activation,
		DirectEntry:           directEntry,
		PasswordRecovery:      passwordRecovery,
		LoginDrafts:           loginDrafts,
		Login:                 login,
		GameSessions:          gameSessions,
		Logout:                logout,
		GameOverview:          gameOverview,
		GameBuildings:         gameBuildings,
		GameEmpire:            gameEmpire,
		GameResources:         gameResources,
		GameMerchant:          gameMerchant,
		GameOfficers:          gameOfficers,
		GameAlliance:          gameAlliance,
		GameAdmin:             gameAdmin,
		GameResearch:          gameResearch,
		GameShipyard:          gameShipyard,
		GameFleet:             gameFleet,
		GameGalaxy:            gameGalaxy,
		GameDefense:           gameDefense,
		GameTechnology:        gameTechnology,
		GameStatistics:        gameStatistics,
		GameSearch:            gameSearch,
		GameBuddy:             gameBuddy,
		GameNotes:             gameNotes,
		GameMessages:          gameMessages,
		GameReport:            gameReport,
		GamePhalanx:           gamePhalanx,
		GameJumpGate:          gameJumpGate,
		GamePranger:           gamePranger,
		GameMaintenance:       gameMaintenance,
		GameFeed:              gameFeed,
		GameOptions:           gameOptions,
		GamePayment:           gamePayment,
		Frontend:              filesystem.StaticDir{Root: cfg.StaticDir},
		LegacyAssets:          filesystem.NewNoListingFS(cfg.LegacyAssetDir),
		Logger:                logger,
		MCPOAuthConsentSecret: cfg.UniDBSecret,
		MCPRateLimit: httpdelivery.RateLimitConfig{
			Disabled:          !cfg.MCPRateLimitEnabled,
			RequestsPerMinute: cfg.MCPRateLimitPerMin,
			Burst:             cfg.MCPRateLimitBurst,
		},
	})
}

func mcpService(cfg config.Config, logger *slog.Logger, health appsystem.HealthService, sessions apppublicsite.GameSessionLookup, pools databasePools) appmcp.Service {
	staticVerifier := mcpauth.NewStaticTokenVerifier(cfg.MCPStaticTokens)
	oidcSigner, oidcErr := mcpOIDCSigner(cfg)
	if oidcErr != nil {
		logger.Warn("mcp oidc signer disabled", "error", oidcErr)
	}
	withCommonMCP := func(service appmcp.Service) appmcp.Service {
		if oidcErr == nil {
			service = service.WithOIDCSigner(oidcSigner)
		}
		return service.WithToolCallAuditor(mcpaudit.NewSlogLogger(logger))
	}
	if !cfg.UniDBEnabled {
		return withCommonMCP(appmcp.NewServiceWithTokenVerifier(health, staticVerifier))
	}

	db := pools.universe
	if db == nil {
		return withCommonMCP(appmcp.NewServiceWithTokenVerifier(health, staticVerifier))
	}

	repository := mysqlgame.NewMCPTokenRepository(db, cfg.UniDBPrefix)
	ensureMCPSchemaEventually(logger, repository)

	logger.Info("universe DB mcp token and oauth management enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	verifier := mcpauth.NewCompositeTokenVerifier(repository, staticVerifier)
	readRepository := mysqlgame.NewMCPReadRepository(db, cfg.UniDBPrefix)
	reportReadRepository := mysqlgame.NewReportRepository(db, cfg.UniDBPrefix)
	writeRepository := mysqlgame.NewMessagesRepository(db, cfg.UniDBPrefix)
	fleetWriteRepository := mysqlgame.NewFleetRepository(db, cfg.UniDBPrefix)
	phalanxWriteRepository := mysqlgame.NewPhalanxRepository(db, cfg.UniDBPrefix)
	queueWriteRepository := mysqlgame.NewBuildingsRepository(db, cfg.UniDBPrefix)
	resourceWriteRepository := mysqlgame.NewResourcesRepository(db, cfg.UniDBPrefix)
	resourceReadRepository := mysqlgame.NewResourcesReadRepository(db, cfg.UniDBPrefix)
	premiumRepository := mysqlgame.NewOfficersRepository(db, cfg.UniDBPrefix)
	searchReadRepository := mysqlgame.NewSearchRepository(db, cfg.UniDBPrefix)
	galaxyReadRepository := mysqlgame.NewGalaxyReadRepository(db, cfg.UniDBPrefix)
	statisticsReadRepository := mysqlgame.NewStatisticsRepository(db, cfg.UniDBPrefix)
	allianceReadRepository := mysqlgame.NewAllianceReadRepository(db, cfg.UniDBPrefix)
	buddyReadRepository := mysqlgame.NewBuddyReadRepository(db, cfg.UniDBPrefix)
	buddyWriteRepository := mysqlgame.NewBuddyRepository(db, cfg.UniDBPrefix)
	prangerReadRepository := mysqlgame.NewPrangerRepository(db, cfg.UniDBPrefix)
	notesReadRepository := mysqlgame.NewNotesReadRepository(db, cfg.UniDBPrefix)
	notesWriteRepository := mysqlgame.NewNotesRepository(db, cfg.UniDBPrefix)
	optionsReadRepository := mysqlgame.NewOptionsReadRepository(db, cfg.UniDBPrefix)
	maintenanceReadRepository := mysqlgame.NewMaintenanceRepository(db, cfg.UniDBPrefix)
	merchantRepository := mysqlgame.NewMerchantRepository(db, cfg.UniDBPrefix)
	jumpGateRepository := mysqlgame.NewJumpGateRepository(db, cfg.UniDBPrefix)
	empireReadRepository := mysqlgame.NewEmpireReadRepository(db, cfg.UniDBPrefix)
	technologyReadRepository := mysqlgame.NewTechnologyRepository(db, cfg.UniDBPrefix)
	buildingReadRepository := mysqlgame.NewBuildingsReadRepository(db, cfg.UniDBPrefix)
	researchReadRepository := mysqlgame.NewResearchReadRepository(db, cfg.UniDBPrefix)
	shipyardReadRepository := mysqlgame.NewShipyardReadRepository(db, cfg.UniDBPrefix)
	defenseReadRepository := mysqlgame.NewDefenseReadRepository(db, cfg.UniDBPrefix)
	fleetReadRepository := mysqlgame.NewFleetReadRepository(db, cfg.UniDBPrefix)
	return withCommonMCP(appmcp.NewServiceWithTokenManagement(health, verifier, repository, sessions, appmcp.SecureTokenGenerator{}, time.Now).
		WithTokenTTL(time.Duration(cfg.MCPTokenTTLSeconds) * time.Second).
		WithOAuthCodeRepository(repository).
		WithOAuthRedirectURIs(appmcp.ParseOAuthRedirectURIs(cfg.MCPOAuthRedirectURIs)).
		WithReadRepository(readRepository).
		WithReportReadRepository(reportReadRepository).
		WithWriteRepository(writeRepository).
		WithFleetWriteRepository(fleetWriteRepository).
		WithPhalanxWriteRepository(phalanxWriteRepository).
		WithQueueWriteRepository(queueWriteRepository).
		WithResourceWriteRepository(resourceWriteRepository).
		WithResourceProductionReadRepository(resourceReadRepository).
		WithPremiumReadRepository(premiumRepository).
		WithSearchReadRepository(searchReadRepository).
		WithGalaxyReadRepository(galaxyReadRepository).
		WithStatisticsReadRepository(statisticsReadRepository).
		WithAllianceReadRepository(allianceReadRepository).
		WithBuddyReadRepository(buddyReadRepository).
		WithBuddyWriteRepository(buddyWriteRepository).
		WithPrangerReadRepository(prangerReadRepository).
		WithNotesReadRepository(notesReadRepository).
		WithNotesWriteRepository(notesWriteRepository).
		WithOptionsReadRepository(optionsReadRepository).
		WithMaintenanceReadRepository(maintenanceReadRepository).
		WithMerchantReadRepository(merchantRepository).
		WithMerchantWriteRepository(merchantRepository).
		WithJumpGateReadRepository(jumpGateRepository).
		WithJumpGateWriteRepository(jumpGateRepository).
		WithEmpireReadRepository(empireReadRepository).
		WithTechnologyReadRepository(technologyReadRepository).
		WithBuildingOptionsReadRepository(buildingReadRepository).
		WithResearchOptionsReadRepository(researchReadRepository).
		WithShipyardOptionsReadRepository(shipyardReadRepository).
		WithDefenseOptionsReadRepository(defenseReadRepository).
		WithFleetOptionsReadRepository(fleetReadRepository).
		WithPremiumWriteRepository(premiumRepository))
}

type mcpSchemaRepository interface {
	EnsureMCPTokenSchema(context.Context) error
	EnsureMCPOAuthCodeSchema(context.Context) error
}

func ensureMCPSchemaEventually(logger *slog.Logger, repository mcpSchemaRepository) {
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err := repository.EnsureMCPTokenSchema(ctx)
			if err == nil {
				err = repository.EnsureMCPOAuthCodeSchema(ctx)
			}
			cancel()
			if err == nil {
				logger.Info("universe DB mcp schemas ready")
				return
			}
			logger.Warn("universe DB mcp schemas unavailable; retrying", "error", err)
			time.Sleep(2 * time.Second)
		}
	}()
}

func mcpOIDCSigner(cfg config.Config) (mcpoidc.Ed25519Signer, error) {
	if strings.TrimSpace(cfg.MCPOIDCSigningSeed) != "" {
		return mcpoidc.NewEd25519SignerFromBase64Seeds(cfg.MCPOIDCSigningSeed, mcpOIDCPreviousSeeds(cfg.MCPOIDCPreviousSeeds), time.Now)
	}
	return mcpoidc.NewEphemeralEd25519Signer()
}

func mcpOIDCPreviousSeeds(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})
}

func registrationActivation(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.RegistrationActivationService {
	if !cfg.UniDBEnabled {
		return apppublicsite.RegistrationActivationService{}
	}

	db := pools.universe
	if db == nil {
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

func registrationRegistrar(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.RegistrationRegistrar {
	if !cfg.UniDBEnabled {
		return apppublicsite.RegistrationRegistrar{}
	}

	db := pools.universe
	if db == nil {
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

func passwordRecoveryService(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.PasswordRecoveryService {
	if !cfg.UniDBEnabled {
		return apppublicsite.PasswordRecoveryService{}
	}

	db := pools.universe
	if db == nil {
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

func loginValidator(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.LoginDraftValidator {
	if !cfg.UniDBEnabled {
		return apppublicsite.NewLoginDraftValidator()
	}

	db := pools.universe
	if db == nil {
		return apppublicsite.NewLoginDraftValidator()
	}

	logger.Info("universe DB login credentials enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return apppublicsite.NewLoginDraftValidatorWithCredentials(mysqlregistration.NewCredentialChecker(db, cfg.UniDBPrefix, cfg.UniDBSecret))
}

func loginAuthenticator(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.LoginAuthenticator {
	if !cfg.UniDBEnabled {
		return apppublicsite.LoginAuthenticator{}
	}

	db := pools.universe
	if db == nil {
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

func gameSessionLookup(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.GameSessionLookup {
	if !cfg.UniDBEnabled {
		return apppublicsite.GameSessionLookup{}
	}

	db := pools.universe
	if db == nil {
		return apppublicsite.GameSessionLookup{}
	}

	logger.Info("universe DB game session lookup enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	store := mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix)
	return apppublicsite.NewGameSessionLookupWithActivity(store, store, cfg.UniNumber)
}

func logoutService(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.LogoutService {
	if !cfg.UniDBEnabled {
		return apppublicsite.LogoutService{}
	}

	db := pools.universe
	if db == nil {
		return apppublicsite.LogoutService{}
	}

	logger.Info("universe DB logout enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return apppublicsite.NewLogoutService(mysqlregistration.NewSessionStore(db, cfg.UniDBPrefix), cfg.UniNumber)
}

func gameOverviewService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.OverviewService {
	if !cfg.UniDBEnabled {
		return appgame.OverviewService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.OverviewService{}
	}

	logger.Info("universe DB game overview enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewOverviewService(sessions, mysqlgame.NewOverviewRepositoryWithSecret(db, cfg.UniDBPrefix, cfg.UniDBSecret))
}

func gameBuildingsService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.BuildingsService {
	if !cfg.UniDBEnabled {
		return appgame.BuildingsService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.BuildingsService{}
	}

	logger.Info("universe DB game buildings enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewBuildingsService(sessions, mysqlgame.NewBuildingsRepository(db, cfg.UniDBPrefix))
}

func gameEmpireService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.EmpireService {
	if !cfg.UniDBEnabled {
		return appgame.EmpireService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.EmpireService{}
	}

	logger.Info("universe DB game empire enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewEmpireService(sessions, mysqlgame.NewEmpireRepository(db, cfg.UniDBPrefix))
}

func gameResourcesService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.ResourcesService {
	if !cfg.UniDBEnabled {
		return appgame.ResourcesService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.ResourcesService{}
	}

	logger.Info("universe DB game resources enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewResourcesService(sessions, mysqlgame.NewResourcesRepository(db, cfg.UniDBPrefix))
}

func gameMerchantService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.MerchantService {
	if !cfg.UniDBEnabled {
		return appgame.MerchantService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.MerchantService{}
	}

	logger.Info("universe DB game merchant enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewMerchantService(sessions, mysqlgame.NewMerchantRepository(db, cfg.UniDBPrefix))
}

func gameOfficersService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.OfficersService {
	if !cfg.UniDBEnabled {
		return appgame.OfficersService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.OfficersService{}
	}

	logger.Info("universe DB game officers enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewOfficersService(sessions, mysqlgame.NewOfficersRepository(db, cfg.UniDBPrefix))
}

func gameAllianceService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.AllianceService {
	if !cfg.UniDBEnabled {
		return appgame.AllianceService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.AllianceService{}
	}

	logger.Info("universe DB game alliance enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewAllianceService(sessions, mysqlgame.NewAllianceRepository(db, cfg.UniDBPrefix))
}

func gameAdminService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.AdminService {
	if !cfg.UniDBEnabled {
		return appgame.AdminService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.AdminService{}
	}

	logger.Info("universe DB game admin enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	repository := mysqlgame.NewAdminRepository(db, cfg.UniDBPrefix).WithLegacyGameDir(cfg.LegacyGameDir).WithSecret(cfg.UniDBSecret)
	if masterDB := pools.master; masterDB != nil {
		masterRunner := mysqlgame.SQLQueryer{DB: masterDB}
		repository = repository.WithMasterRunner(masterRunner, masterRunner).WithUniverseNumber(cfg.UniNumber)
	}
	return appgame.NewAdminService(sessions, repository)
}

func gamePaymentService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.PaymentService {
	if !cfg.UniDBEnabled || !cfg.MasterDBEnabled {
		return appgame.PaymentService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.PaymentService{}
	}

	masterDB := pools.master
	if masterDB == nil {
		return appgame.PaymentService{}
	}

	logger.Info("universe DB game payment enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return appgame.NewPaymentService(sessions, mysqlgame.NewPaymentRepository(db, masterDB, cfg.UniDBPrefix, cfg.UniNumber))
}

func gameResearchService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.ResearchService {
	if !cfg.UniDBEnabled {
		return appgame.ResearchService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.ResearchService{}
	}

	logger.Info("universe DB game research enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewResearchService(sessions, mysqlgame.NewResearchRepository(db, cfg.UniDBPrefix))
}

func gameShipyardService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.ShipyardService {
	if !cfg.UniDBEnabled {
		return appgame.ShipyardService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.ShipyardService{}
	}

	logger.Info("universe DB game shipyard enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewShipyardService(sessions, mysqlgame.NewShipyardRepository(db, cfg.UniDBPrefix))
}

func gameDefenseService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.DefenseService {
	if !cfg.UniDBEnabled {
		return appgame.DefenseService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.DefenseService{}
	}

	logger.Info("universe DB game defense enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewDefenseService(sessions, mysqlgame.NewDefenseRepository(db, cfg.UniDBPrefix))
}

func gameFleetService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.FleetService {
	if !cfg.UniDBEnabled {
		return appgame.FleetService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.FleetService{}
	}

	logger.Info("universe DB game fleet enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewFleetService(sessions, mysqlgame.NewFleetRepository(db, cfg.UniDBPrefix))
}

func gameGalaxyService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.GalaxyService {
	if !cfg.UniDBEnabled {
		return appgame.GalaxyService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.GalaxyService{}
	}

	logger.Info("universe DB game galaxy enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewGalaxyService(sessions, mysqlgame.NewGalaxyRepository(db, cfg.UniDBPrefix))
}

func gameTechnologyService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.TechnologyService {
	if !cfg.UniDBEnabled {
		return appgame.TechnologyService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.TechnologyService{}
	}

	logger.Info("universe DB game technology enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewTechnologyService(sessions, mysqlgame.NewTechnologyRepository(db, cfg.UniDBPrefix))
}

func gameStatisticsService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.StatisticsService {
	if !cfg.UniDBEnabled {
		return appgame.StatisticsService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.StatisticsService{}
	}

	logger.Info("universe DB game statistics enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewStatisticsService(sessions, mysqlgame.NewStatisticsRepository(db, cfg.UniDBPrefix))
}

func gameSearchService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.SearchService {
	if !cfg.UniDBEnabled {
		return appgame.SearchService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.SearchService{}
	}

	logger.Info("universe DB game search enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewSearchService(sessions, mysqlgame.NewSearchRepository(db, cfg.UniDBPrefix))
}

func gameBuddyService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.BuddyService {
	if !cfg.UniDBEnabled {
		return appgame.BuddyService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.BuddyService{}
	}

	logger.Info("universe DB game buddy enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewBuddyService(sessions, mysqlgame.NewBuddyRepository(db, cfg.UniDBPrefix))
}

func gameNotesService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.NotesService {
	if !cfg.UniDBEnabled {
		return appgame.NotesService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.NotesService{}
	}

	logger.Info("universe DB game notes enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewNotesService(sessions, mysqlgame.NewNotesRepository(db, cfg.UniDBPrefix))
}

func gameMessagesService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.MessagesService {
	if !cfg.UniDBEnabled {
		return appgame.MessagesService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.MessagesService{}
	}

	logger.Info("universe DB game messages enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewMessagesService(sessions, mysqlgame.NewMessagesRepository(db, cfg.UniDBPrefix))
}

func gameReportService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.ReportService {
	if !cfg.UniDBEnabled {
		return appgame.ReportService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.ReportService{}
	}

	logger.Info("universe DB game report enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewReportService(sessions, mysqlgame.NewReportRepository(db, cfg.UniDBPrefix))
}

func gamePhalanxService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.PhalanxService {
	if !cfg.UniDBEnabled {
		return appgame.PhalanxService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.PhalanxService{}
	}

	logger.Info("universe DB game phalanx enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewPhalanxService(sessions, mysqlgame.NewPhalanxRepository(db, cfg.UniDBPrefix))
}

func gameJumpGateService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.JumpGateService {
	if !cfg.UniDBEnabled {
		return appgame.JumpGateService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.JumpGateService{}
	}

	logger.Info("universe DB game jump gate enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewJumpGateService(sessions, mysqlgame.NewJumpGateRepository(db, cfg.UniDBPrefix))
}

func gamePrangerService(cfg config.Config, logger *slog.Logger, pools databasePools) appgame.PrangerService {
	if !cfg.UniDBEnabled {
		return appgame.PrangerService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.PrangerService{}
	}

	logger.Info("universe DB game pranger enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix, "universe", cfg.UniNumber)
	return appgame.NewPrangerService(mysqlgame.NewPrangerRepository(db, cfg.UniDBPrefix))
}

func gameMaintenanceService(cfg config.Config, logger *slog.Logger, pools databasePools) appgame.MaintenanceService {
	if !cfg.UniDBEnabled {
		return appgame.MaintenanceService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.MaintenanceService{}
	}

	logger.Info("universe DB game maintenance enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewMaintenanceService(mysqlgame.NewMaintenanceRepository(db, cfg.UniDBPrefix))
}

func gameFeedService(cfg config.Config, logger *slog.Logger, pools databasePools) appgame.FeedService {
	if !cfg.UniDBEnabled {
		return appgame.FeedService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.FeedService{}
	}

	logger.Info("universe DB game feed enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewFeedService(mysqlgame.NewFeedRepository(db, cfg.UniDBPrefix))
}

func gameOptionsService(cfg config.Config, logger *slog.Logger, sessions apppublicsite.GameSessionLookup, pools databasePools) appgame.OptionsService {
	if !cfg.UniDBEnabled {
		return appgame.OptionsService{}
	}

	db := pools.universe
	if db == nil {
		return appgame.OptionsService{}
	}

	logger.Info("universe DB game options enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return appgame.NewOptionsService(sessions, mysqlgame.NewOptionsRepositoryWithSecret(db, cfg.UniDBPrefix, cfg.UniDBSecret))
}

func registrationValidator(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.RegistrationDraftValidator {
	if !cfg.UniDBEnabled {
		return apppublicsite.NewRegistrationDraftValidator()
	}

	db := pools.universe
	if db == nil {
		return apppublicsite.NewRegistrationDraftValidator()
	}

	logger.Info("universe DB registration availability enabled", "host", cfg.UniDBHost, "database", cfg.UniDBName, "prefix", cfg.UniDBPrefix)
	return apppublicsite.NewRegistrationDraftValidatorWithAvailability(mysqlregistration.NewAvailabilityChecker(db, cfg.UniDBPrefix))
}

func universeRepository(cfg config.Config, logger *slog.Logger, pools databasePools) apppublicsite.UniverseRepository {
	fallback := configcatalog.UniverseCatalog{
		RawJSON:       cfg.PublicUniverses,
		LegacyBaseURL: cfg.LegacyBaseURL,
	}

	if strings.TrimSpace(cfg.PublicUniverses) != "" || !cfg.MasterDBEnabled {
		return fallback
	}

	db := pools.master
	if db == nil {
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
