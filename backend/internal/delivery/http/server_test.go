package httpdelivery

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/configcatalog"
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

func TestUniversesEndpointReturnsCatalog(t *testing.T) {
	server := testServer(config.Config{
		StaticDir:       t.TempDir(),
		LegacyAssetDir:  t.TempDir(),
		LegacyBaseURL:   "http://legacy.local",
		PublicUniverses: `[{"number":2,"name":"Beta","baseUrl":"http://beta.local","language":"en","speed":128,"fleetSpeed":64,"status":"online"}]`,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/public/universes", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Universes []universeResponse `json:"universes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Universes) != 1 || body.Universes[0].Number != 2 || !body.Universes[0].Open {
		t.Fatalf("unexpected universe response: %+v", body)
	}
}

func TestUniversesEndpointReturnsUnavailableForInvalidCatalog(t *testing.T) {
	server := testServer(config.Config{
		StaticDir:       t.TempDir(),
		LegacyAssetDir:  t.TempDir(),
		PublicUniverses: `[{"number":0}]`,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/public/universes", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestRegistrationValidationEndpointAcceptsValidDraft(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	body := `{"character":"Commander01","password":"E2E_http123","email":"commander@example.local","universe":"http://localhost:8888","agb":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response registrationValidationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Valid || len(response.Issues) != 0 {
		t.Fatalf("expected valid registration response, got %+v", response)
	}
	if response.Draft.Character != "Commander01" || response.Draft.AGB != true {
		t.Fatalf("unexpected sanitized draft response: %+v", response.Draft)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("E2E_http123")) {
		t.Fatalf("registration response must not echo password: %s", rec.Body.String())
	}
}

func TestRegistrationValidationEndpointReturnsIssues(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	body := `{"character":"ad","password":"short","email":"invalid","universe":"","agb":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected validation result to return 200, got %d", rec.Code)
	}
	var response registrationValidationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Valid {
		t.Fatalf("expected invalid registration response, got %+v", response)
	}
	if !hasRegistrationIssue(response, "character_invalid", 103) || !hasRegistrationIssue(response, "password_too_short", 107) {
		t.Fatalf("expected legacy-compatible issues, got %+v", response.Issues)
	}
}

func TestRegistrationValidationRejectsMalformedJSON(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration/validate", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected malformed JSON to return 400, got %d", rec.Code)
	}
}

func TestLegacyPublicHTMLRoutesServeReactShell(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	server := testServer(config.Config{StaticDir: staticDir, LegacyAssetDir: t.TempDir()})

	for _, target := range []string{
		"/about.php",
		"/home.php",
		"/impressum.php",
		"/index.php",
		"/install.php",
		"/register.php",
		"/regeln.php",
		"/screenshots.php",
		"/story.php",
		"/unis.php",
	} {
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

func TestUnknownPHPRouteDoesNotFallbackToReactShell(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	server := testServer(config.Config{StaticDir: staticDir, LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/unknown.php", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected unknown PHP path to 404, got %d", rec.Code)
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

func TestPostOnlyRejectsReadMethods(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/public/registration/validate", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
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
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{
		RawJSON:       cfg.PublicUniverses,
		LegacyBaseURL: cfg.LegacyBaseURL,
	})
	registrationDrafts := apppublicsite.NewRegistrationDraftValidator()

	return New(Dependencies{
		Health:             health,
		Universes:          universes,
		RegistrationDrafts: registrationDrafts,
		Frontend:           filesystem.StaticDir{Root: cfg.StaticDir},
		LegacyAssets:       filesystem.NewNoListingFS(cfg.LegacyAssetDir),
		Logger:             logger,
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

func hasRegistrationIssue(response registrationValidationResponse, code string, legacyCode int) bool {
	for _, issue := range response.Issues {
		if issue.Code == code && issue.LegacyErrorCode == legacyCode {
			return true
		}
	}
	return false
}
