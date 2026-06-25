package httpdelivery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestPasswordRecoveryAPI(t *testing.T) {
	usecase := &fakePasswordRecoveryUseCase{result: domain.PasswordRecoveryResult{
		Submitted: true,
		Sent:      true,
		Account:   domain.PasswordRecoveryAccount{Character: "Legor"},
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/public/password-recovery", strings.NewReader(`{"email":"legor@example.local"}`))

	New(Dependencies{PasswordRecovery: usecase}).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"sent":true`) || strings.Contains(recorder.Body.String(), "abc123xy") {
		t.Fatalf("unexpected API response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if usecase.command.Email != "legor@example.local" {
		t.Fatalf("unexpected command: %+v", usecase.command)
	}
}

func TestLegacyPasswordRecoveryForm(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/reg/mail.php", nil)

	New(Dependencies{}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "Send Password") || !strings.Contains(body, `name="email"`) || !strings.Contains(body, "fa_pass.php") {
		t.Fatalf("unexpected form response: status=%d body=%s", recorder.Code, body)
	}
}

func TestLegacyPasswordRecoveryPost(t *testing.T) {
	usecase := &fakePasswordRecoveryUseCase{result: domain.PasswordRecoveryResult{
		Submitted: true,
		Sent:      true,
		Account:   domain.PasswordRecoveryAccount{Character: `<Legor>`},
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/game/reg/fa_pass.php", strings.NewReader("email=legor%40example.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	New(Dependencies{PasswordRecovery: usecase}).ServeHTTP(recorder, req)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "Your password has been sent to &lt;Legor&gt;.") || strings.Contains(body, "<Legor>") {
		t.Fatalf("unexpected recovery response: status=%d body=%s", recorder.Code, body)
	}
}

func TestPasswordRecoveryErrorResponses(t *testing.T) {
	tests := []struct {
		name     string
		handler  http.Handler
		method   string
		path     string
		body     string
		wantCode int
		wantBody string
	}{
		{name: "api missing dependency", handler: New(Dependencies{}), method: http.MethodPost, path: "/api/public/password-recovery", body: `{}`, wantCode: http.StatusServiceUnavailable, wantBody: "password recovery unavailable"},
		{name: "api bad json", handler: New(Dependencies{PasswordRecovery: &fakePasswordRecoveryUseCase{}}), method: http.MethodPost, path: "/api/public/password-recovery", body: `{`, wantCode: http.StatusBadRequest, wantBody: "invalid password recovery request"},
		{name: "api usecase error", handler: New(Dependencies{PasswordRecovery: &fakePasswordRecoveryUseCase{err: errors.New("boom")}}), method: http.MethodPost, path: "/api/public/password-recovery", body: `{}`, wantCode: http.StatusServiceUnavailable, wantBody: "password recovery unavailable"},
		{name: "legacy missing dependency", handler: New(Dependencies{}), method: http.MethodPost, path: "/game/reg/fa_pass.php", body: "email=x%40example.local", wantCode: http.StatusServiceUnavailable, wantBody: "password recovery unavailable"},
		{name: "legacy bad form", handler: New(Dependencies{PasswordRecovery: &fakePasswordRecoveryUseCase{}}), method: http.MethodPost, path: "/game/reg/fa_pass.php", body: "%zz", wantCode: http.StatusBadRequest, wantBody: "invalid password recovery request"},
		{name: "legacy usecase error", handler: New(Dependencies{PasswordRecovery: &fakePasswordRecoveryUseCase{err: errors.New("boom")}}), method: http.MethodPost, path: "/game/reg/fa_pass.php", body: "email=x%40example.local", wantCode: http.StatusServiceUnavailable, wantBody: "password recovery unavailable"},
		{name: "legacy not found", handler: New(Dependencies{PasswordRecovery: &fakePasswordRecoveryUseCase{result: domain.PasswordRecoveryResult{Submitted: true}}}), method: http.MethodPost, path: "/game/reg/fa_pass.php", body: "email=x%40example.local", wantCode: http.StatusOK, wantBody: legacyPasswordRecoveryError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			tt.handler.ServeHTTP(recorder, req)
			if recorder.Code != tt.wantCode || !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("unexpected response: status=%d body=%q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

type fakePasswordRecoveryUseCase struct {
	command domain.PasswordRecoveryCommand
	result  domain.PasswordRecoveryResult
	err     error
}

func (f *fakePasswordRecoveryUseCase) RecoverPassword(_ context.Context, command domain.PasswordRecoveryCommand) (domain.PasswordRecoveryResult, error) {
	f.command = command
	return f.result, f.err
}
