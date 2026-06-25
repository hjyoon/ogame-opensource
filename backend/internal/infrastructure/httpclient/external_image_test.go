package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

var png1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
	0x0c, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x60, 0x60, 0x60, 0x00,
	0x00, 0x00, 0x04, 0x00, 0x01, 0xf6, 0x17, 0x38, 0x55, 0x00, 0x00, 0x00,
	0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestExternalImageFetcherFetchesAndValidatesImage(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png; charset=binary"}},
			Body:       io.NopCloser(strings.NewReader(string(png1x1))),
		}, nil
	})}
	fetcher := NewExternalImageFetcherWithClient(client, 1024)
	image, err := fetcher.FetchExternalImage(context.Background(), "https://example.com/logo.png", "image/png")
	if err != nil || image.ContentType != "image/png" || len(image.Body) == 0 {
		t.Fatalf("unexpected image=%+v err=%v", image, err)
	}
}

func TestExternalImageFetcherRejectsMismatches(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader("<html></html>")),
		}, nil
	})}
	fetcher := NewExternalImageFetcherWithClient(client, 1024)
	if _, err := fetcher.FetchExternalImage(context.Background(), "https://example.com/logo.png", "image/png"); err == nil {
		t.Fatal("expected content type mismatch")
	}
	if _, err := fetcher.FetchExternalImage(context.Background(), "http://127.0.0.1/logo.png", "image/png"); err == nil {
		t.Fatal("expected unsafe URL rejection")
	}
}

func TestNewExternalImageFetcherRedirectGuard(t *testing.T) {
	fetcher := NewExternalImageFetcher()
	if fetcher.client == nil || fetcher.maxBytes <= 0 || fetcher.client.CheckRedirect == nil {
		t.Fatalf("unexpected default fetcher: %+v", fetcher)
	}
	safeReq, _ := http.NewRequest(http.MethodGet, "https://example.com/logo.png", nil)
	if err := fetcher.client.CheckRedirect(safeReq, nil); err != nil {
		t.Fatalf("safe redirect rejected: %v", err)
	}
	unsafeReq, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/logo.png", nil)
	if err := fetcher.client.CheckRedirect(unsafeReq, nil); err == nil {
		t.Fatal("expected unsafe redirect rejection")
	}
}

func TestExternalImageFetcherConstructorDefaults(t *testing.T) {
	fetcher := NewExternalImageFetcherWithClient(nil, 0)
	if fetcher.client == nil || fetcher.maxBytes <= 0 {
		t.Fatalf("constructor did not apply defaults: %+v", fetcher)
	}
}

func TestExternalImageFetcherRejectsStatusSizeAndDetectedMismatches(t *testing.T) {
	tests := []struct {
		name      string
		response  *http.Response
		maxBytes  int64
		transport error
	}{
		{
			name:     "non 2xx",
			response: &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))},
			maxBytes: 1024,
		},
		{
			name: "detected mismatch",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
			},
			maxBytes: 1024,
		},
		{
			name: "too large",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(strings.NewReader(string(png1x1))),
			},
			maxBytes: 1,
		},
		{
			name:      "transport error",
			transport: errors.New("transport failed"),
			maxBytes:  1024,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return tt.response, tt.transport
			})}
			fetcher := NewExternalImageFetcherWithClient(client, tt.maxBytes)
			if _, err := fetcher.FetchExternalImage(context.Background(), "https://example.com/logo.png", "image/png"); err == nil {
				t.Fatal("expected fetch error")
			}
		})
	}
}

func TestExternalImageFetcherRejectsReadErrors(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       errorReadCloser{},
		}, nil
	})}
	fetcher := NewExternalImageFetcherWithClient(client, 1024)
	if _, err := fetcher.FetchExternalImage(context.Background(), "https://example.com/logo.png", "image/png"); err == nil {
		t.Fatal("expected read error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorReadCloser struct{}

func (errorReadCloser) Read(_ []byte) (int, error) {
	return 0, errors.New("read failed")
}

func (errorReadCloser) Close() error {
	return nil
}
