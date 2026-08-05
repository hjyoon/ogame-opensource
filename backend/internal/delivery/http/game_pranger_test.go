package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestLegacyPrangerDirectPathRendersPublicPillory(t *testing.T) {
	usePrangerTestTimezone(t, 3*60*60)
	usecase := &fakeGamePrangerUseCase{pranger: domaingame.Pranger{
		Universe: 7,
		From:     50,
		Entries: []domaingame.PrangerEntry{
			{BanWhen: 1710000000, AdminName: "Admin <One>", UserName: "Player <Two>", BanUntil: 1710003600, Reason: `[url=mailto:go@example.test]Contact[/url]`},
		},
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/pranger.php?from=50", nil)

	New(Dependencies{UniverseNumber: 7, GamePranger: usecase}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "OGame Pillory Universe 7") || !strings.Contains(body, "Ban Date") {
		t.Fatalf("unexpected pranger response: status=%d body=%s", recorder.Code, body)
	}
	if !strings.Contains(body, "Sat Mar 9 2024 16:00:00") || !strings.Contains(body, "Sat Mar 9 2024 17:00:00") {
		t.Fatalf("expected UTC date formatting, got %s", body)
	}
	if strings.Count(body, `data-ogame-unix=`) != 2 || !strings.Contains(body, "browser timezone") {
		t.Fatalf("expected browser-local time metadata, got %s", body)
	}
	if !strings.Contains(body, "Admin &lt;One&gt;") || !strings.Contains(body, "Player &lt;Two&gt;") {
		t.Fatalf("expected names to be escaped, got %s", body)
	}
	if !strings.Contains(body, `[url=mailto:go@example.test]Contact[/url]`) {
		t.Fatalf("expected legacy reason markup to pass through, got %s", body)
	}
	if !strings.Contains(body, `pranger.php?from=0`) || strings.Contains(body, `session=`) {
		t.Fatalf("expected public pagination link, got %s", body)
	}
	if usecase.command.Universe != 7 || usecase.command.From != 50 || usecase.command.Internal {
		t.Fatalf("unexpected pranger command: %+v", usecase.command)
	}
}

func TestLegacyPrangerInternalPathUsesSessionPagination(t *testing.T) {
	entries := make([]domaingame.PrangerEntry, domaingame.PrangerPageLimit)
	usecase := &fakeGamePrangerUseCase{pranger: domaingame.Pranger{Universe: 1, From: 0, Internal: true, Entries: entries}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/index.php?page=pranger&session=pub", nil)

	New(Dependencies{GamePranger: usecase}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `index.php?page=pranger&session=pub&from=50`) {
		t.Fatalf("expected internal next link, status=%d body=%s", recorder.Code, body)
	}
	if !usecase.command.Internal || usecase.command.Universe != 1 {
		t.Fatalf("unexpected internal pranger command: %+v", usecase.command)
	}
}

func TestGamePrangerAPIRendersStructuredPillory(t *testing.T) {
	usePrangerTestTimezone(t, 3*60*60)
	usecase := &fakeGamePrangerUseCase{pranger: domaingame.Pranger{
		Universe: 7,
		From:     50,
		Entries: []domaingame.PrangerEntry{{
			BanWhen:   1710000000,
			AdminName: "Admin <One>",
			UserName:  "Player <Two>",
			BanUntil:  1710003600,
			Reason:    "Testing",
		}},
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/game/pranger?from=50", nil)

	New(Dependencies{UniverseNumber: 7, GamePranger: usecase}).ServeHTTP(recorder, req)

	var response gamePrangerResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode pranger response: %v body=%s", err, recorder.Body.String())
	}
	if recorder.Code != http.StatusOK || response.Pranger == nil {
		t.Fatalf("unexpected pranger response: status=%d response=%+v", recorder.Code, response)
	}
	if response.Pranger.Universe != 7 || !response.Pranger.HasPrevious || response.Pranger.PreviousFrom != 0 || response.Pranger.HasNext {
		t.Fatalf("unexpected pagination summary: %+v", response.Pranger)
	}
	if len(response.Pranger.Entries) != 1 || response.Pranger.Entries[0].BanWhen != "Sat Mar 9 2024 16:00:00" || response.Pranger.Entries[0].BanWhenUnix != 1710000000 || response.Pranger.Entries[0].BanUntilUnix != 1710003600 ||
		response.Pranger.Entries[0].AdminName != "Admin <One>" || response.Pranger.Entries[0].Reason != "Testing" {
		t.Fatalf("unexpected mapped entries: %+v", response.Pranger.Entries)
	}
	if usecase.command.Universe != 7 || usecase.command.From != 50 || usecase.command.Internal {
		t.Fatalf("unexpected API command: %+v", usecase.command)
	}
}

func TestGamePrangerAPIErrorsStayJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	New(Dependencies{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/game/pranger", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected missing service response: status=%d type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	var response gamePrangerResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Error != "game pranger unavailable" {
		t.Fatalf("unexpected JSON error: response=%+v err=%v body=%s", response, err, recorder.Body.String())
	}
}

func TestLegacyPrangerGuards(t *testing.T) {
	recorder := httptest.NewRecorder()
	New(Dependencies{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/game/pranger.php", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "game pranger unavailable") {
		t.Fatalf("expected missing dependency error, status=%d body=%q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	New(Dependencies{GamePranger: &fakeGamePrangerUseCase{err: errors.New("repo down")}}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/game/pranger.php", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "game pranger unavailable") {
		t.Fatalf("expected usecase error, status=%d body=%q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	New(Dependencies{GamePranger: &fakeGamePrangerUseCase{}}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/game/pranger.php", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", recorder.Code)
	}
}

func TestLegacyPrangerHelpers(t *testing.T) {
	usePrangerTestTimezone(t, 9*60*60)

	if got := legacyPrangerInt("-12"); got != 0 {
		t.Fatalf("expected negative offset to clamp, got %d", got)
	}
	if got := legacyPrangerInt("bad"); got != 0 {
		t.Fatalf("expected invalid offset to default, got %d", got)
	}
	if got := legacyPrangerDate(0); got != "Thu Jan 1 1970 0:00:00" {
		t.Fatalf("unexpected epoch date: %s", got)
	}
	if got := legacyPrangerBanDate(0); got != "Thu Jan 1 1970 0:00:00" {
		t.Fatalf("unexpected UTC epoch date: %s", got)
	}
	if got := (Dependencies{}).CurrentUniverseNumber(); got != 1 {
		t.Fatalf("expected default universe 1, got %d", got)
	}
}

func usePrangerTestTimezone(t *testing.T, offsetSeconds int) {
	t.Helper()
	original := time.Local
	time.Local = time.FixedZone("test", offsetSeconds)
	t.Cleanup(func() { time.Local = original })
}

type fakeGamePrangerUseCase struct {
	command appgame.PrangerCommand
	pranger domaingame.Pranger
	err     error
}

func (f *fakeGamePrangerUseCase) GetPranger(_ context.Context, command appgame.PrangerCommand) (domaingame.Pranger, error) {
	f.command = command
	return f.pranger, f.err
}
