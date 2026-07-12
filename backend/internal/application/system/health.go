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

type ReadinessProbe interface {
	Ready(context.Context) bool
}

type HealthConfig struct {
	Environment        string
	StaticDir          string
	LegacyAssetDir     string
	LegacyBaseURL      string
	GoTarget           string
	BunTarget          string
	ReactTarget        string
	MasterDBRequired   bool
	UniverseDBRequired bool
}

type HealthService struct {
	cfg        HealthConfig
	assets     AssetProbe
	runtime    RuntimeProvider
	masterDB   ReadinessProbe
	universeDB ReadinessProbe
}

func NewHealthService(cfg HealthConfig, assets AssetProbe, runtime RuntimeProvider, probes ...ReadinessProbe) HealthService {
	var masterDB, universeDB ReadinessProbe
	if len(probes) > 0 {
		masterDB = probes[0]
	}
	if len(probes) > 1 {
		universeDB = probes[1]
	}
	return HealthService{
		cfg:        cfg,
		assets:     assets,
		runtime:    runtime,
		masterDB:   masterDB,
		universeDB: universeDB,
	}
}

func (s HealthService) Get(ctx context.Context) domainsystem.Health {
	staticReady := s.assets.Ready(s.cfg.StaticDir)
	legacyReady := s.assets.Ready(s.cfg.LegacyAssetDir)
	masterReady := !s.cfg.MasterDBRequired || s.masterDB != nil && s.masterDB.Ready(ctx)
	universeReady := !s.cfg.UniverseDBRequired || s.universeDB != nil && s.universeDB.Ready(ctx)
	status := "ok"
	if !staticReady || !legacyReady || !masterReady || !universeReady {
		status = "unavailable"
	}

	return domainsystem.Health{
		Status:      status,
		Service:     "ogame-go",
		Environment: s.cfg.Environment,
		Runtime:     s.runtime.Version(),
		Targets: domainsystem.RuntimeTargets{
			Go:    s.cfg.GoTarget,
			Bun:   s.cfg.BunTarget,
			React: s.cfg.ReactTarget,
		},
		StaticReady:       staticReady,
		LegacyAssetsReady: legacyReady,
		LegacyBaseURL:     s.cfg.LegacyBaseURL,
		MasterDBReady:     masterReady,
		UniverseDBReady:   universeReady,
	}
}
