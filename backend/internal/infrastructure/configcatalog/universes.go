package configcatalog

import (
	"context"
	"encoding/json"
	"strings"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type UniverseCatalog struct {
	RawJSON       string
	LegacyBaseURL string
}

type universeConfig struct {
	Number     int    `json:"number"`
	Name       string `json:"name"`
	BaseURL    string `json:"baseUrl"`
	Language   string `json:"language"`
	Speed      int    `json:"speed"`
	FleetSpeed int    `json:"fleetSpeed"`
	Status     string `json:"status"`
}

func (c UniverseCatalog) ListUniverses(ctx context.Context) ([]domain.Universe, error) {
	_ = ctx

	if strings.TrimSpace(c.RawJSON) == "" {
		return []domain.Universe{{
			Number:     1,
			Name:       "Universe 1",
			BaseURL:    c.LegacyBaseURL,
			Language:   "en",
			Speed:      1,
			FleetSpeed: 1,
			Status:     domain.UniverseOnline,
		}}, nil
	}

	var configs []universeConfig
	if err := json.Unmarshal([]byte(c.RawJSON), &configs); err != nil {
		return nil, err
	}
	universes := make([]domain.Universe, 0, len(configs))
	for _, cfg := range configs {
		universes = append(universes, domain.Universe{
			Number:     cfg.Number,
			Name:       cfg.Name,
			BaseURL:    cfg.BaseURL,
			Language:   cfg.Language,
			Speed:      cfg.Speed,
			FleetSpeed: cfg.FleetSpeed,
			Status:     domain.UniverseStatus(cfg.Status),
		})
	}
	return universes, nil
}
