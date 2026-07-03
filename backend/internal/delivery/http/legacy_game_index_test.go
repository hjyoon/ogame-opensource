package httpdelivery

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestLegacyGameIndexBotEditGetPreviewAndExport(t *testing.T) {
	usecase := &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{
		Authenticated:      true,
		SelectedStrategyID: 7,
		Name:               `Strategy <One>`,
		Source:             `{"node":"<script>"}`,
	}}
	handler := app{deps: Dependencies{GameAdmin: usecase}}

	req := httptest.NewRequest(http.MethodGet, "/game/index.php?page=admin&mode=BotEdit&action=preview&session=pub&cp=99&strat=7", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.AddCookie(&http.Cookie{Name: "prsess_42_1", Value: "private"})
	rec := httptest.NewRecorder()
	handler.handleLegacyGameIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	if usecase.botMutation.Action != domaingame.AdminActionBotEditLoad ||
		usecase.botMutation.StrategyID != 7 ||
		usecase.botMutation.PlanetID != 99 ||
		usecase.botMutation.PublicSession != "pub" ||
		usecase.botMutation.RemoteAddr != "203.0.113.9" ||
		usecase.botMutation.PrivateSessions["prsess_42_1"] != "private" {
		t.Fatalf("unexpected preview command: %+v", usecase.botMutation)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Strategy &lt;One&gt;") || !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, `var session="pub"`) {
		t.Fatalf("preview body was not escaped/rendered as expected: %s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/game/index.php?page=admin&mode=BotEdit&action=export&session=pub&cp=99&strat=7", nil)
	rec = httptest.NewRecorder()
	handler.handleLegacyGameIndex(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"node":"<script>"}` {
		t.Fatalf("export status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLegacyGameIndexBotEditPostLoadAndRename(t *testing.T) {
	usecase := &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{
		Authenticated:      true,
		SelectedStrategyID: 5,
		Name:               "Loaded",
		Source:             "loaded-source",
		Strategies: []domaingame.AdminBotStrategy{
			{ID: 4, Name: "Safe"},
			{ID: 5, Name: "Renamed <Bot>"},
		},
	}}
	handler := app{deps: Dependencies{GameAdmin: usecase}}

	form := url.Values{}
	form.Set("action", "load")
	form.Set("strat", "5")
	form.Set("name", "ignored")
	form.Set("source", url.QueryEscape("encoded source"))
	req := httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&session=pub&cp=88", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.handleLegacyGameIndex(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "loaded-source" {
		t.Fatalf("load status=%d body=%s", rec.Code, rec.Body.String())
	}
	if usecase.botMutation.Action != domaingame.AdminActionBotEditLoad ||
		usecase.botMutation.StrategyID != 5 ||
		usecase.botMutation.Source != "encoded source" {
		t.Fatalf("unexpected load command: %+v", usecase.botMutation)
	}

	form.Set("action", "rename")
	form.Set("name", "Renamed <Bot>")
	req = httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&session=pub&cp=88", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.handleLegacyGameIndex(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `<option value="5" selected>Renamed &lt;Bot&gt;</option>`) {
		t.Fatalf("rename status=%d body=%s", rec.Code, body)
	}
}

func TestLegacyGameIndexBotEditPostSaveNewAndErrors(t *testing.T) {
	usecase := &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{Authenticated: true}}
	handler := app{deps: Dependencies{GameAdmin: usecase}}

	for _, action := range []string{"save", "new"} {
		form := url.Values{}
		form.Set("action", action)
		form.Set("strat", "8")
		form.Set("name", "Bot")
		form.Set("source", "source")
		req := httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&session=pub&cp=88", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.handleLegacyGameIndex(rec, req)

		if rec.Code != http.StatusOK || rec.Body.String() != "" {
			t.Fatalf("%s: expected empty 200 response, got status=%d body=%q", action, rec.Code, rec.Body.String())
		}
		if usecase.botMutation.Action != action || usecase.botMutation.StrategyID != 8 ||
			usecase.botMutation.Name != "Bot" || usecase.botMutation.Source != "source" {
			t.Fatalf("%s: unexpected command %+v", action, usecase.botMutation)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&session=pub&cp=bad", strings.NewReader("action=save"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.handleLegacyGameIndex(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid selected planet 400, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&session=pub&cp=88", strings.NewReader("%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.handleLegacyGameIndex(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid form 400, got %d", rec.Code)
	}

	usecase = &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{Authenticated: false}}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&session=pub&cp=88", strings.NewReader("action=save"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app{deps: Dependencies{GameAdmin: usecase}}.handleLegacyGameIndex(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected unauthenticated post 403, got %d", rec.Code)
	}

	issue := domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)
	usecase = &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{Authenticated: true, ActionIssue: issue}}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&session=pub&cp=88", strings.NewReader("action=save"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app{deps: Dependencies{GameAdmin: usecase}}.handleLegacyGameIndex(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), issue.Message) {
		t.Fatalf("expected action issue post 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLegacyGameIndexAdminLoginsPostRendersSearchResults(t *testing.T) {
	usecase := &fakeGameAdminUseCase{result: appgame.AdminResult{
		Authenticated: true,
		Admin: domaingame.Admin{
			LoginRows: []domaingame.AdminLoginRow{{
				ID:       10,
				UserID:   77,
				UserName: `target<user>`,
				IP:       "203.0.113.77",
				Date:     1700000000,
			}},
		},
	}}
	form := url.Values{}
	form.Set("name", "")
	form.Set("id", "77")
	form.Set("ip", "")
	req := httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=Logins&session=pub&cp=99", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "198.51.100.10:4321"
	rec := httptest.NewRecorder()

	app{deps: Dependencies{GameAdmin: usecase}}.handleLegacyGameIndex(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "203.0.113.77") || !strings.Contains(body, "target&lt;user&gt;") ||
		!strings.Contains(body, `mode=Logins`) {
		t.Fatalf("unexpected legacy logins response: status=%d body=%s", rec.Code, body)
	}
	if usecase.command.Mode != "Logins" || usecase.command.LoginUserID != 77 || usecase.command.PlanetID != 99 ||
		usecase.command.RemoteAddr != "198.51.100.10" {
		t.Fatalf("unexpected legacy logins command: %+v", usecase.command)
	}
}

func TestLegacyGameIndexAdminLoginsPostGuards(t *testing.T) {
	tests := []struct {
		name     string
		handler  app
		path     string
		body     string
		wantCode int
		wantBody string
	}{
		{
			name:     "missing usecase",
			handler:  app{},
			path:     "/game/index.php?page=admin&mode=Logins&session=pub&cp=99",
			body:     "id=77",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game admin unavailable",
		},
		{
			name:     "invalid form",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}},
			path:     "/game/index.php?page=admin&mode=Logins&session=pub&cp=99",
			body:     "%zz",
			wantCode: http.StatusBadRequest,
			wantBody: "invalid admin logins request",
		},
		{
			name:     "invalid cp",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}},
			path:     "/game/index.php?page=admin&mode=Logins&session=pub&cp=bad",
			body:     "id=77",
			wantCode: http.StatusBadRequest,
			wantBody: "invalid selected planet",
		},
		{
			name:     "usecase error",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{err: errors.New("repo down")}}},
			path:     "/game/index.php?page=admin&mode=Logins&session=pub&cp=99",
			body:     "id=77",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game admin unavailable",
		},
		{
			name:     "unauthenticated",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{result: appgame.AdminResult{Authenticated: false}}}},
			path:     "/game/index.php?page=admin&mode=Logins&session=pub&cp=99",
			body:     "id=77",
			wantCode: http.StatusForbidden,
			wantBody: "unauthenticated",
		},
		{
			name:     "access denied",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{result: appgame.AdminResult{Authenticated: true, ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)}}}},
			path:     "/game/index.php?page=admin&mode=Logins&session=pub&cp=99",
			body:     "id=77",
			wantCode: http.StatusForbidden,
			wantBody: "Access denied",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			tt.handler.handleLegacyGameIndex(rec, req)
			if rec.Code != tt.wantCode || !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("unexpected response status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLegacyGameIndexBotEditGetUnknownActionFallsBackToFrontend(t *testing.T) {
	assets := &fakeFrontendAssets{bodies: map[string]string{"index.html": "react shell"}}
	rec := httptest.NewRecorder()
	app{deps: Dependencies{Frontend: assets}}.handleLegacyGameIndex(
		rec,
		httptest.NewRequest(http.MethodGet, "/game/index.php?page=admin&mode=BotEdit&action=unknown", nil),
	)

	if rec.Code != http.StatusOK || rec.Body.String() != "react shell" || assets.rels[len(assets.rels)-1] != "index.html" {
		t.Fatalf("expected frontend fallback, status=%d body=%q rels=%+v", rec.Code, rec.Body.String(), assets.rels)
	}
}

func TestLegacyGameIndexBotEditErrorsAndHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	app{}.handleLegacyGameIndex(rec, httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&session=pub&cp=99", strings.NewReader("action=load")))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected missing usecase 503, got %d", rec.Code)
	}

	usecase := &fakeGameAdminUseCase{botErr: errors.New("repo down")}
	rec = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: usecase}}.handleLegacyGameIndex(rec, httptest.NewRequest(http.MethodGet, "/game/index.php?page=admin&mode=BotEdit&action=preview&session=pub&cp=99", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected usecase error 503, got %d", rec.Code)
	}

	usecase = &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{Authenticated: false}}
	rec = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: usecase}}.handleLegacyGameIndex(rec, httptest.NewRequest(http.MethodGet, "/game/index.php?page=admin&mode=BotEdit&action=preview&session=pub&cp=99", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected unauthenticated 403, got %d", rec.Code)
	}

	issue := domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)
	usecase = &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{Authenticated: true, ActionIssue: issue}}
	rec = httptest.NewRecorder()
	app{deps: Dependencies{GameAdmin: usecase}}.handleLegacyGameIndex(rec, httptest.NewRequest(http.MethodGet, "/game/index.php?page=admin&mode=BotEdit&action=preview&session=pub&cp=99", nil))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), issue.Message) {
		t.Fatalf("expected action issue 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	app{}.handleLegacyGameIndex(rec, httptest.NewRequest(http.MethodDelete, "/game/index.php", nil))
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD, POST" {
		t.Fatalf("expected method not allowed, got %d allow=%s", rec.Code, rec.Header().Get("Allow"))
	}

	if legacyBotEditInt(" 42 ") != 42 || legacyBotEditInt("bad") != 0 {
		t.Fatal("legacyBotEditInt did not parse expected values")
	}
	preview := legacyBotEditPreviewHTML(`a"b`, appgame.AdminBotEditMutationResult{SelectedStrategyID: 9, Source: "<source>"})
	if !strings.Contains(preview, "<title>9</title>") || !strings.Contains(preview, "&lt;source&gt;") || !strings.Contains(preview, `var session="a&#34;b"`) {
		t.Fatalf("unexpected helper preview: %s", preview)
	}
}
