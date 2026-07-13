package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type PhalanxRepository struct {
	queryer         Queryer
	execer          Execer
	prefix          string
	now             func() time.Time
	updateResources bool
}

func NewPhalanxRepository(db *sql.DB, prefix string) PhalanxRepository {
	runner := SQLQueryer{DB: db}
	return PhalanxRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now, updateResources: true}
}

func NewPhalanxRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) PhalanxRepository {
	if now == nil {
		now = time.Now
	}
	return PhalanxRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r PhalanxRepository) PreviewMCPPhalanxScan(ctx context.Context, playerID int, command domainmcp.PhalanxScanCommand) (domainmcp.PhalanxScanResult, error) {
	if r.queryer == nil {
		return domainmcp.PhalanxScanResult{}, errors.New("phalanx reader unavailable")
	}
	phalanx, err := r.previewPhalanx(ctx, appgame.PhalanxQuery{
		PlayerID:       playerID,
		PlanetID:       command.PlanetID,
		TargetPlanetID: command.TargetPlanetID,
	})
	if err != nil {
		return domainmcp.PhalanxScanResult{}, err
	}
	return mcpPhalanxScanResult(playerID, phalanx, r.now().Unix()), nil
}

func (r PhalanxRepository) ScanMCPPhalanx(ctx context.Context, playerID int, command domainmcp.PhalanxScanCommand) (domainmcp.PhalanxScanResult, error) {
	phalanx, err := r.GetPhalanx(ctx, appgame.PhalanxQuery{
		PlayerID:       playerID,
		PlanetID:       command.PlanetID,
		TargetPlanetID: command.TargetPlanetID,
	})
	if err != nil {
		return domainmcp.PhalanxScanResult{}, err
	}
	result := mcpPhalanxScanResult(playerID, phalanx, r.now().Unix())
	result.Executed = phalanx.ActionIssue == nil
	return result, nil
}

func (r PhalanxRepository) previewPhalanx(ctx context.Context, query appgame.PhalanxQuery) (domaingame.Phalanx, error) {
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	overview, err := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	source, found, err := r.loadPhalanxPlanet(ctx, planetsTable, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	if !found {
		source = domaingame.PhalanxPlanet{
			ID:          overview.CurrentPlanet.ID,
			OwnerID:     query.PlayerID,
			Type:        overview.CurrentPlanet.Type,
			Name:        overview.CurrentPlanet.Name,
			Coordinates: overview.CurrentPlanet.Coordinates,
			Deuterium:   overview.CurrentPlanet.Resources.Deuterium,
		}
	}
	target, found, err := r.loadPhalanxPlanet(ctx, planetsTable, query.TargetPlanetID)
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	if !found {
		target.ID = query.TargetPlanetID
	}
	issue := domaingame.PhalanxScanIssue(query.PlayerID, source, target)
	return domaingame.NewPhalanx(overview, source, target, nil, issue), nil
}

func mcpPhalanxScanResult(playerID int, phalanx domaingame.Phalanx, now int64) domainmcp.PhalanxScanResult {
	return domainmcp.PhalanxScanResult{
		PlayerID:           playerID,
		PlanetID:           phalanx.Source.ID,
		TargetPlanetID:     phalanx.Target.ID,
		Commander:          phalanx.Commander,
		Source:             mcpPhalanxPlanet(phalanx.Source),
		Target:             mcpPhalanxPlanet(phalanx.Target),
		Cost:               phalanx.Cost,
		RemainingDeuterium: phalanx.RemainingDeuterium,
		Events:             mcpFleetMovementsFromGame(phalanx.Events, now),
		Issue:              mcpPhalanxIssue(phalanx.ActionIssue),
	}
}

func mcpPhalanxPlanet(planet domaingame.PhalanxPlanet) domainmcp.PhalanxPlanet {
	return domainmcp.PhalanxPlanet{
		ID:       planet.ID,
		OwnerID:  planet.OwnerID,
		Name:     planet.Name,
		Type:     planet.Type,
		TypeName: mcpPlanetTypeName(planet.Type),
		Coordinates: domainmcp.Coordinates{
			Galaxy:   planet.Coordinates.Galaxy,
			System:   planet.Coordinates.System,
			Position: planet.Coordinates.Position,
		},
		PhalanxLevel:  planet.PhalanxLevel,
		Deuterium:     planet.Deuterium,
		ReportHeading: planet.ReportHeading,
	}
}

func mcpPhalanxIssue(issue *domaingame.PhalanxActionIssue) *domainmcp.ActionIssue {
	if issue == nil {
		return nil
	}
	return &domainmcp.ActionIssue{Code: issue.Code, Message: issue.Message}
}

func (r PhalanxRepository) GetPhalanx(ctx context.Context, query appgame.PhalanxQuery) (domaingame.Phalanx, error) {
	if r.queryer == nil {
		return domaingame.Phalanx{}, errors.New("phalanx reader unavailable")
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Phalanx{}, err
	}

	overviewRepository := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix)
	overviewRepository.now = r.now
	overviewRepository.updateResources = r.updateResources
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Phalanx{}, err
	}

	source, found, err := r.loadPhalanxPlanet(ctx, planetsTable, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	if !found {
		source = domaingame.PhalanxPlanet{
			ID:          overview.CurrentPlanet.ID,
			OwnerID:     query.PlayerID,
			Type:        overview.CurrentPlanet.Type,
			Name:        overview.CurrentPlanet.Name,
			Coordinates: overview.CurrentPlanet.Coordinates,
			Deuterium:   overview.CurrentPlanet.Resources.Deuterium,
		}
	}
	target, found, err := r.loadPhalanxPlanet(ctx, planetsTable, query.TargetPlanetID)
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	if !found {
		target.ID = query.TargetPlanetID
	}

	issue := domaingame.PhalanxScanIssue(query.PlayerID, source, target)
	if issue != nil {
		return domaingame.NewPhalanx(overview, source, target, nil, issue), nil
	}
	if r.execer == nil {
		return domaingame.Phalanx{}, errors.New("phalanx writer unavailable")
	}

	events, err := r.loadPhalanxEvents(ctx, queueTable, fleetTable, planetsTable, usersTable, target)
	if err != nil {
		return domaingame.Phalanx{}, err
	}
	remaining := source.Deuterium - domaingame.PhalanxCost
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET `%d` = ?, lastpeek = ? WHERE planet_id = ? AND owner_id = ? LIMIT 1", planetsTable, resourceDeuterium),
		remaining,
		r.now().Unix(),
		source.ID,
		query.PlayerID,
	); err != nil {
		return domaingame.Phalanx{}, err
	}

	phalanx := domaingame.NewPhalanx(overview, source, target, events, nil)
	phalanx.RemainingDeuterium = remaining
	return phalanx, nil
}

