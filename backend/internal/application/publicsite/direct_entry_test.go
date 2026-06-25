package publicsite

import (
	"context"
	"errors"
	"testing"
)

func TestDirectEntryServiceResolveExternalRedirect(t *testing.T) {
	service := NewDirectEntryService(nil)
	if got := service.ResolveExternalRedirect(context.Background(), "https://example.com/path"); !got.Allowed || got.URL != "https://example.com/path" {
		t.Fatalf("expected public redirect to be allowed, got %+v", got)
	}
	for _, raw := range []string{"javascript:alert(1)", "http://127.0.0.1/game", "http://example.com@127.0.0.1/image.png"} {
		if got := service.ResolveExternalRedirect(context.Background(), raw); got.Allowed || got.URL != "" {
			t.Fatalf("expected redirect %q to be rejected, got %+v", raw, got)
		}
	}
}

func TestDirectEntryServiceProxyExternalImage(t *testing.T) {
	fetcher := &fakeExternalImageFetcher{image: ExternalImage{ContentType: "image/png", Body: []byte{1, 2, 3}}}
	service := NewDirectEntryService(fetcher)
	result, err := service.ProxyExternalImage(context.Background(), "https://example.com/logo.png")
	if err != nil || !result.Available || result.ContentType != "image/png" || string(result.Body) != string([]byte{1, 2, 3}) {
		t.Fatalf("unexpected image result=%+v err=%v", result, err)
	}
	if fetcher.url != "https://example.com/logo.png" || fetcher.expected != "image/png" {
		t.Fatalf("unexpected fetcher call: %+v", fetcher)
	}
}

func TestDirectEntryServiceProxyExternalImageRejectsUnsafeAndUnavailable(t *testing.T) {
	service := NewDirectEntryService(&fakeExternalImageFetcher{err: errors.New("boom")})
	tests := []string{
		"javascript:alert(1)",
		"http://127.0.0.1/logo.png",
		"https://example.com/logo.svg",
		"https://example.com/logo.png",
	}
	for _, raw := range tests {
		result, err := service.ProxyExternalImage(context.Background(), raw)
		if err != nil || result.Available {
			t.Fatalf("expected unavailable image for %q, got %+v err=%v", raw, result, err)
		}
	}
	_, err := NewDirectEntryService(nil).ProxyExternalImage(context.Background(), "https://example.com/logo.png")
	if err == nil {
		t.Fatal("expected missing fetcher error")
	}
}

func TestDirectEntryServiceProxyExternalImageRejectsInvalidFetcherResults(t *testing.T) {
	for _, image := range []ExternalImage{
		{ContentType: "image/jpeg", Body: []byte{1, 2, 3}},
		{ContentType: "image/png"},
	} {
		result, err := NewDirectEntryService(&fakeExternalImageFetcher{image: image}).ProxyExternalImage(context.Background(), "https://example.com/logo.png")
		if err != nil || result.Available {
			t.Fatalf("expected unavailable result for image %+v, got %+v err=%v", image, result, err)
		}
	}
}

type fakeExternalImageFetcher struct {
	image    ExternalImage
	url      string
	expected string
	err      error
}

func (f *fakeExternalImageFetcher) FetchExternalImage(_ context.Context, rawURL string, expectedMime string) (ExternalImage, error) {
	f.url = rawURL
	f.expected = expectedMime
	return f.image, f.err
}
