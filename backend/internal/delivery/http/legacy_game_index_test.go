package httpdelivery

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

func TestLegacyGameIndexBotEditPostImport(t *testing.T) {
	usecase := &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{Authenticated: true}}
	handler := app{deps: Dependencies{GameAdmin: usecase}}
	req := newLegacyBotEditImportRequest(t, "/game/index.php?page=admin&mode=BotEdit&action=import&session=pub&cp=88", "7", `{"nodeDataArray":[{"key":1,"category":"Start","text":"Imported"}]}`, true)
	req.RemoteAddr = "198.51.100.9:1234"
	rec := httptest.NewRecorder()

	handler.handleLegacyGameIndex(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected import redirect, got status=%d body=%s", rec.Code, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/game/index.php?page=admin&session=pub&mode=BotEdit&cp=88" {
		t.Fatalf("unexpected redirect location %q", location)
	}
	if usecase.botMutation.Action != domaingame.AdminActionBotEditImport ||
		usecase.botMutation.StrategyID != 7 ||
		usecase.botMutation.Source != `{"nodeDataArray":[{"key":1,"category":"Start","text":"Imported"}]}` ||
		usecase.botMutation.PlanetID != 88 ||
		usecase.botMutation.RemoteAddr != "198.51.100.9" {
		t.Fatalf("unexpected import command: %+v", usecase.botMutation)
	}
}

