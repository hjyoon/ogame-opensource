package system

import (
	"context"

	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

type AssetProbe interface {
	Ready(path string) bool
}

type RuntimeProvider interface {
	Version() string
}

type HealthConfig struct {
	Environment    string
	StaticDir      string
	LegacyAssetDir string
	LegacyBaseURL  string
	GoTarget       string
	BunTarget      string
	ReactTarget    string
}

type HealthService struct {
	cfg     HealthConfig
	assets  AssetProbe
	runtime RuntimeProvider
}

func NewHealthService(cfg HealthConfig, assets AssetProbe, runtime RuntimeProvider) HealthService {
	return HealthService{
		cfg:     cfg,
		assets:  assets,
		runtime: runtime,
	}
}

func (s HealthService) Get(ctx context.Context) domainsystem.Health {
	_ = ctx

	return domainsystem.Health{
		Status:      "ok",
		Service:     "ogame-go",
		Environment: s.cfg.Environment,
		Runtime:     s.runtime.Version(),
		Targets: domainsystem.RuntimeTargets{
			Go:    s.cfg.GoTarget,
			Bun:   s.cfg.BunTarget,
			React: s.cfg.ReactTarget,
		},
		StaticReady:       s.assets.Ready(s.cfg.StaticDir),
		LegacyAssetsReady: s.assets.Ready(s.cfg.LegacyAssetDir),
		LegacyBaseURL:     s.cfg.LegacyBaseURL,
	}
}
