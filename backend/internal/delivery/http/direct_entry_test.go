package httpdelivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
)

func TestLegacyRedirectRejectsUnsafeTargets(t *testing.T) {
	handler := New(Dependencies{DirectEntry: apppublicsite.NewDirectEntryService(nil)})
	for _, raw := range []string{"javascript:alert(1)", "http://127.0.0.1:8888/game/index.php", "http://example.com@127.0.0.1/image.png"} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/game/redir.php?url="+raw, nil)
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusBadRequest || recorder.Header().Get("Location") != "" || strings.Contains(recorder.Body.String(), raw) {
			t.Fatalf("unsafe redirect was not rejected: raw=%q code=%d location=%q body=%q", raw, recorder.Code, recorder.Header().Get("Location"), recorder.Body.String())
		}
	}
}

func TestLegacyRedirectRendersMetaRefreshForSafeTarget(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/redir.php?url=https://example.com/path?a=1", nil)
	New(Dependencies{DirectEntry: apppublicsite.NewDirectEntryService(nil)}).ServeHTTP(recorder, req)
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, "Page has moved") || !strings.Contains(body, "https://example.com/path?a=1") {
		t.Fatalf("unexpected redirect response: code=%d body=%q", recorder.Code, body)
	}
}

func TestLegacyImageProxyRejectsUnsafeTargets(t *testing.T) {
	handler := New(Dependencies{DirectEntry: apppublicsite.NewDirectEntryService(&fakeDirectEntryImageFetcher{})})
	for _, raw := range []string{"javascript:alert(1)", "http://127.0.0.1/logo.png", "https://example.com/logo.svg"} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/game/pic.php?url="+raw, nil)
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK || strings.HasPrefix(recorder.Header().Get("Content-Type"), "image/") || !strings.Contains(recorder.Body.String(), "Графика недоступна") {
			t.Fatalf("unsafe image was not rejected: raw=%q code=%d content-type=%q body=%q", raw, recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
		}
	}
}

func TestLegacyImageProxyReturnsFetchedImage(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/game/pic.php?url=https://example.com/logo.png", nil)
	New(Dependencies{DirectEntry: apppublicsite.NewDirectEntryService(&fakeDirectEntryImageFetcher{
		image: apppublicsite.ExternalImage{ContentType: "image/png", Body: []byte{1, 2, 3}},
	})}).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/png" || string(recorder.Body.Bytes()) != string([]byte{1, 2, 3}) {
		t.Fatalf("unexpected image response: code=%d content-type=%q body=%v", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.Bytes())
	}
}

func TestLegacyImageProxyUnavailableBranches(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
	}{
		{name: "missing dependency", handler: New(Dependencies{})},
		{name: "fetcher error", handler: New(Dependencies{DirectEntry: apppublicsite.NewDirectEntryService(&fakeDirectEntryImageFetcher{err: context.Canceled})})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/game/pic.php?url=https://example.com/logo.png", nil)
			tt.handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Графика недоступна") {
				t.Fatalf("unexpected unavailable response: code=%d body=%q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestLegacyDirectEntryRequiresGET(t *testing.T) {
	for _, path := range []string{"/game/redir.php", "/game/pic.php"} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		New(Dependencies{DirectEntry: apppublicsite.NewDirectEntryService(nil)}).ServeHTTP(recorder, req)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected method guard for %s, got %d", path, recorder.Code)
		}
	}
}

type fakeDirectEntryImageFetcher struct {
	image apppublicsite.ExternalImage
	err   error
}

func (f *fakeDirectEntryImageFetcher) FetchExternalImage(_ context.Context, _ string, _ string) (apppublicsite.ExternalImage, error) {
	return f.image, f.err
}
