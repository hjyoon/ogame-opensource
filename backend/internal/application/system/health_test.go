package system

import (
	"context"
	"testing"
)

func TestHealthServiceBuildsDomainHealth(t *testing.T) {
	service := NewHealthService(HealthConfig{
		Environment:    "test",
		StaticDir:      "/static",
		LegacyAssetDir: "/legacy",
		LegacyBaseURL:  "http://legacy.local",
		GoTarget:       "1.25",
		BunTarget:      "1.3",
		ReactTarget:    "19",
	}, fakeProbe{ready: map[string]bool{"/static": true}}, fakeRuntime{version: "go-test"})

	health := service.Get(context.Background())

	if health.Status != "unavailable" || health.Service != "ogame-go" || health.Environment != "test" {
		t.Fatalf("unexpected identity fields: %+v", health)
	}
	if health.Runtime != "go-test" || health.Targets.Go != "1.25" || health.Targets.Bun != "1.3" || health.Targets.React != "19" {
		t.Fatalf("unexpected runtime fields: %+v", health)
	}
	if !health.StaticReady || health.LegacyAssetsReady {
		t.Fatalf("unexpected readiness: %+v", health)
	}
	if health.LegacyBaseURL != "http://legacy.local" {
		t.Fatalf("unexpected legacy base URL: %q", health.LegacyBaseURL)
	}
}

func TestHealthServiceReportsDatabaseReadiness(t *testing.T) {
	service := NewHealthService(HealthConfig{
		StaticDir:          "/static",
		LegacyAssetDir:     "/legacy",
		MasterDBRequired:   true,
		UniverseDBRequired: true,
	}, fakeProbe{ready: map[string]bool{"/static": true, "/legacy": true}}, fakeRuntime{}, fakeReadiness{ready: true}, fakeReadiness{})

	health := service.Get(context.Background())
	if health.Status != "unavailable" || !health.MasterDBReady || health.UniverseDBReady {
		t.Fatalf("unexpected database readiness: %+v", health)
	}
}

type fakeProbe struct {
	ready map[string]bool
}

func (f fakeProbe) Ready(path string) bool {
	return f.ready[path]
}

type fakeRuntime struct {
	version string
}

type fakeReadiness struct {
	ready bool
}

func (f fakeReadiness) Ready(context.Context) bool {
	return f.ready
}

func (f fakeRuntime) Version() string {
	return f.version
}
