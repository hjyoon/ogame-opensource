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

type RegistrationActivationUseCase interface {
	ActivateAccount(context.Context, apppublicsite.RegistrationActivationCommand) (domainpublicsite.RegistrationActivation, error)
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
	RenamePlanet(context.Context, appgame.OverviewRenameCommand) (appgame.OverviewResult, error)
	DeletePlanet(context.Context, appgame.OverviewDeleteCommand) (appgame.OverviewResult, error)
}

type GameBuildingsUseCase interface {
	GetBuildings(context.Context, appgame.BuildingsCommand) (appgame.BuildingsResult, error)
	MutateBuildings(context.Context, appgame.BuildingsMutationCommand) (appgame.BuildingsResult, error)
}

type GameResourcesUseCase interface {
	GetResources(context.Context, appgame.ResourcesCommand) (appgame.ResourcesResult, error)
	UpdateResources(context.Context, appgame.ResourcesUpdateCommand) (appgame.ResourcesResult, error)
}

type GameResearchUseCase interface {
	GetResearch(context.Context, appgame.ResearchCommand) (appgame.ResearchResult, error)
	MutateResearch(context.Context, appgame.ResearchMutationCommand) (appgame.ResearchResult, error)
}

type GameShipyardUseCase interface {
	GetShipyard(context.Context, appgame.ShipyardCommand) (appgame.ShipyardResult, error)
	MutateShipyard(context.Context, appgame.ShipyardMutationCommand) (appgame.ShipyardResult, error)
}

type GameFleetUseCase interface {
	GetFleet(context.Context, appgame.FleetCommand) (appgame.FleetResult, error)
	MutateFleetTemplate(context.Context, appgame.FleetTemplateMutationCommand) (appgame.FleetResult, error)
}

type GameGalaxyUseCase interface {
	GetGalaxy(context.Context, appgame.GalaxyCommand) (appgame.GalaxyResult, error)
}

type GameDefenseUseCase interface {
	GetDefense(context.Context, appgame.DefenseCommand) (appgame.DefenseResult, error)
	MutateDefense(context.Context, appgame.DefenseMutationCommand) (appgame.DefenseResult, error)
}

type GameTechnologyUseCase interface {
	GetTechnology(context.Context, appgame.TechnologyCommand) (appgame.TechnologyResult, error)
}

type GameStatisticsUseCase interface {
	GetStatistics(context.Context, appgame.StatisticsCommand) (appgame.StatisticsResult, error)
}

type GameSearchUseCase interface {
	GetSearch(context.Context, appgame.SearchCommand) (appgame.SearchResult, error)
}

type GameBuddyUseCase interface {
	GetBuddy(context.Context, appgame.BuddyCommand) (appgame.BuddyResult, error)
	MutateBuddy(context.Context, appgame.BuddyMutationCommand) (appgame.BuddyResult, error)
}

type GameNotesUseCase interface {
	GetNotes(context.Context, appgame.NotesCommand) (appgame.NotesResult, error)
	CreateNote(context.Context, appgame.NotesMutationCommand) (appgame.NotesResult, error)
	UpdateNote(context.Context, appgame.NotesMutationCommand) (appgame.NotesResult, error)
	DeleteNotes(context.Context, appgame.NotesMutationCommand) (appgame.NotesResult, error)
}

type Dependencies struct {
	Health             HealthUseCase
	Universes          UniverseCatalogUseCase
	RegistrationDrafts RegistrationDraftUseCase
	Registration       RegistrationUseCase
	Activation         RegistrationActivationUseCase
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
	GameSearch         GameSearchUseCase
	GameBuddy          GameBuddyUseCase
	GameNotes          GameNotesUseCase
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
	mux.HandleFunc("/game/validate.php", getOnly(a.handleRegistrationActivation))
	mux.HandleFunc("/activation", getOnly(a.handleRegistrationActivation))
	mux.HandleFunc("/api/public/login/validate", postOnly(a.handleLoginValidation))
	mux.HandleFunc("/api/public/login", postOnly(a.handleLogin))
	mux.HandleFunc("/api/game/session", getOnly(a.handleGameSession))
	mux.HandleFunc("/api/game/logout", postOnly(a.handleGameLogout))
	mux.HandleFunc("/api/game/overview", a.handleGameOverview)
	mux.HandleFunc("/api/game/buildings", a.handleGameBuildings)
	mux.HandleFunc("/api/game/resources", a.handleGameResources)
	mux.HandleFunc("/api/game/research", a.handleGameResearch)
	mux.HandleFunc("/api/game/shipyard", a.handleGameShipyard)
	mux.HandleFunc("/api/game/fleet", getOnly(a.handleGameFleet))
	mux.HandleFunc("/api/game/fleet-templates", a.handleGameFleetTemplates)
	mux.HandleFunc("/api/game/galaxy", getOnly(a.handleGameGalaxy))
	mux.HandleFunc("/api/game/defense", a.handleGameDefense)
	mux.HandleFunc("/api/game/technology", getOnly(a.handleGameTechnology))
	mux.HandleFunc("/api/game/statistics", getOnly(a.handleGameStatistics))
	mux.HandleFunc("/api/game/search", getOnly(a.handleGameSearch))
	mux.HandleFunc("/api/game/buddy", a.handleGameBuddy)
	mux.HandleFunc("/api/game/notes", a.handleGameNotes)
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
