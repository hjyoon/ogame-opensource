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
	usersTable, planetsTable, _, err := r.mcpReadTables()
	if err != nil {
		return nil, err
	}

	currentPlanetID, sortBy, sortOrder, err := r.loadMCPPlanetListSettings(ctx, usersTable, playerID)
	if err != nil {
		return nil, err
	}
	return r.loadMCPPlanets(ctx, planetsTable, playerID, currentPlanetID, sortBy, sortOrder)
}

func (r MCPReadRepository) GetMCPAccountOverview(ctx context.Context, playerID int) (domainmcp.AccountOverview, error) {
	if r.queryer == nil {
		return domainmcp.AccountOverview{}, errors.New("mcp read repository queryer unavailable")
	}
	usersTable, planetsTable, messagesTable, err := r.mcpReadTables()
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	account, err := r.loadMCPAccount(ctx, usersTable, playerID)
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	currentPlanetID := account.activePlanetID
	if currentPlanetID <= 0 {
		currentPlanetID = account.homePlanetID
	}
	current, err := r.loadMCPPlanet(ctx, planetsTable, playerID, currentPlanetID)
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	planetCount, err := r.countMCPPlanets(ctx, planetsTable, playerID)
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	unread, err := r.countMCPUnreadMessages(ctx, messagesTable, playerID)
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	current.Current = true
	return domainmcp.AccountOverview{
		PlayerID:  playerID,
		Commander: account.commander,
		Score: domainmcp.Score{
			Raw:     account.score,
			Display: displayMCPScore(account.score),
			Rank:    account.rank,
		},
		CurrentPlanet:  current,
		PlanetCount:    planetCount,
		UnreadMessages: unread,
	}, nil
}

func (r MCPReadRepository) mcpReadTables() (string, string, string, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return "", "", "", err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return "", "", "", err
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return "", "", "", err
	}
	return usersTable, planetsTable, messagesTable, nil
}

type mcpAccountRow struct {
	commander      string
	score          int64
	rank           int
	activePlanetID int
	homePlanetID   int
}

func (r MCPReadRepository) loadMCPAccount(ctx context.Context, usersTable string, playerID int) (mcpAccountRow, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT oname, score1, place1, aktplanet, hplanetid FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return mcpAccountRow{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return mcpAccountRow{}, err
		}
		return mcpAccountRow{}, errors.New("mcp player not found")
	}
	var account mcpAccountRow
	if err := rows.Scan(&account.commander, &account.score, &account.rank, &account.activePlanetID, &account.homePlanetID); err != nil {
		return mcpAccountRow{}, err
	}
	if err := rows.Err(); err != nil {
		return mcpAccountRow{}, err
	}
	return account, nil
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

func (r MCPReadRepository) loadMCPPlanet(ctx context.Context, planetsTable string, playerID int, planetID int) (domainmcp.Planet, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, name, type, g, s, p FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1", planetsTable),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return domainmcp.Planet{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domainmcp.Planet{}, err
		}
		return domainmcp.Planet{}, errors.New("mcp current planet not found")
	}
	planet, err := scanMCPPlanet(rows)
	if err != nil {
		return domainmcp.Planet{}, err
	}
	if err := rows.Err(); err != nil {
		return domainmcp.Planet{}, err
	}
	return planet, nil
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
		planet, err := scanMCPPlanet(rows)
		if err != nil {
			return nil, err
		}
		planet.Current = planet.ID == currentPlanetID
		planets = append(planets, planet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return planets, nil
}

func (r MCPReadRepository) countMCPPlanets(ctx context.Context, planetsTable string, playerID int) (int, error) {
	return r.singleMCPCount(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ? AND type < ?", planetsTable), playerID, planetTypeDebris)
}

func (r MCPReadRepository) countMCPUnreadMessages(ctx context.Context, messagesTable string, playerID int) (int, error) {
	return r.singleMCPCount(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ? AND shown = 0", messagesTable), playerID)
}

func (r MCPReadRepository) singleMCPCount(ctx context.Context, statement string, args ...any) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, statement, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func scanMCPPlanet(rows Rows) (domainmcp.Planet, error) {
	var planet domainmcp.Planet
	if err := rows.Scan(&planet.ID, &planet.Name, &planet.Type, &planet.Coordinates.Galaxy, &planet.Coordinates.System, &planet.Coordinates.Position); err != nil {
		return domainmcp.Planet{}, err
	}
	planet.TypeName = mcpPlanetTypeName(planet.Type)
	return planet, nil
}

func displayMCPScore(score int64) int64 {
	if score < 0 {
		return 0
	}
	return score / 1000
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
