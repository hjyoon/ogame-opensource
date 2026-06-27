package httpdelivery

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
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

func TestLegacyAssetAliasesServeStaticFiles(t *testing.T) {
	staticDir := t.TempDir()
	legacyDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "public-assets", "evolution"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(staticDir, "public-assets", "game", "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	writeFile(t, filepath.Join(staticDir, "public-assets", "evolution", "formate.css"), "body{background:#000}")
	writeFile(t, filepath.Join(staticDir, "public-assets", "game", "css", "default.css"), "th{color:#fff}")
	server := testServer(config.Config{StaticDir: staticDir, LegacyAssetDir: legacyDir})

	for _, target := range []string{"/evolution/formate.css", "/game/css/default.css"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "ogame react shell") {
			t.Fatalf("%s: legacy asset alias fell through to SPA shell", target)
		}
		if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
			t.Fatalf("%s: expected css content type, got %q", target, contentType)
		}
	}
}

func TestLegacyCronScriptIsForbidden(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	server := testServer(config.Config{StaticDir: staticDir, LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/game/cron.php", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<?php") || strings.Contains(body, "ogame react shell") {
		t.Fatalf("cron response leaked source or SPA shell: %q", body)
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

func TestRegistrationValidationReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithRegistrationDrafts(t, failingRegistrationDrafts{err: errors.New("availability failed")})
	body := `{"character":"Commander01","password":"E2E_http123","email":"commander@example.local","universe":"http://localhost:8888","agb":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected use case error to return 503, got %d", rec.Code)
	}
}

func TestRegistrationEndpointCreatesSessionCookie(t *testing.T) {
	registration := &fakeRegistration{
		result: domainpublicsite.RegistrationCreation{
			Valid: true,
			Account: domainpublicsite.RegisteredAccount{
				PlayerID:       42,
				HomePlanetID:   99,
				ActivationCode: "activation-secret",
				Validated:      false,
			},
			Session: domainpublicsite.LoginSession{
				PlayerID:       42,
				PublicID:       "public123456",
				PrivateID:      "private1234567890private1234567890",
				UniverseNumber: 1,
				LastLogin:      1700000000,
				RedirectPath:   "/game/overview",
			},
		},
	}
	server := testServerWithRegistration(t, registration)
	body := `{"character":"Commander01","password":"E2E_http123","email":"commander@example.local","universe":"http://localhost:8888","agb":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.10:4321"
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response registrationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Valid || !response.Created || response.Account == nil || response.Account.PlayerID != 42 || !response.Account.ActivationRequired {
		t.Fatalf("unexpected registration response: %+v", response)
	}
	if response.Session == nil || !strings.Contains(response.Session.RedirectTo, "/game/overview?") || !strings.Contains(response.Session.RedirectTo, "session=public123456") {
		t.Fatalf("unexpected session response: %+v", response.Session)
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), "prsess_42_1=private1234567890private1234567890") {
		t.Fatalf("expected private session cookie, got %q", rec.Header().Get("Set-Cookie"))
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("E2E_http123")) || bytes.Contains(rec.Body.Bytes(), []byte("activation-secret")) {
		t.Fatalf("registration response must not echo password or activation code: %s", rec.Body.String())
	}
	if registration.command.RemoteAddr != "203.0.113.10" || !registration.command.TermsAccepted {
		t.Fatalf("unexpected registration command: %+v", registration.command)
	}
}

func TestRegistrationEndpointReturnsIssuesWithoutCookie(t *testing.T) {
	server := testServerWithRegistration(t, &fakeRegistration{
		result: domainpublicsite.RegistrationCreation{
			Valid: false,
			Issues: []domainpublicsite.RegistrationIssue{{
				Field:           "email",
				Code:            domainpublicsite.RegistrationIssueEmailInvalid,
				Message:         "Email address is invalid.",
				LegacyErrorCode: domainpublicsite.LegacyRegistrationErrorEmail,
			}},
		},
	})
	body := `{"character":"Commander01","password":"E2E_http123","email":"invalid","universe":"http://localhost:8888","agb":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected validation result to return 200, got %d", rec.Code)
	}
	var response registrationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Valid || response.Created || response.Session != nil || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("expected invalid registration without session, got response=%+v cookie=%q", response, rec.Header().Get("Set-Cookie"))
	}
	if len(response.Issues) != 1 || response.Issues[0].LegacyErrorCode != 104 {
		t.Fatalf("unexpected registration issues: %+v", response.Issues)
	}
}

func TestRegistrationEndpointRejectsMalformedJSON(t *testing.T) {
	server := testServerWithRegistration(t, &fakeRegistration{})
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected malformed JSON to return 400, got %d", rec.Code)
	}
}

func TestRegistrationEndpointReturnsUnavailable(t *testing.T) {
	server := testServerWithRegistration(t, &fakeRegistration{err: errors.New("registration failed")})
	body := `{"character":"Commander01","password":"E2E_http123","email":"commander@example.local","universe":"http://localhost:8888","agb":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected use case error to return 503, got %d", rec.Code)
	}
}

func TestRegistrationEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	body := `{"character":"Commander01","password":"E2E_http123","email":"commander@example.local","universe":"http://localhost:8888","agb":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/registration", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing use case to return 503, got %d", rec.Code)
	}
}

func TestRegistrationActivationRedirectsWithSessionCookie(t *testing.T) {
	activation := &fakeActivation{
		result: domainpublicsite.RegistrationActivation{
			Activated: true,
			Account:   domainpublicsite.ActivatedAccount{Found: true, PlayerID: 42},
			Session: domainpublicsite.LoginSession{
				PlayerID:       42,
				PublicID:       "public123456",
				PrivateID:      "private1234567890private1234567890",
				UniverseNumber: 1,
				LastLogin:      1700000000,
				RedirectPath:   "/game/overview",
			},
		},
	}
	server := testServerWithActivation(t, activation)
	req := httptest.NewRequest(http.MethodGet, "/game/validate.php?ack=activation-secret", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected activation redirect, got %d", rec.Code)
	}
	if activation.command.ActivationCode != "activation-secret" || activation.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected activation command: %+v", activation.command)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/game/overview?") || !strings.Contains(rec.Header().Get("Location"), "session=public123456") {
		t.Fatalf("unexpected activation location: %q", rec.Header().Get("Location"))
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), "prsess_42_1=private1234567890private1234567890") {
		t.Fatalf("expected private session cookie, got %q", rec.Header().Get("Set-Cookie"))
	}
}

func TestRegistrationActivationRedirectsHomeForMissingCodeOrAccount(t *testing.T) {
	cases := map[string]struct {
		path       string
		activation *fakeActivation
		called     bool
	}{
		"blank": {
			path:       "/game/validate.php",
			activation: &fakeActivation{},
		},
		"not found": {
			path:       "/activation?ack=missing",
			activation: &fakeActivation{result: domainpublicsite.RegistrationActivation{Account: domainpublicsite.ActivatedAccount{Found: false}}},
			called:     true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := testServerWithActivation(t, tc.activation)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/home" {
				t.Fatalf("expected home redirect, got code=%d location=%q", rec.Code, rec.Header().Get("Location"))
			}
			if (tc.activation.command.ActivationCode != "") != tc.called {
				t.Fatalf("unexpected activation call state: %+v", tc.activation.command)
			}
		})
	}
}

func TestRegistrationActivationReturnsUnavailable(t *testing.T) {
	server := testServerWithActivation(t, &fakeActivation{err: errors.New("activation failed")})
	req := httptest.NewRequest(http.MethodGet, "/game/validate.php?ack=activation-secret", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected activation error to return 503, got %d", rec.Code)
	}

	server = testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req = httptest.NewRequest(http.MethodGet, "/game/validate.php?ack=activation-secret", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing activation use case to return 503, got %d", rec.Code)
	}
}

func TestRemoteIPAcceptsAddressWithoutPort(t *testing.T) {
	if got := remoteIP("203.0.113.10"); got != "203.0.113.10" {
		t.Fatalf("unexpected remote IP: %q", got)
	}
}

func TestLoginValidationEndpointAcceptsValidDraft(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	body := `{"login":"Commander01","pass":"E2E_http123","universe":"http://localhost:8888"}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/login/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response loginValidationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Valid || len(response.Issues) != 0 {
		t.Fatalf("expected valid login response, got %+v", response)
	}
	if response.Draft.Login != "Commander01" {
		t.Fatalf("unexpected sanitized login draft response: %+v", response.Draft)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("E2E_http123")) {
		t.Fatalf("login response must not echo password: %s", rec.Body.String())
	}
}

func TestLoginValidationEndpointAcceptsPasswordAlias(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	body := `{"login":"Commander01","password":"E2E_http123","universe":"http://localhost:8888"}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/login/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response loginValidationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Valid {
		t.Fatalf("expected password alias to produce valid login response, got %+v", response)
	}
}

func TestLoginValidationEndpointReturnsIssues(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	body := `{"login":"","pass":"","universe":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/login/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected validation result to return 200, got %d", rec.Code)
	}
	var response loginValidationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Valid {
		t.Fatalf("expected invalid login response, got %+v", response)
	}
	if !hasLoginIssue(response, "login_required", 2) || !hasLoginIssue(response, "password_required", 2) {
		t.Fatalf("expected legacy-compatible login issues, got %+v", response.Issues)
	}
}

func TestLoginValidationRejectsMalformedJSON(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/public/login/validate", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected malformed JSON to return 400, got %d", rec.Code)
	}
}

func TestLoginValidationReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithLoginDrafts(t, failingLoginDrafts{err: errors.New("login failed")})
	body := `{"login":"Commander01","pass":"E2E_http123","universe":"http://localhost:8888"}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/login/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected use case error to return 503, got %d", rec.Code)
	}
}

func TestLoginEndpointCreatesSessionCookie(t *testing.T) {
	login := &fakeLogin{
		result: domainpublicsite.LoginAuthentication{
			Valid: true,
			Session: domainpublicsite.LoginSession{
				PlayerID:       42,
				PublicID:       "public123456",
				PrivateID:      "private1234567890private1234567890",
				UniverseNumber: 1,
				LastLogin:      1700000000,
				RedirectPath:   "/game/overview",
			},
		},
	}
	server := testServerWithLogin(t, login)
	body := `{"login":"legor","pass":"admin","universe":"http://localhost:8888"}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/login", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.10:4321"
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Valid || response.Session == nil || response.Session.RedirectTo != "/game/overview?lgn=1&session=public123456" {
		t.Fatalf("expected valid login response, got %+v", response)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one private session cookie, got %+v", cookies)
	}
	if cookies[0].Name != "prsess_42_1" || cookies[0].Value != "private1234567890private1234567890" || !cookies[0].HttpOnly {
		t.Fatalf("unexpected private session cookie: %+v", cookies[0])
	}
	if login.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("expected normalized remote IP, got %+v", login.command)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("admin")) || bytes.Contains(rec.Body.Bytes(), []byte("private1234567890private1234567890")) {
		t.Fatalf("login response must not echo password or private session: %s", rec.Body.String())
	}
}

func TestLoginEndpointReturnsIssuesWithoutCookie(t *testing.T) {
	login := &fakeLogin{
		result: domainpublicsite.LoginAuthentication{
			Valid: false,
			Issues: []domainpublicsite.LoginIssue{{
				Field:           "login",
				Code:            "credentials_invalid",
				Message:         "Commander name or password is invalid.",
				LegacyErrorCode: 2,
			}},
		},
	}
	server := testServerWithLogin(t, login)
	body := `{"login":"legor","pass":"wrong","universe":"http://localhost:8888"}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected validation result to return 200, got %d", rec.Code)
	}
	var response loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Valid || !hasLoginIssue(loginValidationResponse{Issues: response.Issues}, "credentials_invalid", 2) {
		t.Fatalf("expected credential issue response, got %+v", response)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("expected no session cookie for invalid login, got %+v", rec.Result().Cookies())
	}
}

func TestLoginEndpointRejectsMalformedJSON(t *testing.T) {
	server := testServerWithLogin(t, &fakeLogin{})
	req := httptest.NewRequest(http.MethodPost, "/api/public/login", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected malformed JSON to return 400, got %d", rec.Code)
	}
}

func TestLoginEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	body := `{"login":"legor","pass":"admin","universe":"http://localhost:8888"}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing login use case to return 503, got %d", rec.Code)
	}
}

func TestLoginEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithLogin(t, &fakeLogin{err: errors.New("login failed")})
	body := `{"login":"legor","pass":"admin","universe":"http://localhost:8888"}`
	req := httptest.NewRequest(http.MethodPost, "/api/public/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected login use case error to return 503, got %d", rec.Code)
	}
}

func TestGameSessionEndpointReturnsAuthenticatedSession(t *testing.T) {
	gameSessions := &fakeGameSessions{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session: domainpublicsite.GameSession{
			PlayerID:       42,
			Commander:      "legor",
			PrivateID:      "private",
			HomePlanetID:   99,
			UniverseNumber: 1,
			VacationMode:   true,
			VacationUntil:  12345,
			DeletionQueued: true,
			DeletionAt:     23456,
		},
	}}
	server := testServerWithGameSessions(t, gameSessions)
	req := httptest.NewRequest(http.MethodGet, "/api/game/session?session=public", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Session == nil || response.Session.PlayerID != 42 || response.Session.HomePlanetID != 99 {
		t.Fatalf("expected authenticated session response, got %+v", response)
	}
	if !response.Session.VacationMode || response.Session.VacationUntil != 12345 || !response.Session.DeletionQueued || response.Session.DeletionAt != 23456 {
		t.Fatalf("expected session state response, got %+v", response.Session)
	}
	if gameSessions.command.PublicSession != "public" || gameSessions.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected session command: %+v", gameSessions.command)
	}
	if gameSessions.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", gameSessions.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game session response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameSessionEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	gameSessions := &fakeGameSessions{result: domainpublicsite.SessionAuthentication{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:        domainpublicsite.SessionIssueBanned,
			Message:     "Commander account is banned.",
			BannedUntil: 12345,
		}},
	}}
	server := testServerWithGameSessions(t, gameSessions)
	req := httptest.NewRequest(http.MethodGet, "/api/game/session?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || len(response.Issues) != 1 || response.Issues[0].Code != domainpublicsite.SessionIssueBanned || response.Issues[0].BannedUntil != 12345 {
		t.Fatalf("expected invalid session response, got %+v", response)
	}
}

func TestGameSessionEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/session?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game session use case to return 503, got %d", rec.Code)
	}
}

func TestGameSessionEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameSessions(t, &fakeGameSessions{err: errors.New("session failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/session?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game session error to return 503, got %d", rec.Code)
	}
}

func TestGameLogoutEndpointClearsSessionCookie(t *testing.T) {
	logout := &fakeLogout{result: apppublicsite.LogoutResult{
		Found: true,
		Session: domainpublicsite.GameSession{
			PlayerID:       42,
			UniverseNumber: 1,
		},
	}}
	server := testServerWithLogout(t, logout)
	req := httptest.NewRequest(http.MethodPost, "/api/game/logout?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameLogoutResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.LoggedOut || response.RedirectTo != "/home" {
		t.Fatalf("unexpected logout response: %+v", response)
	}
	if logout.command.PublicSession != "public" {
		t.Fatalf("unexpected logout command: %+v", logout.command)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "prsess_42_1" || cookies[0].MaxAge != -1 {
		t.Fatalf("expected expired private session cookie, got %+v", cookies)
	}
}

func TestClearLoginSessionCookieIgnoresEmptyName(t *testing.T) {
	rec := httptest.NewRecorder()

	clearLoginSessionCookie(rec, "")

	if rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("empty cookie names should not emit Set-Cookie, got %q", rec.Header().Get("Set-Cookie"))
	}
}

func TestGameLogoutEndpointIsIdempotentForMissingSession(t *testing.T) {
	server := testServerWithLogout(t, &fakeLogout{result: apppublicsite.LogoutResult{Found: false}})
	req := httptest.NewRequest(http.MethodPost, "/api/game/logout?session=missing", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameLogoutResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.LoggedOut || response.RedirectTo != "/home" || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("unexpected missing logout response: response=%+v cookies=%+v", response, rec.Result().Cookies())
	}
}

func TestGameLogoutEndpointReturnsUnavailable(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/game/logout?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing logout use case to return 503, got %d", rec.Code)
	}

	server = testServerWithLogout(t, &fakeLogout{err: errors.New("logout failed")})
	req = httptest.NewRequest(http.MethodPost, "/api/game/logout?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected logout use case error to return 503, got %d", rec.Code)
	}
}

