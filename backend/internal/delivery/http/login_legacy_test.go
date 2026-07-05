package httpdelivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyLoginErrorPageRendersCredentialsError(t *testing.T) {
	server := testServerWithLogin(t, &fakeLogin{})
	req := httptest.NewRequest(http.MethodGet, "/game/reg/errorpage.php?errorcode=2&arg1=1&arg2=BadPilot", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Error",
		"You tried to enter universe 1 under nickname BadPilot.",
		"This account does not exist or you have entered your password incorrectly.",
		"password recovery",
		"new account",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in legacy login error page %q", want, body)
		}
	}
}

func TestLegacyLoginErrorPageEscapesArguments(t *testing.T) {
	server := testServerWithLogin(t, &fakeLogin{})
	req := httptest.NewRequest(http.MethodGet, "/game/reg/errorpage.php?errorcode=2&arg1=%3C1%3E&arg2=%3Cscript%3E", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "&lt;1&gt;") {
		t.Fatalf("expected escaped login error arguments, got %q", body)
	}
}

func TestLegacyLoginErrorPageRendersBannedAndFallbackErrors(t *testing.T) {
	server := testServerWithLogin(t, &fakeLogin{})
	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "banned",
			path: "/game/reg/errorpage.php?errorcode=3&arg1=1&arg2=BadPilot&arg3=Fri%20Jun%2019",
			want: "This account has been locked to Fri Jun 19",
		},
		{
			name: "fallback",
			path: "/game/reg/errorpage.php?errorcode=999&arg1=1&arg2=BadPilot",
			want: "This account does not exist or you have entered your password incorrectly.",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			if body := rec.Body.String(); !strings.Contains(body, test.want) {
				t.Fatalf("expected %q in legacy login error page %q", test.want, body)
			}
		})
	}
}
