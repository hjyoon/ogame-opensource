package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hjyoon/ogame-opensource/backend/internal/config"
)

func TestHealthzReportsMigrationRuntime(t *testing.T) {
	staticDir := t.TempDir()
	legacyDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "<div id=\"root\"></div>")

	server := New(config.Config{
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
	if !body.StaticReady || !body.LegacyAssetsReady {
		t.Fatalf("expected ready dirs: %+v", body)
	}
}

func TestFrontendServesIndexAndSpaFallback(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	writeFile(t, filepath.Join(staticDir, "asset.txt"), "asset")

	server := New(config.Config{StaticDir: staticDir, LegacyAssetDir: t.TempDir()})

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

func writeFile(t *testing.T, name string, data string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
