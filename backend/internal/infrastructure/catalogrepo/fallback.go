package catalogrepo

import (
	"context"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type FallbackUniverseCatalog struct {
	Primary  apppublicsite.UniverseRepository
	Fallback apppublicsite.UniverseRepository
}

func (c FallbackUniverseCatalog) ListUniverses(ctx context.Context) ([]domain.Universe, error) {
	if c.Primary != nil {
		universes, err := c.Primary.ListUniverses(ctx)
		if err == nil && len(universes) > 0 {
			return universes, nil
		}
	}
	return c.Fallback.ListUniverses(ctx)
}
