package config

import (
	"os"
	"strconv"
)

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
	PublicBaseURL    string
	PublicUniverses  string
	SMTPEnabled      bool
	SMTPAddr         string
	SMTPFrom         string
	MasterDBEnabled  bool
	MasterDBHost     string
	MasterDBUser     string
	MasterDBPassword string
	MasterDBName     string
	UniDBEnabled     bool
	UniDBHost        string
	UniDBUser        string
	UniDBPassword    string
	UniDBName        string
	UniDBPrefix      string
	UniDBSecret      string
	UniNumber        int
}

func Load() Config {
	legacyBaseURL := env("OGAME_LEGACY_BASE_URL", "http://localhost:8888")
	return Config{
		Addr:             env("OGAME_HTTP_ADDR", ":8080"),
		Environment:      env("OGAME_ENV", "development"),
		LogLevel:         env("OGAME_LOG_LEVEL", "info"),
		StaticDir:        env("OGAME_STATIC_DIR", "frontend/dist"),
		LegacyAssetDir:   env("OGAME_LEGACY_ASSET_DIR", "download"),
		LegacyBaseURL:    legacyBaseURL,
		PublicBaseURL:    env("OGAME_PUBLIC_BASE_URL", legacyBaseURL),
		PublicUniverses:  env("OGAME_PUBLIC_UNIVERSES", ""),
		SMTPEnabled:      envBool("OGAME_SMTP_ENABLE", false),
		SMTPAddr:         env("OGAME_SMTP_ADDR", "localhost:1025"),
		SMTPFrom:         env("OGAME_SMTP_FROM", "OGame <noreply@localhost>"),
		MasterDBEnabled:  envBool("OGAME_MASTER_DB_ENABLE", true),
		MasterDBHost:     env("OGAME_MDB_HOST", "mysql"),
		MasterDBUser:     env("OGAME_MDB_USER", "root"),
		MasterDBPassword: env("OGAME_MDB_PASS", env("MYSQL_ROOT_PASSWORD", "123")),
		MasterDBName:     env("OGAME_MDB_NAME", "master"),
		UniDBEnabled:     envBool("OGAME_UNI_DB_ENABLE", true),
		UniDBHost:        env("OGAME_UNI_DB_HOST", "mysql"),
		UniDBUser:        env("OGAME_UNI_DB_USER", "root"),
		UniDBPassword:    env("OGAME_UNI_DB_PASS", env("MYSQL_ROOT_PASSWORD", "123")),
		UniDBName:        env("OGAME_UNI_DB_NAME", "uni"),
		UniDBPrefix:      env("OGAME_UNI_DB_PREFIX", "uni1_"),
		UniDBSecret:      env("OGAME_UNI_DB_SECRET", "docker-secret"),
		UniNumber:        envInt("OGAME_UNI_NUMBER", 1),
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

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
