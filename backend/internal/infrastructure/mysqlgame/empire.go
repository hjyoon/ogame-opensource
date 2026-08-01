package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type EmpireRepository struct {
	queryer         Queryer
	execer          Execer
	prefix          string
	now             func() time.Time
	updateResources bool
}

type empireUser struct {
	CommanderActive bool
	EnergyResearch  int
	Geologist       bool
	Engineer        bool
	SortBy          int
	SortOrder       int
}

func NewEmpireRepository(db *sql.DB, prefix string) EmpireRepository {
	runner := SQLQueryer{DB: db}
	return EmpireRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now, updateResources: true}
}

func NewEmpireReadRepository(db *sql.DB, prefix string) EmpireRepository {
	return NewEmpireRepositoryWithRunner(SQLQueryer{DB: db}, nil, prefix, time.Now)
}

func NewEmpireRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) EmpireRepository {
	var execer Execer
	if runner, ok := queryer.(Execer); ok {
		execer = runner
	}
	return NewEmpireRepositoryWithRunner(queryer, execer, prefix, now)
}

func NewEmpireRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) EmpireRepository {
	if now == nil {
		now = time.Now
	}
	return EmpireRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r EmpireRepository) GetEmpire(ctx context.Context, query appgame.EmpireQuery) (domaingame.Empire, *domaingame.EmpireActionIssue, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	if r.execer != nil {
		buildings := BuildingsRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: r.updateResources}
		if err := buildings.FinishDueBuildingQueues(ctx, int(r.currentTime().Unix())); err != nil {
			return domaingame.Empire{}, nil, err
		}
		shipyard := ShipyardRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: r.updateResources}
		if err := shipyard.FinishDueShipyardQueues(ctx, int(r.currentTime().Unix())); err != nil {
			return domaingame.Empire{}, nil, err
		}
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.updateResources = r.updateResources
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Empire{}, nil, err
	}

	user, err := r.loadEmpireUser(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	moonEnabled, err := r.loadMoonEnabled(ctx)
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	planetType := domaingame.NormalizeEmpirePlanetType(query.PlanetType, moonEnabled)
	hasMoons, err := r.loadHasMoons(ctx, planetsTable, query.PlayerID)
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	research, err := ResearchRepository{queryer: r.queryer, prefix: r.prefix}.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	speed, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadUniverseSpeed(ctx)
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	planets, err := r.loadEmpirePlanets(ctx, planetsTable, query.PlayerID, planetType, user, speed)
	if err != nil {
		return domaingame.Empire{}, nil, err
	}
	if err := r.attachEmpireBuildQueues(ctx, buildQueueTable, query.PlayerID, planets); err != nil {
		return domaingame.Empire{}, nil, err
	}
	empire := domaingame.BuildEmpire(overview, user.CommanderActive, planetType, moonEnabled, hasMoons, planets, research)
	if !user.CommanderActive {
		return empire, domaingame.EmpireActionIssueFor(domaingame.EmpireIssueCommanderRequired), nil
	}
	return empire, nil, nil
}

func (r EmpireRepository) GetMCPEmpire(ctx context.Context, playerID int, command domainmcp.EmpireCommand) (domainmcp.EmpireOverview, error) {
	if r.queryer == nil {
		return domainmcp.EmpireOverview{}, errors.New("empire reader unavailable")
	}
	empire, issue, err := r.GetEmpire(ctx, appgame.EmpireQuery{
		PlayerID:   playerID,
		PlanetID:   command.PlanetID,
		PlanetType: command.PlanetType,
	})
	if err != nil {
		return domainmcp.EmpireOverview{}, err
	}
	var actionIssue *domainmcp.ActionIssue
	if issue != nil {
		actionIssue = &domainmcp.ActionIssue{Code: issue.Code, Message: issue.Message}
	}
	return domainmcp.EmpireOverview{
		PlayerID:        playerID,
		PlanetID:        empire.CurrentPlanet.ID,
		CommanderActive: empire.CommanderActive,
		PlanetType:      empire.PlanetType,
		MoonEnabled:     empire.MoonEnabled,
		HasMoons:        empire.HasMoons,
		Issue:           actionIssue,
		Planets:         mcpEmpirePlanets(empire.Planets),
		Resources:       mcpEmpireResourceRows(empire.Resources),
		Buildings:       mcpEmpireLevelRows(empire.Buildings),
		Research:        mcpEmpireLevelRows(empire.Research),
		Fleet:           mcpEmpireCountRows(empire.Fleet),
		Defense:         mcpEmpireCountRows(empire.Defense),
	}, nil
}