func TestGameOverviewEndpointReturnsOverview(t *testing.T) {
	missile := domaingame.BuildFleetMission(12, domaingame.FleetMissionMissile, nil, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}, domaingame.PlanetTypePlanet, "legor", 120, 220)
	missile.OriginName = "Enemy"
	missile.TargetName = "Arakis"
	missile.MissileAmount = 3
	missile.MissileTargetID = domaingame.DefenseRocketLauncher
	missile.MissileTarget = "Rocket Launcher"
	acs := domaingame.BuildFleetMission(13, domaingame.FleetMissionACSAttackHead, nil, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 5}, domaingame.PlanetTypePlanet, "target", 130, 230)
	acs.OriginName = "Arakis"
	acs.TargetName = "Target"
	acs.UnionID = 7
	acs.GroupMissions = []domaingame.FleetMission{
		domaingame.BuildFleetMission(31, domaingame.FleetMissionACSAttackHead, domaingame.FleetCounts{domaingame.FleetCruiser: 2}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 5}, domaingame.PlanetTypePlanet, "target", 130, 230),
		domaingame.BuildFleetMission(32, domaingame.FleetMissionACSAttack, domaingame.FleetCounts{domaingame.FleetLightFighter: 5}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 5}, domaingame.PlanetTypePlanet, "target", 130, 230),
	}
	transport := domaingame.BuildFleetMission(11, domaingame.FleetMissionTransport, domaingame.FleetCounts{domaingame.FleetSmallCargo: 2}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4}, domaingame.PlanetTypePlanet, "target", 100, 200)
	transport.OriginName = "Arakis"
	transport.TargetName = "Colony"
	overviewEvents := domaingame.BuildOverviewEvents([]domaingame.FleetMission{
		transport,
		missile,
		acs,
	})
	overview := &fakeGameOverview{result: appgame.OverviewResult{
		Authenticated: true,
		Overview: domaingame.Overview{
			Commander:      "legor",
			ServerTime:     "Fri Jun 19 18:23:07",
			Messages:       []string{domaingame.OverviewAdminNotice},
			UnreadMessages: 4,
			News: &domaingame.OverviewNews{
				URL:   "https://board.example.test/news",
				Start: "Legacy news",
				End:   "Read more",
			},
			Score: domaingame.ScoreSummary{
				RawScore:        123456,
				Rank:            7,
				UniversePlayers: 2,
			},
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: 1,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Diameter:    12800,
				Temperature: 19,
				Fields:      12,
				MaxFields:   163,
				Resources: domaingame.Resources{
					Metal:             1234.5,
					Crystal:           234.5,
					Deuterium:         12,
					DarkMatter:        37,
					Energy:            140,
					EnergyCapacity:    162,
					MetalCapacity:     100000,
					CrystalCapacity:   150000,
					DeuteriumCapacity: 200000,
				},
				BuildQueue: &domaingame.OverviewBuildQueue{
					TechID:  domaingame.BuildingMetalMine,
					Name:    "Metal Mine",
					Level:   3,
					Destroy: false,
					End:     2000,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: 1,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
				BuildQueue: &domaingame.OverviewBuildQueue{
					TechID:  domaingame.BuildingMetalMine,
					Name:    "Metal Mine",
					Level:   3,
					Destroy: false,
					End:     2000,
				},
			}},
			Events: overviewEvents,
		},
	}}
	server := testServerWithGameOverview(t, overview)
	req := httptest.NewRequest(http.MethodGet, "/api/game/overview?session=public&cp=99&lgn=1", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Overview == nil || response.Overview.Commander != "legor" {
		t.Fatalf("expected authenticated overview response, got %+v", response)
	}
	if response.Overview.Score.Points != 123 ||
		response.Overview.CurrentPlanet.Coordinates.Position != 3 ||
		response.Overview.CurrentPlanet.Resources.Metal != 1234.5 ||
		response.Overview.CurrentPlanet.Resources.DarkMatter != 37 ||
		response.Overview.CurrentPlanet.Resources.Energy != 140 ||
		response.Overview.CurrentPlanet.Resources.EnergyCapacity != 162 ||
		response.Overview.CurrentPlanet.Resources.CrystalCapacity != 150000 ||
		response.Overview.CurrentPlanet.BuildQueue == nil ||
		response.Overview.CurrentPlanet.BuildQueue.Name != "Metal Mine" ||
		response.Overview.CurrentPlanet.BuildQueue.End != 2000 {
		t.Fatalf("unexpected overview mapping: %+v", response.Overview)
	}
	if len(response.Overview.Events) != 3 || !response.Overview.Events[0].Own || response.Overview.Events[0].OwnerID != 0 {
		t.Fatalf("unexpected overview event mapping: %+v", response.Overview.Events)
	}
	if response.Overview.Events[0].MissionName != "Transport" ||
		response.Overview.Events[0].TotalShips != 2 ||
		response.Overview.Events[0].OriginName != "Arakis" ||
		response.Overview.Events[0].TargetName != "Colony" {
		t.Fatalf("expected overview event mapping, got %+v", response.Overview.Events)
	}
	if response.Overview.Events[1].MissionName != "Missile Attack" ||
		response.Overview.Events[1].MissileAmount != 3 ||
		response.Overview.Events[1].MissileTargetID != domaingame.DefenseRocketLauncher ||
		response.Overview.Events[1].MissileTarget != "Rocket Launcher" {
		t.Fatalf("expected overview missile event mapping, got %+v", response.Overview.Events[1])
	}
	if response.Overview.Events[2].UnionID != 7 ||
		len(response.Overview.Events[2].GroupMissions) != 2 ||
		response.Overview.Events[2].GroupMissions[1].MissionName != "Joint attack" {
		t.Fatalf("expected overview ACS group mapping, got %+v", response.Overview.Events[2])
	}
	if len(response.Overview.Messages) != 1 || response.Overview.Messages[0] != domaingame.OverviewAdminNotice {
		t.Fatalf("expected overview messages, got %+v", response.Overview.Messages)
	}
	if response.Overview.ServerTime != "Fri Jun 19 18:23:07" {
		t.Fatalf("expected overview server time, got %q", response.Overview.ServerTime)
	}
	if response.Overview.UnreadMessages != 4 {
		t.Fatalf("expected unread messages to be mapped, got %d", response.Overview.UnreadMessages)
	}
	if response.Overview.News == nil ||
		response.Overview.News.URL != "https://board.example.test/news" ||
		response.Overview.News.Start != "Legacy news" ||
		response.Overview.News.End != "Read more" {
		t.Fatalf("expected overview news mapping, got %+v", response.Overview.News)
	}
	if overview.command.PublicSession != "public" || overview.command.PlanetID != 99 || overview.command.RemoteAddr != "203.0.113.10" ||
		!overview.command.Login {
		t.Fatalf("unexpected overview command: %+v", overview.command)
	}
	if overview.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", overview.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game overview response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameFleetMissionResponseMapsUnionGroupsAndLoadedResources(t *testing.T) {
	mission := domaingame.BuildFleetMission(
		7,
		domaingame.FleetMissionACSAttackHead,
		domaingame.FleetCounts{domaingame.FleetCruiser: 2},
		domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
		domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4},
		domaingame.PlanetTypePlanet,
		"target",
		100,
		200,
	)
	mission.OwnerID = 42
	mission.OwnerName = "legor"
	mission.MissionName = "Joint attack"
	mission.TotalShips = 2
	mission.UnionID = 9
	mission.UnionName = "Alpha Wing"
	mission.UnionPlayers = []domaingame.FleetUnionPlayer{{ID: 42, Name: "legor"}, {ID: 77, Name: "ally"}}
	mission.LoadedResources = map[int]int{domaingame.ResourceMetal: 123, domaingame.ResourceCrystal: 0, domaingame.ResourceDeuterium: -5}
	groupMission := domaingame.BuildFleetMission(8, domaingame.FleetMissionACSAttack, domaingame.FleetCounts{domaingame.FleetLightFighter: 5}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 5}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4}, domaingame.PlanetTypePlanet, "target", 110, 210)
	groupMission.MissionName = "ACS Attack"
	mission.GroupMissions = []domaingame.FleetMission{groupMission}

	response := toGameFleetMissionResponse(mission)

	if response.ID != 7 || response.MissionName != "Joint attack" || response.TotalShips != 2 || response.LoadedResources[strconv.Itoa(domaingame.ResourceMetal)] != 123 {
		t.Fatalf("unexpected mission response: %+v", response)
	}
	if _, ok := response.LoadedResources[strconv.Itoa(domaingame.ResourceCrystal)]; ok {
		t.Fatalf("zero resources must not be exposed: %+v", response.LoadedResources)
	}
	if response.UnionID != 9 || response.UnionName != "Alpha Wing" || len(response.UnionPlayers) != 2 || response.UnionPlayers[1].Name != "ally" {
		t.Fatalf("unexpected union mapping: %+v", response)
	}
	if len(response.GroupMissions) != 1 || response.GroupMissions[0].ID != 8 || response.GroupMissions[0].MissionName != "ACS Attack" {
		t.Fatalf("unexpected group mission mapping: %+v", response.GroupMissions)
	}
}

func TestGameOverviewEndpointRenamesPlanet(t *testing.T) {
	overview := &fakeGameOverview{result: appgame.OverviewResult{
		Authenticated: true,
		Overview: domaingame.Overview{
			Commander:     "legor",
			CurrentPlanet: domaingame.PlanetOverview{ID: 99, Name: "New Colony"},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:      99,
				Name:    "New Colony",
				Current: true,
			}},
		},
	}}
	server := testServerWithGameOverview(t, overview)
	req := httptest.NewRequest(http.MethodPost, "/api/game/overview?session=public&cp=99", strings.NewReader(`{"action":"rename","name":"New Colony"}`))
	req.RemoteAddr = "203.0.113.10:4321"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response gameOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Overview == nil || response.Overview.CurrentPlanet.Name != "New Colony" {
		t.Fatalf("expected renamed overview response, got %+v", response)
	}
	if overview.renameCommand.PublicSession != "public" || overview.renameCommand.PlanetID != 99 ||
		overview.renameCommand.Name != "New Colony" || overview.renameCommand.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected rename command: %+v", overview.renameCommand)
	}
	if overview.renameCommand.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", overview.renameCommand.PrivateSessions)
	}
}

