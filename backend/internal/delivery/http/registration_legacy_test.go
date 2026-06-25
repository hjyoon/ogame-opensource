package httpdelivery

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestLegacyRegistrationRedirectGetOpensLegacyForm(t *testing.T) {
	server := testServerWithRegistration(t, &fakeRegistration{})
	req := httptest.NewRequest(http.MethodGet, "/game/reg/newredirect.php", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "url=new.php") {
		t.Fatalf("expected legacy new.php meta refresh, got %q", body)
	}
}

func TestLegacyRegistrationRedirectRejectedDraftUsesLegacyErrorTarget(t *testing.T) {
	registration := &fakeRegistration{
		result: domainpublicsite.RegistrationCreation{
			Issues: []domainpublicsite.RegistrationIssue{
				{
					Field:           "agb",
					Code:            domainpublicsite.RegistrationIssueTermsRequired,
					LegacyErrorCode: domainpublicsite.LegacyRegistrationErrorTerms,
				},
				{
					Field:           "password",
					Code:            domainpublicsite.RegistrationIssuePasswordTooShort,
					LegacyErrorCode: domainpublicsite.LegacyRegistrationErrorPassword,
				},
			},
		},
	}
	server := testServerWithRegistration(t, registration)
	req := httptest.NewRequest(http.MethodPost, "/game/reg/newredirect.php", strings.NewReader("character=Bad&email=bad%40example.local&universe=http%3A%2F%2Flegacy.local&agb=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	if registration.command != (apppublicsite.RegistrationCommand{
		Character:     "Bad",
		Email:         "bad@example.local",
		Universe:      "http://legacy.local",
		TermsAccepted: true,
		RemoteAddr:    "192.0.2.1",
	}) {
		t.Fatalf("unexpected registration command: %+v", registration.command)
	}
	body := rec.Body.String()
	for _, want := range []string{"register.php?", "errorCode=107", "agb=1", "character=Bad", "email=bad%40example.local"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in legacy redirect body %q", want, body)
		}
	}
}

func TestLegacyRegistrationRedirectTermsOnlyKeepsLegacyZeroError(t *testing.T) {
	registration := &fakeRegistration{
		result: domainpublicsite.RegistrationCreation{
			Issues: []domainpublicsite.RegistrationIssue{
				{
					Field:           "agb",
					Code:            domainpublicsite.RegistrationIssueTermsRequired,
					LegacyErrorCode: domainpublicsite.LegacyRegistrationErrorTerms,
				},
			},
		},
	}
	server := testServerWithRegistration(t, registration)
	req := httptest.NewRequest(http.MethodPost, "/game/reg/newredirect.php", strings.NewReader("character=ValidPilot&password=password123&email=pilot%40example.local&universe=http%3A%2F%2Flegacy.local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "errorCode=0") || !strings.Contains(body, "agb=0") {
		t.Fatalf("expected legacy terms-only redirect, status=%d body=%q", rec.Code, body)
	}
}

func TestLegacyRegistrationRedirectSuccessSetsCookieAndRedirects(t *testing.T) {
	registration := &fakeRegistration{
		result: domainpublicsite.RegistrationCreation{
			Valid: true,
			Account: domainpublicsite.RegisteredAccount{
				PlayerID:     42,
				HomePlanetID: 1001,
			},
			Session: domainpublicsite.LoginSession{
				PlayerID:       42,
				PublicID:       "public-session",
				PrivateID:      "private-session",
				UniverseNumber: 1,
				LastLogin:      1_700_000_000,
				RedirectPath:   "/game/overview",
			},
		},
	}
	server := testServerWithRegistration(t, registration)
	req := httptest.NewRequest(http.MethodPost, "/game/reg/newredirect.php", strings.NewReader("character=Pilot&password=password123&email=pilot%40example.local&universe=http%3A%2F%2Flegacy.local&agb=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/game/overview?lgn=1&session=public-session" {
		t.Fatalf("unexpected redirect %q", location)
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "prsess_42_1=private-session") {
		t.Fatalf("expected private session cookie, got %q", cookie)
	}
}

func TestLegacyRegistrationRedirectRejectsUnsupportedMethod(t *testing.T) {
	server := testServerWithRegistration(t, &fakeRegistration{})
	req := httptest.NewRequest(http.MethodPut, "/game/reg/newredirect.php", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("expected method rejection, status=%d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestLegacyRegistrationRedirectUnavailableBranches(t *testing.T) {
	tests := []struct {
		name     string
		server   http.Handler
		body     string
		wantCode int
		wantBody string
	}{
		{
			name:     "missing dependency",
			server:   New(Dependencies{}),
			body:     "character=Pilot",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "registration unavailable",
		},
		{
			name:     "invalid form",
			server:   testServerWithRegistration(t, &fakeRegistration{}),
			body:     "%zz",
			wantCode: http.StatusBadRequest,
			wantBody: "invalid registration request",
		},
		{
			name:     "usecase error",
			server:   testServerWithRegistration(t, &fakeRegistration{err: errors.New("boom")}),
			body:     "character=Pilot",
			wantCode: http.StatusServiceUnavailable,
			wantBody: "registration unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/game/reg/newredirect.php", strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			test.server.ServeHTTP(rec, req)

			if rec.Code != test.wantCode {
				t.Fatalf("unexpected status %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), test.wantBody) {
				t.Fatalf("expected %q in body %q", test.wantBody, rec.Body.String())
			}
		})
	}
}
