package mysqlgame

import (
	"context"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

// projectMCPPlanetResources keeps read-only option views consistent with
// get_planet_resources without persisting lastpeek or accrued resources.
func projectMCPPlanetResources(ctx context.Context, queryer Queryer, prefix string, now func() time.Time, playerID int, planet domaingame.PlanetOverview) (domaingame.PlanetOverview, error) {
	reader := NewMCPReadRepositoryWithQueryer(queryer, prefix)
	if now != nil {
		reader.now = now
	}
	projected, err := reader.GetMCPPlanetResources(ctx, playerID, planet.ID)
	if err != nil {
		return domaingame.PlanetOverview{}, err
	}
	planet.Resources.Metal = projected.Resources.Metal
	planet.Resources.Crystal = projected.Resources.Crystal
	planet.Resources.Deuterium = projected.Resources.Deuterium
	planet.Resources.DarkMatter = projected.Resources.DarkMatter
	planet.Resources.MetalCapacity = projected.Capacity.Metal
	planet.Resources.CrystalCapacity = projected.Capacity.Crystal
	planet.Resources.DeuteriumCapacity = projected.Capacity.Deuterium
	planet.Resources.Energy = projected.Energy.Available
	planet.Resources.EnergyCapacity = projected.Energy.Capacity
	return planet, nil
}
