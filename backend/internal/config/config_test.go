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
	t.Setenv("OGAME_MASTER_DB_ENABLE", "")
	t.Setenv("OGAME_MDB_HOST", "")
	t.Setenv("OGAME_MDB_USER", "")
	t.Setenv("OGAME_MDB_PASS", "")
	t.Setenv("OGAME_MDB_NAME", "")
	t.Setenv("MYSQL_ROOT_PASSWORD", "")

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
	if !cfg.MasterDBEnabled || cfg.MasterDBHost != "mysql" || cfg.MasterDBUser != "root" || cfg.MasterDBPassword != "123" || cfg.MasterDBName != "master" {
		t.Fatalf("unexpected default master DB config: %+v", cfg)
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
	t.Setenv("OGAME_MASTER_DB_ENABLE", "0")
	t.Setenv("OGAME_MDB_HOST", "db.local:3307")
	t.Setenv("OGAME_MDB_USER", "ogame")
	t.Setenv("OGAME_MDB_PASS", "secret")
	t.Setenv("OGAME_MDB_NAME", "master_test")

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
	if cfg.MasterDBEnabled || cfg.MasterDBHost != "db.local:3307" || cfg.MasterDBUser != "ogame" || cfg.MasterDBPassword != "secret" || cfg.MasterDBName != "master_test" {
		t.Fatalf("unexpected master DB override: %+v", cfg)
	}
}

func TestLoadMasterDBPasswordFromMySQLRootPassword(t *testing.T) {
	t.Setenv("OGAME_MDB_PASS", "")
	t.Setenv("MYSQL_ROOT_PASSWORD", "root-secret")

	cfg := Load()

	if cfg.MasterDBPassword != "root-secret" {
		t.Fatalf("expected MySQL root password fallback, got %+v", cfg)
	}
}
