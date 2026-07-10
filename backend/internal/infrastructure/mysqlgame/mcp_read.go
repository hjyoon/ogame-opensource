package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type MCPReadRepository struct {
	queryer Queryer
	prefix  string
}

func NewMCPReadRepository(db *sql.DB, prefix string) MCPReadRepository {
	return NewMCPReadRepositoryWithQueryer(SQLQueryer{DB: db}, prefix)
}

func NewMCPReadRepositoryWithQueryer(queryer Queryer, prefix string) MCPReadRepository {
	return MCPReadRepository{queryer: queryer, prefix: prefix}
}

func (r MCPReadRepository) ListMCPPlanets(ctx context.Context, playerID int) ([]domainmcp.Planet, error) {
	if r.queryer == nil {
		return nil, errors.New("mcp read repository queryer unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}

	currentPlanetID, sortBy, sortOrder, err := r.loadMCPPlanetListSettings(ctx, usersTable, playerID)
	if err != nil {
		return nil, err
	}
	return r.loadMCPPlanets(ctx, planetsTable, playerID, currentPlanetID, sortBy, sortOrder)
}

func (r MCPReadRepository) loadMCPPlanetListSettings(ctx context.Context, usersTable string, playerID int) (int, int, int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT aktplanet, hplanetid, sortby, sortorder FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, 0, err
		}
		return 0, 0, 0, errors.New("mcp player not found")
	}
	var activePlanetID int
	var homePlanetID int
	var sortBy int
	var sortOrder int
	if err := rows.Scan(&activePlanetID, &homePlanetID, &sortBy, &sortOrder); err != nil {
		return 0, 0, 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}
	if activePlanetID > 0 {
		return activePlanetID, sortBy, sortOrder, nil
	}
	return homePlanetID, sortBy, sortOrder, nil
}

func (r MCPReadRepository) loadMCPPlanets(ctx context.Context, planetsTable string, playerID int, currentPlanetID int, sortBy int, sortOrder int) ([]domainmcp.Planet, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, name, type, g, s, p FROM %s WHERE owner_id = ? AND type < ?%s", planetsTable, planetOrder(sortBy, sortOrder)),
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	planets := make([]domainmcp.Planet, 0)
	for rows.Next() {
		var planet domainmcp.Planet
		if err := rows.Scan(&planet.ID, &planet.Name, &planet.Type, &planet.Coordinates.Galaxy, &planet.Coordinates.System, &planet.Coordinates.Position); err != nil {
			return nil, err
		}
		planet.TypeName = mcpPlanetTypeName(planet.Type)
		planet.Current = planet.ID == currentPlanetID
		planets = append(planets, planet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return planets, nil
}

func mcpPlanetTypeName(planetType int) string {
	switch planetType {
	case domaingame.PlanetTypePlanet:
		return "planet"
	case domaingame.PlanetTypeMoon:
		return "moon"
	default:
		return "unknown"
	}
}
