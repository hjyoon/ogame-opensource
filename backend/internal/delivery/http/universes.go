package httpdelivery

import (
	"encoding/json"
	"net/http"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type universeResponse struct {
	Number     int    `json:"number"`
	Name       string `json:"name"`
	BaseURL    string `json:"baseUrl"`
	Language   string `json:"language"`
	Speed      int    `json:"speed"`
	FleetSpeed int    `json:"fleetSpeed"`
	Status     string `json:"status"`
	Open       bool   `json:"open"`
}

func (a app) handleUniverses(w http.ResponseWriter, r *http.Request) {
	universes, err := a.deps.Universes.ListUniverses(r.Context())
	if err != nil {
		http.Error(w, "universe catalog unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string][]universeResponse{
		"universes": toUniverseResponses(universes),
	})
}

func toUniverseResponses(universes []domain.Universe) []universeResponse {
	responses := make([]universeResponse, 0, len(universes))
	for _, universe := range universes {
		responses = append(responses, universeResponse{
			Number:     universe.Number,
			Name:       universe.Name,
			BaseURL:    universe.BaseURL,
			Language:   universe.Language,
			Speed:      universe.Speed,
			FleetSpeed: universe.FleetSpeed,
			Status:     string(universe.Status),
			Open:       universe.Open(),
		})
	}
	return responses
}
