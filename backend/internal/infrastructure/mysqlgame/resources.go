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

type ResourcesRepository struct {
	queryer Queryer
	prefix  string
	now     func() time.Time
}

func NewResourcesRepository(db *sql.DB, prefix string) ResourcesRepository {
	return ResourcesRepository{queryer: SQLQueryer{DB: db}, prefix: prefix, now: time.Now}
}

func NewResourcesRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) ResourcesRepository {
	if now == nil {
		now = time.Now
	}
	return ResourcesRepository{queryer: queryer, prefix: prefix, now: now}
}

func (r ResourcesRepository) GetResources(ctx context.Context, query appgame.ResourcesQuery) (domaingame.ResourceProduction, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}

	overviewRepository := OverviewRepository{queryer: r.queryer, prefix: r.prefix}
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}

	levels, satellites, factors, err := r.loadProductionSettings(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}
	energyResearch, geologist, engineer, err := r.loadResourceUser(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}
	speed, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadUniverseSpeed(ctx)
	if err != nil {
		return domaingame.ResourceProduction{}, err
	}

	return domaingame.BuildResourceProduction(overview, domaingame.ResourceProductionInputs{
		Levels:            levels,
		SolarSatellites:   satellites,
		ProductionFactors: factors,
		EnergyResearch:    energyResearch,
		UniverseSpeed:     speed,
		Geologist:         geologist,
		Engineer:          engineer,
	}), nil
}

func (r ResourcesRepository) loadProductionSettings(ctx context.Context, planetsTable string, playerID int, planetID int) (domaingame.BuildingLevels, int, domaingame.ProductionFactors, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, prod%d, prod%d, prod%d, prod%d, prod%d, prod%d FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1",
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			planetsTable,
		),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return nil, 0, nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, 0, nil, err
		}
		return nil, 0, nil, errors.New("resource production settings not found")
	}

	var metalMine, crystalMine, deuteriumSynth, solarPlant, fusionReactor, solarSatellites int
	var prodMetal, prodCrystal, prodDeuterium, prodSolar, prodFusion, prodSatellite float64
	if err := rows.Scan(
		&metalMine,
		&crystalMine,
		&deuteriumSynth,
		&solarPlant,
		&fusionReactor,
		&solarSatellites,
		&prodMetal,
		&prodCrystal,
		&prodDeuterium,
		&prodSolar,
		&prodFusion,
		&prodSatellite,
	); err != nil {
		return nil, 0, nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, err
	}

	return domaingame.BuildingLevels{
			domaingame.BuildingMetalMine:      metalMine,
			domaingame.BuildingCrystalMine:    crystalMine,
			domaingame.BuildingDeuteriumSynth: deuteriumSynth,
			domaingame.BuildingSolarPlant:     solarPlant,
			domaingame.BuildingFusionReactor:  fusionReactor,
		},
		solarSatellites,
		domaingame.ProductionFactors{
			domaingame.BuildingMetalMine:      prodMetal,
			domaingame.BuildingCrystalMine:    prodCrystal,
			domaingame.BuildingDeuteriumSynth: prodDeuterium,
			domaingame.BuildingSolarPlant:     prodSolar,
			domaingame.BuildingFusionReactor:  prodFusion,
			domaingame.FleetSolarSatellite:    prodSatellite,
		},
		nil
}

func (r ResourcesRepository) loadResourceUser(ctx context.Context, usersTable string, playerID int) (int, bool, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT `%d`, geo_until, eng_until FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchEnergy, usersTable),
		playerID,
	)
	if err != nil {
		return 0, false, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, false, false, err
		}
		return 0, false, false, errors.New("resource user not found")
	}
	var energyResearch int
	var geologistUntil int64
	var engineerUntil int64
	if err := rows.Scan(&energyResearch, &geologistUntil, &engineerUntil); err != nil {
		return 0, false, false, err
	}
	if err := rows.Err(); err != nil {
		return 0, false, false, err
	}
	now := r.now().Unix()
	return energyResearch, geologistUntil > now, engineerUntil > now, nil
}