func (r EmpireRepository) MutateEmpire(ctx context.Context, query appgame.EmpireMutationQuery) (appgame.EmpireMutationOutcome, error) {
	if r.execer == nil {
		return appgame.EmpireMutationOutcome{}, errors.New("empire updater unavailable")
	}
	buildings := BuildingsRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: r.updateResources}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return appgame.EmpireMutationOutcome{}, err
	}
	planet, err := buildings.loadBuildingMutationPlanet(ctx, planetsTable, query.PlayerID, query.PlanetID)
	if err != nil {
		return appgame.EmpireMutationOutcome{}, err
	}
	if planet.ID == 0 {
		return appgame.EmpireMutationOutcome{
			ActionIssue: empireActionIssueFromBuildings(domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid)),
		}, nil
	}
	outcome, err := buildings.MutateBuildings(ctx, appgame.BuildingsMutationQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
		Action:   query.Action,
		TechID:   query.TechID,
		ListID:   query.ListID,
	})
	if err != nil {
		return appgame.EmpireMutationOutcome{}, err
	}
	return appgame.EmpireMutationOutcome{ActionIssue: empireActionIssueFromBuildings(outcome.ActionIssue)}, nil
}

func empireActionIssueFromBuildings(issue *domaingame.BuildingsActionIssue) *domaingame.EmpireActionIssue {
	if issue == nil {
		return nil
	}
	return &domaingame.EmpireActionIssue{Code: issue.Code, Message: issue.Message}
}

func mcpEmpirePlanets(planets []domaingame.EmpirePlanet) []domainmcp.EmpirePlanet {
	result := make([]domainmcp.EmpirePlanet, 0, len(planets))
	for _, planet := range planets {
		result = append(result, domainmcp.EmpirePlanet{
			ID:       planet.ID,
			Name:     planet.Name,
			Type:     planet.Type,
			TypeName: mcpPlanetTypeName(planet.Type),
			Coordinates: domainmcp.Coordinates{
				Galaxy:   planet.Coordinates.Galaxy,
				System:   planet.Coordinates.System,
				Position: planet.Coordinates.Position,
			},
			Fields:    planet.Fields,
			MaxFields: planet.MaxFields,
			Resources: domainmcp.EmpireResources{
				Metal:     int(planet.Resources.Metal),
				Crystal:   int(planet.Resources.Crystal),
				Deuterium: int(planet.Resources.Deuterium),
			},
			Production: domainmcp.EmpireProduction{
				MetalHourly:     planet.Production.MetalHourly,
				CrystalHourly:   planet.Production.CrystalHourly,
				DeuteriumHourly: planet.Production.DeuteriumHourly,
				EnergyBalance:   planet.Production.EnergyBalance,
				EnergyCapacity:  planet.Production.EnergyCapacity,
			},
		})
	}
	return result
}

func mcpEmpireResourceRows(rows []domaingame.EmpireResourceRow) []domainmcp.EmpireResourceRow {
	result := make([]domainmcp.EmpireResourceRow, 0, len(rows))
	for _, row := range rows {
		values := make([]domainmcp.EmpireResourceValue, 0, len(row.Values))
		totalProduction := 0
		for _, value := range row.Values {
			totalProduction += value.Production
			values = append(values, domainmcp.EmpireResourceValue{
				PlanetID:   value.PlanetID,
				Amount:     value.Amount,
				Production: value.Production,
			})
		}
		averageProduction := 0.0
		if len(row.Values) > 0 {
			averageProduction = float64(totalProduction) / float64(len(row.Values))
		}
		aggregation := "average"
		if row.ID == domaingame.ResourceEnergy {
			aggregation = "total"
		}
		result = append(result, domainmcp.EmpireResourceRow{
			ID:                    row.ID,
			Name:                  row.Name,
			Values:                values,
			Total:                 row.Total,
			Production:            row.Production,
			ProductionAggregation: aggregation,
			TotalProduction:       totalProduction,
			AverageProduction:     averageProduction,
		})
	}
	return result
}

func mcpEmpireLevelRows(rows []domaingame.EmpireLevelRow) []domainmcp.EmpireLevelRow {
	result := make([]domainmcp.EmpireLevelRow, 0, len(rows))
	for _, row := range rows {
		values := make([]domainmcp.EmpireLevelValue, 0, len(row.Values))
		for _, value := range row.Values {
			values = append(values, domainmcp.EmpireLevelValue{
				PlanetID: value.PlanetID,
				Level:    value.Level,
				CanBuild: value.CanBuild,
				Queue:    mcpEmpireBuildQueue(value.Queue),
			})
		}
		result = append(result, domainmcp.EmpireLevelRow{
			ID:      row.ID,
			Name:    row.Name,
			Values:  values,
			Total:   row.Total,
			Average: row.Average,
		})
	}
	return result
}

