package httpdelivery

import (
	"encoding/json"
	"net/http"

	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

type healthResponse struct {
	Status            string                    `json:"status"`
	Service           string                    `json:"service"`
	Environment       string                    `json:"environment"`
	Runtime           string                    `json:"runtime"`
	GoTarget          string                    `json:"goTarget"`
	BunTarget         string                    `json:"bunTarget"`
	ReactTarget       string                    `json:"reactTarget"`
	StaticReady       bool                      `json:"staticReady"`
	LegacyAssetsReady bool                      `json:"legacyAssetsReady"`
	LegacyBaseURL     string                    `json:"legacyBaseUrl"`
	MasterDBReady     bool                      `json:"masterDbReady"`
	UniverseDBReady   bool                      `json:"universeDbReady"`
	ModRuntimeReady   bool                      `json:"modRuntimeReady"`
	QueueWorker       queueWorkerHealthResponse `json:"queueWorker"`
}

type queueWorkerHealthResponse struct {
	Enabled             bool  `json:"enabled"`
	Ready               bool  `json:"ready"`
	IntervalMS          int   `json:"intervalMs"`
	LastAttemptAt       int64 `json:"lastAttemptAt"`
	LastSuccessAt       int64 `json:"lastSuccessAt"`
	LagSeconds          int64 `json:"lagSeconds"`
	ConsecutiveFailures int   `json:"consecutiveFailures"`
}

func (a app) handleHealthz(w http.ResponseWriter, r *http.Request) {
	health := a.deps.Health.Get(r.Context())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if health.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(toHealthResponse(health))
}

func (a app) handleLivez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"status":"ok","service":"ogame-go"}`))
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
		MasterDBReady:     health.MasterDBReady,
		UniverseDBReady:   health.UniverseDBReady,
		ModRuntimeReady:   health.ModRuntimeReady,
		QueueWorker: queueWorkerHealthResponse{
			Enabled:             health.QueueWorker.Enabled,
			Ready:               health.QueueWorker.Ready,
			IntervalMS:          health.QueueWorker.IntervalMS,
			LastAttemptAt:       health.QueueWorker.LastAttemptAt,
			LastSuccessAt:       health.QueueWorker.LastSuccessAt,
			LagSeconds:          health.QueueWorker.LagSeconds,
			ConsecutiveFailures: health.QueueWorker.ConsecutiveFailures,
		},
	}
}
