package publicsite

import (
	"context"
	"sort"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type UniverseRepository interface {
	ListUniverses(context.Context) ([]domain.Universe, error)
}

type UniverseCatalogService struct {
	repository UniverseRepository
}

func NewUniverseCatalogService(repository UniverseRepository) UniverseCatalogService {
	return UniverseCatalogService{repository: repository}
}

func (s UniverseCatalogService) ListUniverses(ctx context.Context) ([]domain.Universe, error) {
	universes, err := s.repository.ListUniverses(ctx)
	if err != nil {
		return nil, err
	}
	for _, universe := range universes {
		if err := universe.Validate(); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(universes, func(i, j int) bool {
		return universes[i].Number < universes[j].Number
	})
	return universes, nil
}
