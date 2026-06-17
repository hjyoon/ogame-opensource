package config

import "os"

const (
	GoTarget    = "1.25"
	BunTarget   = "1.3"
	ReactTarget = "19"
)

type Config struct {
	Addr             string
	Environment      string
	LogLevel         string
	StaticDir        string
	LegacyAssetDir   string
	LegacyBaseURL    string
	PublicUniverses  string
	MasterDBEnabled  bool
	MasterDBHost     string
	MasterDBUser     string
	MasterDBPassword string
	MasterDBName     string
}

func Load() Config {
	return Config{
		Addr:             env("OGAME_HTTP_ADDR", ":8080"),
		Environment:      env("OGAME_ENV", "development"),
		LogLevel:         env("OGAME_LOG_LEVEL", "info"),
		StaticDir:        env("OGAME_STATIC_DIR", "frontend/dist"),
		LegacyAssetDir:   env("OGAME_LEGACY_ASSET_DIR", "download"),
		LegacyBaseURL:    env("OGAME_LEGACY_BASE_URL", "http://localhost:8888"),
		PublicUniverses:  env("OGAME_PUBLIC_UNIVERSES", ""),
		MasterDBEnabled:  envBool("OGAME_MASTER_DB_ENABLE", true),
		MasterDBHost:     env("OGAME_MDB_HOST", "mysql"),
		MasterDBUser:     env("OGAME_MDB_USER", "root"),
		MasterDBPassword: env("OGAME_MDB_PASS", env("MYSQL_ROOT_PASSWORD", "123")),
		MasterDBName:     env("OGAME_MDB_NAME", "master"),
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "TRUE" || value == "on" || value == "ON"
}
