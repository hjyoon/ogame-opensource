package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type ExternalImageFetcher struct {
	client   *http.Client
	maxBytes int64
}

func NewExternalImageFetcher() ExternalImageFetcher {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if _, ok := domain.NormalizeExternalURL(req.URL.String(), true); !ok {
				return errors.New("unsafe external image redirect")
			}
			return nil
		},
	}
	return ExternalImageFetcher{client: client, maxBytes: 5 * 1024 * 1024}
}

func NewExternalImageFetcherWithClient(client *http.Client, maxBytes int64) ExternalImageFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	return ExternalImageFetcher{client: client, maxBytes: maxBytes}
}

func (f ExternalImageFetcher) FetchExternalImage(ctx context.Context, rawURL string, expectedMime string) (apppublicsite.ExternalImage, error) {
	if _, ok := domain.NormalizeExternalURL(rawURL, true); !ok {
		return apppublicsite.ExternalImage{}, errors.New("unsafe external image URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return apppublicsite.ExternalImage{}, err
	}
	req.Header.Set("User-Agent", "ogame-go")
	resp, err := f.client.Do(req)
	if err != nil {
		return apppublicsite.ExternalImage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apppublicsite.ExternalImage{}, errors.New("external image returned non-2xx status")
	}
	if contentType := cleanContentType(resp.Header.Get("Content-Type")); contentType != "" && contentType != expectedMime {
		return apppublicsite.ExternalImage{}, errors.New("external image content type mismatch")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return apppublicsite.ExternalImage{}, err
	}
	if int64(len(body)) > f.maxBytes {
		return apppublicsite.ExternalImage{}, errors.New("external image exceeds size limit")
	}
	if detected := http.DetectContentType(body); cleanContentType(detected) != expectedMime {
		return apppublicsite.ExternalImage{}, errors.New("external image detected content type mismatch")
	}
	return apppublicsite.ExternalImage{ContentType: expectedMime, Body: body}, nil
}

func cleanContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
}
