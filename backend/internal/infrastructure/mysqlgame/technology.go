package mysqlgame

import (
	"context"
	"database/sql"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type TechnologyRepository struct {
	queryer Queryer
	prefix  string
}

func NewTechnologyRepository(db *sql.DB, prefix string) TechnologyRepository {
	return TechnologyRepository{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewTechnologyRepositoryWithQueryer(queryer Queryer, prefix string) TechnologyRepository {
	return TechnologyRepository{queryer: queryer, prefix: prefix}
}

func (r TechnologyRepository) GetTechnology(ctx context.Context, query appgame.TechnologyQuery) (domaingame.Technology, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Technology{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Technology{}, err
	}

	overviewRepository := OverviewRepository{queryer: r.queryer, prefix: r.prefix}
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Technology{}, err
	}

	buildings := BuildingsRepository{queryer: r.queryer, prefix: r.prefix}
	levels, err := buildings.loadBuildingLevels(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Technology{}, err
	}
	research, err := ResearchRepository{queryer: r.queryer, prefix: r.prefix}.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Technology{}, err
	}

	return domaingame.BuildTechnology(overview, levels, research), nil
}
