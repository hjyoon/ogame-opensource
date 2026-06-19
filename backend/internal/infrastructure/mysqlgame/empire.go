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
	empire := domaingame.BuildEmpire(overview, user.CommanderActive, planetType, moonEnabled, hasMoons, planets, research)
	if !user.CommanderActive {
		return empire, domaingame.EmpireActionIssueFor(domaingame.EmpireIssueCommanderRequired), nil
	}
	return empire, nil, nil
}

func (r EmpireRepository) MutateEmpire(ctx context.Context, query appgame.EmpireMutationQuery) (appgame.EmpireMutationOutcome, error) {
	if r.execer == nil {
		return appgame.EmpireMutationOutcome{}, errors.New("empire updater unavailable")
	}
	buildings := BuildingsRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.now, updateResources: r.updateResources}
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
			"SELECT planet_id, name, type, g, s, p, fields, maxfields, temp, `%d`, `%d`, `%d`, prod%d, prod%d, prod%d, prod%d, prod%d, prod%d, %s FROM %s WHERE owner_id = ? AND type = ?%s",
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
	for rows.Next() {
		planet, err := scanEmpirePlanet(rows, levelIDs, user, speed)
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

func scanEmpirePlanet(rows Rows, levelIDs []int, user empireUser, speed float64) (domaingame.EmpirePlanet, error) {
	var planet domaingame.EmpirePlanet
	var temperature int
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
		EnergyCapacity:  int(production.Totals.Hour.Energy),
	}
	return planet, nil
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
