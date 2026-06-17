package publicsite

import (
	"context"
	"errors"
	"testing"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestUniverseCatalogServiceSortsAndValidatesUniverses(t *testing.T) {
	service := NewUniverseCatalogService(fakeUniverseRepository{universes: []domain.Universe{
		universe(2, "Universe 2"),
		universe(1, "Universe 1"),
	}})

	universes, err := service.ListUniverses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if universes[0].Number != 1 || universes[1].Number != 2 {
		t.Fatalf("expected sorted universes, got %+v", universes)
	}
}

func TestUniverseCatalogServiceReturnsRepositoryError(t *testing.T) {
	wantErr := errors.New("repository failed")
	service := NewUniverseCatalogService(fakeUniverseRepository{err: wantErr})

	if _, err := service.ListUniverses(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestUniverseCatalogServiceRejectsInvalidUniverse(t *testing.T) {
	service := NewUniverseCatalogService(fakeUniverseRepository{universes: []domain.Universe{{Number: 1}}})

	if _, err := service.ListUniverses(context.Background()); err == nil {
		t.Fatal("expected validation error")
	}
}

type fakeUniverseRepository struct {
	universes []domain.Universe
	err       error
}

func (f fakeUniverseRepository) ListUniverses(context.Context) ([]domain.Universe, error) {
	return f.universes, f.err
}

func universe(number int, name string) domain.Universe {
	return domain.Universe{
		Number:     number,
		Name:       name,
		BaseURL:    "http://localhost:8888",
		Language:   "en",
		Speed:      1,
		FleetSpeed: 1,
		Status:     domain.UniverseOnline,
	}
}