func mcpEmpireBuildQueue(entries []domaingame.EmpireBuildQueueEntry) []domainmcp.EmpireBuildQueueEntry {
	result := make([]domainmcp.EmpireBuildQueueEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, domainmcp.EmpireBuildQueueEntry{
			ListID:   entry.ListID,
			Level:    entry.Level,
			Active:   entry.Active,
			Demolish: entry.Demolish,
		})
	}
	return result
}

func mcpEmpireCountRows(rows []domaingame.EmpireCountRow) []domainmcp.EmpireCountRow {
	result := make([]domainmcp.EmpireCountRow, 0, len(rows))
	for _, row := range rows {
		values := make([]domainmcp.EmpireCountValue, 0, len(row.Values))
		for _, value := range row.Values {
			values = append(values, domainmcp.EmpireCountValue{PlanetID: value.PlanetID, Count: value.Count})
		}
		result = append(result, domainmcp.EmpireCountRow{
			ID:     row.ID,
			Name:   row.Name,
			Values: values,
			Total:  row.Total,
		})
	}
	return result
}

func (r EmpireRepository) loadEmpireUser(ctx context.Context, usersTable string, playerID int) (empireUser, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(com_until, 0), COALESCE(geo_until, 0), COALESCE(eng_until, 0), `%d`, COALESCE(sortby, 0), COALESCE(sortorder, 0) FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchEnergy, usersTable),
		playerID,
	)
	if err != nil {
		return empireUser{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return empireUser{}, err
		}
		return empireUser{}, errors.New("empire user not found")
	}
	var commanderUntil int64
	var geologistUntil int64
	var engineerUntil int64
	var user empireUser
	if err := rows.Scan(&commanderUntil, &geologistUntil, &engineerUntil, &user.EnergyResearch, &user.SortBy, &user.SortOrder); err != nil {
		return empireUser{}, err
	}
	if err := rows.Err(); err != nil {
		return empireUser{}, err
	}
	now := r.currentTime().Unix()
	user.CommanderActive = commanderUntil > now
	user.Geologist = geologistUntil > now
	user.Engineer = engineerUntil > now
	return user, nil
}

func (r EmpireRepository) loadMoonEnabled(ctx context.Context) (bool, error) {
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return false, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(moons, 0) FROM %s LIMIT 1", uniTable))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var enabled int
	if err := rows.Scan(&enabled); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return enabled != 0, nil
}