func TestGameOverviewEndpointRenameReturnsUnauthorizedForInvalidSession(t *testing.T) {
	overview := &fakeGameOverview{result: appgame.OverviewResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameOverview(t, overview)
	req := httptest.NewRequest(http.MethodPost, "/api/game/overview?session=public", strings.NewReader(`{"action":"rename","name":"New Colony"}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Overview != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid overview rename session response, got %+v", response)
	}
}

func TestGameOverviewEndpointDeletesPlanet(t *testing.T) {
	overview := &fakeGameOverview{result: appgame.OverviewResult{
		Authenticated: true,
		Overview: domaingame.Overview{
			Commander:     "legor",
			CurrentPlanet: domaingame.PlanetOverview{ID: 1, Name: "Homeworld"},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:      1,
				Name:    "Homeworld",
				Current: true,
			}},
		},
	}}
	server := testServerWithGameOverview(t, overview)
	req := httptest.NewRequest(http.MethodPost, "/api/game/overview?session=public&cp=99", strings.NewReader(`{"action":"delete","deleteId":99,"password":"admin"}`))
	req.RemoteAddr = "203.0.113.10:4321"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response gameOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Overview == nil || response.Overview.CurrentPlanet.Name != "Homeworld" {
		t.Fatalf("expected delete overview response, got %+v", response)
	}
	if overview.deleteCommand.PublicSession != "public" || overview.deleteCommand.PlanetID != 99 ||
		overview.deleteCommand.DeleteID != 99 || overview.deleteCommand.Password != "admin" ||
		overview.deleteCommand.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected delete command: %+v", overview.deleteCommand)
	}
	if overview.deleteCommand.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", overview.deleteCommand.PrivateSessions)
	}
}

func TestGameOverviewEndpointDeleteReturnsActionIssue(t *testing.T) {
	overview := &fakeGameOverview{result: appgame.OverviewResult{
		Authenticated: true,
		ActionIssue: &domaingame.OverviewActionIssue{
			Code:    domaingame.OverviewIssuePasswordInvalid,
			Message: "The password is wrong.",
		},
		Overview: domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{ID: 99, Name: "Colony"}},
	}}
	server := testServerWithGameOverview(t, overview)
	req := httptest.NewRequest(http.MethodPost, "/api/game/overview?session=public&cp=99", strings.NewReader(`{"action":"delete","deleteId":99,"password":"wrong"}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response gameOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ActionIssue == nil || response.ActionIssue.Code != domaingame.OverviewIssuePasswordInvalid {
		t.Fatalf("expected action issue response, got %+v", response)
	}
}

func TestGameOverviewEndpointRejectsInvalidRenameRequests(t *testing.T) {
	for _, tt := range []struct {
		name   string
		target string
		body   string
	}{
		{"invalid planet", "/api/game/overview?session=public&cp=bad", `{"action":"rename","name":"New Colony"}`},
		{"malformed", "/api/game/overview?session=public", `{`},
		{"unknown action", "/api/game/overview?session=public", `{"action":"delete","name":"New Colony"}`},
		{"missing delete id", "/api/game/overview?session=public", `{"action":"delete","password":"admin"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := testServerWithGameOverview(t, &fakeGameOverview{})
			req := httptest.NewRequest(http.MethodPost, tt.target, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
		})
	}
}

func TestGameOverviewEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	overview := &fakeGameOverview{result: appgame.OverviewResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameOverview(t, overview)
	req := httptest.NewRequest(http.MethodGet, "/api/game/overview?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Overview != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid overview session response, got %+v", response)
	}
}

func TestGameOverviewEndpointRejectsInvalidPlanetID(t *testing.T) {
	server := testServerWithGameOverview(t, &fakeGameOverview{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/overview?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameOverviewEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/overview?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game overview use case to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/overview?session=public", strings.NewReader(`{"action":"rename","name":"New Colony"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game overview rename use case to return 503, got %d", rec.Code)
	}
}

func TestGameOverviewEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameOverview(t, &fakeGameOverview{err: errors.New("overview failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/overview?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game overview error to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/overview?session=public", strings.NewReader(`{"action":"rename","name":"New Colony"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game overview rename error to return 503, got %d", rec.Code)
	}
}

func TestGameBuildingsEndpointReturnsBuildings(t *testing.T) {
	buildings := &fakeGameBuildings{result: appgame.BuildingsResult{
		Authenticated: true,
		Buildings: domaingame.Buildings{
			Commander:       "legor",
			CommanderActive: true,
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Resources: domaingame.Resources{
					Metal:     1000,
					Crystal:   500,
					Deuterium: 100,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   100,
				Name: "Moon",
				Type: domaingame.PlanetTypeMoon,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			}},
			Items: []domaingame.BuildingItem{{
				ID:              domaingame.BuildingMetalMine,
				Name:            "Metal Mine",
				Description:     "Used in the extraction of metal ore.",
				Level:           2,
				NextLevel:       3,
				Cost:            domaingame.BuildingCost{Metal: 135, Crystal: 33.75},
				DurationSeconds: 121,
				CanBuild:        true,
				Action:          "Build level",
			}},
			Queue: []domaingame.BuildingQueueEntry{{
				ListID:           1,
				TechID:           domaingame.BuildingMetalMine,
				Name:             "Metal Mine",
				Level:            3,
				Start:            100,
				End:              160,
				RemainingSeconds: 60,
			}},
		},
	}}
	server := testServerWithGameBuildings(t, buildings)
	req := httptest.NewRequest(http.MethodGet, "/api/game/buildings?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameBuildingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Buildings == nil || response.Buildings.Commander != "legor" || len(response.Buildings.Items) != 1 {
		t.Fatalf("expected authenticated buildings response, got %+v", response)
	}
	if !response.Buildings.CommanderActive || len(response.Buildings.Queue) != 1 || response.Buildings.Queue[0].Name != "Metal Mine" || response.Buildings.Queue[0].RemainingSeconds != 60 {
		t.Fatalf("expected buildings queue mapping, got %+v", response.Buildings)
	}
	if len(response.Buildings.PlanetSwitcher) != 1 || response.Buildings.PlanetSwitcher[0].Name != "Moon" {
		t.Fatalf("expected planet switcher mapping, got %+v", response.Buildings.PlanetSwitcher)
	}
	if response.Buildings.Items[0].Cost.Metal != 135 || response.Buildings.Items[0].DurationSeconds != 121 {
		t.Fatalf("unexpected buildings mapping: %+v", response.Buildings.Items[0])
	}
	if buildings.command.PublicSession != "public" || buildings.command.PlanetID != 99 || buildings.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected buildings command: %+v", buildings.command)
	}
	if buildings.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", buildings.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game buildings response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameBuildingsEndpointMutatesBuildings(t *testing.T) {
	issue := domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources)
	buildings := &fakeGameBuildings{result: appgame.BuildingsResult{
		Authenticated: true,
		ActionIssue:   issue,
		Buildings: domaingame.Buildings{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
			},
			Items: []domaingame.BuildingItem{{
				ID:     domaingame.BuildingMetalMine,
				Name:   "Metal Mine",
				Action: "build",
			}},
		},
	}}
	server := testServerWithGameBuildings(t, buildings)
	req := httptest.NewRequest(http.MethodPost, "/api/game/buildings?session=public&cp=99", strings.NewReader(`{"action":"add","techId":1,"listId":2}`))
	req.RemoteAddr = "203.0.113.10:4321"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameBuildingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.ActionIssue == nil || response.ActionIssue.Code != issue.Code || response.Buildings == nil {
		t.Fatalf("expected mutation buildings response with issue, got %+v", response)
	}
	if buildings.mutation.PublicSession != "public" || buildings.mutation.PlanetID != 99 ||
		buildings.mutation.Action != "add" || buildings.mutation.TechID != 1 || buildings.mutation.ListID != 2 {
		t.Fatalf("unexpected buildings mutation command: %+v", buildings.mutation)
	}
}

func TestGameBuildingsEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	buildings := &fakeGameBuildings{result: appgame.BuildingsResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameBuildings(t, buildings)
	req := httptest.NewRequest(http.MethodGet, "/api/game/buildings?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameBuildingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Buildings != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid buildings session response, got %+v", response)
	}
}

func TestGameBuildingsEndpointRejectsInvalidPlanetID(t *testing.T) {
	server := testServerWithGameBuildings(t, &fakeGameBuildings{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/buildings?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/buildings?session=public&cp=abc", strings.NewReader(`{"action":"add"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet mutation to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/buildings?session=public", strings.NewReader(`{`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid buildings mutation JSON to return 400, got %d", rec.Code)
	}
}

func TestGameBuildingsEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameBuildings(t, &fakeGameBuildings{err: errors.New("buildings failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/buildings?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game buildings error to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/buildings?session=public", strings.NewReader(`{"action":"add"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game buildings mutation error to return 503, got %d", rec.Code)
	}
}

func TestGameBuildingsEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/buildings?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing buildings use case to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/buildings?session=public", strings.NewReader(`{"action":"add"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing buildings mutation use case to return 503, got %d", rec.Code)
	}
}

func TestGameResearchEndpointReturnsResearch(t *testing.T) {
	research := &fakeGameResearch{result: appgame.ResearchResult{
		Authenticated: true,
		Research: domaingame.Research{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
			}},
			HasLab: true,
			Active: &domaingame.ResearchQueue{
				TaskID:           7,
				PlanetID:         99,
				TechID:           domaingame.ResearchComputer,
				Level:            2,
				RemainingSeconds: 120,
				Cancelable:       true,
			},
			Items: []domaingame.BuildingItem{{
				ID:              domaingame.ResearchComputer,
				Name:            "Computer Technology",
				Description:     "More fleets can be commanded.",
				Level:           1,
				NextLevel:       2,
				Cost:            domaingame.BuildingCost{Crystal: 800, Deuterium: 1200},
				DurationSeconds: 240,
				CanBuild:        true,
				Action:          "Research level",
			}},
		},
	}}
	server := testServerWithGameResearch(t, research)
	req := httptest.NewRequest(http.MethodGet, "/api/game/research?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameResearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Research == nil || response.Research.Commander != "legor" || !response.Research.HasLab || len(response.Research.Items) != 1 || len(response.Research.PlanetSwitcher) != 1 {
		t.Fatalf("expected authenticated research response, got %+v", response)
	}
	if response.Research.Items[0].Cost.Crystal != 800 || response.Research.Items[0].DurationSeconds != 240 {
		t.Fatalf("unexpected research mapping: %+v", response.Research.Items[0])
	}
	if response.Research.Active == nil || response.Research.Active.TechID != domaingame.ResearchComputer || !response.Research.Active.Cancelable {
		t.Fatalf("unexpected active research mapping: %+v", response.Research.Active)
	}
	if research.command.PublicSession != "public" || research.command.PlanetID != 99 || research.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected research command: %+v", research.command)
	}
	if research.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", research.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game research response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameResearchEndpointMutatesResearch(t *testing.T) {
	research := &fakeGameResearch{result: appgame.ResearchResult{
		Authenticated: true,
		ActionIssue:   domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources),
		Research:      domaingame.Research{Commander: "legor", HasLab: true},
	}}
	server := testServerWithGameResearch(t, research)
	req := httptest.NewRequest(http.MethodPost, "/api/game/research?session=public&cp=99", strings.NewReader(`{"action":"start","techId":108}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response gameResearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.ActionIssue == nil || response.ActionIssue.Code != domaingame.BuildingsIssueNoResources {
		t.Fatalf("expected research mutation response, got %+v", response)
	}
	if research.mutation.PublicSession != "public" || research.mutation.PlanetID != 99 || research.mutation.Action != "start" || research.mutation.TechID != domaingame.ResearchComputer {
		t.Fatalf("unexpected research mutation command: %+v", research.mutation)
	}
	if research.mutation.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", research.mutation.PrivateSessions)
	}
}

func TestGameResearchEndpointRejectsInvalidMutation(t *testing.T) {
	server := testServerWithGameResearch(t, &fakeGameResearch{})
	req := httptest.NewRequest(http.MethodPost, "/api/game/research?session=public&cp=99", strings.NewReader(`{`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid research mutation to return 400, got %d", rec.Code)
	}
}

func TestGameResearchEndpointRejectsInvalidMutationPlanetID(t *testing.T) {
	server := testServerWithGameResearch(t, &fakeGameResearch{})
	req := httptest.NewRequest(http.MethodPost, "/api/game/research?session=public&cp=abc", strings.NewReader(`{"action":"start","techId":108}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameResearchEndpointMutationReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/game/research?session=public", strings.NewReader(`{"action":"start","techId":108}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game research mutation use case to return 503, got %d", rec.Code)
	}
}

func TestGameResearchEndpointMutationReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameResearch(t, &fakeGameResearch{err: errors.New("research failed")})
	req := httptest.NewRequest(http.MethodPost, "/api/game/research?session=public", strings.NewReader(`{"action":"start","techId":108}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game research mutation error to return 503, got %d", rec.Code)
	}
}

func TestGameResearchEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	research := &fakeGameResearch{result: appgame.ResearchResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameResearch(t, research)
	req := httptest.NewRequest(http.MethodGet, "/api/game/research?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameResearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Research != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid research session response, got %+v", response)
	}
}

func TestGameResearchEndpointRejectsInvalidPlanetID(t *testing.T) {
	server := testServerWithGameResearch(t, &fakeGameResearch{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/research?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameResearchEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/research?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game research use case to return 503, got %d", rec.Code)
	}
}

func TestGameResearchEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameResearch(t, &fakeGameResearch{err: errors.New("research failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/research?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game research error to return 503, got %d", rec.Code)
	}
}

func TestGameShipyardEndpointReturnsShipyard(t *testing.T) {
	shipyard := &fakeGameShipyard{result: appgame.ShipyardResult{
		Authenticated: true,
		Shipyard: domaingame.Shipyard{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
			}},
			HasShipyard: true,
			Queue: []domaingame.ShipyardQueueEntry{{
				TaskID:           12,
				UnitID:           domaingame.FleetSmallCargo,
				Name:             "Small Cargo",
				Count:            3,
				Start:            100,
				End:              220,
				RemainingSeconds: 60,
			}},
			Items: []domaingame.ShipyardItem{{
				ID:               domaingame.FleetSmallCargo,
				Name:             "Small Cargo",
				Description:      "The small cargo is an agile ship.",
				Count:            2,
				Cost:             domaingame.BuildingCost{Metal: 2000, Crystal: 2000},
				DurationSeconds:  240,
				CanBuild:         true,
				MeetsRequirement: true,
				MaxBuild:         5,
			}},
		},
	}}
	server := testServerWithGameShipyard(t, shipyard)
	req := httptest.NewRequest(http.MethodGet, "/api/game/shipyard?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameShipyardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Shipyard == nil || response.Shipyard.Commander != "legor" || !response.Shipyard.HasShipyard || len(response.Shipyard.Items) != 1 || len(response.Shipyard.PlanetSwitcher) != 1 {
		t.Fatalf("expected authenticated shipyard response, got %+v", response)
	}
	if response.Shipyard.Items[0].Cost.Metal != 2000 || response.Shipyard.Items[0].DurationSeconds != 240 || response.Shipyard.Items[0].MaxBuild != 5 {
		t.Fatalf("unexpected shipyard mapping: %+v", response.Shipyard.Items[0])
	}
	if len(response.Shipyard.Queue) != 1 || response.Shipyard.Queue[0].TaskID != 12 || response.Shipyard.Queue[0].RemainingSeconds != 60 {
		t.Fatalf("unexpected shipyard queue mapping: %+v", response.Shipyard.Queue)
	}
	if shipyard.command.PublicSession != "public" || shipyard.command.PlanetID != 99 || shipyard.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected shipyard command: %+v", shipyard.command)
	}
	if shipyard.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", shipyard.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game shipyard response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameShipyardEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	shipyard := &fakeGameShipyard{result: appgame.ShipyardResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameShipyard(t, shipyard)
	req := httptest.NewRequest(http.MethodGet, "/api/game/shipyard?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameShipyardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Shipyard != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid shipyard session response, got %+v", response)
	}
}

func TestGameShipyardEndpointMutatesOrders(t *testing.T) {
	issue := domaingame.BuildingActionIssue(domaingame.BuildingsIssueNoResources)
	shipyard := &fakeGameShipyard{result: appgame.ShipyardResult{
		Authenticated: true,
		ActionIssue:   issue,
		Shipyard:      domaingame.Shipyard{Commander: "legor", HasShipyard: true},
	}}
	server := testServerWithGameShipyard(t, shipyard)
	req := httptest.NewRequest(http.MethodPost, "/api/game/shipyard?session=public&cp=99", strings.NewReader(`{"orders":{"bad":9,"204":3}}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response gameShipyardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.ActionIssue == nil || response.ActionIssue.Code != domaingame.BuildingsIssueNoResources || response.Shipyard == nil {
		t.Fatalf("unexpected shipyard mutation response: %+v", response)
	}
	if shipyard.mutation.PublicSession != "public" || shipyard.mutation.PlanetID != 99 || shipyard.mutation.Orders[domaingame.FleetLightFighter] != 3 {
		t.Fatalf("unexpected shipyard mutation command: %+v", shipyard.mutation)
	}
	if _, exists := shipyard.mutation.Orders[0]; exists || len(shipyard.mutation.Orders) != 1 {
		t.Fatalf("expected non-integer order keys to be ignored, got %+v", shipyard.mutation.Orders)
	}
	if shipyard.mutation.PrivateSessions["prsess_42_1"] != "private" || shipyard.mutation.RemoteAddr != "203.0.113.10" {
		t.Fatalf("expected private session and remote address to be passed, got %+v", shipyard.mutation)
	}
}

func TestGameShipyardEndpointPostErrors(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/game/shipyard?session=public", strings.NewReader(`{"orders":{"204":1}}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing shipyard use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameShipyard(t, &fakeGameShipyard{})
	req = httptest.NewRequest(http.MethodPost, "/api/game/shipyard?session=public&cp=abc", strings.NewReader(`{"orders":{"204":1}}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cp to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/shipyard?session=public", strings.NewReader(`{bad`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid JSON to return 400, got %d", rec.Code)
	}

	server = testServerWithGameShipyard(t, &fakeGameShipyard{err: errors.New("shipyard failed")})
	req = httptest.NewRequest(http.MethodPost, "/api/game/shipyard?session=public", strings.NewReader(`{"orders":{"204":1}}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected use case error to return 503, got %d", rec.Code)
	}
}

func TestGameShipyardEndpointRejectsInvalidPlanetID(t *testing.T) {
	server := testServerWithGameShipyard(t, &fakeGameShipyard{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/shipyard?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameShipyardEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/shipyard?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game shipyard use case to return 503, got %d", rec.Code)
	}
}

func TestGameShipyardEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameShipyard(t, &fakeGameShipyard{err: errors.New("shipyard failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/shipyard?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game shipyard error to return 503, got %d", rec.Code)
	}
}

func TestGameFleetEndpointReturnsFleet(t *testing.T) {
	fleet := &fakeGameFleet{result: appgame.FleetResult{
		Authenticated: true,
		Fleet: domaingame.Fleet{
			Commander:       "legor",
			CommanderActive: true,
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
			}},
			Slots:         domaingame.FleetSlots{Used: 1, Max: 6, BaseMax: 4, Admiral: true},
			Expeditions:   domaingame.ExpeditionSlots{Used: 0, Max: 2},
			TemplateLimit: 4,
			Templates: []domaingame.FleetTemplate{{
				ID:        7,
				Name:      "raid wing",
				UpdatedAt: 1000,
				Ships: []domaingame.FleetTemplateShip{{
					ID:    domaingame.FleetSmallCargo,
					Name:  "Small Cargo",
					Count: 3,
				}},
			}},
			Missions: []domaingame.FleetMission{{
				ID:              11,
				Mission:         domaingame.FleetMissionTransport,
				MissionName:     "Transport",
				StateTitle:      "Going on a mission",
				StateShort:      "(G)",
				TotalShips:      2,
				Origin:          domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
				Target:          domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4},
				TargetType:      domaingame.PlanetTypePlanet,
				TargetOwnerName: "target",
				DepartureAt:     100,
				ArrivalAt:       200,
				CanRecall:       true,
				Ships: []domaingame.FleetShipCount{{
					ID:    domaingame.FleetSmallCargo,
					Name:  "Small Cargo",
					Count: 2,
				}},
			}},
			Ships: []domaingame.FleetShipSelection{{
				ID:          domaingame.FleetSmallCargo,
				Name:        "Small Cargo",
				Count:       4,
				Speed:       20000,
				Cargo:       5000,
				Consumption: 20,
				Selectable:  true,
			}},
		},
	}}
	server := testServerWithGameFleet(t, fleet)
	req := httptest.NewRequest(http.MethodGet, "/api/game/fleet?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameFleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Fleet == nil || response.Fleet.Commander != "legor" || len(response.Fleet.Missions) != 1 || len(response.Fleet.Ships) != 1 || len(response.Fleet.PlanetSwitcher) != 1 {
		t.Fatalf("expected authenticated fleet response, got %+v", response)
	}
	if response.Fleet.Slots.Used != 1 || response.Fleet.Slots.Max != 6 || response.Fleet.Expeditions.Max != 2 {
		t.Fatalf("unexpected fleet slot mapping: %+v", response.Fleet)
	}
	if !response.Fleet.CommanderActive || !response.Fleet.Templates.CommanderActive || response.Fleet.Templates.Max != 4 || len(response.Fleet.Templates.Items) != 1 || response.Fleet.Templates.Items[0].Ships[0].Count != 3 {
		t.Fatalf("unexpected fleet template mapping: %+v", response.Fleet.Templates)
	}
	if response.Fleet.Missions[0].MissionName != "Transport" || response.Fleet.Missions[0].Origin.Galaxy != 1 || !response.Fleet.Missions[0].CanRecall {
		t.Fatalf("unexpected fleet mission mapping: %+v", response.Fleet.Missions[0])
	}
	if response.Fleet.Ships[0].Speed != 20000 || response.Fleet.Ships[0].Cargo != 5000 || !response.Fleet.Ships[0].Selectable {
		t.Fatalf("unexpected fleet ship mapping: %+v", response.Fleet.Ships[0])
	}
	if fleet.command.PublicSession != "public" || fleet.command.PlanetID != 99 || fleet.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected fleet command: %+v", fleet.command)
	}
	if fleet.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", fleet.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game fleet response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameFleetEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	fleet := &fakeGameFleet{result: appgame.FleetResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameFleet(t, fleet)
	req := httptest.NewRequest(http.MethodGet, "/api/game/fleet?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameFleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Fleet != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid fleet session response, got %+v", response)
	}
}

func TestGameFleetEndpointRejectsInvalidPlanetID(t *testing.T) {
	server := testServerWithGameFleet(t, &fakeGameFleet{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/fleet?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameFleetEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/fleet?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game fleet use case to return 503, got %d", rec.Code)
	}
}

func TestGameFleetEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameFleet(t, &fakeGameFleet{err: errors.New("fleet failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/fleet?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game fleet error to return 503, got %d", rec.Code)
	}
}

func TestGameFleetEndpointRecallsFleet(t *testing.T) {
	fleet := &fakeGameFleet{result: appgame.FleetResult{
		Authenticated: true,
		Fleet: domaingame.Fleet{
			Commander: "legor",
			Missions: []domaingame.FleetMission{{
				ID:        123,
				Mission:   domaingame.FleetMissionTransport + domaingame.FleetMissionReturnOffset,
				CanRecall: false,
			}},
		},
	}}
	server := testServerWithGameFleet(t, fleet)
	req := httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public&cp=99", bytes.NewBufferString(`{"action":"recall","fleetId":123}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fleet.recall.PublicSession != "public" || fleet.recall.PlanetID != 99 || fleet.recall.FleetID != 123 || fleet.recall.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected fleet recall command: %+v", fleet.recall)
	}
	if fleet.recall.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", fleet.recall.PrivateSessions)
	}
	var response gameFleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Fleet == nil || len(response.Fleet.Missions) != 1 || response.Fleet.Missions[0].CanRecall {
		t.Fatalf("unexpected recall response: %+v", response)
	}
}

func TestGameFleetEndpointPreparesDispatchDraft(t *testing.T) {
	fleet := &fakeGameFleet{result: appgame.FleetResult{
		Authenticated: true,
		Fleet: domaingame.Fleet{
			DispatchDraft: &domaingame.FleetDispatchDraft{
				Ships: []domaingame.FleetShipCount{{
					ID:    domaingame.FleetSmallCargo,
					Name:  "Small Cargo",
					Count: 3,
				}},
				TotalShips:      3,
				Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
				TargetType:      domaingame.GamePlanetTypeMoon,
				Mission:         domaingame.FleetMissionTransport,
				Speed:           9,
				Cargo:           15000,
				Distance:        20000,
				DurationSeconds: 2121,
				MaxSpeed:        5500,
				FuelConsumption: 2765,
				SpeedFactor:     1,
				HasSelection:    true,
				MissionOptions: []domaingame.FleetMissionOption{{
					ID:       domaingame.FleetMissionTransport,
					Name:     "Transport",
					Selected: true,
				}},
				Resources: []domaingame.FleetResourceLoad{{
					ID:        domaingame.ResourceMetal,
					Name:      "Metal",
					Available: 1200,
				}},
			},
		},
	}}
	server := testServerWithGameFleet(t, fleet)
	body := `{"action":"prepare","ships":{"202":3},"target":{"galaxy":2,"system":3,"position":4},"targetType":3,"mission":3,"speed":9}`
	req := httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public&cp=99", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fleet.prepare.PublicSession != "public" || fleet.prepare.PlanetID != 99 || fleet.prepare.Ships[domaingame.FleetSmallCargo] != 3 || fleet.prepare.Target.Position != 4 || fleet.prepare.TargetType != 3 || fleet.prepare.Mission != 3 || fleet.prepare.Speed != 9 {
		t.Fatalf("unexpected fleet prepare command: %+v", fleet.prepare)
	}
	if fleet.prepare.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", fleet.prepare.PrivateSessions)
	}
	var response gameFleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Fleet == nil || response.Fleet.DispatchDraft == nil || response.Fleet.DispatchDraft.Cargo != 15000 || response.Fleet.DispatchDraft.Ships[0].Count != 3 {
		t.Fatalf("unexpected prepare response: %+v", response)
	}
	if response.Fleet.DispatchDraft.MissionOptions[0].ID != domaingame.FleetMissionTransport || response.Fleet.DispatchDraft.Resources[0].Available != 1200 {
		t.Fatalf("unexpected prepare draft mapping: %+v", response.Fleet.DispatchDraft)
	}
	if response.Fleet.DispatchDraft.Distance != 20000 || response.Fleet.DispatchDraft.DurationSeconds != 2121 || response.Fleet.DispatchDraft.MaxSpeed != 5500 || response.Fleet.DispatchDraft.FuelConsumption != 2765 || response.Fleet.DispatchDraft.SpeedFactor != 1 {
		t.Fatalf("unexpected prepare draft mapping: %+v", response.Fleet.DispatchDraft)
	}
}

func TestGameFleetEndpointValidatesDispatchDraft(t *testing.T) {
	fleet := &fakeGameFleet{result: appgame.FleetResult{
		Authenticated: true,
		ActionIssue:   domaingame.FleetActionIssueFor(domaingame.FleetIssueNoCargo),
		Fleet: domaingame.Fleet{
			DispatchDraft: &domaingame.FleetDispatchDraft{
				Ships: []domaingame.FleetShipCount{{
					ID:    domaingame.FleetBomber,
					Name:  "Bomber",
					Count: 1,
				}},
				TotalShips:      1,
				Target:          domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4},
				TargetType:      domaingame.GamePlanetTypePlanet,
				Mission:         domaingame.FleetMissionTransport,
				Speed:           10,
				Cargo:           500,
				Distance:        20000,
				DurationSeconds: 2485,
				MaxSpeed:        4000,
				FuelConsumption: 69136,
				RemainingCargo:  0,
				HasSelection:    true,
				Resources: []domaingame.FleetResourceLoad{{
					ID:        domaingame.ResourceMetal,
					Name:      "Metal",
					Available: 900,
					Requested: 900,
					Loaded:    0,
				}},
			},
		},
	}}
	server := testServerWithGameFleet(t, fleet)
	body := `{"action":"validate-dispatch","ships":{"211":1},"resources":{"700":900},"target":{"galaxy":2,"system":3,"position":4},"targetType":1,"mission":3,"speed":10,"holdHours":4,"unionId":7}`
	req := httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public&cp=99", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fleet.validate.PublicSession != "public" || fleet.validate.PlanetID != 99 || fleet.validate.Ships[domaingame.FleetBomber] != 1 || fleet.validate.Resources[domaingame.ResourceMetal] != 900 || fleet.validate.HoldHours != 4 || fleet.validate.UnionID != 7 {
		t.Fatalf("unexpected fleet validate command: %+v", fleet.validate)
	}
	var response gameFleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.ActionIssue == nil || response.ActionIssue.Code != domaingame.FleetIssueNoCargo {
		t.Fatalf("expected fleet validation issue, got %+v", response)
	}
	if response.Fleet == nil || response.Fleet.DispatchDraft == nil || response.Fleet.DispatchDraft.Resources[0].Requested != 900 || response.Fleet.DispatchDraft.Resources[0].Loaded != 0 {
		t.Fatalf("unexpected validate draft mapping: %+v", response.Fleet)
	}
}

