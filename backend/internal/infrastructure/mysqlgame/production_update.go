package mysqlgame

import (
	"context"
	"fmt"
	"math"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type resourceUpdatePlanet struct {
	ID              int
	OwnerID         int
	Type            int
	Temperature     int
	LastPeek        int
	Resources       domaingame.Resources
	Levels          domaingame.BuildingLevels
	SolarSatellites int
	Production      domaingame.ProductionFactors
}

type resourceUpdateUser struct {
	ID             int
	EnergyResearch int
	GeologistUntil int64
	EngineerUntil  int64
}

func (r OverviewRepository) updatePlanetResources(ctx context.Context, usersTable string, planetsTable string, playerID int, planetID int, until int) error {
	return updatePlanetResources(ctx, r.queryer, r.execer, r.prefix, usersTable, planetsTable, playerID, planetID, until, r.currentTime())
}

func (r BuildingsRepository) updatePlanetResources(ctx context.Context, usersTable string, planetsTable string, playerID int, planetID int, until int) error {
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	return updatePlanetResources(ctx, r.queryer, r.execer, r.prefix, usersTable, planetsTable, playerID, planetID, until, now)
}

func (r ResourcesRepository) updatePlanetResources(ctx context.Context, usersTable string, planetsTable string, playerID int, planetID int, until int) error {
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	return updatePlanetResources(ctx, r.queryer, r.execer, r.prefix, usersTable, planetsTable, playerID, planetID, until, now)
}

func updatePlanetResources(ctx context.Context, queryer Queryer, execer Execer, prefix string, usersTable string, planetsTable string, playerID int, planetID int, until int, now time.Time) error {
	if execer == nil || planetID <= 0 || until <= 0 {
		return nil
	}
	planet, found, err := loadResourceUpdatePlanet(ctx, queryer, planetsTable, playerID, planetID)
	if err != nil || !found {
		return err
	}
	if planet.Type != domaingame.PlanetTypePlanet {
		return nil
	}
	user, found, err := loadResourceUpdateUser(ctx, queryer, usersTable, planet.OwnerID)
	if err != nil || !found || user.ID == userSpace {
		return err
	}
	diff := until - planet.LastPeek
	if diff <= 0 {
		return nil
	}
	speed, err := (BuildingsRepository{queryer: queryer, prefix: prefix}).loadUniverseSpeed(ctx)
	if err != nil {
		return err
	}
	production := domaingame.BuildResourceProduction(
		domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{
			ID:          planet.ID,
			Type:        planet.Type,
			Temperature: planet.Temperature,
			Resources:   planet.Resources,
		}},
		domaingame.ResourceProductionInputs{
			Levels:            planet.Levels,
			SolarSatellites:   planet.SolarSatellites,
			ProductionFactors: planet.Production,
			EnergyResearch:    user.EnergyResearch,
			UniverseSpeed:     speed,
			Geologist:         user.GeologistUntil > now.Unix(),
			Engineer:          user.EngineerUntil > now.Unix(),
		},
	)
	next := planet.Resources
	next.Metal = accruedResource(next.Metal, production.Totals.Hour.Metal, diff, float64(next.MetalCapacity))
	next.Crystal = accruedResource(next.Crystal, production.Totals.Hour.Crystal, diff, float64(next.CrystalCapacity))
	next.Deuterium = accruedResource(next.Deuterium, production.Totals.Hour.Deuterium, diff, float64(next.DeuteriumCapacity))
	_, err = execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET `%d` = ?, `%d` = ?, `%d` = ?, lastpeek = ? WHERE planet_id = ? AND owner_id = ? AND lastpeek = ?", planetsTable, resourceMetal, resourceCrystal, resourceDeuterium),
		next.Metal,
		next.Crystal,
		next.Deuterium,
		until,
		planet.ID,
		planet.OwnerID,
		planet.LastPeek,
	)
	return err
}