func TestLegacyGameIndexBotEditPostImportGuards(t *testing.T) {
	tests := []struct {
		name    string
		handler app
		request *http.Request
		want    int
		body    string
	}{
		{
			name:    "invalid multipart",
			handler: app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}},
			request: httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=BotEdit&action=import&session=pub&cp=88", strings.NewReader("not multipart")),
			want:    http.StatusBadRequest,
			body:    "invalid botedit import request",
		},
		{
			name:    "invalid selected planet",
			handler: app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}},
			request: newLegacyBotEditImportRequest(t, "/game/index.php?page=admin&mode=BotEdit&action=import&session=pub&cp=bad", "7", "source", true),
			want:    http.StatusBadRequest,
			body:    "invalid selected planet",
		},
		{
			name:    "missing file",
			handler: app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}},
			request: newLegacyBotEditImportRequest(t, "/game/index.php?page=admin&mode=BotEdit&action=import&session=pub&cp=88", "7", "", false),
			want:    http.StatusBadRequest,
			body:    "invalid botedit import file",
		},
		{
			name:    "usecase error",
			handler: app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{botErr: errors.New("botedit down")}}},
			request: newLegacyBotEditImportRequest(t, "/game/index.php?page=admin&mode=BotEdit&action=import&session=pub&cp=88", "7", "source", true),
			want:    http.StatusServiceUnavailable,
			body:    "game admin botedit unavailable",
		},
		{
			name:    "unauthenticated",
			handler: app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{Authenticated: false}}}},
			request: newLegacyBotEditImportRequest(t, "/game/index.php?page=admin&mode=BotEdit&action=import&session=pub&cp=88", "7", "source", true),
			want:    http.StatusForbidden,
			body:    "unauthenticated",
		},
		{
			name: "access denied",
			handler: app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{
				Authenticated: true,
				ActionIssue:   domaingame.AdminIssue(domaingame.AdminIssueAccessDenied),
			}}}},
			request: newLegacyBotEditImportRequest(t, "/game/index.php?page=admin&mode=BotEdit&action=import&session=pub&cp=88", "7", "source", true),
			want:    http.StatusForbidden,
			body:    "Access denied.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler.handleLegacyGameIndex(rec, tt.request)
			if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.body) {
				t.Fatalf("status=%d body=%q, want status=%d body containing %q", rec.Code, rec.Body.String(), tt.want, tt.body)
			}
		})
	}

	usecase := &fakeGameAdminUseCase{botResult: appgame.AdminBotEditMutationResult{Authenticated: true}}
	rec := httptest.NewRecorder()
	req := newLegacyBotEditImportRequest(t, "/game/index.php?page=admin&mode=BotEdit&action=import&session=pub", "7", "source", true)
	app{deps: Dependencies{GameAdmin: usecase}}.handleLegacyGameIndex(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/game/index.php?page=admin&session=pub&mode=BotEdit" {
		t.Fatalf("unexpected no-cp redirect status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func newLegacyBotEditImportRequest(t *testing.T, path string, strategyID string, source string, includeFile bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("strategyId_ForImport", strategyID); err != nil {
		t.Fatalf("WriteField returned error: %v", err)
	}
	if includeFile {
		file, err := writer.CreateFormFile("fileToUpload", "strategy.json")
		if err != nil {
			t.Fatalf("CreateFormFile returned error: %v", err)
		}
		if _, err := file.Write([]byte(source)); err != nil {
			t.Fatalf("file write returned error: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
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

func TestLegacyGameIndexAdminLocaPostRendersComparison(t *testing.T) {
	usecase := &fakeGameAdminUseCase{result: appgame.AdminResult{
		Authenticated: true,
		Admin: domaingame.Admin{
			Localization: &domaingame.AdminLocalization{
				Languages: []string{"de_de", "en_en"},
				Source:    "en_en",
				Target:    "de_de",
				Files: []domaingame.AdminLocalizationFile{{
					Name: "admin.php",
					Rows: []domaingame.AdminLocalizationRow{
						{
							Key:    "ADM_TEST",
							Source: "<source>",
							Target: "The string is missing!",
							Status: "missing",
						},
						{
							Key:    "ADM_SAME",
							Source: "Same",
							Target: "Same",
							Status: "same",
						},
						{
							Key:    "ADM_OK",
							Source: "Source",
							Target: "Ziel",
							Status: "ok",
						},
					},
				}, {
					Name:          "missing.php",
					TargetMissing: true,
				}},
			},
		},
	}}
	form := url.Values{}
	form.Set("loca_src", "en_en")
	form.Set("loca_dst", "de_de")
	req := httptest.NewRequest(http.MethodPost, "/game/index.php?page=admin&mode=Loca&session=pub&cp=99", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app{deps: Dependencies{GameAdmin: usecase}}.handleLegacyGameIndex(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "admin.php") || !strings.Contains(body, "ADM_TEST") ||
		!strings.Contains(body, "&lt;source&gt;") || !strings.Contains(body, "background-color: red") ||
		!strings.Contains(body, "background-color: orange") || !strings.Contains(body, "background-color: green") ||
		!strings.Contains(body, "The file is not localized!") {
		t.Fatalf("unexpected legacy loca response: status=%d body=%s", rec.Code, body)
	}
	if usecase.command.Mode != "Loca" || usecase.command.LocaSource != "en_en" || usecase.command.LocaTarget != "de_de" {
		t.Fatalf("unexpected legacy loca command: %+v", usecase.command)
	}
	if html := legacyAdminLocaHTML("pub", nil); !strings.Contains(html, "mode=Loca") {
		t.Fatalf("expected nil localization helper to render form, got %s", html)
	}
}

func TestLegacyGameIndexAdminLocaPostGuards(t *testing.T) {
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
			path:     "/game/index.php?page=admin&mode=Loca&session=pub&cp=99",
			body:     "loca_src=en_en&loca_dst=de_de",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game admin unavailable",
		},
		{
			name:     "invalid form",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}},
			path:     "/game/index.php?page=admin&mode=Loca&session=pub&cp=99",
			body:     "%zz",
			wantCode: http.StatusBadRequest,
			wantBody: "invalid admin localization request",
		},
		{
			name:     "invalid cp",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{}}},
			path:     "/game/index.php?page=admin&mode=Loca&session=pub&cp=bad",
			body:     "loca_src=en_en&loca_dst=de_de",
			wantCode: http.StatusBadRequest,
			wantBody: "invalid selected planet",
		},
		{
			name:     "usecase error",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{err: errors.New("repo down")}}},
			path:     "/game/index.php?page=admin&mode=Loca&session=pub&cp=99",
			body:     "loca_src=en_en&loca_dst=de_de",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game admin unavailable",
		},
		{
			name:     "unauthenticated",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{result: appgame.AdminResult{Authenticated: false}}}},
			path:     "/game/index.php?page=admin&mode=Loca&session=pub&cp=99",
			body:     "loca_src=en_en&loca_dst=de_de",
			wantCode: http.StatusForbidden,
			wantBody: "unauthenticated",
		},
		{
			name:     "access denied",
			handler:  app{deps: Dependencies{GameAdmin: &fakeGameAdminUseCase{result: appgame.AdminResult{Authenticated: true, ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)}}}},
			path:     "/game/index.php?page=admin&mode=Loca&session=pub&cp=99",
			body:     "loca_src=en_en&loca_dst=de_de",
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
	original := time.Local
	time.Local = time.FixedZone("test", -5*60*60)
	t.Cleanup(func() { time.Local = original })
	if got := legacyAdminDateTime(0); got != "1970-01-01 00:00:00" {
		t.Fatalf("legacy admin date was not normalized to UTC: %s", got)
	}
	preview := legacyBotEditPreviewHTML(`a"b`, appgame.AdminBotEditMutationResult{SelectedStrategyID: 9, Source: "<source>"})
	if !strings.Contains(preview, "<title>9</title>") || !strings.Contains(preview, "&lt;source&gt;") || !strings.Contains(preview, `var session="a&#34;b"`) {
		t.Fatalf("unexpected helper preview: %s", preview)
	}
}
