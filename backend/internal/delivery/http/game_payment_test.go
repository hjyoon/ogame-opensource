package httpdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestHandleGamePaymentGetWritesAuthenticatedShell(t *testing.T) {
	useCase := &fakeGamePaymentUseCase{getResult: appgame.PaymentResult{Authenticated: true}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/game/payment?session=public", nil)
	request.RemoteAddr = "203.0.113.1:7000"
	request.AddCookie(&http.Cookie{Name: "private", Value: "token"})

	app{deps: Dependencies{GamePayment: useCase}}.handleGamePayment(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response gamePaymentResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Authenticated || response.Payment == nil || response.Payment.Coupon != nil {
		t.Fatalf("expected authenticated empty payment shell, got %+v", response)
	}
	if useCase.getCommand.PublicSession != "public" || useCase.getCommand.RemoteAddr != "203.0.113.1" ||
		useCase.getCommand.PrivateSessions["private"] != "token" {
		t.Fatalf("unexpected command: %+v", useCase.getCommand)
	}
}

func TestHandleGamePaymentPostWritesCouponResult(t *testing.T) {
	coupon := domaingame.PaymentCoupon{ID: 7, Code: "ABCD-EFGH-IJKL-MNOP-QRST", Amount: 5000, Used: true, UserUniverse: 1, UserID: 42, UserName: "legor"}
	useCase := &fakeGamePaymentUseCase{
		mutateResult: appgame.PaymentResult{
			Authenticated: true,
			Payment:       domaingame.Payment{Coupon: &coupon},
			ActionIssue:   domaingame.PaymentIssue(domaingame.PaymentIssueCouponActivated),
		},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/game/payment?session=public", strings.NewReader(`{"action":"activate","couponCode":"ABCD-EFGH-IJKL-MNOP-QRST"}`))
	request.Header.Set("Content-Type", "application/json")

	app{deps: Dependencies{GamePayment: useCase}}.handleGamePayment(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if useCase.mutateCommand.Action != "activate" || useCase.mutateCommand.CouponCode != "ABCD-EFGH-IJKL-MNOP-QRST" {
		t.Fatalf("unexpected mutation command: %+v", useCase.mutateCommand)
	}
	var response gamePaymentResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Payment == nil || response.Payment.Coupon == nil || response.Payment.Coupon.Amount != 5000 ||
		response.ActionIssue == nil || response.ActionIssue.Code != domaingame.PaymentIssueCouponActivated {
		t.Fatalf("unexpected payment response: %+v", response)
	}
}

func TestHandleGamePaymentReportsAuthAndRequestErrors(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing_session", Message: "Session is invalid."}
	tests := []struct {
		name     string
		app      app
		request  *http.Request
		wantCode int
		wantBody string
	}{
		{
			name:     "missing dependency",
			app:      app{},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/payment", nil),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game payment unavailable",
		},
		{
			name:     "post missing dependency",
			app:      app{},
			request:  jsonPaymentRequest(`{"action":"check"}`),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game payment unavailable",
		},
		{
			name:     "invalid json",
			app:      app{deps: Dependencies{GamePayment: &fakeGamePaymentUseCase{}}},
			request:  jsonPaymentRequest("{"),
			wantCode: http.StatusBadRequest,
			wantBody: "invalid payment request",
		},
		{
			name: "unauthenticated",
			app: app{deps: Dependencies{GamePayment: &fakeGamePaymentUseCase{
				getResult: appgame.PaymentResult{Authenticated: false, Issues: []domainpublicsite.SessionIssue{issue}},
			}}},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/payment", nil),
			wantCode: http.StatusUnauthorized,
			wantBody: "missing_session",
		},
		{
			name:     "use case error",
			app:      app{deps: Dependencies{GamePayment: &fakeGamePaymentUseCase{getErr: errors.New("database down")}}},
			request:  httptest.NewRequest(http.MethodGet, "/api/game/payment", nil),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game payment unavailable",
		},
		{
			name:     "post use case error",
			app:      app{deps: Dependencies{GamePayment: &fakeGamePaymentUseCase{mutateErr: errors.New("database down")}}},
			request:  jsonPaymentRequest(`{"action":"activate"}`),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "game payment unavailable",
		},
		{
			name:     "method not allowed",
			app:      app{deps: Dependencies{GamePayment: &fakeGamePaymentUseCase{}}},
			request:  httptest.NewRequest(http.MethodPut, "/api/game/payment", nil),
			wantCode: http.StatusMethodNotAllowed,
			wantBody: "method not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tt.app.handleGamePayment(recorder, tt.request)
			if recorder.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("expected body to contain %q, got %q", tt.wantBody, recorder.Body.String())
			}
		})
	}
}

func TestLogGamePaymentErrorWritesStructuredContext(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/game/payment", nil)
	logGamePaymentError(nil, request, "ignored", errors.New("ignored"))
	logGamePaymentError(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), request, "ignored", nil)

	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))
	logGamePaymentError(logger, request, "game payment mutation failed", errors.New("boom"))

	output := buffer.String()
	if !strings.Contains(output, "game payment mutation failed") ||
		!strings.Contains(output, `"path":"/api/game/payment"`) {
		t.Fatalf("expected structured payment error log, got %s", output)
	}
}

func jsonPaymentRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/game/payment", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

type fakeGamePaymentUseCase struct {
	getResult     appgame.PaymentResult
	mutateResult  appgame.PaymentResult
	getErr        error
	mutateErr     error
	getCommand    appgame.PaymentCommand
	mutateCommand appgame.PaymentMutationCommand
}

func (f *fakeGamePaymentUseCase) GetPayment(_ context.Context, command appgame.PaymentCommand) (appgame.PaymentResult, error) {
	f.getCommand = command
	return f.getResult, f.getErr
}

func (f *fakeGamePaymentUseCase) MutatePayment(_ context.Context, command appgame.PaymentMutationCommand) (appgame.PaymentResult, error) {
	f.mutateCommand = command
	return f.mutateResult, f.mutateErr
}
