package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type BuildingsRepository struct {
	queryer Queryer
	prefix  string
}

func NewBuildingsRepository(db *sql.DB, prefix string) BuildingsRepository {
	return BuildingsRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewBuildingsRepositoryWithQueryer(queryer Queryer, prefix string) BuildingsRepository {
	return BuildingsRepository{queryer: queryer, prefix: prefix}
}

func (r BuildingsRepository) GetBuildings(ctx context.Context, query appgame.BuildingsQuery) (domaingame.Buildings, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Buildings{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Buildings{}, err
	}

	overviewRepository := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix)
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Buildings{}, err
	}

	levels, err := r.loadBuildingLevels(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Buildings{}, err
	}
	research, err := r.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Buildings{}, err
	}
	speed, err := r.loadUniverseSpeed(ctx)
	if err != nil {
		return domaingame.Buildings{}, err
	}

	return domaingame.BuildBuildings(overview, levels, research, speed), nil
}

func (r BuildingsRepository) loadBuildingLevels(ctx context.Context, planetsTable string, playerID int, planetID int) (domaingame.BuildingLevels, error) {
	ids := domaingame.BuildingIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1", numericColumns(ids), planetsTable),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("building levels not found")
	}
	levels, err := scanLevelMap(rows, ids)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return levels, nil
}

func (r BuildingsRepository) loadResearchLevels(ctx context.Context, usersTable string, playerID int) (domaingame.ResearchLevels, error) {
	ids := domaingame.BuildingResearchIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT %s FROM %s WHERE player_id = ? LIMIT 1", numericColumns(ids), usersTable),
		playerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("research levels not found")
	}
	levels, err := scanResearchMap(rows, ids)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return levels, nil
}

func (r BuildingsRepository) loadUniverseSpeed(ctx context.Context) (float64, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return 0, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT speed FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 1, nil
	}
	var speed float64
	if err := rows.Scan(&speed); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if speed <= 0 {
		return 1, nil
	}
	return speed, nil
}

func numericColumns(ids []int) string {
	columns := make([]string, 0, len(ids))
	for _, id := range ids {
		columns = append(columns, fmt.Sprintf("`%d`", id))
	}
	return strings.Join(columns, ", ")
}

func scanLevelMap(rows Rows, ids []int) (domaingame.BuildingLevels, error) {
	values := make([]int, len(ids))
	dest := make([]any, len(ids))
	for index := range values {
		dest[index] = &values[index]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	levels := make(domaingame.BuildingLevels, len(ids))
	for index, id := range ids {
		levels[id] = values[index]
	}
	return levels, nil
}

func scanResearchMap(rows Rows, ids []int) (domaingame.ResearchLevels, error) {
	values := make([]int, len(ids))
	dest := make([]any, len(ids))
	for index := range values {
		dest[index] = &values[index]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	levels := make(domaingame.ResearchLevels, len(ids))
	for index, id := range ids {
		levels[id] = values[index]
	}
	return levels, nil
}