func TestGameFleetEndpointLaunchesDispatchDraft(t *testing.T) {
	fleet := &fakeGameFleet{result: appgame.FleetResult{
		Authenticated: true,
		Fleet: domaingame.Fleet{
			Missions: []domaingame.FleetMission{
				domaingame.BuildFleetMission(44, domaingame.FleetMissionTransport, domaingame.FleetCounts{domaingame.FleetSmallCargo: 1}, domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}, domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4}, domaingame.GamePlanetTypePlanet, "target", 100, 200),
			},
		},
	}}
	server := testServerWithGameFleet(t, fleet)
	body := `{"action":"launch-dispatch","ships":{"202":1},"resources":{"700":123,"701":45},"target":{"galaxy":2,"system":3,"position":4},"targetType":1,"mission":3,"speed":9,"expeditionHours":2,"unionId":7}`
	req := httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public&cp=99", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fleet.launch.PublicSession != "public" || fleet.launch.PlanetID != 99 || fleet.launch.Ships[domaingame.FleetSmallCargo] != 1 || fleet.launch.Resources[domaingame.ResourceMetal] != 123 || fleet.launch.Resources[domaingame.ResourceCrystal] != 45 || fleet.launch.ExpeditionHours != 2 || fleet.launch.UnionID != 7 {
		t.Fatalf("unexpected fleet launch command: %+v", fleet.launch)
	}
	if fleet.launch.Target.Galaxy != 2 || fleet.launch.Target.System != 3 || fleet.launch.Target.Position != 4 || fleet.launch.TargetType != 1 || fleet.launch.Mission != 3 || fleet.launch.Speed != 9 {
		t.Fatalf("unexpected fleet launch target command: %+v", fleet.launch)
	}
	var response gameFleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Fleet == nil || len(response.Fleet.Missions) != 1 || response.Fleet.Missions[0].ID != 44 {
		t.Fatalf("unexpected launch response: %+v", response)
	}
}

func TestGameFleetEndpointRejectsBadRecallPayload(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public", strings.NewReader(`{"action":"recall","fleetId":123}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing fleet recall use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameFleet(t, &fakeGameFleet{})
	req = httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public&cp=abc", strings.NewReader(`{"action":"recall","fleetId":123}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid recall selected planet to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public", strings.NewReader(`{`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad recall payload to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public", strings.NewReader(`{"action":"launch"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported recall action to return 400, got %d", rec.Code)
	}

	server = testServerWithGameFleet(t, &fakeGameFleet{err: errors.New("recall failed")})
	req = httptest.NewRequest(http.MethodPost, "/api/game/fleet?session=public", strings.NewReader(`{"action":"recall","fleetId":123}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected recall use case error to return 503, got %d", rec.Code)
	}
}

func TestGameFleetTemplatesEndpointMutatesTemplates(t *testing.T) {
	fleet := &fakeGameFleet{result: appgame.FleetResult{
		Authenticated: true,
		Fleet: domaingame.Fleet{
			Commander:       "legor",
			CommanderActive: true,
			TemplateLimit:   4,
			Templates: []domaingame.FleetTemplate{{
				ID:   7,
				Name: "raid wing",
				Ships: []domaingame.FleetTemplateShip{{
					ID:    domaingame.FleetSmallCargo,
					Name:  "Small Cargo",
					Count: 3,
				}},
			}},
		},
	}}
	server := testServerWithGameFleet(t, fleet)
	req := httptest.NewRequest(http.MethodPost, "/api/game/fleet-templates?session=public&cp=99", bytes.NewBufferString(`{"action":"save","templateId":7,"name":"raid wing","ships":{"202":3,"bad":9}}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fleet.mutation.PublicSession != "public" || fleet.mutation.PlanetID != 99 || fleet.mutation.TemplateID != 7 || fleet.mutation.Ships[domaingame.FleetSmallCargo] != 3 {
		t.Fatalf("unexpected fleet template mutation: %+v", fleet.mutation)
	}
	var response gameFleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Fleet == nil || len(response.Fleet.Templates.Items) != 1 || response.Fleet.Templates.Items[0].Name != "raid wing" {
		t.Fatalf("unexpected fleet template response: %+v", response.Fleet)
	}
}

func TestGameFleetTemplatesEndpointReturnsTemplates(t *testing.T) {
	fleet := &fakeGameFleet{result: appgame.FleetResult{
		Authenticated: true,
		Fleet: domaingame.Fleet{
			CommanderActive: true,
			TemplateLimit:   4,
			Templates: []domaingame.FleetTemplate{{
				ID:   7,
				Name: "raid wing",
			}},
		},
	}}
	server := testServerWithGameFleet(t, fleet)
	req := httptest.NewRequest(http.MethodGet, "/api/game/fleet-templates?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fleet.command.PublicSession != "public" || fleet.command.PlanetID != 99 || fleet.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected fleet template query command: %+v", fleet.command)
	}
	var response gameFleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Fleet == nil || response.Fleet.Templates.Max != 4 || len(response.Fleet.Templates.Items) != 1 {
		t.Fatalf("unexpected fleet template response: %+v", response.Fleet)
	}
}

func TestGameFleetTemplatesEndpointHandlesUnavailableAndInvalidRequests(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/fleet-templates?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing fleet templates use case to return 503, got %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/game/fleet-templates?session=public", bytes.NewBufferString(`{"action":"save"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing fleet templates mutation use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameFleet(t, &fakeGameFleet{})
	req = httptest.NewRequest(http.MethodGet, "/api/game/fleet-templates?session=public&cp=abc", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}

	server = testServerWithGameFleet(t, &fakeGameFleet{err: errors.New("fleet failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/fleet-templates?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected query use case error to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/fleet-templates?session=public&cp=abc", bytes.NewBufferString(`{"action":"save"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid post selected planet to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/fleet-templates?session=public", bytes.NewBufferString(`{"action":"save"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected mutation use case error to return 503, got %d", rec.Code)
	}
}

func TestGameFleetTemplatesEndpointRejectsBadPayload(t *testing.T) {
	server := testServerWithGameFleet(t, &fakeGameFleet{})
	req := httptest.NewRequest(http.MethodPost, "/api/game/fleet-templates?session=public", strings.NewReader(`{`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad payload to return 400, got %d", rec.Code)
	}
}

func TestGameGalaxyEndpointReturnsGalaxy(t *testing.T) {
	galaxy := &fakeGameGalaxy{result: appgame.GalaxyResult{
		Authenticated: true,
		Galaxy: domaingame.Galaxy{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:          99,
				Name:        "Arakis",
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:          99,
				Name:        "Arakis",
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
				Current:     true,
			}},
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Bounds:      domaingame.GalaxyBounds{Galaxies: 9, Systems: 499},
			Rows: []domaingame.GalaxyRow{{
				Position: 4,
				Planet: &domaingame.GalaxyPlanet{
					ID:           200,
					Name:         "Target",
					DisplayName:  "Target",
					Type:         domaingame.PlanetTypePlanet,
					Coordinates:  domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4},
					ActivityText: "(*)",
					Player: &domaingame.GalaxyPlayerStatus{
						ID:          7,
						Name:        "enemy",
						Rank:        12,
						Status:      "noob",
						StatusClass: "noob",
						Suffixes:    []domaingame.GalaxyStatusSuffix{{Text: "n", Class: "noob"}},
					},
					Alliance: &domaingame.GalaxyAlliance{ID: 5, Tag: "TAG"},
					Actions:  domaingame.GalaxyActions{Spy: true, Message: true, Buddy: true},
				},
				Debris: &domaingame.GalaxyDebris{ID: 201, Metal: 200, Crystal: 100, Harvesters: 1, Visible: true},
			}},
			Populated: 1,
			Slots:     domaingame.FleetSlots{Used: 1, Max: 4, BaseMax: 4},
			Extra:     domaingame.GalaxyExtra{Commander: true, SpyProbes: 4, Recyclers: 3, Missiles: 2, Slots: domaingame.FleetSlots{Used: 1, Max: 4, BaseMax: 4}},
		},
	}}
	server := testServerWithGameGalaxy(t, galaxy)
	req := httptest.NewRequest(http.MethodGet, "/api/game/galaxy?session=public&cp=99&p1=1&p2=2&p3=4", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameGalaxyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Galaxy == nil || response.Galaxy.Commander != "legor" || len(response.Galaxy.Rows) != 1 {
		t.Fatalf("expected authenticated galaxy response, got %+v", response)
	}
	if response.Galaxy.Rows[0].Planet == nil || response.Galaxy.Rows[0].Planet.Player == nil ||
		response.Galaxy.Rows[0].Planet.Player.Suffixes[0].Text != "n" || response.Galaxy.Rows[0].Debris.Harvesters != 1 {
		t.Fatalf("unexpected galaxy row mapping: %+v", response.Galaxy.Rows[0])
	}
	if response.Galaxy.Extra.SpyProbes != 4 || !response.Galaxy.Extra.Commander {
		t.Fatalf("unexpected galaxy extra mapping: %+v", response.Galaxy.Extra)
	}
	if galaxy.command.PublicSession != "public" || galaxy.command.PlanetID != 99 || galaxy.command.Coordinates.Position != 4 ||
		galaxy.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected galaxy command: %+v", galaxy.command)
	}
	if galaxy.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", galaxy.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game galaxy response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameGalaxyEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	galaxy := &fakeGameGalaxy{result: appgame.GalaxyResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameGalaxy(t, galaxy)
	req := httptest.NewRequest(http.MethodGet, "/api/game/galaxy?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameGalaxyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Galaxy != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid galaxy session response, got %+v", response)
	}
}

func TestGameGalaxyEndpointLaunchesMissiles(t *testing.T) {
	galaxy := &fakeGameGalaxy{result: appgame.GalaxyResult{
		Authenticated: true,
		Galaxy: domaingame.Galaxy{
			Commander:   "legor",
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Bounds:      domaingame.GalaxyBounds{Galaxies: 9, Systems: 499},
		},
		ActionIssue: domaingame.GalaxyMissileLaunchedIssue(1),
	}}
	server := testServerWithGameGalaxy(t, galaxy)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/game/galaxy?session=public&cp=99&p1=1&p2=2&p3=4",
		strings.NewReader(`{"action":"launch-missile","targetPlanetId":77,"amount":1,"targetDefenseId":401}`),
	)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response gameGalaxyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ActionIssue == nil || response.ActionIssue.Code != domaingame.GalaxyIssueRocketLaunched {
		t.Fatalf("expected launch action issue, got %+v", response.ActionIssue)
	}
	if galaxy.missile.PlanetID != 99 || galaxy.missile.TargetPlanetID != 77 || galaxy.missile.Amount != 1 ||
		galaxy.missile.TargetDefenseID != domaingame.DefenseRocketLauncher || galaxy.missile.Coordinates.Position != 4 {
		t.Fatalf("unexpected missile command: %+v", galaxy.missile)
	}
	if galaxy.missile.PublicSession != "public" || galaxy.missile.PrivateSessions["prsess_42_1"] != "private" ||
		galaxy.missile.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected missile session command: %+v", galaxy.missile)
	}
}

func TestGameGalaxyEndpointDispatchesInstantFleet(t *testing.T) {
	galaxy := &fakeGameGalaxy{result: appgame.GalaxyResult{
		Authenticated: true,
		Galaxy: domaingame.Galaxy{
			Commander:   "legor",
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Bounds:      domaingame.GalaxyBounds{Galaxies: 9, Systems: 499},
		},
		ActionIssue: domaingame.GalaxyFleetDispatchedIssue(),
	}}
	server := testServerWithGameGalaxy(t, galaxy)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/game/galaxy?session=public&cp=99&p1=1&p2=2&p3=4",
		strings.NewReader(`{"action":"dispatch-spy","targetGalaxy":1,"targetSystem":2,"targetPosition":5,"targetType":1,"amount":3}`),
	)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response gameGalaxyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ActionIssue == nil || response.ActionIssue.Code != domaingame.GalaxyIssueFleetDispatched {
		t.Fatalf("expected dispatch action issue, got %+v", response.ActionIssue)
	}
	if galaxy.dispatch.PlanetID != 99 || galaxy.dispatch.Target.Position != 5 ||
		galaxy.dispatch.TargetType != domaingame.GamePlanetTypePlanet || galaxy.dispatch.Mission != domaingame.FleetMissionSpy ||
		galaxy.dispatch.Amount != 3 || galaxy.dispatch.Coordinates.Position != 4 {
		t.Fatalf("unexpected dispatch command: %+v", galaxy.dispatch)
	}
	if galaxy.dispatch.PublicSession != "public" || galaxy.dispatch.PrivateSessions["prsess_42_1"] != "private" ||
		galaxy.dispatch.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected dispatch session command: %+v", galaxy.dispatch)
	}
}

func TestGameGalaxyEndpointDispatchesRecycleWithDefaultTargetType(t *testing.T) {
	galaxy := &fakeGameGalaxy{result: appgame.GalaxyResult{
		Authenticated: true,
		Galaxy: domaingame.Galaxy{
			Commander:   "legor",
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Bounds:      domaingame.GalaxyBounds{Galaxies: 9, Systems: 499},
		},
		ActionIssue: domaingame.GalaxyFleetDispatchedIssue(),
	}}
	server := testServerWithGameGalaxy(t, galaxy)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/game/galaxy?session=public&cp=99&p1=1&p2=2&p3=4",
		strings.NewReader(`{"action":"dispatch-recycle","targetGalaxy":1,"targetSystem":2,"targetPosition":5,"amount":1}`),
	)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if galaxy.dispatch.Mission != domaingame.FleetMissionRecycle || galaxy.dispatch.TargetType != domaingame.GamePlanetTypeDebris {
		t.Fatalf("unexpected recycle dispatch command: %+v", galaxy.dispatch)
	}
}

func TestGameGalaxyEndpointRejectsBadMutationPayload(t *testing.T) {
	server := testServerWithGameGalaxy(t, &fakeGameGalaxy{})
	for _, body := range []string{`{`, `{"action":"unsupported"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/game/galaxy?session=public", strings.NewReader(body))
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected bad request, got %d", body, rec.Code)
		}
	}
}

func TestGameGalaxyEndpointRejectsInvalidInputs(t *testing.T) {
	server := testServerWithGameGalaxy(t, &fakeGameGalaxy{})
	for _, target := range []string{
		"/api/game/galaxy?session=public&cp=abc",
		"/api/game/galaxy?session=public&galaxy=bad",
		"/api/game/galaxy?session=public&p2=bad",
		"/api/game/galaxy?session=public&p3=bad",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected bad request, got %d", target, rec.Code)
		}
	}

	for _, target := range []string{
		"/api/game/galaxy?session=public&cp=abc",
		"/api/game/galaxy?session=public&p1=bad",
		"/api/game/galaxy?session=public&p2=bad",
		"/api/game/galaxy?session=public&p3=bad",
	} {
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{"action":"launch-missile"}`))
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected post bad request, got %d", target, rec.Code)
		}
	}
}

