package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OGAME_HTTP_ADDR", "")
	t.Setenv("OGAME_ENV", "")
	t.Setenv("OGAME_LOG_LEVEL", "")
	t.Setenv("OGAME_STATIC_DIR", "")
	t.Setenv("OGAME_LEGACY_ASSET_DIR", "")
	t.Setenv("OGAME_LEGACY_BASE_URL", "")
	t.Setenv("OGAME_PUBLIC_BASE_URL", "")
	t.Setenv("OGAME_PUBLIC_UNIVERSES", "")
	t.Setenv("OGAME_MCP_STATIC_TOKENS", "")
	t.Setenv("OGAME_MCP_OAUTH_REDIRECT_URIS", "")
	t.Setenv("OGAME_MCP_OIDC_ED25519_SEED_B64", "")
	t.Setenv("OGAME_MCP_OIDC_ED25519_PREVIOUS_SEEDS_B64", "")
	t.Setenv("OGAME_MCP_TOKEN_TTL_SECONDS", "")
	t.Setenv("OGAME_MCP_RATE_LIMIT_ENABLE", "")
	t.Setenv("OGAME_MCP_RATE_LIMIT_PER_MINUTE", "")
	t.Setenv("OGAME_MCP_RATE_LIMIT_BURST", "")
	t.Setenv("OGAME_SMTP_ENABLE", "")
	t.Setenv("OGAME_SMTP_ADDR", "")
	t.Setenv("OGAME_SMTP_FROM", "")
	t.Setenv("OGAME_DB_DRIVER", "")
	t.Setenv("OGAME_SQLITE_MASTER_PATH", "")
	t.Setenv("OGAME_SQLITE_UNIVERSE_PATH", "")
	t.Setenv("OGAME_SQLITE_AUTO_MIGRATE", "")
	t.Setenv("OGAME_ADMIN_EMAIL", "")
	t.Setenv("OGAME_ADMIN_PASSWORD", "")
	t.Setenv("OGAME_MASTER_DB_ENABLE", "")
	t.Setenv("OGAME_MDB_HOST", "")
	t.Setenv("OGAME_MDB_USER", "")
	t.Setenv("OGAME_MDB_PASS", "")
	t.Setenv("OGAME_MDB_NAME", "")
	t.Setenv("OGAME_UNI_DB_ENABLE", "")
	t.Setenv("OGAME_UNI_DB_HOST", "")
	t.Setenv("OGAME_UNI_DB_USER", "")
	t.Setenv("OGAME_UNI_DB_PASS", "")
	t.Setenv("OGAME_UNI_DB_NAME", "")
	t.Setenv("OGAME_UNI_DB_PREFIX", "")
	t.Setenv("OGAME_UNI_DB_SECRET", "")
	t.Setenv("OGAME_UNI_NUMBER", "")
	t.Setenv("OGAME_UNI_NUM", "")
	t.Setenv("OGAME_UNI_LANG", "")
	t.Setenv("OGAME_UNI_SPEED", "")
	t.Setenv("OGAME_UNI_FLEET_SPEED", "")
	t.Setenv("OGAME_UNI_GALAXIES", "")
	t.Setenv("OGAME_UNI_SYSTEMS", "")
	t.Setenv("OGAME_UNI_MAX_USERS", "")
	t.Setenv("OGAME_UNI_START_DM", "")
	t.Setenv("OGAME_UNI_ACS", "")
	t.Setenv("OGAME_UNI_FID", "")
	t.Setenv("OGAME_UNI_DID", "")
	t.Setenv("OGAME_UNI_RAPID", "")
	t.Setenv("OGAME_UNI_MOONS", "")
	t.Setenv("OGAME_UNI_BATTLE_ENGINE", "")
	t.Setenv("OGAME_UNI_PHP_BATTLE", "")
	t.Setenv("OGAME_UNI_BATTLE_MAX", "")
	t.Setenv("OGAME_UNI_FORCE_LANG", "")
	t.Setenv("OGAME_UNI_MAX_WERF", "")
	t.Setenv("OGAME_UNI_FEED_AGE", "")
	t.Setenv("OGAME_EXT_BOARD", "")
	t.Setenv("OGAME_EXT_DISCORD", "")
	t.Setenv("OGAME_EXT_TUTORIAL", "")
	t.Setenv("OGAME_EXT_RULES", "")
	t.Setenv("OGAME_EXT_IMPRESSUM", "")
	t.Setenv("OGAME_DB_MAX_OPEN_CONNS", "")
	t.Setenv("OGAME_DB_MAX_IDLE_CONNS", "")
	t.Setenv("OGAME_DB_CONN_MAX_LIFETIME_SECONDS", "")
	t.Setenv("OGAME_QUEUE_POLL_INTERVAL_MS", "")
	t.Setenv("MYSQL_ROOT_PASSWORD", "")

	cfg := Load()

	if cfg.Addr != ":8080" || cfg.Environment != "development" || cfg.LogLevel != "info" {
		t.Fatalf("unexpected default config: %+v", cfg)
	}
	if cfg.StaticDir != "frontend/dist" || cfg.LegacyAssetDir != "download" || cfg.LegacyBaseURL != "http://localhost:8888" || cfg.PublicBaseURL != "http://localhost:8888" {
		t.Fatalf("unexpected default paths: %+v", cfg)
	}
	if cfg.PublicUniverses != "" || cfg.MCPStaticTokens != "" || cfg.MCPOAuthRedirectURIs != "" || cfg.MCPOIDCSigningSeed != "" || cfg.MCPOIDCPreviousSeeds != "" || cfg.MCPTokenTTLSeconds != 2592000 || cfg.SMTPEnabled || cfg.SMTPAddr != "localhost:1025" || cfg.SMTPFrom != "OGame <noreply@localhost>" {
		t.Fatalf("unexpected default public universes: %+v", cfg)
	}
	if !cfg.MCPRateLimitEnabled || cfg.MCPRateLimitPerMin != 300 || cfg.MCPRateLimitBurst != 60 {
		t.Fatalf("unexpected default MCP rate limit config: %+v", cfg)
	}
	if cfg.DBDriver != "mysql" || cfg.SQLiteMasterPath != "data/ogame-master.sqlite" || cfg.SQLiteUniversePath != "data/ogame-universe.sqlite" || !cfg.SQLiteAutoMigrate || cfg.SQLiteAdminEmail != "admin@example.local" || cfg.SQLiteAdminPassword != "admin" {
		t.Fatalf("unexpected default SQLite config: %+v", cfg)
	}
	if !cfg.MasterDBEnabled || cfg.MasterDBHost != "mysql" || cfg.MasterDBUser != "root" || cfg.MasterDBPassword != "123" || cfg.MasterDBName != "master" {
		t.Fatalf("unexpected default master DB config: %+v", cfg)
	}
	if !cfg.UniDBEnabled || cfg.UniDBHost != "mysql" || cfg.UniDBUser != "root" || cfg.UniDBPassword != "123" || cfg.UniDBName != "uni" || cfg.UniDBPrefix != "uni1_" || cfg.UniDBSecret != "docker-secret" || cfg.UniNumber != 1 || !cfg.UniRapidFire {
		t.Fatalf("unexpected default universe DB config: %+v", cfg)
	}
	if cfg.UniLanguage != "en" || cfg.UniSpeed != 1 || cfg.UniFleetSpeed != 1 || cfg.UniGalaxies != 9 || cfg.UniSystems != 499 || cfg.UniMaxUsers != 12500 || cfg.UniStartDarkMatter != 0 || cfg.UniACS != 4 || cfg.UniFID != 30 || cfg.UniDID != 0 || !cfg.UniMoons || cfg.UniBattleEngine != "../cgi-bin/battle" || !cfg.UniPHPBattle || cfg.UniBattleMax != 1000000 || cfg.UniForceLanguage || cfg.UniMaxShipyard != 999 || cfg.UniFeedAge != 60 {
		t.Fatalf("unexpected default universe settings: %+v", cfg)
	}
	if cfg.ExtBoard != "" || cfg.ExtDiscord != "" || cfg.ExtTutorial != "" || cfg.ExtRules != "" || cfg.ExtImpressum != "" {
		t.Fatalf("unexpected default universe links: %+v", cfg)
	}
	if cfg.DBMaxOpenConns != 25 || cfg.DBMaxIdleConns != 5 || cfg.DBConnMaxLifetimeSec != 1800 || cfg.QueuePollIntervalMS != 1000 {
		t.Fatalf("unexpected default DB pool config: %+v", cfg)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("OGAME_HTTP_ADDR", ":9090")
	t.Setenv("OGAME_ENV", "test")
	t.Setenv("OGAME_LOG_LEVEL", "debug")
	t.Setenv("OGAME_STATIC_DIR", "/static")
	t.Setenv("OGAME_LEGACY_ASSET_DIR", "/legacy")
	t.Setenv("OGAME_LEGACY_BASE_URL", "http://legacy.local")
	t.Setenv("OGAME_PUBLIC_BASE_URL", "http://public.local")
	t.Setenv("OGAME_PUBLIC_UNIVERSES", `[{"number":1}]`)
	t.Setenv("OGAME_MCP_STATIC_TOKENS", "token:42:mcp:read")
	t.Setenv("OGAME_MCP_OAUTH_REDIRECT_URIS", "https://client.example/callback")
	t.Setenv("OGAME_MCP_OIDC_ED25519_SEED_B64", "MTIz")
	t.Setenv("OGAME_MCP_OIDC_ED25519_PREVIOUS_SEEDS_B64", "YWJj")
	t.Setenv("OGAME_MCP_TOKEN_TTL_SECONDS", "3600")
	t.Setenv("OGAME_MCP_RATE_LIMIT_ENABLE", "0")
	t.Setenv("OGAME_MCP_RATE_LIMIT_PER_MINUTE", "120")
	t.Setenv("OGAME_MCP_RATE_LIMIT_BURST", "20")
	t.Setenv("OGAME_SMTP_ENABLE", "1")
	t.Setenv("OGAME_SMTP_ADDR", "mailhog:1025")
	t.Setenv("OGAME_SMTP_FROM", "No Reply <no-reply@example.local>")
	t.Setenv("OGAME_DB_DRIVER", "sqlite")
	t.Setenv("OGAME_SQLITE_MASTER_PATH", "/data/master.db")
	t.Setenv("OGAME_SQLITE_UNIVERSE_PATH", "/data/universe.db")
	t.Setenv("OGAME_SQLITE_AUTO_MIGRATE", "0")
	t.Setenv("OGAME_ADMIN_EMAIL", "operator@example.local")
	t.Setenv("OGAME_ADMIN_PASSWORD", "sqlite-secret")
	t.Setenv("OGAME_MASTER_DB_ENABLE", "0")
	t.Setenv("OGAME_MDB_HOST", "db.local:3307")
	t.Setenv("OGAME_MDB_USER", "ogame")
	t.Setenv("OGAME_MDB_PASS", "secret")
	t.Setenv("OGAME_MDB_NAME", "master_test")
	t.Setenv("OGAME_UNI_DB_ENABLE", "0")
	t.Setenv("OGAME_UNI_DB_HOST", "uni-db.local:3307")
	t.Setenv("OGAME_UNI_DB_USER", "uni")
	t.Setenv("OGAME_UNI_DB_PASS", "uni-secret")
	t.Setenv("OGAME_UNI_DB_NAME", "uni_test")
	t.Setenv("OGAME_UNI_DB_PREFIX", "u2_")
	t.Setenv("OGAME_UNI_DB_SECRET", "legacy-secret")
	t.Setenv("OGAME_UNI_NUMBER", "2")
	t.Setenv("OGAME_UNI_LANG", "ko")
	t.Setenv("OGAME_UNI_SPEED", "256")
	t.Setenv("OGAME_UNI_FLEET_SPEED", "128")
	t.Setenv("OGAME_UNI_GALAXIES", "5")
	t.Setenv("OGAME_UNI_SYSTEMS", "200")
	t.Setenv("OGAME_UNI_MAX_USERS", "5000")
	t.Setenv("OGAME_UNI_START_DM", "8000")
	t.Setenv("OGAME_UNI_ACS", "8")
	t.Setenv("OGAME_UNI_FID", "40")
	t.Setenv("OGAME_UNI_DID", "20")
	t.Setenv("OGAME_UNI_RAPID", "0")
	t.Setenv("OGAME_UNI_MOONS", "0")
	t.Setenv("OGAME_UNI_BATTLE_ENGINE", "/opt/battle")
	t.Setenv("OGAME_UNI_PHP_BATTLE", "0")
	t.Setenv("OGAME_UNI_BATTLE_MAX", "2000000")
	t.Setenv("OGAME_UNI_FORCE_LANG", "1")
	t.Setenv("OGAME_UNI_MAX_WERF", "500")
	t.Setenv("OGAME_UNI_FEED_AGE", "30")
	t.Setenv("OGAME_EXT_BOARD", "https://board.example")
	t.Setenv("OGAME_EXT_DISCORD", "https://discord.example")
	t.Setenv("OGAME_EXT_TUTORIAL", "https://tutorial.example")
	t.Setenv("OGAME_EXT_RULES", "https://rules.example")
	t.Setenv("OGAME_EXT_IMPRESSUM", "https://legal.example")
	t.Setenv("OGAME_DB_MAX_OPEN_CONNS", "40")
	t.Setenv("OGAME_DB_MAX_IDLE_CONNS", "8")
	t.Setenv("OGAME_DB_CONN_MAX_LIFETIME_SECONDS", "900")
	t.Setenv("OGAME_QUEUE_POLL_INTERVAL_MS", "250")

	cfg := Load()

	if cfg.Addr != ":9090" || cfg.Environment != "test" || cfg.LogLevel != "debug" {
		t.Fatalf("unexpected override config: %+v", cfg)
	}
	if cfg.StaticDir != "/static" || cfg.LegacyAssetDir != "/legacy" || cfg.LegacyBaseURL != "http://legacy.local" || cfg.PublicBaseURL != "http://public.local" {
		t.Fatalf("unexpected override paths: %+v", cfg)
	}
	if cfg.PublicUniverses != `[{"number":1}]` || cfg.MCPStaticTokens != "token:42:mcp:read" || cfg.MCPOAuthRedirectURIs != "https://client.example/callback" || cfg.MCPOIDCSigningSeed != "MTIz" || cfg.MCPOIDCPreviousSeeds != "YWJj" || cfg.MCPTokenTTLSeconds != 3600 || !cfg.SMTPEnabled || cfg.SMTPAddr != "mailhog:1025" || cfg.SMTPFrom != "No Reply <no-reply@example.local>" {
		t.Fatalf("unexpected public universes override: %+v", cfg)
	}
	if cfg.MCPRateLimitEnabled || cfg.MCPRateLimitPerMin != 120 || cfg.MCPRateLimitBurst != 20 {
		t.Fatalf("unexpected MCP rate limit override: %+v", cfg)
	}
	if cfg.DBDriver != "sqlite" || cfg.SQLiteMasterPath != "/data/master.db" || cfg.SQLiteUniversePath != "/data/universe.db" || cfg.SQLiteAutoMigrate || cfg.SQLiteAdminEmail != "operator@example.local" || cfg.SQLiteAdminPassword != "sqlite-secret" {
		t.Fatalf("unexpected SQLite override: %+v", cfg)
	}
	if cfg.MasterDBEnabled || cfg.MasterDBHost != "db.local:3307" || cfg.MasterDBUser != "ogame" || cfg.MasterDBPassword != "secret" || cfg.MasterDBName != "master_test" {
		t.Fatalf("unexpected master DB override: %+v", cfg)
	}
	if cfg.UniDBEnabled || cfg.UniDBHost != "uni-db.local:3307" || cfg.UniDBUser != "uni" || cfg.UniDBPassword != "uni-secret" || cfg.UniDBName != "uni_test" || cfg.UniDBPrefix != "u2_" || cfg.UniDBSecret != "legacy-secret" || cfg.UniNumber != 2 || cfg.UniRapidFire {
		t.Fatalf("unexpected universe DB override: %+v", cfg)
	}
	if cfg.UniLanguage != "ko" || cfg.UniSpeed != 256 || cfg.UniFleetSpeed != 128 || cfg.UniGalaxies != 5 || cfg.UniSystems != 200 || cfg.UniMaxUsers != 5000 || cfg.UniStartDarkMatter != 8000 || cfg.UniACS != 8 || cfg.UniFID != 40 || cfg.UniDID != 20 || cfg.UniMoons || cfg.UniBattleEngine != "/opt/battle" || cfg.UniPHPBattle || cfg.UniBattleMax != 2000000 || !cfg.UniForceLanguage || cfg.UniMaxShipyard != 500 || cfg.UniFeedAge != 30 {
		t.Fatalf("unexpected universe settings override: %+v", cfg)
	}
	if cfg.ExtBoard != "https://board.example" || cfg.ExtDiscord != "https://discord.example" || cfg.ExtTutorial != "https://tutorial.example" || cfg.ExtRules != "https://rules.example" || cfg.ExtImpressum != "https://legal.example" {
		t.Fatalf("unexpected universe links override: %+v", cfg)
	}
	if cfg.DBMaxOpenConns != 40 || cfg.DBMaxIdleConns != 8 || cfg.DBConnMaxLifetimeSec != 900 || cfg.QueuePollIntervalMS != 250 {
		t.Fatalf("unexpected DB pool override: %+v", cfg)
	}
}

func TestLoadInvalidUniverseNumberFallsBack(t *testing.T) {
	t.Setenv("OGAME_UNI_NUM", "")
	t.Setenv("OGAME_UNI_NUMBER", "invalid")

	cfg := Load()

	if cfg.UniNumber != 1 {
		t.Fatalf("expected default universe number, got %+v", cfg)
	}
}

func TestLoadLegacyUniverseNumberFallback(t *testing.T) {
	t.Setenv("OGAME_UNI_NUMBER", "")
	t.Setenv("OGAME_UNI_NUM", "3")

	cfg := Load()

	if cfg.UniNumber != 3 {
		t.Fatalf("expected legacy universe number fallback, got %+v", cfg)
	}
}

func TestLoadMasterDBPasswordFromMySQLRootPassword(t *testing.T) {
	t.Setenv("OGAME_MDB_PASS", "")
	t.Setenv("MYSQL_ROOT_PASSWORD", "root-secret")

	cfg := Load()

	if cfg.MasterDBPassword != "root-secret" {
		t.Fatalf("expected MySQL root password fallback, got %+v", cfg)
	}
	if cfg.UniDBPassword != "root-secret" {
		t.Fatalf("expected universe DB password to use MySQL root password fallback, got %+v", cfg)
	}
}
