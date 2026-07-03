package httpdelivery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestLegacyMaintenanceRedirectsWhenUniverseIsNotFrozen(t *testing.T) {
	usecase := &fakeGameMaintenanceUseCase{maintenance: domaingame.Maintenance{Frozen: false, Language: "en"}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/maintenance.php", nil)

	New(Dependencies{MaintenanceStartPage: "/home.php", GameMaintenance: usecase}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "content='0;url=/home.php'") {
		t.Fatalf("unexpected maintenance redirect: status=%d body=%s", recorder.Code, body)
	}
}

func TestLegacyMaintenanceRendersFrozenPage(t *testing.T) {
	usecase := &fakeGameMaintenanceUseCase{maintenance: domaingame.Maintenance{
		Frozen:   true,
		Language: "en",
		BoardURL: "https://board.example.test",
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/maintenance.php", nil)

	New(Dependencies{GameMaintenance: usecase}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `<body id="maintenance">`) || !strings.Contains(body, "maintenance-background.jpg") {
		t.Fatalf("unexpected maintenance page: status=%d body=%s", recorder.Code, body)
	}
	if !strings.Contains(body, "OGame is currently under maintenance.") || !strings.Contains(body, `href="https://board.example.test"`) {
		t.Fatalf("maintenance page missing copy/link: %s", body)
	}

	recorder = httptest.NewRecorder()
	usecase = &fakeGameMaintenanceUseCase{maintenance: domaingame.Maintenance{
		Frozen:   true,
		Language: "fr",
		BoardURL: "https://board.example.test/fr",
	}}
	New(Dependencies{GameMaintenance: usecase}).ServeHTTP(recorder, req)
	body = recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "OGame est en maintenance.") || !strings.Contains(body, `href="https://board.example.test/fr"`) {
		t.Fatalf("maintenance page missing French copy/link: status=%d body=%s", recorder.Code, body)
	}
}

func TestLegacyMaintenanceGuards(t *testing.T) {
	recorder := httptest.NewRecorder()
	New(Dependencies{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/game/maintenance.php", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "game maintenance unavailable") {
		t.Fatalf("expected missing dependency error, status=%d body=%q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	New(Dependencies{GameMaintenance: &fakeGameMaintenanceUseCase{err: errors.New("repo down")}}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/game/maintenance.php", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "game maintenance unavailable") {
		t.Fatalf("expected usecase error, status=%d body=%q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	New(Dependencies{GameMaintenance: &fakeGameMaintenanceUseCase{}}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/game/maintenance.php", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", recorder.Code)
	}
	if got := (Dependencies{}).CurrentMaintenanceStartPage(); got != "/" {
		t.Fatalf("expected default maintenance start page, got %q", got)
	}
}

type fakeGameMaintenanceUseCase struct {
	maintenance domaingame.Maintenance
	err         error
}

func (f *fakeGameMaintenanceUseCase) GetMaintenance(context.Context) (domaingame.Maintenance, error) {
	return f.maintenance, f.err
}
