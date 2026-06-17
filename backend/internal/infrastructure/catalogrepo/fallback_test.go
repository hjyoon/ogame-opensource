package catalogrepo

import (
	"context"
	"errors"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestFallbackUniverseCatalogUsesPrimaryWhenAvailable(t *testing.T) {
	catalog := FallbackUniverseCatalog{
		Primary:  fakeRepo{universes: []domain.Universe{universe(2)}},
		Fallback: fakeRepo{universes: []domain.Universe{universe(1)}},
	}

	universes, err := catalog.ListUniverses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universes) != 1 || universes[0].Number != 2 {
		t.Fatalf("expected primary universe, got %+v", universes)
	}
}

func TestFallbackUniverseCatalogUsesFallbackWhenPrimaryFails(t *testing.T) {
	catalog := FallbackUniverseCatalog{
		Primary:  fakeRepo{err: errors.New("primary failed")},
		Fallback: fakeRepo{universes: []domain.Universe{universe(1)}},
	}

	universes, err := catalog.ListUniverses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universes) != 1 || universes[0].Number != 1 {
		t.Fatalf("expected fallback universe, got %+v", universes)
	}
}

func TestFallbackUniverseCatalogUsesFallbackWhenPrimaryEmpty(t *testing.T) {
	catalog := FallbackUniverseCatalog{
		Primary:  fakeRepo{},
		Fallback: fakeRepo{universes: []domain.Universe{universe(1)}},
	}

	universes, err := catalog.ListUniverses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(universes) != 1 || universes[0].Number != 1 {
		t.Fatalf("expected fallback universe, got %+v", universes)
	}
}

type fakeRepo struct {
	universes []domain.Universe
	err       error
}

func (f fakeRepo) ListUniverses(context.Context) ([]domain.Universe, error) {
	return f.universes, f.err
}

func universe(number int) domain.Universe {
	return domain.Universe{
		Number:     number,
		Name:       "Universe",
		BaseURL:    "http://localhost:8888",
		Language:   "en",
		Speed:      1,
		FleetSpeed: 1,
		Status:     domain.UniverseOnline,
	}
}