func (r EmpireRepository) loadHasMoons(ctx context.Context, planetsTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ? AND type = ?", planetsTable), playerID, domaingame.PlanetTypeMoon)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r EmpireRepository) loadEmpirePlanets(ctx context.Context, planetsTable string, playerID int, planetType int, user empireUser, speed float64) ([]domaingame.EmpirePlanet, error) {
	dbPlanetType := domaingame.PlanetTypePlanet
	if planetType == domaingame.EmpirePlanetTypeMoons {
		dbPlanetType = domaingame.PlanetTypeMoon
	}
	levelIDs := empirePlanetLevelIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT planet_id, name, type, g, s, p, fields, maxfields, temp, lastpeek, `%d`, `%d`, `%d`, prod%d, prod%d, prod%d, prod%d, prod%d, prod%d, %s FROM %s WHERE owner_id = ? AND type = ?%s",
			domaingame.ResourceMetal,
			domaingame.ResourceCrystal,
			domaingame.ResourceDeuterium,
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			numericColumns(levelIDs),
			planetsTable,
			planetOrder(user.SortBy, user.SortOrder),
		),
		playerID,
		dbPlanetType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	planets := []domaingame.EmpirePlanet{}
	now := int(r.currentTime().Unix())
	for rows.Next() {
		planet, err := scanEmpirePlanet(rows, levelIDs, user, speed, now)
		if err != nil {
			return nil, err
		}
		planets = append(planets, planet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return planets, nil
}

func scanEmpirePlanet(rows Rows, levelIDs []int, user empireUser, speed float64, now int) (domaingame.EmpirePlanet, error) {
	var planet domaingame.EmpirePlanet
	var temperature int
	var lastPeek int
	var prodMetal float64
	var prodCrystal float64
	var prodDeuterium float64
	var prodSolar float64
	var prodFusion float64
	var prodSatellite float64
	values := make([]int, len(levelIDs))
	dest := []any{
		&planet.ID,
		&planet.Name,
		&planet.Type,
		&planet.Coordinates.Galaxy,
		&planet.Coordinates.System,
		&planet.Coordinates.Position,
		&planet.Fields,
		&planet.MaxFields,
		&temperature,
		&lastPeek,
		&planet.Resources.Metal,
		&planet.Resources.Crystal,
		&planet.Resources.Deuterium,
		&prodMetal,
		&prodCrystal,
		&prodDeuterium,
		&prodSolar,
		&prodFusion,
		&prodSatellite,
	}
	for index := range values {
		dest = append(dest, &values[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return domaingame.EmpirePlanet{}, err
	}

	planet.Levels = make(domaingame.BuildingLevels, len(domaingame.BuildingIDs()))
	planet.Fleet = make(domaingame.FleetCounts, len(domaingame.FleetIDs()))
	planet.Defense = make(domaingame.DefenseCounts, len(domaingame.DefenseIDs()))
	for index, id := range levelIDs {
		switch {
		case containsInt(domaingame.BuildingIDs(), id):
			planet.Levels[id] = values[index]
		case containsInt(domaingame.FleetIDs(), id):
			planet.Fleet[id] = values[index]
		case containsInt(domaingame.DefenseIDs(), id):
			planet.Defense[id] = values[index]
		}
	}

	production := domaingame.BuildResourceProduction(
		domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{
			ID:          planet.ID,
			Type:        planet.Type,
			Temperature: temperature,
			Resources:   planet.Resources,
		}},
		domaingame.ResourceProductionInputs{
			Levels:          planet.Levels,
			SolarSatellites: planet.Fleet[domaingame.FleetSolarSatellite],
			ProductionFactors: domaingame.ProductionFactors{
				domaingame.BuildingMetalMine:      prodMetal,
				domaingame.BuildingCrystalMine:    prodCrystal,
				domaingame.BuildingDeuteriumSynth: prodDeuterium,
				domaingame.BuildingSolarPlant:     prodSolar,
				domaingame.BuildingFusionReactor:  prodFusion,
				domaingame.FleetSolarSatellite:    prodSatellite,
			},
			EnergyResearch: user.EnergyResearch,
			UniverseSpeed:  speed,
			Geologist:      user.Geologist,
			Engineer:       user.Engineer,
		},
	)
	planet.Production = domaingame.EmpireProduction{
		MetalHourly:     int(production.Totals.Hour.Metal),
		CrystalHourly:   int(production.Totals.Hour.Crystal),
		DeuteriumHourly: int(production.Totals.Hour.Deuterium),
		EnergyBalance:   int(production.Totals.Hour.EnergyRaw),
		EnergyCapacity:  overviewEnergyCapacity(production),
	}
	if planet.Type == domaingame.PlanetTypePlanet {
		planet.Resources.MetalCapacity = storageCapacity(planet.Levels[domaingame.BuildingMetalStorage])
		planet.Resources.CrystalCapacity = storageCapacity(planet.Levels[domaingame.BuildingCrystalStorage])
		planet.Resources.DeuteriumCapacity = storageCapacity(planet.Levels[domaingame.BuildingDeuteriumTank])
		planet.Resources = domaingame.AccrueResources(planet.Resources, production.Totals.Hour, now-lastPeek)
	}
	return planet, nil
}

func (r EmpireRepository) attachEmpireBuildQueues(ctx context.Context, buildQueueTable string, playerID int, planets []domaingame.EmpirePlanet) error {
	if len(planets) == 0 {
		return nil
	}
	planetIndexes := make(map[int]int, len(planets))
	for index := range planets {
		planetIndexes[planets[index].ID] = index
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, list_id, tech_id, level, destroy FROM %s WHERE owner_id = ? ORDER BY planet_id ASC, list_id ASC", buildQueueTable),
		playerID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var planetID int
		var listID int
		var techID int
		var level int
		var destroy int
		if err := rows.Scan(&planetID, &listID, &techID, &level, &destroy); err != nil {
			return err
		}
		index, ok := planetIndexes[planetID]
		if !ok || !containsInt(domaingame.BuildingIDs(), techID) {
			continue
		}
		if planets[index].BuildQueue == nil {
			planets[index].BuildQueue = make(map[int][]domaingame.EmpireBuildQueueEntry)
		}
		planets[index].BuildQueue[techID] = append(planets[index].BuildQueue[techID], domaingame.EmpireBuildQueueEntry{
			ListID:   listID,
			Level:    level,
			Active:   listID == 1,
			Demolish: destroy != 0,
		})
	}
	return rows.Err()
}

func empirePlanetLevelIDs() []int {
	ids := append([]int{}, domaingame.BuildingIDs()...)
	ids = append(ids, domaingame.FleetIDs()...)
	ids = append(ids, domaingame.DefenseIDs()...)
	return ids
}

func containsInt(values []int, needle int) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func (r EmpireRepository) currentTime() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}