func TestGameGalaxyMappingHandlesOptionalNilValues(t *testing.T) {
	row := toGameGalaxyRow(domaingame.GalaxyRow{Position: 1})
	if row.Position != 1 || row.Planet != nil || row.Moon != nil || row.Debris != nil {
		t.Fatalf("unexpected empty galaxy row mapping: %+v", row)
	}
	if toGameGalaxyPlanet(nil) != nil || toGameGalaxyPlayerStatus(nil) != nil || toGameGalaxyAlliance(nil) != nil || toGameGalaxyDebris(nil) != nil {
		t.Fatal("expected nil optional mappings to stay nil")
	}
	player := toGameGalaxyPlayerStatus(&domaingame.GalaxyPlayerStatus{ID: 1, Name: "legor"})
	if player == nil || len(player.Suffixes) != 0 {
		t.Fatalf("unexpected no-suffix player mapping: %+v", player)
	}
}

func TestGameGalaxyEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/galaxy?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game galaxy use case to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/galaxy?session=public", strings.NewReader(`{"action":"launch-missile"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game galaxy post use case to return 503, got %d", rec.Code)
	}
}

func TestGameGalaxyEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameGalaxy(t, &fakeGameGalaxy{err: errors.New("galaxy failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/galaxy?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game galaxy error to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/galaxy?session=public", strings.NewReader(`{"action":"launch-missile"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game galaxy post error to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/galaxy?session=public", strings.NewReader(`{"action":"dispatch-spy","targetGalaxy":1,"targetSystem":2,"targetPosition":3,"amount":1}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game galaxy dispatch error to return 503, got %d", rec.Code)
	}
}

func TestGameDefenseEndpointReturnsDefense(t *testing.T) {
	defense := &fakeGameDefense{result: appgame.DefenseResult{
		Authenticated: true,
		Defense: domaingame.Defense{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
			}},
			HasShipyard: true,
			Items: []domaingame.ShipyardItem{{
				ID:               domaingame.DefenseRocketLauncher,
				Name:             "Rocket Launcher",
				Description:      "The rocket launcher is a simple defensive option.",
				Count:            2,
				Cost:             domaingame.BuildingCost{Metal: 2000},
				DurationSeconds:  720,
				CanBuild:         true,
				MeetsRequirement: true,
				MaxBuild:         5,
			}},
		},
	}}
	server := testServerWithGameDefense(t, defense)
	req := httptest.NewRequest(http.MethodGet, "/api/game/defense?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameDefenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Defense == nil || response.Defense.Commander != "legor" || !response.Defense.HasShipyard || len(response.Defense.Items) != 1 || len(response.Defense.PlanetSwitcher) != 1 {
		t.Fatalf("expected authenticated defense response, got %+v", response)
	}
	if response.Defense.Items[0].Cost.Metal != 2000 || response.Defense.Items[0].DurationSeconds != 720 || response.Defense.Items[0].MaxBuild != 5 {
		t.Fatalf("unexpected defense mapping: %+v", response.Defense.Items[0])
	}
	if defense.command.PublicSession != "public" || defense.command.PlanetID != 99 || defense.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected defense command: %+v", defense.command)
	}
	if defense.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", defense.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game defense response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameDefenseEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	defense := &fakeGameDefense{result: appgame.DefenseResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameDefense(t, defense)
	req := httptest.NewRequest(http.MethodGet, "/api/game/defense?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameDefenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Defense != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid defense session response, got %+v", response)
	}
}

func TestGameDefenseEndpointMutatesOrders(t *testing.T) {
	issue := domaingame.BuildingActionIssue(domaingame.BuildingsIssueQueueFull)
	defense := &fakeGameDefense{result: appgame.DefenseResult{
		Authenticated: true,
		ActionIssue:   issue,
		Defense:       domaingame.Defense{Commander: "legor", HasShipyard: true},
	}}
	server := testServerWithGameDefense(t, defense)
	req := httptest.NewRequest(http.MethodPost, "/api/game/defense?session=public&cp=99", strings.NewReader(`{"orders":{"401":4}}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response gameDefenseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.ActionIssue == nil || response.ActionIssue.Code != domaingame.BuildingsIssueQueueFull || response.Defense == nil {
		t.Fatalf("unexpected defense mutation response: %+v", response)
	}
	if defense.mutation.PublicSession != "public" || defense.mutation.PlanetID != 99 || defense.mutation.Orders[domaingame.DefenseRocketLauncher] != 4 {
		t.Fatalf("unexpected defense mutation command: %+v", defense.mutation)
	}
	if defense.mutation.PrivateSessions["prsess_42_1"] != "private" || defense.mutation.RemoteAddr != "203.0.113.10" {
		t.Fatalf("expected private session and remote address to be passed, got %+v", defense.mutation)
	}
}

func TestGameDefenseEndpointPostErrors(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/game/defense?session=public", strings.NewReader(`{"orders":{"401":1}}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing defense use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameDefense(t, &fakeGameDefense{})
	req = httptest.NewRequest(http.MethodPost, "/api/game/defense?session=public&cp=abc", strings.NewReader(`{"orders":{"401":1}}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cp to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/defense?session=public", strings.NewReader(`{bad`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid JSON to return 400, got %d", rec.Code)
	}

	server = testServerWithGameDefense(t, &fakeGameDefense{err: errors.New("defense failed")})
	req = httptest.NewRequest(http.MethodPost, "/api/game/defense?session=public", strings.NewReader(`{"orders":{"401":1}}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected use case error to return 503, got %d", rec.Code)
	}
}

func TestGameDefenseEndpointRejectsInvalidPlanetID(t *testing.T) {
	server := testServerWithGameDefense(t, &fakeGameDefense{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/defense?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameDefenseEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/defense?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game defense use case to return 503, got %d", rec.Code)
	}
}

func TestGameDefenseEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameDefense(t, &fakeGameDefense{err: errors.New("defense failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/defense?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game defense error to return 503, got %d", rec.Code)
	}
}

func TestGameTechnologyEndpointReturnsTechnology(t *testing.T) {
	if toGameTechnologyDetailsResponse(nil) != nil {
		t.Fatal("expected nil technology details to map to nil")
	}
	if toGameTechnologyDemolishResponse(nil) != nil {
		t.Fatal("expected nil technology demolish info to map to nil")
	}

	technology := &fakeGameTechnology{result: appgame.TechnologyResult{
		Authenticated: true,
		Technology: domaingame.Technology{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
			}},
			Groups: []domaingame.TechnologyGroup{{
				Key:  "building",
				Name: "Buildings",
				Items: []domaingame.TechnologyItem{{
					ID:               domaingame.BuildingFusionReactor,
					Name:             "Fusion Reactor",
					DetailsAvailable: true,
					Requirements: []domaingame.TechnologyRequirement{{
						ID:           domaingame.BuildingDeuteriumSynth,
						Name:         "Deuterium Synthesizer",
						Level:        5,
						CurrentLevel: 4,
						Met:          false,
					}},
				}},
			}},
			Details: &domaingame.TechnologyDetails{
				Target: domaingame.TechnologyItem{
					ID:               domaingame.FleetCruiser,
					Name:             "Cruiser",
					DetailsAvailable: true,
				},
				Demolish: &domaingame.TechnologyDemolish{
					Level:           2,
					Cost:            domaingame.BuildingCost{Metal: 2000},
					DurationSeconds: 30,
				},
				Levels: []domaingame.TechnologyDetailsLevel{{
					Step: 1,
					Requirements: []domaingame.TechnologyRequirement{{
						ID:           domaingame.ResearchImpulseDrive,
						Name:         "Impulse Drive",
						Level:        4,
						CurrentLevel: 3,
						Met:          false,
					}},
				}},
			},
		},
	}}
	server := testServerWithGameTechnology(t, technology)
	req := httptest.NewRequest(http.MethodGet, "/api/game/technology?session=public&cp=99&tid=206", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameTechnologyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Technology == nil || response.Technology.Commander != "legor" || len(response.Technology.Groups) != 1 || len(response.Technology.PlanetSwitcher) != 1 {
		t.Fatalf("expected authenticated technology response, got %+v", response)
	}
	requirement := response.Technology.Groups[0].Items[0].Requirements[0]
	if requirement.ID != domaingame.BuildingDeuteriumSynth || requirement.Level != 5 || requirement.CurrentLevel != 4 || requirement.Met {
		t.Fatalf("unexpected technology requirement mapping: %+v", requirement)
	}
	if response.Technology.Details == nil || response.Technology.Details.Target.ID != domaingame.FleetCruiser ||
		response.Technology.Details.Levels[0].Requirements[0].ID != domaingame.ResearchImpulseDrive {
		t.Fatalf("unexpected technology detail mapping: %+v", response.Technology.Details)
	}
	if response.Technology.Details.Demolish == nil || response.Technology.Details.Demolish.Level != 2 ||
		response.Technology.Details.Demolish.Cost.Metal != 2000 || response.Technology.Details.Demolish.DurationSeconds != 30 {
		t.Fatalf("unexpected technology demolish mapping: %+v", response.Technology.Details.Demolish)
	}
	if technology.command.PublicSession != "public" || technology.command.PlanetID != 99 ||
		technology.command.TechnologyID != domaingame.FleetCruiser || technology.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected technology command: %+v", technology.command)
	}
	if technology.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", technology.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game technology response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameTechnologyEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	technology := &fakeGameTechnology{result: appgame.TechnologyResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameTechnology(t, technology)
	req := httptest.NewRequest(http.MethodGet, "/api/game/technology?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameTechnologyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Technology != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid technology session response, got %+v", response)
	}
}

func TestGameTechnologyEndpointRejectsInvalidPlanetID(t *testing.T) {
	server := testServerWithGameTechnology(t, &fakeGameTechnology{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/technology?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameTechnologyEndpointRejectsInvalidTechnologyID(t *testing.T) {
	server := testServerWithGameTechnology(t, &fakeGameTechnology{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/technology?session=public&tid=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected technology to return 400, got %d", rec.Code)
	}
}

func TestGameTechnologyEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/technology?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game technology use case to return 503, got %d", rec.Code)
	}
}

func TestGameTechnologyEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameTechnology(t, &fakeGameTechnology{err: errors.New("technology failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/technology?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game technology error to return 503, got %d", rec.Code)
	}
}

func TestGameStatisticsEndpointReturnsStatistics(t *testing.T) {
	statistics := &fakeGameStatistics{result: appgame.StatisticsResult{
		Authenticated: true,
		Statistics: domaingame.Statistics{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
			}},
			Who:         domaingame.StatisticsWhoPlayer,
			Type:        domaingame.StatisticsTypeResources,
			Start:       1,
			Total:       2,
			GeneratedAt: 123456,
			Rows: []domaingame.StatisticsRow{{
				Place:         1,
				PreviousPlace: 3,
				Score:         950000000,
				ScoreDate:     123400,
				Player:        domaingame.StatisticsPlayer{ID: 42, Name: "legor"},
				Alliance:      &domaingame.StatisticsAlliance{ID: 7, Tag: "TAG"},
				Coordinates:   domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
				Members:       3,
				Own:           true,
				SameAlliance:  true,
			}},
		},
	}}
	server := testServerWithGameStatistics(t, statistics)
	req := httptest.NewRequest(http.MethodGet, "/api/game/statistics?session=public&cp=99&type=ressources&who=player&start=1", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameStatisticsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Statistics == nil || response.Statistics.Commander != "legor" || len(response.Statistics.Rows) != 1 {
		t.Fatalf("expected authenticated statistics response, got %+v", response)
	}
	row := response.Statistics.Rows[0]
	if row.DisplayScore != 950000 || row.Members != 3 || row.PerMember != 316667 || row.Delta != -2 ||
		!row.Own || !row.SameAlliance || row.Alliance == nil || row.Alliance.Tag != "TAG" {
		t.Fatalf("unexpected statistics row mapping: %+v", row)
	}
	if statistics.command.PublicSession != "public" || statistics.command.PlanetID != 99 ||
		statistics.command.Type != "ressources" || statistics.command.Who != "player" || statistics.command.Start != 1 ||
		statistics.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected statistics command: %+v", statistics.command)
	}
	if statistics.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", statistics.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game statistics response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameStatisticsEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	statistics := &fakeGameStatistics{result: appgame.StatisticsResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameStatistics(t, statistics)
	req := httptest.NewRequest(http.MethodGet, "/api/game/statistics?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameStatisticsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Statistics != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid statistics session response, got %+v", response)
	}
}

func TestGameStatisticsEndpointRejectsInvalidQuery(t *testing.T) {
	server := testServerWithGameStatistics(t, &fakeGameStatistics{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/statistics?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/statistics?session=public&start=abc", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid start to return 400, got %d", rec.Code)
	}
}

func TestGameStatisticsEndpointReturnsUnavailable(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/statistics?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game statistics use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameStatistics(t, &fakeGameStatistics{err: errors.New("statistics failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/statistics?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game statistics error to return 503, got %d", rec.Code)
	}
}

func TestGameSearchEndpointReturnsSearch(t *testing.T) {
	search := &fakeGameSearch{result: appgame.SearchResult{
		Authenticated: true,
		Search: domaingame.Search{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
			}},
			Type: domaingame.SearchTypePlayerName,
			Text: "legor",
			PlayerRows: []domaingame.SearchPlayerRow{{
				PlayerID:    42,
				PlayerName:  "legor",
				Alliance:    &domaingame.StatisticsAlliance{ID: 7, Tag: "TAG"},
				PlanetID:    99,
				PlanetName:  "Arakis",
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
				Place:       101,
				Own:         true,
			}},
			AllianceRows: []domaingame.SearchAllianceRow{{
				AllianceID: 7,
				Tag:        "TAG",
				Name:       "The Alliance",
				Members:    3,
				Score:      950000000,
				Own:        true,
			}},
		},
	}}
	server := testServerWithGameSearch(t, search)
	req := httptest.NewRequest(http.MethodGet, "/api/game/search?session=public&cp=99&type=playername&searchtext=legor", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameSearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Search == nil || response.Search.Commander != "legor" || len(response.Search.PlayerRows) != 1 || len(response.Search.AllianceRows) != 1 {
		t.Fatalf("expected authenticated search response, got %+v", response)
	}
	player := response.Search.PlayerRows[0]
	if player.PlayerName != "legor" || !player.Own || player.Alliance == nil || player.Alliance.Tag != "TAG" || player.Coordinates.Galaxy != 1 {
		t.Fatalf("unexpected search player row mapping: %+v", player)
	}
	alliance := response.Search.AllianceRows[0]
	if alliance.Tag != "TAG" || alliance.DisplayScore != 950000 || alliance.Members != 3 || !alliance.Own {
		t.Fatalf("unexpected search alliance row mapping: %+v", alliance)
	}
	if search.command.PublicSession != "public" || search.command.PlanetID != 99 || search.command.Type != "playername" ||
		search.command.Text != "legor" || search.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected search command: %+v", search.command)
	}
	if search.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", search.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game search response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameSearchEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	search := &fakeGameSearch{result: appgame.SearchResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameSearch(t, search)
	req := httptest.NewRequest(http.MethodGet, "/api/game/search?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameSearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Search != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid search session response, got %+v", response)
	}
}

