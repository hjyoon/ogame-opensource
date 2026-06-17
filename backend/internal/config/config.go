package config

import "os"

const (
	GoTarget    = "1.25"
	BunTarget   = "1.3"
	ReactTarget = "19"
)

type Config struct {
	Addr           string
	Environment    string
	StaticDir      string
	LegacyAssetDir string
	LegacyBaseURL  string
}

func Load() Config {
	return Config{
		Addr:           env("OGAME_HTTP_ADDR", ":8080"),
		Environment:    env("OGAME_ENV", "development"),
		StaticDir:      env("OGAME_STATIC_DIR", "frontend/dist"),
		LegacyAssetDir: env("OGAME_LEGACY_ASSET_DIR", "download"),
		LegacyBaseURL:  env("OGAME_LEGACY_BASE_URL", "http://localhost:8888"),
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
