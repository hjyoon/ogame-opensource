package mysqlgame

import (
	"context"
	"database/sql"
	"errors"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
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

	overviewRepository := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix)
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

	technology := domaingame.BuildTechnology(overview, levels, research)
	if query.TechnologyDetailsID > 0 || query.TechnologyInfoID > 0 {
		speed, err := buildings.loadUniverseSpeed(ctx)
		if err != nil {
			return domaingame.Technology{}, err
		}
		if query.TechnologyDetailsID > 0 {
			if details, ok := domaingame.BuildTechnologyDetailsWithSpeed(query.TechnologyDetailsID, levels, research, speed); ok {
				technology.Details = &details
			}
		}
		if query.TechnologyInfoID > 0 {
			if info, ok := domaingame.BuildTechnologyInfoWithSpeed(query.TechnologyInfoID, overview.CurrentPlanet, levels, research, speed); ok {
				technology.Info = &info
			}
		}
	}
	return technology, nil
}

func (r TechnologyRepository) GetMCPTechnology(ctx context.Context, playerID int, command domainmcp.TechnologyCommand) (domainmcp.TechnologyTree, error) {
	if r.queryer == nil {
		return domainmcp.TechnologyTree{}, errors.New("technology reader unavailable")
	}
	technology, err := r.GetTechnology(ctx, appgame.TechnologyQuery{
		PlayerID:            playerID,
		PlanetID:            command.PlanetID,
		TechnologyDetailsID: command.DetailsID,
		TechnologyInfoID:    command.InfoID,
	})
	if err != nil {
		return domainmcp.TechnologyTree{}, err
	}
	return domainmcp.TechnologyTree{
		PlayerID: playerID,
		PlanetID: technology.CurrentPlanet.ID,
		Groups:   mcpTechnologyGroups(technology.Groups),
		Details:  mcpTechnologyDetails(technology.Details),
		Info:     mcpTechnologyInfo(technology.Info),
	}, nil
}

func mcpTechnologyGroups(groups []domaingame.TechnologyGroup) []domainmcp.TechnologyGroup {
	result := make([]domainmcp.TechnologyGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, domainmcp.TechnologyGroup{
			Key:   group.Key,
			Name:  group.Name,
			Items: mcpTechnologyItems(group.Items),
		})
	}
	return result
}

func mcpTechnologyItems(items []domaingame.TechnologyItem) []domainmcp.TechnologyItem {
	result := make([]domainmcp.TechnologyItem, 0, len(items))
	for _, item := range items {
		result = append(result, domainmcp.TechnologyItem{
			ID:               item.ID,
			Name:             item.Name,
			Requirements:     mcpTechnologyRequirements(item.Requirements),
			DetailsAvailable: item.DetailsAvailable,
		})
	}
	return result
}

func mcpTechnologyRequirements(requirements []domaingame.TechnologyRequirement) []domainmcp.TechnologyRequirement {
	result := make([]domainmcp.TechnologyRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		result = append(result, domainmcp.TechnologyRequirement{
			ID:           requirement.ID,
			Name:         requirement.Name,
			Level:        requirement.Level,
			CurrentLevel: requirement.CurrentLevel,
			Met:          requirement.Met,
		})
	}
	return result
}

func mcpTechnologyDetails(details *domaingame.TechnologyDetails) *domainmcp.TechnologyDetails {
	if details == nil {
		return nil
	}
	levels := make([]domainmcp.TechnologyDetailsLevel, 0, len(details.Levels))
	for _, level := range details.Levels {
		levels = append(levels, domainmcp.TechnologyDetailsLevel{
			Step:         level.Step,
			Requirements: mcpTechnologyRequirements(level.Requirements),
		})
	}
	return &domainmcp.TechnologyDetails{
		Target:   mcpTechnologyItems([]domaingame.TechnologyItem{details.Target})[0],
		Levels:   levels,
		Demolish: mcpTechnologyDemolish(details.Demolish),
	}
}

func mcpTechnologyInfo(info *domaingame.TechnologyInfo) *domainmcp.TechnologyInfo {
	if info == nil {
		return nil
	}
	rows := make([]domainmcp.TechnologyInfoRow, 0, len(info.Rows))
	for _, row := range info.Rows {
		rows = append(rows, domainmcp.TechnologyInfoRow{
			Level:                row.Level,
			Current:              row.Current,
			Production:           row.Production,
			ProductionDifference: row.ProductionDifference,
			Energy:               row.Energy,
			EnergyDifference:     row.EnergyDifference,
			Storage:              row.Storage,
			StorageDifference:    row.StorageDifference,
			DeuteriumConsumption: row.DeuteriumConsumption,
			DeuteriumDifference:  row.DeuteriumDifference,
		})
	}
	return &domainmcp.TechnologyInfo{
		ID:          info.ID,
		Name:        info.Name,
		Description: info.Description,
		Level:       info.Level,
		Kind:        info.Kind,
		Rows:        rows,
		Demolish:    mcpTechnologyDemolish(info.Demolish),
	}
}

func mcpTechnologyDemolish(demolish *domaingame.TechnologyDemolish) *domainmcp.TechnologyDemolish {
	if demolish == nil {
		return nil
	}
	return &domainmcp.TechnologyDemolish{
		Level:           demolish.Level,
		Cost:            mcpTechnologyCost(demolish.Cost),
		DurationSeconds: demolish.DurationSeconds,
	}
}

func mcpTechnologyCost(cost domaingame.BuildingCost) domainmcp.TechnologyCost {
	return domainmcp.TechnologyCost{
		Metal:     cost.Metal,
		Crystal:   cost.Crystal,
		Deuterium: cost.Deuterium,
		Energy:    cost.Energy,
	}
}