func TestGameSearchEndpointRejectsInvalidPlanet(t *testing.T) {
	server := testServerWithGameSearch(t, &fakeGameSearch{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/search?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameSearchEndpointReturnsUnavailable(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/search?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game search use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameSearch(t, &fakeGameSearch{err: errors.New("search failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/search?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game search error to return 503, got %d", rec.Code)
	}
}

func TestGameBuddyEndpointReturnsBuddy(t *testing.T) {
	buddy := &fakeGameBuddy{result: appgame.BuddyResult{
		Authenticated: true,
		Buddy: domaingame.Buddy{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Current: true,
			}},
			Action: domaingame.BuddyActionIncoming,
			Rows: []domaingame.BuddyRow{{
				BuddyID: 11,
				Player: domaingame.BuddyPlayer{
					PlayerID:    43,
					Name:        "target",
					Alliance:    &domaingame.BuddyAlliance{ID: 7, Tag: "TAG", Founder: true},
					Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4},
				},
				Text:   "hello",
				Status: domaingame.BuddyStatus{Text: "On", Color: "lime"},
			}},
			Target: &domaingame.BuddyPlayer{
				PlayerID:    44,
				Name:        "request target",
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 5},
			},
		},
	}}
	server := testServerWithGameBuddy(t, buddy)
	req := httptest.NewRequest(http.MethodGet, "/api/game/buddy?session=public&cp=99&action=5&buddy_id=11", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameBuddyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Buddy == nil || response.Buddy.Commander != "legor" || response.Buddy.Action != domaingame.BuddyActionIncoming || len(response.Buddy.Rows) != 1 {
		t.Fatalf("expected authenticated buddy response, got %+v", response)
	}
	row := response.Buddy.Rows[0]
	if row.BuddyID != 11 || row.Player.Name != "target" || row.Player.Alliance == nil || !row.Player.Alliance.Founder ||
		row.Player.Coordinates.Position != 4 || row.Text != "hello" || row.Status.Color != "lime" || response.Buddy.Target == nil {
		t.Fatalf("unexpected buddy row mapping: %+v target=%+v", row, response.Buddy.Target)
	}
	if buddy.command.PublicSession != "public" || buddy.command.PlanetID != 99 || buddy.command.Action != 5 ||
		buddy.command.BuddyID != 11 || buddy.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected buddy command: %+v", buddy.command)
	}
	if buddy.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", buddy.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game buddy response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameBuddyEndpointMutatesBuddy(t *testing.T) {
	buddy := &fakeGameBuddy{result: appgame.BuddyResult{
		Authenticated: true,
		ActionIssue:   domaingame.BuddyAlreadySentIssue(),
		Buddy: domaingame.Buddy{
			Commander: "legor",
			Action:    domaingame.BuddyActionHome,
		},
	}}
	server := testServerWithGameBuddy(t, buddy)
	req := httptest.NewRequest(http.MethodPost, "/api/game/buddy?session=public&cp=99", strings.NewReader(`{"action":1,"buddyId":43,"text":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameBuddyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Buddy == nil || response.ActionIssue == nil ||
		response.ActionIssue.Code != domaingame.BuddyIssueAlreadySent {
		t.Fatalf("expected authenticated buddy mutation response with action issue, got %+v", response)
	}
	if buddy.mutation.PublicSession != "public" || buddy.mutation.PlanetID != 99 || buddy.mutation.Action != 1 ||
		buddy.mutation.BuddyID != 43 || buddy.mutation.Text != "hello" || buddy.mutation.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected buddy mutation command: %+v", buddy.mutation)
	}
	if buddy.mutation.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", buddy.mutation.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game buddy mutation response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameBuddyEndpointMutationReturnsUnauthorizedForInvalidSession(t *testing.T) {
	buddy := &fakeGameBuddy{result: appgame.BuddyResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameBuddy(t, buddy)
	req := httptest.NewRequest(http.MethodPost, "/api/game/buddy?session=public", strings.NewReader(`{"action":2,"buddyId":1}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameBuddyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Buddy != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid buddy mutation session response, got %+v", response)
	}
}

func TestGameBuddyEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	buddy := &fakeGameBuddy{result: appgame.BuddyResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameBuddy(t, buddy)
	req := httptest.NewRequest(http.MethodGet, "/api/game/buddy?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameBuddyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Buddy != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid buddy session response, got %+v", response)
	}
}

func TestGameBuddyEndpointRejectsInvalidQuery(t *testing.T) {
	server := testServerWithGameBuddy(t, &fakeGameBuddy{})
	tests := []string{
		"/api/game/buddy?session=public&cp=abc",
		"/api/game/buddy?session=public&action=bad",
		"/api/game/buddy?session=public&buddy_id=bad",
	}
	for _, target := range tests {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %s to return 400, got %d", target, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/game/buddy?session=public", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid buddy mutation JSON to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/buddy?session=public&cp=abc", strings.NewReader(`{"action":1}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid buddy mutation selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameBuddyEndpointReturnsUnavailable(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/buddy?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game buddy use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameBuddy(t, &fakeGameBuddy{err: errors.New("buddy failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/buddy?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game buddy error to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/buddy?session=public", strings.NewReader(`{"action":1}`))
	rec = httptest.NewRecorder()
	testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()}).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game buddy mutation use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameBuddy(t, &fakeGameBuddy{err: errors.New("buddy failed")})
	req = httptest.NewRequest(http.MethodPost, "/api/game/buddy?session=public", strings.NewReader(`{"action":1}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game buddy mutation error to return 503, got %d", rec.Code)
	}
}

func TestGameMessagesEndpointReturnsInbox(t *testing.T) {
	messages := &fakeGameMessages{result: appgame.MessagesResult{
		Authenticated: true,
		Messages: domaingame.Messages{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:          99,
				Name:        "Arakis",
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:          99,
				Name:        "Arakis",
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
				Current:     true,
			}},
			Action: domaingame.MessagesActionInbox,
			Rows: []domaingame.Message{{
				ID:         11,
				Type:       domaingame.MessageTypePM,
				From:       "Sender",
				Subject:    "Subject",
				Text:       "<b>Body</b>",
				Date:       1700000000,
				Unread:     true,
				Reportable: true,
			}},
		},
	}}
	server := testServerWithGameMessages(t, messages)
	req := httptest.NewRequest(http.MethodGet, "/api/game/messages?session=public&cp=99&messageziel=77&betreff=Re%3ASubject", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameMessagesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Messages == nil || response.Messages.Commander != "legor" || len(response.Messages.Rows) != 1 {
		t.Fatalf("expected authenticated messages response, got %+v", response)
	}
	row := response.Messages.Rows[0]
	if row.Subject != "Subject" || row.Text != "<b>Body</b>" || !row.Reportable {
		t.Fatalf("unexpected message row mapping: %+v", row)
	}
	if messages.command.PublicSession != "public" || messages.command.PlanetID != 99 || messages.command.TargetPlayerID != 77 ||
		messages.command.Subject != "Re:Subject" || messages.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected messages command: %+v", messages.command)
	}
	if messages.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", messages.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game messages response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameMessagesEndpointReturnsComposeTarget(t *testing.T) {
	messages := &fakeGameMessages{result: appgame.MessagesResult{
		Authenticated: true,
		Messages: domaingame.Messages{
			Action: domaingame.MessagesActionCompose,
			Compose: &domaingame.MessageCompose{
				Target:   domaingame.MessageTarget{PlayerID: 77, Name: "Target", Coordinates: domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4}},
				Subject:  "no subject",
				MaxChars: domaingame.MessageComposeMaxChars,
			},
		},
	}}
	server := testServerWithGameMessages(t, messages)
	req := httptest.NewRequest(http.MethodGet, "/api/game/messages?session=public&messageziel=77", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameMessagesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Messages == nil || response.Messages.Action != "compose" || response.Messages.Compose == nil || response.Messages.Compose.Target.PlayerID != 77 {
		t.Fatalf("unexpected compose messages response: %+v", response)
	}
}

func TestGameMessagesEndpointMutatesMessages(t *testing.T) {
	messages := &fakeGameMessages{result: appgame.MessagesResult{
		Authenticated: true,
		ActionIssue:   domaingame.MessageSentIssue(),
		Messages: domaingame.Messages{
			Action: domaingame.MessagesActionCompose,
			Compose: &domaingame.MessageCompose{
				Target:   domaingame.MessageTarget{PlayerID: 77, Name: "Target", Coordinates: domaingame.Coordinates{Galaxy: 2, System: 3, Position: 4}},
				Subject:  "no subject",
				MaxChars: domaingame.MessageComposeMaxChars,
			},
		},
	}}
	server := testServerWithGameMessages(t, messages)
	req := httptest.NewRequest(http.MethodPost, "/api/game/messages?session=public&cp=99", strings.NewReader(`{"action":"send","targetPlayerId":77,"subject":"Hi","text":"Body","deleteMode":"deletemarked","messageIds":[1,2],"reportIds":[3]}`))
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameMessagesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ActionIssue == nil || response.ActionIssue.Code != domaingame.MessageIssueSent || response.Messages == nil || response.Messages.Compose == nil {
		t.Fatalf("unexpected mutation response: %+v", response)
	}
	if messages.mutationCommand.Action != "send" || messages.mutationCommand.TargetPlayerID != 77 ||
		messages.mutationCommand.Subject != "Hi" || messages.mutationCommand.Text != "Body" ||
		messages.mutationCommand.DeleteMode != "deletemarked" || len(messages.mutationCommand.MessageIDs) != 2 ||
		len(messages.mutationCommand.ReportIDs) != 1 || messages.mutationCommand.PlanetID != 99 ||
		messages.mutationCommand.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected mutation command: %+v", messages.mutationCommand)
	}
}

func TestGameMessagesEndpointPostReturnsUnauthorizedForInvalidSession(t *testing.T) {
	messages := &fakeGameMessages{result: appgame.MessagesResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameMessages(t, messages)
	req := httptest.NewRequest(http.MethodPost, "/api/game/messages?session=public", strings.NewReader(`{"action":"delete"}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameMessagesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Messages != nil || len(response.Issues) != 1 {
		t.Fatalf("expected unauthenticated mutation response, got %+v", response)
	}
}

func TestGameMessagesEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	messages := &fakeGameMessages{result: appgame.MessagesResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameMessages(t, messages)
	req := httptest.NewRequest(http.MethodGet, "/api/game/messages?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameMessagesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Messages != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid messages session response, got %+v", response)
	}
}

func TestGameMessagesEndpointRejectsInvalidQuery(t *testing.T) {
	for _, target := range []string{
		"/api/game/messages?session=public&cp=abc",
		"/api/game/messages?session=public&messageziel=bad",
	} {
		server := testServerWithGameMessages(t, &fakeGameMessages{})
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, rec.Code)
		}
	}

	server := testServerWithGameMessages(t, &fakeGameMessages{})
	req := httptest.NewRequest(http.MethodPost, "/api/game/messages?session=public&cp=abc", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid post selected planet to return 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/messages?session=public", strings.NewReader(`{bad`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid post json to return 400, got %d", rec.Code)
	}
}

func TestGameMessagesEndpointReturnsUnavailable(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/messages?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game messages use case to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/messages?session=public", strings.NewReader(`{"action":"delete"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game messages mutation use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameMessages(t, &fakeGameMessages{err: errors.New("messages failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/messages?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game messages error to return 503, got %d", rec.Code)
	}

	server = testServerWithGameMessages(t, &fakeGameMessages{err: errors.New("messages failed")})
	req = httptest.NewRequest(http.MethodPost, "/api/game/messages?session=public", strings.NewReader(`{"action":"delete"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game messages mutation error to return 503, got %d", rec.Code)
	}
}

func TestGameReportEndpointReturnsReport(t *testing.T) {
	reportUseCase := &fakeGameReport{result: appgame.ReportResult{
		Authenticated: true,
		Report:        domaingame.NewReport(11, domaingame.MessageTypeSpyReport, "<table>spy</table>", true),
	}}
	server := testServerWithGameReport(t, reportUseCase)
	req := httptest.NewRequest(http.MethodGet, "/api/game/report?session=public&bericht=11", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameReportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Report == nil || response.Report.Title != domaingame.ReportTitleSpy || response.Report.Text != "<table>spy</table>" {
		t.Fatalf("unexpected report response: %+v", response)
	}
	if reportUseCase.command.PublicSession != "public" || reportUseCase.command.ReportID != 11 ||
		reportUseCase.command.RemoteAddr != "203.0.113.10" || reportUseCase.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected report command: %+v", reportUseCase.command)
	}
}

func TestGameReportEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	reportUseCase := &fakeGameReport{result: appgame.ReportResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameReport(t, reportUseCase)
	req := httptest.NewRequest(http.MethodGet, "/api/game/report?session=public&report=11", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameReportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Report != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid report session response, got %+v", response)
	}
}

func TestGameReportEndpointRejectsInvalidID(t *testing.T) {
	for _, target := range []string{"/api/game/report?session=public", "/api/game/report?session=public&bericht=bad", "/api/game/report?session=public&bericht=0"} {
		server := testServerWithGameReport(t, &fakeGameReport{})
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, rec.Code)
		}
	}
}

func TestGameReportEndpointReturnsUnavailable(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/report?session=public&bericht=11", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game report use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameReport(t, &fakeGameReport{err: errors.New("report failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/report?session=public&bericht=11", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game report error to return 503, got %d", rec.Code)
	}
}

func TestGameOptionsEndpointReturnsOptions(t *testing.T) {
	optionsUseCase := &fakeGameOptions{result: appgame.OptionsResult{
		Authenticated: true,
		Options:       sampleGameOptions(),
	}}
	server := testServerWithGameOptions(t, optionsUseCase)
	req := httptest.NewRequest(http.MethodGet, "/api/game/options?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameOptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Options == nil || response.Options.User.Name != "Legor" || response.Options.Settings.MaxSpy != 5 {
		t.Fatalf("unexpected options response: %+v", response)
	}
	if optionsUseCase.command.PublicSession != "public" || optionsUseCase.command.PlanetID != 99 ||
		optionsUseCase.command.RemoteAddr != "203.0.113.10" || optionsUseCase.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected options command: %+v", optionsUseCase.command)
	}
}

func TestGameOptionsEndpointUpdatesOptionsFromJSON(t *testing.T) {
	optionsUseCase := &fakeGameOptions{updateResult: appgame.OptionsResult{
		Authenticated: true,
		Options:       sampleGameOptions(),
		ActionIssue:   domaingame.OptionsSavedIssue(),
	}}
	server := testServerWithGameOptions(t, optionsUseCase)
	body := strings.NewReader(`{"language":"fr","skinPath":"http://127.0.0.1:8890/evolution","useSkin":true,"deactivateIp":true,"sortBy":2,"sortOrder":1,"maxSpy":9,"maxFleetMessages":11,"oldPassword":"oldpass123","newPassword":"newpass123","newPasswordRepeat":"newpass123","email":"new@example.test","vacationMode":true,"deleteAccount":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/game/options?session=public&cp=99", body)
	req.Host = "10.8.0.2:8890"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if optionsUseCase.updateCommand.Mutation.SkinPath != "/evolution/" ||
		optionsUseCase.updateCommand.Mutation.SortBy != 2 ||
		optionsUseCase.updateCommand.Mutation.MaxSpy != 9 ||
		optionsUseCase.updateCommand.Mutation.OldPassword != "oldpass123" ||
		optionsUseCase.updateCommand.Mutation.NewPassword != "newpass123" ||
		optionsUseCase.updateCommand.Mutation.NewPasswordRepeat != "newpass123" ||
		optionsUseCase.updateCommand.Mutation.Email != "new@example.test" ||
		!optionsUseCase.updateCommand.Mutation.VacationMode ||
		!optionsUseCase.updateCommand.Mutation.VacationModeSet ||
		!optionsUseCase.updateCommand.Mutation.DeleteAccount {
		t.Fatalf("unexpected update command: %+v", optionsUseCase.updateCommand)
	}
	var response gameOptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ActionIssue == nil || response.ActionIssue.Code != domaingame.OptionsIssueSaved {
		t.Fatalf("expected saved action issue, got %+v", response)
	}
}

func TestGameOptionsEndpointUpdatesOptionsFromLegacyForm(t *testing.T) {
	optionsUseCase := &fakeGameOptions{updateResult: appgame.OptionsResult{Authenticated: true, Options: sampleGameOptions()}}
	server := testServerWithGameOptions(t, optionsUseCase)
	body := strings.NewReader("lang=english&dpath=http%3A%2F%2F10.8.0.2%3A8890%2Fevolution&design=on&noipcheck=on&settings_sort=9999&settings_order=-9999&spio_anz=-42&settings_fleetactions=99999&db_password=oldpass123&newpass1=newpass123&newpass2=newpass123&db_email=new%40example.test&urlaubs_modus=on&db_deaktjava=on")
	req := httptest.NewRequest(http.MethodPost, "/api/game/options?session=public", body)
	req.Host = "10.8.0.2:8890"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	mutation := optionsUseCase.updateCommand.Mutation
	if mutation.Language != "english" || mutation.SkinPath != "/evolution/" ||
		mutation.SortBy != 9999 || mutation.SortOrder != -9999 ||
		mutation.MaxSpy != -42 || mutation.MaxFleetMessages != 99999 ||
		mutation.OldPassword != "oldpass123" || mutation.NewPassword != "newpass123" ||
		mutation.NewPasswordRepeat != "newpass123" || mutation.Email != "new@example.test" ||
		!mutation.UseSkin || !mutation.DeactivateIP || !mutation.VacationMode || !mutation.VacationModeSet || !mutation.DeleteAccount {
		t.Fatalf("unexpected form mutation before domain normalization: %+v", mutation)
	}
}

func TestGameOptionsEndpointRejectsInvalidMethodAndSelectedPlanet(t *testing.T) {
	server := testServerWithGameOptions(t, &fakeGameOptions{})
	req := httptest.NewRequest(http.MethodPut, "/api/game/options?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected PUT 405 with Allow header, got %d %q", rec.Code, rec.Header().Get("Allow"))
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/options?session=public&cp=-1", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid POST selected planet 400, got %d", rec.Code)
	}
}

