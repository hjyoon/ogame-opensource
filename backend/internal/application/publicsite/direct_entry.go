package publicsite

import (
	"context"
	"errors"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type ExternalImage struct {
	ContentType string
	Body        []byte
}

type ExternalImageFetcher interface {
	FetchExternalImage(context.Context, string, string) (ExternalImage, error)
}

type ExternalRedirectResult struct {
	Allowed bool
	URL     string
}

type ExternalImageProxyResult struct {
	Available   bool
	ContentType string
	Body        []byte
}

type DirectEntryService struct {
	images ExternalImageFetcher
}

func NewDirectEntryService(images ExternalImageFetcher) DirectEntryService {
	return DirectEntryService{images: images}
}

func (s DirectEntryService) ResolveExternalRedirect(_ context.Context, rawURL string) ExternalRedirectResult {
	normalized, ok := domain.NormalizeExternalURL(rawURL, true)
	return ExternalRedirectResult{Allowed: ok, URL: normalized}
}

func (s DirectEntryService) ProxyExternalImage(ctx context.Context, rawURL string) (ExternalImageProxyResult, error) {
	normalized, ok := domain.NormalizeExternalURL(rawURL, true)
	if !ok {
		return ExternalImageProxyResult{}, nil
	}
	expectedMime, ok := domain.ExternalImageMimeType(normalized)
	if !ok {
		return ExternalImageProxyResult{}, nil
	}
	if s.images == nil {
		return ExternalImageProxyResult{}, errors.New("external image fetcher unavailable")
	}
	image, err := s.images.FetchExternalImage(ctx, normalized, expectedMime)
	if err != nil {
		return ExternalImageProxyResult{}, nil
	}
	if image.ContentType != expectedMime || len(image.Body) == 0 {
		return ExternalImageProxyResult{}, nil
	}
	return ExternalImageProxyResult{Available: true, ContentType: image.ContentType, Body: image.Body}, nil
}
