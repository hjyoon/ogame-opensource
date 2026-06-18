package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type ResearchRepository struct {
	queryer Queryer
	prefix  string
	now     func() time.Time
}

func NewResearchRepository(db *sql.DB, prefix string) ResearchRepository {
	return ResearchRepository{queryer: SQLQueryer{DB: db}, prefix: prefix, now: time.Now}
}

func NewResearchRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) ResearchRepository {
	if now == nil {
		now = time.Now
	}
	return ResearchRepository{queryer: queryer, prefix: prefix, now: now}
}

func (r ResearchRepository) GetResearch(ctx context.Context, query appgame.ResearchQuery) (domaingame.Research, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Research{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Research{}, err
	}

	overviewRepository := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix)
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Research{}, err
	}

	buildings := BuildingsRepository{queryer: r.queryer, prefix: r.prefix}
	levels, err := buildings.loadBuildingLevels(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Research{}, err
	}
	research, err := r.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Research{}, err
	}
	otherLabs, err := r.loadOtherResearchLabs(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Research{}, err
	}
	speed, err := buildings.loadUniverseSpeed(ctx)
	if err != nil {
		return domaingame.Research{}, err
	}
	technocrat, err := r.loadTechnocrat(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Research{}, err
	}

	labLevels := domaingame.BuildResearchLabLevels(levels[domaingame.BuildingResearchLab], otherLabs, research)
	return domaingame.BuildResearch(overview, levels, research, labLevels, speed, technocrat), nil
}

func (r ResearchRepository) loadResearchLevels(ctx context.Context, usersTable string, playerID int) (domaingame.ResearchLevels, error) {
	ids := domaingame.ResearchIDs()
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

func (r ResearchRepository) loadOtherResearchLabs(ctx context.Context, planetsTable string, playerID int, currentPlanetID int) ([]int, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, `%d` FROM %s WHERE owner_id = ? AND type = ? AND `%d` > 0", domaingame.BuildingResearchLab, planetsTable, domaingame.BuildingResearchLab),
		playerID,
		domaingame.PlanetTypePlanet,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	labs := []int{}
	for rows.Next() {
		var planetID int
		var lab int
		if err := rows.Scan(&planetID, &lab); err != nil {
			return nil, err
		}
		if planetID != currentPlanetID {
			labs = append(labs, lab)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return labs, nil
}

func (r ResearchRepository) loadTechnocrat(ctx context.Context, usersTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT tec_until FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("research premium state not found")
	}
	var technocratUntil int64
	if err := rows.Scan(&technocratUntil); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return technocratUntil > r.now().Unix(), nil
}