func TestGameOptionsEndpointReturnsUnauthorizedAndErrors(t *testing.T) {
	optionsUseCase := &fakeGameOptions{result: appgame.OptionsResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameOptions(t, optionsUseCase)
	req := httptest.NewRequest(http.MethodGet, "/api/game/options?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	optionsUseCase = &fakeGameOptions{updateResult: appgame.OptionsResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server = testServerWithGameOptions(t, optionsUseCase)
	req = httptest.NewRequest(http.MethodPost, "/api/game/options?session=public", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected POST 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/options?session=public", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid JSON 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/options?session=public&cp=bad", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet 400, got %d", rec.Code)
	}

	server = testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req = httptest.NewRequest(http.MethodGet, "/api/game/options?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing use case 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/options?session=public", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing POST use case 503, got %d", rec.Code)
	}

	server = testServerWithGameOptions(t, &fakeGameOptions{err: errors.New("options failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/options?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected use case error 503, got %d", rec.Code)
	}

	server = testServerWithGameOptions(t, &fakeGameOptions{updateErr: errors.New("options update failed")})
	req = httptest.NewRequest(http.MethodPost, "/api/game/options?session=public", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected update use case error 503, got %d", rec.Code)
	}
}

func TestGameOptionsRequestHostPortHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/game/options", nil)
	req.Host = "[::1]:8890"
	host, port := requestHostPort(req)
	if host != "::1" || port != 8890 {
		t.Fatalf("unexpected IPv6 host/port: %q %d", host, port)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/options", nil)
	req.Host = "[::1]"
	host, port = requestHostPort(req)
	if host != "::1" || port != 80 {
		t.Fatalf("unexpected IPv6 default host/port: %q %d", host, port)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/game/options", nil)
	req.Host = ""
	req.TLS = &tls.ConnectionState{}
	host, port = requestHostPort(req)
	if host != "" || port != 443 {
		t.Fatalf("unexpected TLS default host/port: %q %d", host, port)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/options", strings.NewReader("flag=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if formChecked(req, "flag") || formLast(req, "missing") != "" {
		t.Fatalf("unexpected form helper result: checked=%v last=%q", formChecked(req, "flag"), formLast(req, "missing"))
	}
}

func TestGameNotesEndpointReturnsNotes(t *testing.T) {
	notes := &fakeGameNotes{result: appgame.NotesResult{
		Authenticated: true,
		Notes: domaingame.Notes{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:          99,
				Name:        "Arakis",
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:          99,
				Name:        "Arakis",
				Type:        domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
				Current:     true,
			}},
			Action: domaingame.NotesActionList,
			Rows: []domaingame.Note{{
				ID:       11,
				Subject:  "Important",
				Text:     "Remember this",
				TextSize: 13,
				Priority: 2,
				Date:     1700000000,
			}},
		},
	}}
	server := testServerWithGameNotes(t, notes)
	req := httptest.NewRequest(http.MethodGet, "/api/game/notes?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameNotesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Notes == nil || response.Notes.Commander != "legor" || len(response.Notes.Rows) != 1 {
		t.Fatalf("expected authenticated notes response, got %+v", response)
	}
	row := response.Notes.Rows[0]
	if row.Subject != "Important" || row.PriorityColor != "red" || row.Date != 1700000000 {
		t.Fatalf("unexpected notes row mapping: %+v", row)
	}
	if notes.command.PublicSession != "public" || notes.command.PlanetID != 99 || notes.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected notes command: %+v", notes.command)
	}
	if notes.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", notes.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game notes response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameNotesEndpointReturnsEditNote(t *testing.T) {
	notes := &fakeGameNotes{result: appgame.NotesResult{
		Authenticated: true,
		Notes: domaingame.Notes{
			Commander: "legor",
			Action:    domaingame.NotesActionEdit,
			EditNote:  &domaingame.Note{ID: 11, Subject: "Subject", Text: "Body", TextSize: 4, Priority: 0, Date: 1700000000},
		},
	}}
	server := testServerWithGameNotes(t, notes)
	req := httptest.NewRequest(http.MethodGet, "/api/game/notes?session=public&a=2&n=11", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameNotesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Notes == nil || response.Notes.Action != "edit" || response.Notes.EditNote == nil || response.Notes.EditNote.PriorityColor != "lime" {
		t.Fatalf("unexpected edit notes response: %+v", response)
	}
	if notes.command.Action != 2 || notes.command.NoteID != 11 {
		t.Fatalf("unexpected edit command: %+v", notes.command)
	}
}

func TestGameNotesEndpointMutatesNotes(t *testing.T) {
	for _, tt := range []struct {
		name       string
		body       string
		wantAction string
		assert     func(t *testing.T, notes *fakeGameNotes)
	}{
		{
			name:       "create",
			body:       `{"action":"create","subject":"Subject","text":"Body","priority":2}`,
			wantAction: "create",
			assert: func(t *testing.T, notes *fakeGameNotes) {
				t.Helper()
				if notes.createCommand.Subject != "Subject" || notes.createCommand.Text != "Body" || notes.createCommand.Priority != 2 {
					t.Fatalf("unexpected create command: %+v", notes.createCommand)
				}
			},
		},
		{
			name:       "update",
			body:       `{"action":"update","noteId":11,"subject":"Subject","text":"Body","priority":1}`,
			wantAction: "update",
			assert: func(t *testing.T, notes *fakeGameNotes) {
				t.Helper()
				if notes.updateCommand.NoteID != 11 || notes.updateCommand.Priority != 1 {
					t.Fatalf("unexpected update command: %+v", notes.updateCommand)
				}
			},
		},
		{
			name:       "delete",
			body:       `{"action":"delete","noteIds":[11,12]}`,
			wantAction: "delete",
			assert: func(t *testing.T, notes *fakeGameNotes) {
				t.Helper()
				if len(notes.deleteCommand.NoteIDs) != 2 || notes.deleteCommand.NoteIDs[1] != 12 {
					t.Fatalf("unexpected delete command: %+v", notes.deleteCommand)
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			notes := &fakeGameNotes{result: appgame.NotesResult{
				Authenticated: true,
				Notes: domaingame.Notes{
					Commander: "legor",
					Action:    domaingame.NotesActionList,
					Rows:      []domaingame.Note{{ID: 11, Subject: "Subject", TextSize: 4, Priority: 2, Date: 1700000000}},
				},
			}}
			server := testServerWithGameNotes(t, notes)
			req := httptest.NewRequest(http.MethodPost, "/api/game/notes?session=public&cp=99", strings.NewReader(tt.body))
			req.RemoteAddr = "203.0.113.10:4321"
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			var response gameNotesResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if !response.Authenticated || response.Notes == nil || len(response.Notes.Rows) != 1 {
				t.Fatalf("unexpected notes mutation response: %+v", response)
			}
			tt.assert(t, notes)
		})
	}
}

func TestGameNotesEndpointMutationRejectsInvalidRequests(t *testing.T) {
	for _, tt := range []struct {
		name   string
		target string
		body   string
	}{
		{"malformed", "/api/game/notes?session=public", `{`},
		{"unknown action", "/api/game/notes?session=public", `{"action":"bogus"}`},
		{"missing update note", "/api/game/notes?session=public", `{"action":"update"}`},
		{"invalid planet", "/api/game/notes?session=public&cp=bad", `{"action":"create"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := testServerWithGameNotes(t, &fakeGameNotes{})
			req := httptest.NewRequest(http.MethodPost, tt.target, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
		})
	}
}

func TestGameNotesEndpointMutationReturnsUnauthorizedForInvalidSession(t *testing.T) {
	notes := &fakeGameNotes{result: appgame.NotesResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameNotes(t, notes)
	req := httptest.NewRequest(http.MethodPost, "/api/game/notes?session=public", strings.NewReader(`{"action":"create"}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameNotesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Notes != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid notes mutation session response, got %+v", response)
	}
}

func TestGameNotesEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	notes := &fakeGameNotes{result: appgame.NotesResult{
		Authenticated: false,
		Issues:        []domainpublicsite.SessionIssue{{Code: "missing", Message: "missing session"}},
	}}
	server := testServerWithGameNotes(t, notes)
	req := httptest.NewRequest(http.MethodGet, "/api/game/notes?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameNotesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Notes != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid notes session response, got %+v", response)
	}
}

func TestGameNotesEndpointRejectsInvalidQuery(t *testing.T) {
	for _, target := range []string{
		"/api/game/notes?session=public&cp=abc",
		"/api/game/notes?session=public&a=9",
		"/api/game/notes?session=public&a=2",
		"/api/game/notes?session=public&n=bad",
	} {
		server := testServerWithGameNotes(t, &fakeGameNotes{})
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", target, rec.Code)
		}
	}
}

func TestGameNotesEndpointReturnsUnavailable(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/notes?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game notes use case to return 503, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/notes?session=public", strings.NewReader(`{"action":"create"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game notes mutation use case to return 503, got %d", rec.Code)
	}

	server = testServerWithGameNotes(t, &fakeGameNotes{err: errors.New("notes failed")})
	req = httptest.NewRequest(http.MethodGet, "/api/game/notes?session=public", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game notes error to return 503, got %d", rec.Code)
	}

	server = testServerWithGameNotes(t, &fakeGameNotes{err: errors.New("notes failed")})
	req = httptest.NewRequest(http.MethodPost, "/api/game/notes?session=public", strings.NewReader(`{"action":"create"}`))
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game notes mutation error to return 503, got %d", rec.Code)
	}
}

func TestGameResourcesEndpointReturnsResources(t *testing.T) {
	resources := &fakeGameResources{result: appgame.ResourcesResult{
		Authenticated: true,
		Resources: domaingame.ResourceProduction{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
				Coordinates: domaingame.Coordinates{
					Galaxy:   1,
					System:   2,
					Position: 3,
				},
				Resources: domaingame.Resources{
					Metal:             1000,
					Crystal:           500,
					Deuterium:         100,
					MetalCapacity:     100000,
					CrystalCapacity:   150000,
					DeuteriumCapacity: 200000,
				},
			},
			PlanetSwitcher: []domaingame.PlanetSummary{{
				ID:      99,
				Name:    "Arakis",
				Type:    domaingame.PlanetTypePlanet,
				Current: true,
			}},
			Factor: 1,
			Natural: domaingame.ResourceProductionValues{
				Metal:   20,
				Crystal: 10,
			},
			Rows: []domaingame.ResourceProductionRow{
				{
					ID:         domaingame.BuildingMetalMine,
					Name:       "Metal Mine",
					Level:      2,
					Percent:    100,
					Values:     domaingame.ResourceProductionValues{Metal: 72, Energy: -25, EnergyRaw: -25},
					BonusIcons: []domaingame.ResourceProductionBonusIcon{{Image: "geologe_ikon.gif", Alt: "Geologist"}},
				},
				{
					ID:      domaingame.BuildingCrystalMine,
					Name:    "Crystal Mine",
					Level:   1,
					Percent: 0,
					Values:  domaingame.ResourceProductionValues{},
				},
			},
			Totals: domaingame.ResourceProductionTotals{
				Hour: domaingame.ResourceProductionValues{Metal: 92, Crystal: 10, Energy: -25, EnergyRaw: -25},
				Day:  domaingame.ResourceProductionValues{Metal: 2208, Crystal: 240, Energy: -25, EnergyRaw: -25},
				Week: domaingame.ResourceProductionValues{Metal: 15456, Crystal: 1680, Energy: -25, EnergyRaw: -25},
			},
		},
	}}
	server := testServerWithGameResources(t, resources)
	req := httptest.NewRequest(http.MethodGet, "/api/game/resources?session=public&cp=99", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response gameResourceProductionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Resources == nil || response.Resources.Commander != "legor" || len(response.Resources.Rows) != 2 {
		t.Fatalf("expected authenticated resources response, got %+v", response)
	}
	if len(response.Resources.PlanetSwitcher) != 1 || !response.Resources.PlanetSwitcher[0].Current {
		t.Fatalf("expected planet switcher mapping, got %+v", response.Resources.PlanetSwitcher)
	}
	if response.Resources.Rows[0].Values.Metal != 72 || response.Resources.Totals.Day.Metal != 2208 {
		t.Fatalf("unexpected resources mapping: %+v", response.Resources)
	}
	if len(response.Resources.Rows[0].BonusIcons) != 1 || response.Resources.Rows[0].BonusIcons[0].Image != "geologe_ikon.gif" {
		t.Fatalf("expected resources bonus icon mapping, got %+v", response.Resources.Rows[0].BonusIcons)
	}
	if response.Resources.Rows[1].BonusIcons != nil {
		t.Fatalf("expected empty resources bonus icon mapping to stay nil, got %+v", response.Resources.Rows[1].BonusIcons)
	}
	if resources.command.PublicSession != "public" || resources.command.PlanetID != 99 || resources.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected resources command: %+v", resources.command)
	}
	if resources.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", resources.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game resources response must not echo private session: %s", rec.Body.String())
	}
}

