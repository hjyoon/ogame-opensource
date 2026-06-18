package httpdelivery

import (
	"context"
	"log/slog"
	"net/http"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

type HealthUseCase interface {
	Get(context.Context) domainsystem.Health
}

type FrontendAssets interface {
	Serve(w http.ResponseWriter, r *http.Request, rel string) bool
}

type UniverseCatalogUseCase interface {
	ListUniverses(context.Context) ([]domainpublicsite.Universe, error)
}

type RegistrationDraftUseCase interface {
	ValidateRegistrationDraft(context.Context, apppublicsite.RegistrationDraftCommand) (domainpublicsite.RegistrationValidation, error)
}

type RegistrationUseCase interface {
	RegisterAccount(context.Context, apppublicsite.RegistrationCommand) (domainpublicsite.RegistrationCreation, error)
}

type LoginDraftUseCase interface {
	ValidateLoginDraft(context.Context, apppublicsite.LoginDraftCommand) (domainpublicsite.LoginValidation, error)
}

type LoginUseCase interface {
	AuthenticateLogin(context.Context, apppublicsite.LoginCommand) (domainpublicsite.LoginAuthentication, error)
}

type GameSessionUseCase interface {
	GetGameSession(context.Context, apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error)
}

type LogoutUseCase interface {
	Logout(context.Context, apppublicsite.LogoutCommand) (apppublicsite.LogoutResult, error)
}

type GameOverviewUseCase interface {
	GetOverview(context.Context, appgame.OverviewCommand) (appgame.OverviewResult, error)
}

type GameBuildingsUseCase interface {
	GetBuildings(context.Context, appgame.BuildingsCommand) (appgame.BuildingsResult, error)
}

type GameResourcesUseCase interface {
	GetResources(context.Context, appgame.ResourcesCommand) (appgame.ResourcesResult, error)
	UpdateResources(context.Context, appgame.ResourcesUpdateCommand) (appgame.ResourcesResult, error)
}

type GameResearchUseCase interface {
	GetResearch(context.Context, appgame.ResearchCommand) (appgame.ResearchResult, error)
}

type GameShipyardUseCase interface {
	GetShipyard(context.Context, appgame.ShipyardCommand) (appgame.ShipyardResult, error)
}

type GameFleetUseCase interface {
	GetFleet(context.Context, appgame.FleetCommand) (appgame.FleetResult, error)
}

type GameGalaxyUseCase interface {
	GetGalaxy(context.Context, appgame.GalaxyCommand) (appgame.GalaxyResult, error)
}

type GameDefenseUseCase interface {
	GetDefense(context.Context, appgame.DefenseCommand) (appgame.DefenseResult, error)
}

type GameTechnologyUseCase interface {
	GetTechnology(context.Context, appgame.TechnologyCommand) (appgame.TechnologyResult, error)
}

type GameStatisticsUseCase interface {
	GetStatistics(context.Context, appgame.StatisticsCommand) (appgame.StatisticsResult, error)
}

type Dependencies struct {
	Health             HealthUseCase
	Universes          UniverseCatalogUseCase
	RegistrationDrafts RegistrationDraftUseCase
	Registration       RegistrationUseCase
	LoginDrafts        LoginDraftUseCase
	Login              LoginUseCase
	GameSessions       GameSessionUseCase
	Logout             LogoutUseCase
	GameOverview       GameOverviewUseCase
	GameBuildings      GameBuildingsUseCase
	GameResources      GameResourcesUseCase
	GameResearch       GameResearchUseCase
	GameShipyard       GameShipyardUseCase
	GameFleet          GameFleetUseCase
	GameGalaxy         GameGalaxyUseCase
	GameDefense        GameDefenseUseCase
	GameTechnology     GameTechnologyUseCase
	GameStatistics     GameStatisticsUseCase
	Frontend           FrontendAssets
	LegacyAssets       http.FileSystem
	Logger             *slog.Logger
}

type app struct {
	deps Dependencies
}

func New(deps Dependencies) http.Handler {
	a := app{deps: deps}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", getOnly(a.handleHealthz))
	mux.HandleFunc("/api/public/universes", getOnly(a.handleUniverses))
	mux.HandleFunc("/api/public/registration/validate", postOnly(a.handleRegistrationValidation))
	mux.HandleFunc("/api/public/registration", postOnly(a.handleRegistration))
	mux.HandleFunc("/api/public/login/validate", postOnly(a.handleLoginValidation))
	mux.HandleFunc("/api/public/login", postOnly(a.handleLogin))
	mux.HandleFunc("/api/game/session", getOnly(a.handleGameSession))
	mux.HandleFunc("/api/game/logout", postOnly(a.handleGameLogout))
	mux.HandleFunc("/api/game/overview", getOnly(a.handleGameOverview))
	mux.HandleFunc("/api/game/buildings", getOnly(a.handleGameBuildings))
	mux.HandleFunc("/api/game/resources", a.handleGameResources)
	mux.HandleFunc("/api/game/research", getOnly(a.handleGameResearch))
	mux.HandleFunc("/api/game/shipyard", getOnly(a.handleGameShipyard))
	mux.HandleFunc("/api/game/fleet", getOnly(a.handleGameFleet))
	mux.HandleFunc("/api/game/galaxy", getOnly(a.handleGameGalaxy))
	mux.HandleFunc("/api/game/defense", getOnly(a.handleGameDefense))
	mux.HandleFunc("/api/game/technology", getOnly(a.handleGameTechnology))
	mux.HandleFunc("/api/game/statistics", getOnly(a.handleGameStatistics))
	mux.Handle("/legacy-assets/", http.StripPrefix("/legacy-assets/", http.FileServer(deps.LegacyAssets)))
	mux.HandleFunc("/", getOnly(a.handleFrontend))
	handler := securityHeaders(mux)
	if deps.Logger != nil {
		return accessLog(deps.Logger, handler)
	}
	return handler
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

func postOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}
