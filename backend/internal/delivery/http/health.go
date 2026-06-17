package httpdelivery

import (
	"encoding/json"
	"net/http"

	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

type healthResponse struct {
	Status            string `json:"status"`
	Service           string `json:"service"`
	Environment       string `json:"environment"`
	Runtime           string `json:"runtime"`
	GoTarget          string `json:"goTarget"`
	BunTarget         string `json:"bunTarget"`
	ReactTarget       string `json:"reactTarget"`
	StaticReady       bool   `json:"staticReady"`
	LegacyAssetsReady bool   `json:"legacyAssetsReady"`
	LegacyBaseURL     string `json:"legacyBaseUrl"`
}

func (a app) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(toHealthResponse(a.deps.Health.Get(r.Context())))
}

func toHealthResponse(health domainsystem.Health) healthResponse {
	return healthResponse{
		Status:            health.Status,
		Service:           health.Service,
		Environment:       health.Environment,
		Runtime:           health.Runtime,
		GoTarget:          health.Targets.Go,
		BunTarget:         health.Targets.Bun,
		ReactTarget:       health.Targets.React,
		StaticReady:       health.StaticReady,
		LegacyAssetsReady: health.LegacyAssetsReady,
		LegacyBaseURL:     health.LegacyBaseURL,
	}
}