func TestGameResourcesEndpointUpdatesProduction(t *testing.T) {
	resources := &fakeGameResources{updateResult: appgame.ResourcesResult{
		Authenticated: true,
		Resources: domaingame.ResourceProduction{
			Commander: "legor",
			CurrentPlanet: domaingame.PlanetOverview{
				ID:   99,
				Name: "Arakis",
				Type: domaingame.PlanetTypePlanet,
			},
			Factor: 1,
		},
	}}
	server := testServerWithGameResources(t, resources)
	req := httptest.NewRequest(http.MethodPost, "/api/game/resources?session=public&cp=99", strings.NewReader(`{"production":{"1":"-250","2":"not-a-number","3":35,"4":100,"9999":70}}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4321"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response gameResourceProductionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.Resources == nil || response.Resources.Commander != "legor" {
		t.Fatalf("expected authenticated resources update response, got %+v", response)
	}
	if resources.updateCommand.PublicSession != "public" || resources.updateCommand.PlanetID != 99 || resources.updateCommand.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected resources update command: %+v", resources.updateCommand)
	}
	if resources.updateCommand.Production[domaingame.BuildingMetalMine] != -250 ||
		resources.updateCommand.Production[domaingame.BuildingCrystalMine] != 0 ||
		resources.updateCommand.Production[domaingame.BuildingDeuteriumSynth] != 35 ||
		resources.updateCommand.Production[domaingame.BuildingSolarPlant] != 100 ||
		resources.updateCommand.Production[9999] != 70 {
		t.Fatalf("unexpected parsed production settings: %+v", resources.updateCommand.Production)
	}
}

func TestGameResourcesEndpointUpdatesProductionFromLegacyForm(t *testing.T) {
	resources := &fakeGameResources{updateResult: appgame.ResourcesResult{Authenticated: true}}
	server := testServerWithGameResources(t, resources)
	req := httptest.NewRequest(http.MethodPost, "/api/game/resources?session=public", strings.NewReader("last1=35&last2=bad&action=Calculate"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if resources.updateCommand.Production[domaingame.BuildingMetalMine] != 35 || resources.updateCommand.Production[domaingame.BuildingCrystalMine] != 0 {
		t.Fatalf("unexpected form production settings: %+v", resources.updateCommand.Production)
	}
}

func TestGameResourcesEndpointRejectsInvalidProductionUpdate(t *testing.T) {
	server := testServerWithGameResources(t, &fakeGameResources{updateErr: domaingame.ErrProductionPercentTooHigh})
	req := httptest.NewRequest(http.MethodPost, "/api/game/resources?session=public", strings.NewReader(`{"production":{"1":101}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid production update to return 400, got %d", rec.Code)
	}
}

func TestGameResourcesEndpointReturnsUnauthorizedForInvalidUpdateSession(t *testing.T) {
	resources := &fakeGameResources{updateResult: appgame.ResourcesResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameResources(t, resources)
	req := httptest.NewRequest(http.MethodPost, "/api/game/resources?session=public", strings.NewReader(`{"production":{"1":50}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameResourceProductionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Resources != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid resources update session response, got %+v", response)
	}
}

func TestGameResourcesEndpointRejectsMalformedProductionUpdate(t *testing.T) {
	server := testServerWithGameResources(t, &fakeGameResources{})
	req := httptest.NewRequest(http.MethodPost, "/api/game/resources?session=public", strings.NewReader(`{"production":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected malformed production update to return 400, got %d", rec.Code)
	}
}

func TestGameResourcesEndpointRejectsInvalidUpdatePlanetID(t *testing.T) {
	server := testServerWithGameResources(t, &fakeGameResources{})
	req := httptest.NewRequest(http.MethodPost, "/api/game/resources?session=public&cp=abc", strings.NewReader(`{"production":{"1":50}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameResourcesEndpointReturnsUnavailableForUpdateUseCaseError(t *testing.T) {
	server := testServerWithGameResources(t, &fakeGameResources{updateErr: errors.New("resources update failed")})
	req := httptest.NewRequest(http.MethodPost, "/api/game/resources?session=public", strings.NewReader(`{"production":{"1":50}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game resources update error to return 503, got %d", rec.Code)
	}
}

func TestGameResourcesEndpointReturnsUnavailableForUpdateWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodPost, "/api/game/resources?session=public", strings.NewReader(`{"production":{"1":50}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game resources update use case to return 503, got %d", rec.Code)
	}
}

func TestDecodeResourceProductionUpdateUsesLegacyCoercion(t *testing.T) {
	if legacyInt(nil) != 0 {
		t.Fatal("expected empty legacy form values to coerce to 0")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/game/resources", strings.NewReader(`{"production":{"last1":" 90 ","2":25.9,"bad":100,"last3":"bad","4":true}}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	settings, err := decodeResourceProductionUpdate(req)
	if err != nil {
		t.Fatal(err)
	}
	if settings[domaingame.BuildingMetalMine] != 90 || settings[domaingame.BuildingCrystalMine] != 25 ||
		settings[domaingame.BuildingDeuteriumSynth] != 0 || settings[domaingame.BuildingSolarPlant] != 0 {
		t.Fatalf("unexpected json production settings: %+v", settings)
	}
	if _, ok := settings[0]; ok {
		t.Fatalf("expected invalid keys to be ignored: %+v", settings)
	}

	form := "last1=75&last2=&lastbad=80&other=100"
	req = httptest.NewRequest(http.MethodPost, "/api/game/resources", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	settings, err = decodeResourceProductionUpdate(req)
	if err != nil {
		t.Fatal(err)
	}
	if settings[domaingame.BuildingMetalMine] != 75 || settings[domaingame.BuildingCrystalMine] != 0 {
		t.Fatalf("unexpected form production settings: %+v", settings)
	}
	if _, ok := settings[0]; ok {
		t.Fatalf("expected invalid form keys to be ignored: %+v", settings)
	}
}

func TestGameResourcesEndpointReturnsUnauthorizedForInvalidSession(t *testing.T) {
	resources := &fakeGameResources{result: appgame.ResourcesResult{
		Authenticated: false,
		Issues: []domainpublicsite.SessionIssue{{
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
		}},
	}}
	server := testServerWithGameResources(t, resources)
	req := httptest.NewRequest(http.MethodGet, "/api/game/resources?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var response gameResourceProductionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Authenticated || response.Resources != nil || len(response.Issues) != 1 {
		t.Fatalf("expected invalid resources session response, got %+v", response)
	}
}

func TestGameResourcesEndpointRejectsInvalidPlanetID(t *testing.T) {
	server := testServerWithGameResources(t, &fakeGameResources{})
	req := httptest.NewRequest(http.MethodGet, "/api/game/resources?session=public&cp=abc", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet to return 400, got %d", rec.Code)
	}
}

func TestGameResourcesEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameResources(t, &fakeGameResources{err: errors.New("resources failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/resources?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game resources error to return 503, got %d", rec.Code)
	}
}

func TestGameResourcesEndpointReturnsUnavailableWithoutUseCase(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/game/resources?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing game resources use case to return 503, got %d", rec.Code)
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

func TestLegacyGameIndexRouteServesReactShell(t *testing.T) {
	staticDir := t.TempDir()
	writeFile(t, filepath.Join(staticDir, "index.html"), "ogame react shell")
	server := testServer(config.Config{StaticDir: staticDir, LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/game/index.php?page=overview&session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ogame react shell" {
		t.Fatalf("unexpected body %q", rec.Body.String())
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

	req = httptest.NewRequest(http.MethodPost, "/api/game/session", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game session method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/overview", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game overview method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/buildings", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game buildings method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/research", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game research method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/shipyard", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game shipyard method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/fleet", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game fleet method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/fleet-templates", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game fleet templates method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/galaxy", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game galaxy method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/defense", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game defense method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/technology", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game technology method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/statistics", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game statistics method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/search", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game search method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/buddy", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game buddy method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/notes", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game notes method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/messages", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game messages method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/game/resources", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected game resources method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}
}

func TestPostOnlyRejectsReadMethods(t *testing.T) {
	server := testServer(config.Config{StaticDir: t.TempDir(), LegacyAssetDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/public/registration/validate", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("expected registration method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/public/login/validate", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("expected login method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/public/registration", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("expected registration submit method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/public/login", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("expected login submit method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/game/logout", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("expected logout method rejection, got code=%d headers=%v", rec.Code, rec.Header())
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

func testServerWithRegistrationDrafts(t *testing.T, registrationDrafts RegistrationDraftUseCase) http.Handler {
	t.Helper()
	return testServerWithCustomDrafts(t, registrationDrafts, apppublicsite.NewLoginDraftValidator())
}

func testServerWithLoginDrafts(t *testing.T, loginDrafts LoginDraftUseCase) http.Handler {
	t.Helper()
	return testServerWithCustomDrafts(t, apppublicsite.NewRegistrationDraftValidator(), loginDrafts)
}

func testServerWithLogin(t *testing.T, login LoginUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		Login:              login,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithRegistration(t *testing.T, registration RegistrationUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		Registration:       registration,
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithActivation(t *testing.T, activation RegistrationActivationUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		Activation:         activation,
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameSessions(t *testing.T, gameSessions GameSessionUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameSessions:       gameSessions,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithLogout(t *testing.T, logout LogoutUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		Logout:             logout,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameOverview(t *testing.T, overview GameOverviewUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameOverview:       overview,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameBuildings(t *testing.T, buildings GameBuildingsUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameBuildings:      buildings,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameResources(t *testing.T, resources GameResourcesUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameResources:      resources,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameResearch(t *testing.T, research GameResearchUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameResearch:       research,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameShipyard(t *testing.T, shipyard GameShipyardUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameShipyard:       shipyard,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameFleet(t *testing.T, fleet GameFleetUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameFleet:          fleet,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameGalaxy(t *testing.T, galaxy GameGalaxyUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameGalaxy:         galaxy,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameDefense(t *testing.T, defense GameDefenseUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameDefense:        defense,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameTechnology(t *testing.T, technology GameTechnologyUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameTechnology:     technology,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameStatistics(t *testing.T, statistics GameStatisticsUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameStatistics:     statistics,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameSearch(t *testing.T, search GameSearchUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameSearch:         search,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameBuddy(t *testing.T, buddy GameBuddyUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameBuddy:          buddy,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameNotes(t *testing.T, notes GameNotesUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameNotes:          notes,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameMessages(t *testing.T, messages GameMessagesUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameMessages:       messages,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameReport(t *testing.T, report GameReportUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameReport:         report,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithGameOptions(t *testing.T, options GameOptionsUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: apppublicsite.NewRegistrationDraftValidator(),
		LoginDrafts:        apppublicsite.NewLoginDraftValidator(),
		GameOptions:        options,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
}

func testServerWithCustomDrafts(t *testing.T, registrationDrafts RegistrationDraftUseCase, loginDrafts LoginDraftUseCase) http.Handler {
	t.Helper()
	universes := apppublicsite.NewUniverseCatalogService(configcatalog.UniverseCatalog{LegacyBaseURL: "http://legacy.local"})
	return New(Dependencies{
		Universes:          universes,
		RegistrationDrafts: registrationDrafts,
		LoginDrafts:        loginDrafts,
		Frontend:           filesystem.StaticDir{Root: t.TempDir()},
		LegacyAssets:       filesystem.NewNoListingFS(t.TempDir()),
	})
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
	loginDrafts := apppublicsite.NewLoginDraftValidator()

	return New(Dependencies{
		Health:             health,
		Universes:          universes,
		RegistrationDrafts: registrationDrafts,
		LoginDrafts:        loginDrafts,
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

func hasLoginIssue(response loginValidationResponse, code string, legacyCode int) bool {
	for _, issue := range response.Issues {
		if issue.Code == code && issue.LegacyErrorCode == legacyCode {
			return true
		}
	}
	return false
}

type failingRegistrationDrafts struct {
	err error
}

func (f failingRegistrationDrafts) ValidateRegistrationDraft(context.Context, apppublicsite.RegistrationDraftCommand) (domainpublicsite.RegistrationValidation, error) {
	return domainpublicsite.RegistrationValidation{}, f.err
}

type failingLoginDrafts struct {
	err error
}

func (f failingLoginDrafts) ValidateLoginDraft(context.Context, apppublicsite.LoginDraftCommand) (domainpublicsite.LoginValidation, error) {
	return domainpublicsite.LoginValidation{}, f.err
}

type fakeLogin struct {
	result  domainpublicsite.LoginAuthentication
	err     error
	command apppublicsite.LoginCommand
}

func (f *fakeLogin) AuthenticateLogin(_ context.Context, command apppublicsite.LoginCommand) (domainpublicsite.LoginAuthentication, error) {
	f.command = command
	return f.result, f.err
}

type fakeRegistration struct {
	result  domainpublicsite.RegistrationCreation
	err     error
	command apppublicsite.RegistrationCommand
}

func (f *fakeRegistration) RegisterAccount(_ context.Context, command apppublicsite.RegistrationCommand) (domainpublicsite.RegistrationCreation, error) {
	f.command = command
	return f.result, f.err
}

type fakeActivation struct {
	result  domainpublicsite.RegistrationActivation
	err     error
	command apppublicsite.RegistrationActivationCommand
}

func (f *fakeActivation) ActivateAccount(_ context.Context, command apppublicsite.RegistrationActivationCommand) (domainpublicsite.RegistrationActivation, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameSessions struct {
	result  domainpublicsite.SessionAuthentication
	err     error
	command apppublicsite.GameSessionCommand
}

func (f *fakeGameSessions) GetGameSession(_ context.Context, command apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error) {
	f.command = command
	return f.result, f.err
}

type fakeLogout struct {
	result  apppublicsite.LogoutResult
	err     error
	command apppublicsite.LogoutCommand
}

func (f *fakeLogout) Logout(_ context.Context, command apppublicsite.LogoutCommand) (apppublicsite.LogoutResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameOverview struct {
	result        appgame.OverviewResult
	err           error
	command       appgame.OverviewCommand
	renameCommand appgame.OverviewRenameCommand
	deleteCommand appgame.OverviewDeleteCommand
}

func (f *fakeGameOverview) GetOverview(_ context.Context, command appgame.OverviewCommand) (appgame.OverviewResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameOverview) RenamePlanet(_ context.Context, command appgame.OverviewRenameCommand) (appgame.OverviewResult, error) {
	f.renameCommand = command
	return f.result, f.err
}

func (f *fakeGameOverview) DeletePlanet(_ context.Context, command appgame.OverviewDeleteCommand) (appgame.OverviewResult, error) {
	f.deleteCommand = command
	return f.result, f.err
}

type fakeGameBuildings struct {
	result   appgame.BuildingsResult
	err      error
	command  appgame.BuildingsCommand
	mutation appgame.BuildingsMutationCommand
}

func (f *fakeGameBuildings) GetBuildings(_ context.Context, command appgame.BuildingsCommand) (appgame.BuildingsResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameBuildings) MutateBuildings(_ context.Context, command appgame.BuildingsMutationCommand) (appgame.BuildingsResult, error) {
	f.mutation = command
	return f.result, f.err
}

type fakeGameResearch struct {
	result   appgame.ResearchResult
	err      error
	command  appgame.ResearchCommand
	mutation appgame.ResearchMutationCommand
}

func (f *fakeGameResearch) GetResearch(_ context.Context, command appgame.ResearchCommand) (appgame.ResearchResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameResearch) MutateResearch(_ context.Context, command appgame.ResearchMutationCommand) (appgame.ResearchResult, error) {
	f.mutation = command
	return f.result, f.err
}

type fakeGameShipyard struct {
	result   appgame.ShipyardResult
	err      error
	command  appgame.ShipyardCommand
	mutation appgame.ShipyardMutationCommand
}

func (f *fakeGameShipyard) GetShipyard(_ context.Context, command appgame.ShipyardCommand) (appgame.ShipyardResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameShipyard) MutateShipyard(_ context.Context, command appgame.ShipyardMutationCommand) (appgame.ShipyardResult, error) {
	f.mutation = command
	return f.result, f.err
}

type fakeGameFleet struct {
	result   appgame.FleetResult
	err      error
	command  appgame.FleetCommand
	mutation appgame.FleetTemplateMutationCommand
	prepare  appgame.FleetDispatchPrepareCommand
	validate appgame.FleetDispatchValidateCommand
	launch   appgame.FleetDispatchLaunchCommand
	recall   appgame.FleetRecallCommand
}

func (f *fakeGameFleet) GetFleet(_ context.Context, command appgame.FleetCommand) (appgame.FleetResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameFleet) MutateFleetTemplate(_ context.Context, command appgame.FleetTemplateMutationCommand) (appgame.FleetResult, error) {
	f.mutation = command
	return f.result, f.err
}

func (f *fakeGameFleet) PrepareFleetDispatch(_ context.Context, command appgame.FleetDispatchPrepareCommand) (appgame.FleetResult, error) {
	f.prepare = command
	return f.result, f.err
}

func (f *fakeGameFleet) ValidateFleetDispatch(_ context.Context, command appgame.FleetDispatchValidateCommand) (appgame.FleetResult, error) {
	f.validate = command
	return f.result, f.err
}

func (f *fakeGameFleet) LaunchFleetDispatch(_ context.Context, command appgame.FleetDispatchLaunchCommand) (appgame.FleetResult, error) {
	f.launch = command
	return f.result, f.err
}

func (f *fakeGameFleet) RecallFleet(_ context.Context, command appgame.FleetRecallCommand) (appgame.FleetResult, error) {
	f.recall = command
	return f.result, f.err
}

type fakeGameGalaxy struct {
	result   appgame.GalaxyResult
	err      error
	command  appgame.GalaxyCommand
	missile  appgame.GalaxyMissileLaunchCommand
	dispatch appgame.GalaxyInstantDispatchCommand
}

func (f *fakeGameGalaxy) GetGalaxy(_ context.Context, command appgame.GalaxyCommand) (appgame.GalaxyResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameGalaxy) LaunchMissiles(_ context.Context, command appgame.GalaxyMissileLaunchCommand) (appgame.GalaxyResult, error) {
	f.missile = command
	return f.result, f.err
}

func (f *fakeGameGalaxy) DispatchInstantFleet(_ context.Context, command appgame.GalaxyInstantDispatchCommand) (appgame.GalaxyResult, error) {
	f.dispatch = command
	return f.result, f.err
}

type fakeGameDefense struct {
	result   appgame.DefenseResult
	err      error
	command  appgame.DefenseCommand
	mutation appgame.DefenseMutationCommand
}

func (f *fakeGameDefense) GetDefense(_ context.Context, command appgame.DefenseCommand) (appgame.DefenseResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameDefense) MutateDefense(_ context.Context, command appgame.DefenseMutationCommand) (appgame.DefenseResult, error) {
	f.mutation = command
	return f.result, f.err
}

type fakeGameTechnology struct {
	result  appgame.TechnologyResult
	err     error
	command appgame.TechnologyCommand
}

func (f *fakeGameTechnology) GetTechnology(_ context.Context, command appgame.TechnologyCommand) (appgame.TechnologyResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameStatistics struct {
	result  appgame.StatisticsResult
	err     error
	command appgame.StatisticsCommand
}

func (f *fakeGameStatistics) GetStatistics(_ context.Context, command appgame.StatisticsCommand) (appgame.StatisticsResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameSearch struct {
	result  appgame.SearchResult
	err     error
	command appgame.SearchCommand
}

func (f *fakeGameSearch) GetSearch(_ context.Context, command appgame.SearchCommand) (appgame.SearchResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameBuddy struct {
	result   appgame.BuddyResult
	err      error
	command  appgame.BuddyCommand
	mutation appgame.BuddyMutationCommand
}

func (f *fakeGameBuddy) GetBuddy(_ context.Context, command appgame.BuddyCommand) (appgame.BuddyResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameBuddy) MutateBuddy(_ context.Context, command appgame.BuddyMutationCommand) (appgame.BuddyResult, error) {
	f.mutation = command
	return f.result, f.err
}

type fakeGameNotes struct {
	result        appgame.NotesResult
	err           error
	command       appgame.NotesCommand
	createCommand appgame.NotesMutationCommand
	updateCommand appgame.NotesMutationCommand
	deleteCommand appgame.NotesMutationCommand
}

func (f *fakeGameNotes) GetNotes(_ context.Context, command appgame.NotesCommand) (appgame.NotesResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameNotes) CreateNote(_ context.Context, command appgame.NotesMutationCommand) (appgame.NotesResult, error) {
	f.createCommand = command
	return f.result, f.err
}

func (f *fakeGameNotes) UpdateNote(_ context.Context, command appgame.NotesMutationCommand) (appgame.NotesResult, error) {
	f.updateCommand = command
	return f.result, f.err
}

func (f *fakeGameNotes) DeleteNotes(_ context.Context, command appgame.NotesMutationCommand) (appgame.NotesResult, error) {
	f.deleteCommand = command
	return f.result, f.err
}

type fakeGameMessages struct {
	result          appgame.MessagesResult
	err             error
	command         appgame.MessagesCommand
	mutationCommand appgame.MessagesMutationCommand
}

func (f *fakeGameMessages) GetMessages(_ context.Context, command appgame.MessagesCommand) (appgame.MessagesResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameMessages) MutateMessages(_ context.Context, command appgame.MessagesMutationCommand) (appgame.MessagesResult, error) {
	f.mutationCommand = command
	return f.result, f.err
}

type fakeGameReport struct {
	result  appgame.ReportResult
	err     error
	command appgame.ReportCommand
}

func (f *fakeGameReport) GetReport(_ context.Context, command appgame.ReportCommand) (appgame.ReportResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameOptions struct {
	result        appgame.OptionsResult
	updateResult  appgame.OptionsResult
	err           error
	updateErr     error
	command       appgame.OptionsCommand
	updateCommand appgame.OptionsUpdateCommand
}

func (f *fakeGameOptions) GetOptions(_ context.Context, command appgame.OptionsCommand) (appgame.OptionsResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameOptions) UpdateOptions(_ context.Context, command appgame.OptionsUpdateCommand) (appgame.OptionsResult, error) {
	f.updateCommand = command
	return f.updateResult, f.updateErr
}

func sampleGameOptions() domaingame.Options {
	return domaingame.Options{
		Commander: "legor",
		CurrentPlanet: domaingame.PlanetOverview{
			ID:          99,
			Name:        "Arakis",
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
		},
		PlanetSwitcher: []domaingame.PlanetSummary{{
			ID:          99,
			Name:        "Arakis",
			Type:        domaingame.PlanetTypePlanet,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			Current:     true,
		}},
		User: domaingame.OptionsUser{
			Name:        "Legor",
			Email:       "legor@example.test",
			PlainEmail:  "permanent@example.test",
			Validated:   true,
			CommanderOn: true,
		},
		Universe: domaingame.OptionsUniverse{Language: "en", FeedAge: 60},
		Settings: domaingame.OptionsSettings{
			Language:         "en",
			SkinPath:         "/evolution/",
			UseSkin:          true,
			SortBy:           1,
			SortOrder:        0,
			MaxSpy:           5,
			MaxFleetMessages: 8,
		},
		Flags: domaingame.OptionsFlags{ShowEspionageButton: true},
	}
}

type fakeGameResources struct {
	result        appgame.ResourcesResult
	updateResult  appgame.ResourcesResult
	err           error
	updateErr     error
	command       appgame.ResourcesCommand
	updateCommand appgame.ResourcesUpdateCommand
}

func (f *fakeGameResources) GetResources(_ context.Context, command appgame.ResourcesCommand) (appgame.ResourcesResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *fakeGameResources) UpdateResources(_ context.Context, command appgame.ResourcesUpdateCommand) (appgame.ResourcesResult, error) {
	f.updateCommand = command
	return f.updateResult, f.updateErr
}
