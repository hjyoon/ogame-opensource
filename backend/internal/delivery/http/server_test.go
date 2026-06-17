package httpdelivery

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/filesystem"
)

func TestHealthzReportsMigrationRuntime(t *testing.T) {
	staticDir := t.TempDir()
	legacyDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "<div id=\"root\"></div>")

	server := testServer(config.Config{
		Environment:    "test",
		StaticDir:      staticDir,
		LegacyAssetDir: legacyDir,
		LegacyBaseURL:  "http://legacy.local",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected security headers, got %v", rec.Header())
	}
	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.GoTarget != config.GoTarget || body.ReactTarget != config.ReactTarget || body.BunTarget != config.BunTarget {
		t.Fatalf("unexpected targets: %+v", body)
	}
	if body.Runtime != "go-test" {
		t.Fatalf("expected fake runtime, got %q", body.Runtime)
	}
	if !body.StaticReady || !body.LegacyAssetsReady {
		t.Fatalf("expected ready dirs: %+v", body)
	}
}

func TestFrontendServesIndexAndSpaFallback(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	writeFile(t, filepath.Join(staticDir, "asset.txt"), "asset")

	server := testServer(config.Config{StaticDir: staticDir, LegacyAssetDir: t.TempDir()})

	for _, target := range []string{"/", "/game/overview"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", target, rec.Code)
		}
		if rec.Body.String() != "ogame react shell" {
			t.Fatalf("%s: unexpected body %q", target, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/missing.js", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected missing assets to 404, got %d", rec.Code)
	}
}

func TestFrontendReturnsUnavailableWhenBuildMissing(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/game/overview", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing frontend build to return 503, got %d", rec.Code)
	}
}

func TestFrontendDoesNotFallbackForAPIPaths(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	server := testServer(config.Config{StaticDir: staticDir, LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected unknown API path to 404, got %d", rec.Code)
	}
}

func TestGetOnlyRejectsStateChangingMethods(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodPost, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}
}

func TestSecurityHeadersIncludeHSTSForForwardedHTTPS(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatalf("expected HSTS for forwarded HTTPS, got headers=%v", rec.Header())
	}
}

func TestLegacyAssetsDisableDirectoryListing(t *testing.T) {
	legacyDir := t.TempDir()
	writeFile(t, filepath.Join(legacyDir, "planet.jpg"), "jpeg")
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: legacyDir})

	fileReq := httptest.NewRequest(http.MethodGet, "/legacy-assets/planet.jpg", nil)
	fileRec := httptest.NewRecorder()
	server.ServeHTTP(fileRec, fileReq)
	if fileRec.Code != http.StatusOK {
		t.Fatalf("expected file 200, got %d", fileRec.Code)
	}

	dirReq := httptest.NewRequest(http.MethodGet, "/legacy-assets/", nil)
	dirRec := httptest.NewRecorder()
	server.ServeHTTP(dirRec, dirReq)
	if dirRec.Code != http.StatusNotFound {
		t.Fatalf("expected directory listing to 404, got %d", dirRec.Code)
	}
}

func TestAccessLogUsesJSON(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	server := testServerWithLogger(config.Config{StaticDir: staticDir, LegacyAssetDir: t.TempDir()}, logger)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	var event map[string]any
	if err := json.Unmarshal(logs.Bytes(), &event); err != nil {
		t.Fatalf("expected JSON access log, got %q: %v", logs.String(), err)
	}
	if event["msg"] != "http request" || event["method"] != http.MethodGet || event["path"] != "/" {
		t.Fatalf("unexpected access log event: %+v", event)
	}
}

func testServer(cfg config.Config) http.Handler {
	return testServerWithLogger(cfg, nil)
}

func testServerWithLogger(cfg config.Config, logger *slog.Logger) http.Handler {
	health := appsystem.NewHealthService(appsystem.HealthConfig{
		Environment:    cfg.Environment,
		StaticDir:      cfg.StaticDir,
		LegacyAssetDir: cfg.LegacyAssetDir,
		LegacyBaseURL:  cfg.LegacyBaseURL,
		GoTarget:       config.GoTarget,
		BunTarget:      config.BunTarget,
		ReactTarget:    config.ReactTarget,
	}, filesystem.Probe{}, fakeRuntime{})

	return New(Dependencies{
		Health:       health,
		Frontend:     filesystem.StaticDir{Root: cfg.StaticDir},
		LegacyAssets: filesystem.NewNoListingFS(cfg.LegacyAssetDir),
		Logger:       logger,
	})
}

type fakeRuntime struct{}

func (fakeRuntime) Version() string {
	return "go-test"
}

func writeFile(t *testing.T, name string, data string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