func loadResourceUpdatePlanet(ctx context.Context, queryer Queryer, planetsTable string, playerID int, planetID int) (resourceUpdatePlanet, bool, error) {
	ids := []int{
		domaingame.BuildingMetalMine,
		domaingame.BuildingCrystalMine,
		domaingame.BuildingDeuteriumSynth,
		domaingame.BuildingSolarPlant,
		domaingame.BuildingFusionReactor,
		domaingame.FleetSolarSatellite,
	}
	rows, err := queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT planet_id, owner_id, type, temp, lastpeek, `%d`, `%d`, `%d`, `%d`, `%d`, `%d`, %s, prod%d, prod%d, prod%d, prod%d, prod%d, prod%d FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1",
			resourceMetal,
			resourceCrystal,
			resourceDeuterium,
			buildingMetalStorage,
			buildingCrystalStorage,
			buildingDeuteriumStorage,
			numericColumns(ids),
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
		return resourceUpdatePlanet{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return resourceUpdatePlanet{}, false, err
		}
		return resourceUpdatePlanet{}, false, nil
	}
	var planet resourceUpdatePlanet
	var metalStorageLevel int
	var crystalStorageLevel int
	var deuteriumStorageLevel int
	var metalMine int
	var crystalMine int
	var deuteriumSynth int
	var solarPlant int
	var fusionReactor int
	var solarSatellites int
	var prodMetal float64
	var prodCrystal float64
	var prodDeuterium float64
	var prodSolar float64
	var prodFusion float64
	var prodSatellite float64
	if err := rows.Scan(
		&planet.ID,
		&planet.OwnerID,
		&planet.Type,
		&planet.Temperature,
		&planet.LastPeek,
		&planet.Resources.Metal,
		&planet.Resources.Crystal,
		&planet.Resources.Deuterium,
		&metalStorageLevel,
		&crystalStorageLevel,
		&deuteriumStorageLevel,
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
		return resourceUpdatePlanet{}, false, err
	}
	if err := rows.Err(); err != nil {
		return resourceUpdatePlanet{}, false, err
	}
	planet.Resources.MetalCapacity = storageCapacity(metalStorageLevel)
	planet.Resources.CrystalCapacity = storageCapacity(crystalStorageLevel)
	planet.Resources.DeuteriumCapacity = storageCapacity(deuteriumStorageLevel)
	planet.Levels = domaingame.BuildingLevels{
		domaingame.BuildingMetalMine:      metalMine,
		domaingame.BuildingCrystalMine:    crystalMine,
		domaingame.BuildingDeuteriumSynth: deuteriumSynth,
		domaingame.BuildingSolarPlant:     solarPlant,
		domaingame.BuildingFusionReactor:  fusionReactor,
	}
	planet.SolarSatellites = solarSatellites
	planet.Production = domaingame.ProductionFactors{
		domaingame.BuildingMetalMine:      prodMetal,
		domaingame.BuildingCrystalMine:    prodCrystal,
		domaingame.BuildingDeuteriumSynth: prodDeuterium,
		domaingame.BuildingSolarPlant:     prodSolar,
		domaingame.BuildingFusionReactor:  prodFusion,
		domaingame.FleetSolarSatellite:    prodSatellite,
	}
	return planet, true, nil
}

func loadResourceUpdateUser(ctx context.Context, queryer Queryer, usersTable string, playerID int) (resourceUpdateUser, bool, error) {
	rows, err := queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, `%d`, geo_until, eng_until FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchEnergy, usersTable),
		playerID,
	)
	if err != nil {
		return resourceUpdateUser{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return resourceUpdateUser{}, false, err
		}
		return resourceUpdateUser{}, false, nil
	}
	var user resourceUpdateUser
	if err := rows.Scan(&user.ID, &user.EnergyResearch, &user.GeologistUntil, &user.EngineerUntil); err != nil {
		return resourceUpdateUser{}, false, err
	}
	if err := rows.Err(); err != nil {
		return resourceUpdateUser{}, false, err
	}
	return user, true, nil
}

func accruedResource(current float64, hourly float64, seconds int, capacity float64) float64 {
	if current >= capacity {
		return current
	}
	next := current + hourly*float64(seconds)/3600
	return math.Min(next, capacity)
}
