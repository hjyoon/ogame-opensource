package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OGAME_HTTP_ADDR", "")
	t.Setenv("OGAME_ENV", "")
	t.Setenv("OGAME_LOG_LEVEL", "")
	t.Setenv("OGAME_STATIC_DIR", "")
	t.Setenv("OGAME_LEGACY_ASSET_DIR", "")
	t.Setenv("OGAME_LEGACY_BASE_URL", "")
	t.Setenv("OGAME_PUBLIC_UNIVERSES", "")

	cfg := Load()

	if cfg.Addr != ":8080" || cfg.Environment != "development" || cfg.LogLevel != "info" {
		t.Fatalf("unexpected default config: %+v", cfg)
	}
	if cfg.StaticDir != "frontend/dist" || cfg.LegacyAssetDir != "download" || cfg.LegacyBaseURL != "http://localhost:8888" {
		t.Fatalf("unexpected default paths: %+v", cfg)
	}
	if cfg.PublicUniverses != "" {
		t.Fatalf("unexpected default public universes: %+v", cfg)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("OGAME_HTTP_ADDR", ":9090")
	t.Setenv("OGAME_ENV", "test")
	t.Setenv("OGAME_LOG_LEVEL", "debug")
	t.Setenv("OGAME_STATIC_DIR", "/static")
	t.Setenv("OGAME_LEGACY_ASSET_DIR", "/legacy")
	t.Setenv("OGAME_LEGACY_BASE_URL", "http://legacy.local")
	t.Setenv("OGAME_PUBLIC_UNIVERSES", `[{"number":1}]`)

	cfg := Load()

	if cfg.Addr != ":9090" || cfg.Environment != "test" || cfg.LogLevel != "debug" {
		t.Fatalf("unexpected override config: %+v", cfg)
	}
	if cfg.StaticDir != "/static" || cfg.LegacyAssetDir != "/legacy" || cfg.LegacyBaseURL != "http://legacy.local" {
		t.Fatalf("unexpected override paths: %+v", cfg)
	}
	if cfg.PublicUniverses != `[{"number":1}]` {
		t.Fatalf("unexpected public universes override: %+v", cfg)
	}
}
