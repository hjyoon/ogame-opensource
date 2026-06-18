package httpdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
			Code:    domainpublicsite.SessionIssuePrivateInvalid,
			Message: "Private session is invalid.",
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
	if response.Authenticated || len(response.Issues) != 1 || response.Issues[0].Code != domainpublicsite.SessionIssuePrivateInvalid {
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

func TestGameOverviewEndpointReturnsOverview(t *testing.T) {
	overview := &fakeGameOverview{result: appgame.OverviewResult{
		Authenticated: true,
		Overview: domaingame.Overview{
			Commander: "legor",
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
					MetalCapacity:     100000,
					CrystalCapacity:   150000,
					DeuteriumCapacity: 200000,
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
			}},
		},
	}}
	server := testServerWithGameOverview(t, overview)
	req := httptest.NewRequest(http.MethodGet, "/api/game/overview?session=public&cp=99", nil)
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
		response.Overview.CurrentPlanet.Resources.CrystalCapacity != 150000 {
		t.Fatalf("unexpected overview mapping: %+v", response.Overview)
	}
	if overview.command.PublicSession != "public" || overview.command.PlanetID != 99 || overview.command.RemoteAddr != "203.0.113.10" {
		t.Fatalf("unexpected overview command: %+v", overview.command)
	}
	if overview.command.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("expected private session cookie to be passed, got %+v", overview.command.PrivateSessions)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private")) {
		t.Fatalf("game overview response must not echo private session: %s", rec.Body.String())
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
}

func TestGameOverviewEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameOverview(t, &fakeGameOverview{err: errors.New("overview failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/overview?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game overview error to return 503, got %d", rec.Code)
	}
}

func TestGameBuildingsEndpointReturnsBuildings(t *testing.T) {
	buildings := &fakeGameBuildings{result: appgame.BuildingsResult{
		Authenticated: true,
		Buildings: domaingame.Buildings{
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
					Metal:     1000,
					Crystal:   500,
					Deuterium: 100,
				},
			},
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
}

func TestGameBuildingsEndpointReturnsUnavailableForUseCaseError(t *testing.T) {
	server := testServerWithGameBuildings(t, &fakeGameBuildings{err: errors.New("buildings failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/game/buildings?session=public", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected game buildings error to return 503, got %d", rec.Code)
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
		},
	}}
	server := testServerWithGameTechnology(t, technology)
	req := httptest.NewRequest(http.MethodGet, "/api/game/technology?session=public&cp=99", nil)
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
	if technology.command.PublicSession != "public" || technology.command.PlanetID != 99 || technology.command.RemoteAddr != "203.0.113.10" {
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
			Factor: 1,
			Natural: domaingame.ResourceProductionValues{
				Metal:   20,
				Crystal: 10,
			},
			Rows: []domaingame.ResourceProductionRow{{
				ID:      domaingame.BuildingMetalMine,
				Name:    "Metal Mine",
				Level:   2,
				Percent: 100,
				Values:  domaingame.ResourceProductionValues{Metal: 72, Energy: -25, EnergyRaw: -25},
			}},
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
	if !response.Authenticated || response.Resources == nil || response.Resources.Commander != "legor" || len(response.Resources.Rows) != 1 {
		t.Fatalf("expected authenticated resources response, got %+v", response)
	}
	if response.Resources.Rows[0].Values.Metal != 72 || response.Resources.Totals.Day.Metal != 2208 {
		t.Fatalf("unexpected resources mapping: %+v", response.Resources)
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

	req = httptest.NewRequest(http.MethodPost, "/api/game/overview", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game overview method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/buildings", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game buildings method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/research", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game research method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/shipyard", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game shipyard method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/defense", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game defense method rejection, got code=%d headers=%v", rec.Code, rec.Header())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/technology", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("expected game technology method rejection, got code=%d headers=%v", rec.Code, rec.Header())
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

type fakeGameSessions struct {
	result  domainpublicsite.SessionAuthentication
	err     error
	command apppublicsite.GameSessionCommand
}

func (f *fakeGameSessions) GetGameSession(_ context.Context, command apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameOverview struct {
	result  appgame.OverviewResult
	err     error
	command appgame.OverviewCommand
}

func (f *fakeGameOverview) GetOverview(_ context.Context, command appgame.OverviewCommand) (appgame.OverviewResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameBuildings struct {
	result  appgame.BuildingsResult
	err     error
	command appgame.BuildingsCommand
}

func (f *fakeGameBuildings) GetBuildings(_ context.Context, command appgame.BuildingsCommand) (appgame.BuildingsResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameResearch struct {
	result  appgame.ResearchResult
	err     error
	command appgame.ResearchCommand
}

func (f *fakeGameResearch) GetResearch(_ context.Context, command appgame.ResearchCommand) (appgame.ResearchResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameShipyard struct {
	result  appgame.ShipyardResult
	err     error
	command appgame.ShipyardCommand
}

func (f *fakeGameShipyard) GetShipyard(_ context.Context, command appgame.ShipyardCommand) (appgame.ShipyardResult, error) {
	f.command = command
	return f.result, f.err
}

type fakeGameDefense struct {
	result  appgame.DefenseResult
	err     error
	command appgame.DefenseCommand
}

func (f *fakeGameDefense) GetDefense(_ context.Context, command appgame.DefenseCommand) (appgame.DefenseResult, error) {
	f.command = command
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
