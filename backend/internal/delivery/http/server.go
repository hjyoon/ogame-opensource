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

type GameOverviewUseCase interface {
	GetOverview(context.Context, appgame.OverviewCommand) (appgame.OverviewResult, error)
}

type Dependencies struct {
	Health             HealthUseCase
	Universes          UniverseCatalogUseCase
	RegistrationDrafts RegistrationDraftUseCase
	Registration       RegistrationUseCase
	LoginDrafts        LoginDraftUseCase
	Login              LoginUseCase
	GameSessions       GameSessionUseCase
	GameOverview       GameOverviewUseCase
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
	mux.HandleFunc("/api/game/overview", getOnly(a.handleGameOverview))
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