func (r PhalanxRepository) loadPhalanxPlanet(ctx context.Context, planetsTable string, planetID int) (domaingame.PhalanxPlanet, bool, error) {
	if planetID <= 0 {
		return domaingame.PhalanxPlanet{}, false, nil
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, owner_id, name, type, g, s, p, `%d`, `%d` FROM %s WHERE planet_id = ? LIMIT 1", domaingame.BuildingSensorPhalanx, resourceDeuterium, planetsTable),
		planetID,
	)
	if err != nil {
		return domaingame.PhalanxPlanet{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.PhalanxPlanet{}, false, err
		}
		return domaingame.PhalanxPlanet{}, false, nil
	}
	var planet domaingame.PhalanxPlanet
	if err := rows.Scan(
		&planet.ID,
		&planet.OwnerID,
		&planet.Name,
		&planet.Type,
		&planet.Coordinates.Galaxy,
		&planet.Coordinates.System,
		&planet.Coordinates.Position,
		&planet.PhalanxLevel,
		&planet.Deuterium,
	); err != nil {
		return domaingame.PhalanxPlanet{}, false, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.PhalanxPlanet{}, false, err
	}
	return planet, true, nil
}

func (r PhalanxRepository) loadPhalanxEvents(ctx context.Context, queueTable string, fleetTable string, planetsTable string, usersTable string, target domaingame.PhalanxPlanet) ([]domaingame.FleetMission, error) {
	fleetIDs := domaingame.FleetIDs()
	resourceIDs := overviewTransportableResourceIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.sub_id, q.start, q.end, COALESCE(f.flight_time, 0), COALESCE(f.deploy_time, 0), f.mission, COALESCE(f.ipm_amount, 0), COALESCE(f.ipm_target, 0), f.owner_id, COALESCE(owner_user.oname, ''), f.start_planet, f.target_planet, %s, %s, COALESCE(o.name, ''), COALESCE(o.g, 0), COALESCE(o.s, 0), COALESCE(o.p, 0), COALESCE(t.name, ''), COALESCE(t.g, 0), COALESCE(t.s, 0), COALESCE(t.p, 0), COALESCE(t.type, ?), COALESCE(target_user.oname, 'space') FROM %s q JOIN %s f ON f.fleet_id = q.sub_id LEFT JOIN %s o ON o.planet_id = f.start_planet LEFT JOIN %s owner_user ON owner_user.player_id = f.owner_id LEFT JOIN %s t ON t.planet_id = f.target_planet LEFT JOIN %s target_user ON target_user.player_id = t.owner_id WHERE q.type = ? AND (f.start_planet = ? OR f.target_planet = ?) AND (COALESCE(f.union_id, 0) = 0 OR f.target_planet <> ?) ORDER BY q.end ASC, q.prio DESC",
			prefixedNumericColumns("f", fleetIDs),
			prefixedNumericColumns("f", resourceIDs),
			queueTable,
			fleetTable,
			planetsTable,
			usersTable,
			planetsTable,
			usersTable,
		),
		domaingame.PlanetTypePlanet,
		queueTypeFleet,
		target.ID,
		target.ID,
		target.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := r.now().Unix()
	events := make([]domaingame.FleetMission, 0)
	for rows.Next() {
		scanned, err := scanOverviewEventRow(rows, fleetIDs, resourceIDs, target.OwnerID, 99)
		if err != nil {
			return nil, err
		}
		for _, event := range phalanxNonUnionMissions(scanned, target) {
			if event.ArrivalAt > now {
				events = append(events, event)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	unionEvents, err := r.loadPhalanxUnionEvents(ctx, queueTable, fleetTable, planetsTable, usersTable, target, fleetIDs)
	if err != nil {
		return nil, err
	}
	for _, event := range unionEvents {
		if event.ArrivalAt > now {
			events = append(events, event)
		}
	}
	sort.SliceStable(events, func(left int, right int) bool {
		return events[left].ArrivalAt < events[right].ArrivalAt
	})
	return events, nil
}

func phalanxNonUnionMissions(scanned overviewEventScan, target domaingame.PhalanxPlanet) []domaingame.FleetMission {
	mission := scanned.Mission
	baseMission := overviewBaseMission(mission.Mission)
	startsAtTarget := scanned.StartPlanetID == target.ID
	endsAtTarget := scanned.TargetPlanetID == target.ID

	if mission.Mission == domaingame.FleetMissionDeploy+domaingame.FleetMissionReturnOffset ||
		(mission.Mission == domaingame.FleetMissionDeploy && startsAtTarget) ||
		(mission.Mission > domaingame.FleetMissionReturnOffset && mission.Mission < domaingame.FleetMissionOrbitingOffset && endsAtTarget) {
		return nil
	}
	if baseMission == domaingame.FleetMissionExpedition {
		return overviewNonUnionMissions(scanned, target.OwnerID)
	}

	events := make([]domaingame.FleetMission, 0, 2)
	if baseMission == domaingame.FleetMissionACSHold &&
		mission.Mission < domaingame.FleetMissionReturnOffset &&
		mission.OwnerID != target.OwnerID && endsAtTarget {
		events = append(events, overviewHoldPseudoMission(mission, scanned.DeployTime))
	}
	if mission.Mission < domaingame.FleetMissionReturnOffset && startsAtTarget {
		if scanned.FlightTime < 0 {
			scanned.FlightTime = 0
		}
		if mission.Mission == domaingame.FleetMissionMissile {
			mission.ArrivalAt += scanned.FlightTime
			return append(events, mission)
		}
		returnMission := mission
		returnMission.ID = overviewPseudoEventID(mission.ID, 2)
		returnMission.Mission = baseMission + domaingame.FleetMissionReturnOffset
		returnMission.DepartureAt = mission.ArrivalAt
		returnMission.ArrivalAt = mission.ArrivalAt + scanned.FlightTime
		returnMission.Origin, returnMission.Target = mission.Target, mission.Origin
		returnMission.OriginName, returnMission.TargetName = mission.TargetName, mission.OriginName
		returnMission.Foreign = false
		returnMission.GroupMissions = nil
		return append(events, returnMission)
	}
	return append(events, overviewNormalizeEventGeometry(mission))
}

func (r PhalanxRepository) loadPhalanxUnionEvents(ctx context.Context, queueTable string, fleetTable string, planetsTable string, usersTable string, target domaingame.PhalanxPlanet, fleetIDs []int) ([]domaingame.FleetMission, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT DISTINCT f.union_id FROM %s q JOIN %s f ON f.fleet_id = q.sub_id WHERE q.type = ? AND q.end > ? AND f.union_id > 0 AND f.target_planet = ? ORDER BY f.union_id", queueTable, fleetTable),
		queueTypeFleet,
		r.now().Unix(),
		target.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	unionIDs := make([]int, 0)
	for rows.Next() {
		var unionID int
		if err := rows.Scan(&unionID); err != nil {
			return nil, err
		}
		unionIDs = append(unionIDs, unionID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	overview := OverviewRepository{queryer: r.queryer, now: r.now}
	events := make([]domaingame.FleetMission, 0, len(unionIDs))
	for _, unionID := range unionIDs {
		group, err := overview.loadOverviewUnionEvent(ctx, queueTable, fleetTable, planetsTable, usersTable, fleetIDs, target.OwnerID, unionID, 99)
		if err != nil {
			return nil, err
		}
		for _, event := range group {
			if len(event.GroupMissions) > 0 {
				events = append(events, event)
				break
			}
		}
	}
	return events, nil
}
