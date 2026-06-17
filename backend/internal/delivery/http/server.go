package httpdelivery

import (
	"context"
	"log/slog"
	"net/http"

	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

type HealthUseCase interface {
	Get(context.Context) domainsystem.Health
}

type FrontendAssets interface {
	Serve(w http.ResponseWriter, r *http.Request, rel string) bool
}

type Dependencies struct {
	Health       HealthUseCase
	Frontend     FrontendAssets
	LegacyAssets http.FileSystem
	Logger       *slog.Logger
}

type app struct {
	deps Dependencies
}

func New(deps Dependencies) http.Handler {
	a := app{deps: deps}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", getOnly(a.handleHealthz))
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
