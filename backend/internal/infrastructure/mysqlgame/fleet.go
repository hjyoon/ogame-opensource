package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const (
	queueTypeFleet            = "Fleet"
	legacyPlanetTypeColony    = 10002
	legacyPlanetTypeAbandoned = 10004
	legacyPlanetTypeFarSpace  = 20000
	queuePriorityFleet        = 200
)

type recallFleetRow struct {
	ID             int
	OwnerID        int
	UnionID        int
	Metal          float64
	Crystal        float64
	Deuterium      float64
	Fuel           int
	Mission        int
	StartPlanetID  int
	TargetPlanetID int
	FlightTime     int
	DeployTime     int
	Ships          domaingame.FleetCounts
}

type recallQueueRow struct {
	TaskID int
	Start  int64
	End    int64
}

type recallFleetLoader func(context.Context, string, int) (recallFleetRow, bool, error)

type fleetLaunchTarget struct {
	ID      int
	OwnerID int
	Type    int
}

type fleetLaunchUserState struct {
	ID            int
	Score         int64
	Admin         int
	Vacation      bool
	Banned        bool
	NoAttack      bool
	NoAttackUntil int64
	LastClick     int64
}

type fleetLaunchACSUnion struct {
	ACSLimit   int
	Players    string
	ArrivalAt  int64
	FleetCount int
}

type FleetRepository struct {
	queryer         Queryer
	execer          Execer
	prefix          string
	now             func() time.Time
	finishDueQueues bool
}

func NewFleetRepository(db *sql.DB, prefix string) FleetRepository {
	runner := SQLQueryer{DB: db}
	return FleetRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now, finishDueQueues: true}
}

func NewFleetRepositoryWithQueryer(queryer Queryer, prefix string, now func() time.Time) FleetRepository {
	if now == nil {
		now = time.Now
	}
	execer, _ := queryer.(Execer)
	return FleetRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func NewFleetRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) FleetRepository {
	if now == nil {
		now = time.Now
	}
	return FleetRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r FleetRepository) GetFleet(ctx context.Context, query appgame.FleetQuery) (domaingame.Fleet, error) {
	if r.finishDueQueues && r.execer != nil {
		if err := r.FinishDueFleetQueues(ctx, int(r.now().Unix())); err != nil {
			return domaingame.Fleet{}, err
		}
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	templateTable, err := tableName(r.prefix, "template")
	if err != nil {
		return domaingame.Fleet{}, err
	}
	unionTable, err := tableName(r.prefix, "union")
	if err != nil {
		return domaingame.Fleet{}, err
	}

	overviewRepository := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix)
	overview, err := overviewRepository.GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.Fleet{}, err
	}

	research, err := ResearchRepository{queryer: r.queryer, prefix: r.prefix}.loadResearchLevels(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	counts, err := ShipyardRepository{queryer: r.queryer, prefix: r.prefix}.loadFleetCounts(ctx, planetsTable, query.PlayerID, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	admiral, err := r.loadAdmiral(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	commanderActive, err := r.loadCommanderActive(ctx, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	acsEnabled, err := r.loadACSEnabled(ctx, uniTable)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	speedFactor, err := r.loadFleetSpeedFactor(ctx, uniTable)
	if err != nil {
		return domaingame.Fleet{}, err
	}
	missions, err := r.loadActiveMissions(ctx, queueTable, fleetTable, planetsTable, usersTable, unionTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}

	fleet := domaingame.BuildFleet(overview, counts, research, missions, admiral, acsEnabled)
	fleet.SpeedFactor = speedFactor
	fleet.CommanderActive = commanderActive
	fleet.TemplateLimit = research[domaingame.ResearchComputer] + 1
	if commanderActive {
		templates, err := r.loadFleetTemplates(ctx, templateTable, query.PlayerID, fleet.TemplateLimit)
		if err != nil {
			return domaingame.Fleet{}, err
		}
		fleet.Templates = templates
	}
	return fleet, nil
}

func (r FleetRepository) MutateFleetTemplate(ctx context.Context, query appgame.FleetTemplateMutationQuery) error {
	if r.execer == nil {
		return errors.New("fleet template writer unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return err
	}
	templateTable, err := tableName(r.prefix, "template")
	if err != nil {
		return err
	}

	commanderActive, maxTemplates, err := r.loadFleetTemplateAccess(ctx, usersTable, query.PlayerID)
	if err != nil {
		return err
	}
	if !commanderActive {
		return nil
	}

	switch query.Action {
	case "save":
		return r.saveFleetTemplate(ctx, templateTable, query.PlayerID, maxTemplates, query.TemplateID, query.Name, query.Ships)
	case "delete":
		return r.deleteFleetTemplate(ctx, templateTable, query.PlayerID, query.TemplateID)
	default:
		return nil
	}
}

func (r FleetRepository) LaunchFleetDispatch(ctx context.Context, query appgame.FleetLaunchQuery) (*domaingame.FleetActionIssue, error) {
	if r.execer == nil {
		return nil, errors.New("fleet launch writer unavailable")
	}
	if !query.Draft.Ready || query.PlayerID <= 0 || query.PlanetID <= 0 {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidOrder), nil
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return nil, err
	}
	fleetLogsTable, err := tableName(r.prefix, "fleetlogs")
	if err != nil {
		return nil, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return nil, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return nil, err
	}
	unionTable, err := tableName(r.prefix, "union")
	if err != nil {
		return nil, err
	}
	buddyTable, err := tableName(r.prefix, "buddy")
	if err != nil {
		return nil, err
	}

	frozen, err := r.loadUniverseFrozen(ctx, uniTable)
	if err != nil {
		return nil, err
	}
	if frozen {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueFrozen), nil
	}

	resources := fleetLaunchResources(query.Draft.Resources)
	ships := fleetLaunchShips(query.Draft.Ships)
	if issue := validateFleetLaunchShipRequirements(query.Draft.Mission, ships); issue != nil {
		return issue, nil
	}

	now := r.now().Unix()
	target, found, err := r.resolveFleetLaunchTarget(ctx, planetsTable, query, now)
	if err != nil {
		return nil, err
	}
	if !found {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), nil
	}
	if issue := validateFleetLaunchTarget(query, target); issue != nil {
		return issue, nil
	}
	if issue, err := r.validateFleetLaunchACS(ctx, uniTable, unionTable, fleetTable, queueTable, query, now); err != nil || issue != nil {
		return issue, err
	}
	if issue, err := r.validateFleetLaunchUserState(ctx, usersTable, query, target, now); err != nil || issue != nil {
		return issue, err
	}
	if issue, err := r.validateFleetLaunchACSHoldRelation(ctx, usersTable, buddyTable, query, target); err != nil || issue != nil {
		return issue, err
	}
	if err := r.deleteOldFleetLogs(ctx, fleetLogsTable, now); err != nil {
		return nil, err
	}
	reserved, err := r.reserveFleetLaunchOrigin(ctx, planetsTable, query.PlayerID, query.PlanetID, ships, resources, query.Draft.FuelConsumption, int(now))
	if err != nil {
		return nil, err
	}
	if !reserved {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueLaunchRace), nil
	}

	fleetID, err := r.insertFleetLaunchFleet(ctx, fleetTable, query, target, ships, resources)
	if err != nil {
		return nil, err
	}
	if err := r.insertFleetLaunchLog(ctx, fleetLogsTable, query, target, ships, resources, now); err != nil {
		return nil, err
	}
	if err := r.insertRecallQueue(ctx, queueTable, query.PlayerID, fleetID, query.Draft.Mission, now, int64(query.Draft.DurationSeconds)); err != nil {
		return nil, err
	}
	if query.Draft.Mission == domaingame.FleetMissionACSAttack && query.UnionID > 0 {
		if err := r.syncFleetLaunchACSQueue(ctx, queueTable, fleetTable, query.UnionID, now+int64(query.Draft.DurationSeconds)); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (r FleetRepository) resolveFleetLaunchTarget(ctx context.Context, planetsTable string, query appgame.FleetLaunchQuery, now int64) (fleetLaunchTarget, bool, error) {
	switch query.Draft.Mission {
	case domaingame.FleetMissionColonize:
		return r.resolveFleetLaunchColonizeTarget(ctx, planetsTable, query.Draft.Target, query.Draft.TargetType, now)
	case domaingame.FleetMissionExpedition:
		return r.resolveFleetLaunchExpeditionTarget(ctx, planetsTable, query.Draft.Target, query.Draft.TargetType, now)
	default:
		return r.loadFleetLaunchTarget(ctx, planetsTable, query.Draft.Target, query.Draft.TargetType)
	}
}

func validateFleetLaunchShipRequirements(mission int, ships domaingame.FleetCounts) *domaingame.FleetActionIssue {
	switch mission {
	case domaingame.FleetMissionSpy:
		if ships[domaingame.FleetEspionageProbe] <= 0 {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionColonize:
		if ships[domaingame.FleetColonyShip] <= 0 {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionRecycle:
		if ships[domaingame.FleetRecycler] <= 0 {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionDestroy:
		if ships[domaingame.FleetDeathstar] <= 0 {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionExpedition:
		if !fleetLaunchHasMannedShips(ships) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueExpRequired)
		}
	}
	return nil
}

func validateFleetLaunchTarget(query appgame.FleetLaunchQuery, target fleetLaunchTarget) *domaingame.FleetActionIssue {
	switch query.Draft.Mission {
	case domaingame.FleetMissionAttack, domaingame.FleetMissionACSAttack, domaingame.FleetMissionACSAttackHead:
		if target.OwnerID == query.PlayerID || !fleetLaunchTargetIsPlanetOrMoon(target.Type) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionTransport, domaingame.FleetMissionACSHold:
		if !fleetLaunchTargetIsPlanetOrMoon(target.Type) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionDeploy:
		if target.OwnerID != query.PlayerID || !fleetLaunchTargetIsPlanetOrMoon(target.Type) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionSpy:
		if target.OwnerID == query.PlayerID || !fleetLaunchTargetIsPlanetOrMoon(target.Type) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionColonize:
		if target.Type != legacyPlanetTypeColony {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionRecycle:
		if target.Type != domaingame.PlanetTypeDebris {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionDestroy:
		if target.OwnerID == query.PlayerID || target.Type != domaingame.PlanetTypeMoon {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	case domaingame.FleetMissionExpedition:
		if target.Type != legacyPlanetTypeFarSpace {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget)
		}
	}
	return nil
}

func (r FleetRepository) validateFleetLaunchUserState(ctx context.Context, usersTable string, query appgame.FleetLaunchQuery, target fleetLaunchTarget, now int64) (*domaingame.FleetActionIssue, error) {
	if !fleetLaunchNeedsUserState(query.Draft.Mission, target.OwnerID) {
		return nil, nil
	}
	origin, found, err := r.loadFleetLaunchUserState(ctx, usersTable, query.PlayerID)
	if err != nil || !found {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), err
	}
	targetUser, found, err := r.loadFleetLaunchUserState(ctx, usersTable, target.OwnerID)
	if err != nil || !found {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), err
	}
	return validateFleetLaunchProtection(query.Draft.Mission, origin, targetUser, now), nil
}

func fleetLaunchNeedsUserState(mission int, ownerID int) bool {
	if ownerID <= 0 || ownerID == userSpace {
		return false
	}
	switch mission {
	case domaingame.FleetMissionAttack,
		domaingame.FleetMissionACSAttack,
		domaingame.FleetMissionACSAttackHead,
		domaingame.FleetMissionTransport,
		domaingame.FleetMissionDeploy,
		domaingame.FleetMissionACSHold,
		domaingame.FleetMissionSpy,
		domaingame.FleetMissionDestroy:
		return true
	case domaingame.FleetMissionRecycle, domaingame.FleetMissionColonize, domaingame.FleetMissionExpedition:
		return false
	default:
		return false
	}
}

func validateFleetLaunchProtection(mission int, origin fleetLaunchUserState, target fleetLaunchUserState, now int64) *domaingame.FleetActionIssue {
	if origin.Vacation {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueVacationSelf)
	}
	if target.Vacation && mission != domaingame.FleetMissionRecycle {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueVacationOther)
	}

	switch mission {
	case domaingame.FleetMissionAttack, domaingame.FleetMissionACSAttack, domaingame.FleetMissionACSAttackHead:
		if fleetLaunchAdminProtected(target) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueTargetAdmin)
		}
		if fleetLaunchNoobProtected(origin, target, now) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueTargetNoob)
		}
		if origin.NoAttack {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueAttackBan)
		}
	case domaingame.FleetMissionSpy:
		if fleetLaunchAdminProtected(target) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueTargetAdmin)
		}
		if fleetLaunchNoobProtected(origin, target, now) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueTargetNoob)
		}
		if origin.NoAttack {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueAttackBan)
		}
	case domaingame.FleetMissionACSHold:
		if fleetLaunchNoobProtected(origin, target, now) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueTargetNoob)
		}
	case domaingame.FleetMissionDestroy:
		if fleetLaunchAdminProtected(target) {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueTargetAdmin)
		}
		if origin.NoAttack {
			return domaingame.FleetActionIssueFor(domaingame.FleetIssueAttackBan)
		}
	}
	return nil
}

func fleetLaunchAdminProtected(target fleetLaunchUserState) bool {
	return target.Admin > domaingame.AdminLevelPlayer && target.ID != userSpace
}

func fleetLaunchNoobProtected(origin fleetLaunchUserState, target fleetLaunchUserState, now int64) bool {
	active := target.LastClick > now-604800 && !target.Vacation && !target.Banned
	return (active && target.Score < origin.Score && target.Score < domaingame.GalaxyNoobScoreLimit && origin.Score > target.Score*5) ||
		(active && origin.Score < target.Score && origin.Score < domaingame.GalaxyNoobScoreLimit && target.Score > origin.Score*5)
}

func fleetLaunchTargetIsPlanetOrMoon(targetType int) bool {
	return targetType == domaingame.PlanetTypePlanet || targetType == domaingame.PlanetTypeMoon
}

func fleetLaunchHasMannedShips(ships domaingame.FleetCounts) bool {
	for id, count := range ships {
		if count > 0 && id != domaingame.FleetEspionageProbe {
			return true
		}
	}
	return false
}

func (r FleetRepository) validateFleetLaunchACS(ctx context.Context, uniTable string, unionTable string, fleetTable string, queueTable string, query appgame.FleetLaunchQuery, now int64) (*domaingame.FleetActionIssue, error) {
	if query.Draft.Mission != domaingame.FleetMissionACSAttack {
		return nil, nil
	}
	if query.UnionID <= 0 {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), nil
	}
	union, found, err := r.loadFleetLaunchACSUnion(ctx, uniTable, unionTable, fleetTable, queueTable, query.UnionID)
	if err != nil || !found {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), err
	}
	if union.ACSLimit <= 0 {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), nil
	}
	if !fleetLaunchUnionContainsPlayer(union.Players, query.PlayerID) {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), nil
	}
	remaining := union.ArrivalAt - now
	if remaining <= 0 || int64(query.Draft.DurationSeconds)*10 > remaining*13 {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueInvalidTarget), nil
	}
	if union.FleetCount >= union.ACSLimit*union.ACSLimit {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueMaxFleet), nil
	}
	return nil, nil
}

func (r FleetRepository) validateFleetLaunchACSHoldRelation(ctx context.Context, usersTable string, buddyTable string, query appgame.FleetLaunchQuery, target fleetLaunchTarget) (*domaingame.FleetActionIssue, error) {
	if query.Draft.Mission != domaingame.FleetMissionACSHold {
		return nil, nil
	}
	if target.OwnerID <= 0 || target.OwnerID == userSpace {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueHoldAlliance), nil
	}
	related, err := r.fleetLaunchUsersCanHold(ctx, usersTable, buddyTable, query.PlayerID, target.OwnerID)
	if err != nil {
		return nil, err
	}
	if !related {
		return domaingame.FleetActionIssueFor(domaingame.FleetIssueHoldAlliance), nil
	}
	return nil, nil
}

func (r FleetRepository) fleetLaunchUsersCanHold(ctx context.Context, usersTable string, buddyTable string, originID int, targetID int) (bool, error) {
	if originID <= 0 || targetID <= 0 {
		return false, nil
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, COALESCE(ally_id, 0) FROM %s WHERE player_id IN (?, ?)", usersTable),
		originID,
		targetID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	allianceIDs := make(map[int]int, 2)
	for rows.Next() {
		var playerID int
		var allianceID int
		if err := rows.Scan(&playerID, &allianceID); err != nil {
			return false, err
		}
		allianceIDs[playerID] = allianceID
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if allianceIDs[originID] > 0 && allianceIDs[originID] == allianceIDs[targetID] {
		return true, nil
	}

	buddyRows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE ((request_from = ? AND request_to = ?) OR (request_from = ? AND request_to = ?)) AND accepted = 1", buddyTable),
		originID,
		targetID,
		targetID,
		originID,
	)
	if err != nil {
		return false, err
	}
	defer buddyRows.Close()
	if !buddyRows.Next() {
		if err := buddyRows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var count int
	if err := buddyRows.Scan(&count); err != nil {
		return false, err
	}
	if err := buddyRows.Err(); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r FleetRepository) resolveFleetLaunchColonizeTarget(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates, targetType int, now int64) (fleetLaunchTarget, bool, error) {
	if targetType != domaingame.GamePlanetTypePlanet || !coordinates.Valid() || coordinates.Position > domaingame.GalaxyPositions {
		return fleetLaunchTarget{}, false, nil
	}
	occupied, err := r.fleetLaunchColonizeOccupied(ctx, planetsTable, coordinates)
	if err != nil || occupied {
		return fleetLaunchTarget{}, false, err
	}
	target, err := r.insertFleetLaunchPlanetTarget(ctx, planetsTable, "Planet", legacyPlanetTypeColony, coordinates, now)
	if err != nil {
		return fleetLaunchTarget{}, false, err
	}
	return target, true, nil
}

func (r FleetRepository) resolveFleetLaunchExpeditionTarget(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates, targetType int, now int64) (fleetLaunchTarget, bool, error) {
	if targetType != domaingame.GamePlanetTypePlanet || !coordinates.Valid() || coordinates.Position != domaingame.GalaxyFarSpace {
		return fleetLaunchTarget{}, false, nil
	}
	target, found, err := r.loadFleetLaunchSpecialTarget(ctx, planetsTable, coordinates, legacyPlanetTypeFarSpace)
	if err != nil || found {
		return target, found, err
	}
	target, err = r.insertFleetLaunchPlanetTarget(ctx, planetsTable, "Far space", legacyPlanetTypeFarSpace, coordinates, now)
	if err != nil {
		return fleetLaunchTarget{}, false, err
	}
	return target, true, nil
}

func (r FleetRepository) loadFleetLaunchACSUnion(ctx context.Context, uniTable string, unionTable string, fleetTable string, queueTable string, unionID int) (fleetLaunchACSUnion, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT uni.acs, u.players, q.end, (SELECT COUNT(*) FROM %s uf WHERE uf.union_id = u.union_id) FROM %s uni JOIN %s u ON u.union_id = ? JOIN %s f ON f.fleet_id = u.fleet_id AND f.union_id = u.union_id AND f.mission = ? JOIN %s q ON q.type = ? AND q.sub_id = u.fleet_id LIMIT 1", fleetTable, uniTable, unionTable, fleetTable, queueTable),
		unionID,
		domaingame.FleetMissionACSAttackHead,
		queueTypeFleet,
	)
	if err != nil {
		return fleetLaunchACSUnion{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fleetLaunchACSUnion{}, false, err
		}
		return fleetLaunchACSUnion{}, false, nil
	}
	var union fleetLaunchACSUnion
	if err := rows.Scan(&union.ACSLimit, &union.Players, &union.ArrivalAt, &union.FleetCount); err != nil {
		return fleetLaunchACSUnion{}, false, err
	}
	if err := rows.Err(); err != nil {
		return fleetLaunchACSUnion{}, false, err
	}
	return union, true, nil
}

func (r FleetRepository) syncFleetLaunchACSQueue(ctx context.Context, queueTable string, fleetTable string, unionID int, requestedEnd int64) error {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(MAX(q.end), ?) FROM %s q JOIN %s f ON f.fleet_id = q.sub_id WHERE q.type = ? AND f.union_id = ?", queueTable, fleetTable),
		requestedEnd,
		queueTypeFleet,
		unionID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	unionEnd := requestedEnd
	if rows.Next() {
		var maxEnd int64
		if err := rows.Scan(&maxEnd); err != nil {
			return err
		}
		if maxEnd > unionEnd {
			unionEnd = maxEnd
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s q JOIN %s f ON f.fleet_id = q.sub_id SET q.end = ? WHERE q.type = ? AND f.union_id = ?", queueTable, fleetTable),
		unionEnd,
		queueTypeFleet,
		unionID,
	)
	return err
}

func fleetLaunchUnionContainsPlayer(players string, playerID int) bool {
	for _, part := range strings.Split(players, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && id == playerID {
			return true
		}
	}
	return false
}

func (r FleetRepository) RecallFleet(ctx context.Context, query appgame.FleetRecallQuery) error {
	if r.execer == nil {
		return errors.New("fleet writer unavailable")
	}
	if query.FleetID <= 0 {
		return nil
	}
	return r.recallFleet(ctx, query.FleetID, func(ctx context.Context, fleetTable string, fleetID int) (recallFleetRow, bool, error) {
		return r.loadRecallFleet(ctx, fleetTable, query.PlayerID, fleetID)
	}, query.PlayerID)
}

func (r FleetRepository) RecallFleetAnyOwner(ctx context.Context, fleetID int) error {
	if r.execer == nil {
		return errors.New("fleet writer unavailable")
	}
	if fleetID <= 0 {
		return nil
	}
	return r.recallFleet(ctx, fleetID, r.loadRecallFleetAnyOwner, 0)
}

func (r FleetRepository) recallFleet(ctx context.Context, fleetID int, loader recallFleetLoader, deleteOwnerID int) error {
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return err
	}
	unionTable, err := tableName(r.prefix, "union")
	if err != nil {
		return err
	}

	frozen, err := r.loadUniverseFrozen(ctx, uniTable)
	if err != nil {
		return err
	}
	if frozen {
		return nil
	}

	fleet, found, err := loader(ctx, fleetTable, fleetID)
	if err != nil || !found {
		return err
	}
	if !fleetRecallable(fleet.Mission) {
		return nil
	}
	queue, found, err := r.loadRecallQueue(ctx, queueTable, fleet.ID)
	if err != nil || !found {
		return err
	}
	originOwner, found, err := r.loadRecallOriginOwner(ctx, planetsTable, fleet.StartPlanetID)
	if err != nil || !found {
		return err
	}
	if exists, err := r.recallPlanetExists(ctx, planetsTable, fleet.TargetPlanetID); err != nil || !exists {
		return err
	}

	now := r.now().Unix()
	newMission, seconds := recallMissionAndDuration(fleet, queue, now)
	newFleetID, err := r.insertRecallFleet(ctx, fleetTable, originOwner, fleet, newMission, seconds)
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, originOwner, newFleetID, newMission, now, seconds); err != nil {
		return err
	}
	if deleteOwnerID > 0 {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE fleet_id = ? AND owner_id = ? LIMIT 1", fleetTable), fleet.ID, deleteOwnerID); err != nil {
			return err
		}
	} else {
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE fleet_id = ? LIMIT 1", fleetTable), fleet.ID); err != nil {
			return err
		}
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE task_id = ? AND type = ? LIMIT 1", queueTable), queue.TaskID, queueTypeFleet); err != nil {
		return err
	}
	if fleet.UnionID > 0 && (fleet.Mission == domaingame.FleetMissionACSAttack || fleet.Mission == domaingame.FleetMissionACSAttackHead) {
		return r.removeEmptyRecallUnion(ctx, fleetTable, unionTable, fleet.UnionID)
	}
	return nil
}

func (r FleetRepository) loadFleetLaunchTarget(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates, targetType int) (fleetLaunchTarget, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, owner_id, type FROM %s WHERE g = ? AND s = ? AND p = ? AND type = ? LIMIT 1", planetsTable),
		coordinates.Galaxy,
		coordinates.System,
		coordinates.Position,
		fleetLaunchPlanetType(targetType),
	)
	if err != nil {
		return fleetLaunchTarget{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fleetLaunchTarget{}, false, err
		}
		return fleetLaunchTarget{}, false, nil
	}
	var target fleetLaunchTarget
	if err := rows.Scan(&target.ID, &target.OwnerID, &target.Type); err != nil {
		return fleetLaunchTarget{}, false, err
	}
	if err := rows.Err(); err != nil {
		return fleetLaunchTarget{}, false, err
	}
	return target, true, nil
}

func (r FleetRepository) loadFleetLaunchUserState(ctx context.Context, usersTable string, playerID int) (fleetLaunchUserState, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, COALESCE(score1, 0), COALESCE(admin, 0), COALESCE(vacation, 0), COALESCE(banned, 0), COALESCE(noattack, 0), COALESCE(noattack_until, 0), COALESCE(lastclick, 0) FROM %s WHERE player_id = ? LIMIT 1", usersTable),
		playerID,
	)
	if err != nil {
		return fleetLaunchUserState{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fleetLaunchUserState{}, false, err
		}
		return fleetLaunchUserState{}, false, nil
	}
	var state fleetLaunchUserState
	var vacation, banned, noAttack int
	if err := rows.Scan(&state.ID, &state.Score, &state.Admin, &vacation, &banned, &noAttack, &state.NoAttackUntil, &state.LastClick); err != nil {
		return fleetLaunchUserState{}, false, err
	}
	if err := rows.Err(); err != nil {
		return fleetLaunchUserState{}, false, err
	}
	state.Vacation = vacation != 0
	state.Banned = banned != 0
	state.NoAttack = noAttack != 0
	return state, true, nil
}

func (r FleetRepository) loadFleetLaunchSpecialTarget(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates, objectType int) (fleetLaunchTarget, bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, owner_id, type FROM %s WHERE g = ? AND s = ? AND p = ? AND type = ? LIMIT 1", planetsTable),
		coordinates.Galaxy,
		coordinates.System,
		coordinates.Position,
		objectType,
	)
	if err != nil {
		return fleetLaunchTarget{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fleetLaunchTarget{}, false, err
		}
		return fleetLaunchTarget{}, false, nil
	}
	var target fleetLaunchTarget
	if err := rows.Scan(&target.ID, &target.OwnerID, &target.Type); err != nil {
		return fleetLaunchTarget{}, false, err
	}
	if err := rows.Err(); err != nil {
		return fleetLaunchTarget{}, false, err
	}
	return target, true, nil
}

func (r FleetRepository) fleetLaunchColonizeOccupied(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates) (bool, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id FROM %s WHERE g = ? AND s = ? AND p = ? AND type IN (?, ?, ?) LIMIT 1", planetsTable),
		coordinates.Galaxy,
		coordinates.System,
		coordinates.Position,
		domaingame.PlanetTypePlanet,
		domaingame.PlanetTypeDestroyedPlanet,
		legacyPlanetTypeAbandoned,
	)
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
	var id int
	if err := rows.Scan(&id); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return id > 0, nil
}

func (r FleetRepository) insertFleetLaunchPlanetTarget(ctx context.Context, planetsTable string, name string, objectType int, coordinates domaingame.Coordinates, now int64) (fleetLaunchTarget, error) {
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (name, type, g, s, p, owner_id, diameter, temp, fields, maxfields, date, `%d`, `%d`, `%d`, lastpeek, lastakt, gate_until, remove) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", planetsTable, resourceMetal, resourceCrystal, resourceDeuterium),
		name,
		objectType,
		coordinates.Galaxy,
		coordinates.System,
		coordinates.Position,
		userSpace,
		0,
		0,
		0,
		0,
		now,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
	)
	if err != nil {
		return fleetLaunchTarget{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fleetLaunchTarget{}, err
	}
	if id <= 0 {
		return fleetLaunchTarget{}, errors.New("fleet launch target id unavailable")
	}
	return fleetLaunchTarget{ID: int(id), OwnerID: userSpace, Type: objectType}, nil
}

func (r FleetRepository) deleteOldFleetLogs(ctx context.Context, fleetLogsTable string, now int64) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE start < ?", fleetLogsTable), now-4*7*24*60*60)
	return err
}

func (r FleetRepository) reserveFleetLaunchOrigin(ctx context.Context, planetsTable string, playerID int, planetID int, ships domaingame.FleetCounts, resources map[int]int, fuel int, now int) (bool, error) {
	metal := max(0, resources[resourceMetal])
	crystal := max(0, resources[resourceCrystal])
	deuterium := max(0, resources[resourceDeuterium]+fuel)
	setParts := []string{
		fmt.Sprintf("`%d` = `%d` - ?", resourceMetal, resourceMetal),
		fmt.Sprintf("`%d` = `%d` - ?", resourceCrystal, resourceCrystal),
		fmt.Sprintf("`%d` = `%d` - ?", resourceDeuterium, resourceDeuterium),
	}
	args := []any{metal, crystal, deuterium}
	whereParts := []string{
		"planet_id = ?",
		"owner_id = ?",
		fmt.Sprintf("`%d` >= ?", resourceMetal),
		fmt.Sprintf("`%d` >= ?", resourceCrystal),
		fmt.Sprintf("`%d` >= ?", resourceDeuterium),
	}
	whereArgs := []any{planetID, playerID, metal, crystal, deuterium}
	for _, id := range domaingame.FleetIDs() {
		count := ships[id]
		if count <= 0 {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("`%d` = `%d` - ?", id, id))
		args = append(args, count)
		whereParts = append(whereParts, fmt.Sprintf("`%d` >= ?", id))
		whereArgs = append(whereArgs, count)
	}
	setParts = append(setParts, "lastpeek = ?")
	args = append(args, now)
	args = append(args, whereArgs...)
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE %s", planetsTable, strings.Join(setParts, ", "), strings.Join(whereParts, " AND ")), args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return true, nil
	}
	return affected > 0, nil
}

func (r FleetRepository) insertFleetLaunchFleet(ctx context.Context, fleetTable string, query appgame.FleetLaunchQuery, target fleetLaunchTarget, ships domaingame.FleetCounts, resources map[int]int) (int, error) {
	ids := domaingame.FleetIDs()
	args := []any{
		query.PlayerID,
		query.UnionID,
		resources[resourceMetal],
		resources[resourceCrystal],
		resources[resourceDeuterium],
		query.Draft.FuelConsumption,
		query.Draft.Mission,
		query.PlanetID,
		target.ID,
		query.Draft.DurationSeconds,
		query.HoldSeconds,
	}
	args = append(args, fleetCountValues(ids, ships)...)
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, union_id, `%d`, `%d`, `%d`, fuel, mission, start_planet, target_planet, flight_time, deploy_time, %s) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s)", fleetTable, resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), placeholders(len(ids))),
		args...,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("fleet launch id unavailable")
	}
	return int(id), nil
}

func (r FleetRepository) insertFleetLaunchLog(ctx context.Context, fleetLogsTable string, query appgame.FleetLaunchQuery, target fleetLaunchTarget, ships domaingame.FleetCounts, resources map[int]int, now int64) error {
	ids := domaingame.FleetIDs()
	args := []any{
		query.PlayerID,
		target.OwnerID,
		query.UnionID,
		query.Draft.FuelConsumption,
		query.Draft.Mission,
		query.Draft.DurationSeconds,
		query.HoldSeconds,
		now,
		now + int64(query.Draft.DurationSeconds),
		query.Origin.Coordinates.Galaxy,
		query.Origin.Coordinates.System,
		query.Origin.Coordinates.Position,
		query.Origin.Type,
		query.Draft.Target.Galaxy,
		query.Draft.Target.System,
		query.Draft.Target.Position,
		target.Type,
		int(query.Origin.Resources.Metal),
		int(query.Origin.Resources.Crystal),
		int(query.Origin.Resources.Deuterium),
		resources[resourceMetal],
		resources[resourceCrystal],
		resources[resourceDeuterium],
	}
	args = append(args, fleetCountValues(ids, ships)...)
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, target_id, union_id, fuel, mission, flight_time, deploy_time, start, end, origin_g, origin_s, origin_p, origin_type, target_g, target_s, target_p, target_type, `p%d`, `p%d`, `p%d`, `%d`, `%d`, `%d`, %s) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s)", fleetLogsTable, resourceMetal, resourceCrystal, resourceDeuterium, resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), placeholders(len(ids))),
		args...,
	)
	return err
}

func (r FleetRepository) loadUniverseFrozen(ctx context.Context, uniTable string) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT freeze FROM %s LIMIT 1", uniTable))
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
	var frozen int
	if err := rows.Scan(&frozen); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return frozen != 0, nil
}

func (r FleetRepository) loadRecallFleet(ctx context.Context, fleetTable string, playerID int, fleetID int) (recallFleetRow, bool, error) {
	ids := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT fleet_id, owner_id, union_id, `%d`, `%d`, `%d`, fuel, mission, start_planet, target_planet, flight_time, deploy_time, %s FROM %s WHERE fleet_id = ? AND owner_id = ? LIMIT 1", resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), fleetTable),
		fleetID,
		playerID,
	)
	if err != nil {
		return recallFleetRow{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return recallFleetRow{}, false, err
		}
		return recallFleetRow{}, false, nil
	}
	fleet, err := scanRecallFleetRow(rows, ids)
	if err != nil {
		return recallFleetRow{}, false, err
	}
	if err := rows.Err(); err != nil {
		return recallFleetRow{}, false, err
	}
	return fleet, true, nil
}

func (r FleetRepository) loadRecallFleetAnyOwner(ctx context.Context, fleetTable string, fleetID int) (recallFleetRow, bool, error) {
	ids := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT fleet_id, owner_id, union_id, `%d`, `%d`, `%d`, fuel, mission, start_planet, target_planet, flight_time, deploy_time, %s FROM %s WHERE fleet_id = ? LIMIT 1", resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), fleetTable),
		fleetID,
	)
	if err != nil {
		return recallFleetRow{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return recallFleetRow{}, false, err
		}
		return recallFleetRow{}, false, nil
	}
	fleet, err := scanRecallFleetRow(rows, ids)
	if err != nil {
		return recallFleetRow{}, false, err
	}
	if err := rows.Err(); err != nil {
		return recallFleetRow{}, false, err
	}
	return fleet, true, nil
}

func scanRecallFleetRow(rows Rows, ids []int) (recallFleetRow, error) {
	fleet := recallFleetRow{Ships: make(domaingame.FleetCounts, len(ids))}
	shipValues := make([]int, len(ids))
	dest := []any{
		&fleet.ID,
		&fleet.OwnerID,
		&fleet.UnionID,
		&fleet.Metal,
		&fleet.Crystal,
		&fleet.Deuterium,
		&fleet.Fuel,
		&fleet.Mission,
		&fleet.StartPlanetID,
		&fleet.TargetPlanetID,
		&fleet.FlightTime,
		&fleet.DeployTime,
	}
	for index := range shipValues {
		dest = append(dest, &shipValues[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return recallFleetRow{}, err
	}
	for index, id := range ids {
		fleet.Ships[id] = shipValues[index]
	}
	return fleet, nil
}

func (r FleetRepository) loadRecallQueue(ctx context.Context, queueTable string, fleetID int) (recallQueueRow, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT task_id, start, end FROM %s WHERE type = ? AND sub_id = ? LIMIT 1", queueTable), queueTypeFleet, fleetID)
	if err != nil {
		return recallQueueRow{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return recallQueueRow{}, false, err
		}
		return recallQueueRow{}, false, nil
	}
	var queue recallQueueRow
	if err := rows.Scan(&queue.TaskID, &queue.Start, &queue.End); err != nil {
		return recallQueueRow{}, false, err
	}
	if err := rows.Err(); err != nil {
		return recallQueueRow{}, false, err
	}
	return queue, true, nil
}

func (r FleetRepository) loadRecallOriginOwner(ctx context.Context, planetsTable string, planetID int) (int, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT owner_id FROM %s WHERE planet_id = ? LIMIT 1", planetsTable), planetID)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}
	var ownerID int
	if err := rows.Scan(&ownerID); err != nil {
		return 0, false, err
	}
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	return ownerID, true, nil
}

func (r FleetRepository) recallPlanetExists(ctx context.Context, planetsTable string, planetID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT planet_id FROM %s WHERE planet_id = ? LIMIT 1", planetsTable), planetID)
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
	var id int
	if err := rows.Scan(&id); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return id > 0, nil
}

func (r FleetRepository) insertRecallFleet(ctx context.Context, fleetTable string, ownerID int, fleet recallFleetRow, mission int, seconds int64) (int, error) {
	return r.insertFleetTransition(ctx, fleetTable, ownerID, fleet, mission, seconds, 0)
}

func (r FleetRepository) insertFleetTransition(ctx context.Context, fleetTable string, ownerID int, fleet recallFleetRow, mission int, seconds int64, deploySeconds int64) (int, error) {
	ids := domaingame.FleetIDs()
	args := []any{
		ownerID,
		0,
		fleet.Metal,
		fleet.Crystal,
		fleet.Deuterium,
		fleet.Fuel / 2,
		mission,
		fleet.StartPlanetID,
		fleet.TargetPlanetID,
		seconds,
		deploySeconds,
	}
	args = append(args, fleetCountValues(ids, fleet.Ships)...)
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, union_id, `%d`, `%d`, `%d`, fuel, mission, start_planet, target_planet, flight_time, deploy_time, %s) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s)", fleetTable, resourceMetal, resourceCrystal, resourceDeuterium, numericColumns(ids), placeholders(len(ids))),
		args...,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("recall return fleet id unavailable")
	}
	return int(id), nil
}

func (r FleetRepository) insertRecallQueue(ctx context.Context, queueTable string, ownerID int, fleetID int, mission int, now int64, seconds int64) error {
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", queueTable),
		ownerID,
		queueTypeFleet,
		fleetID,
		0,
		0,
		now,
		now+seconds,
		fleetQueuePriority(mission),
	)
	return err
}

func (r FleetRepository) removeEmptyRecallUnion(ctx context.Context, fleetTable string, unionTable string, unionID int) error {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE union_id = ?", fleetTable), unionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	}
	var remaining int
	if err := rows.Scan(&remaining); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if remaining > 0 {
		return nil
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE union_id = ? LIMIT 1", unionTable), unionID)
	return err
}

func fleetRecallable(mission int) bool {
	return mission < domaingame.FleetMissionReturnOffset || mission > domaingame.FleetMissionOrbitingOffset
}

func recallMissionAndDuration(fleet recallFleetRow, queue recallQueueRow, now int64) (int, int64) {
	if fleet.Mission < domaingame.FleetMissionReturnOffset {
		seconds := now - queue.Start
		if seconds < 0 {
			seconds = 0
		}
		return fleet.Mission + domaingame.FleetMissionReturnOffset, seconds
	}
	seconds := int64(fleet.DeployTime)
	if seconds < 0 {
		seconds = 0
	}
	return fleet.Mission - domaingame.FleetMissionReturnOffset, seconds
}

func fleetQueuePriority(mission int) int {
	if mission == domaingame.FleetMissionMissile {
		return queuePriorityFleet + 1300
	}
	switch mission {
	case domaingame.FleetMissionAttack, domaingame.FleetMissionACSAttack, domaingame.FleetMissionACSAttackHead, domaingame.FleetMissionDestroy:
		return queuePriorityFleet + 1000 + mission
	case domaingame.FleetMissionRecycle:
		return queuePriorityFleet + 900
	default:
		return queuePriorityFleet + mission
	}
}

func (r FleetRepository) loadAdmiral(ctx context.Context, usersTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT adm_until FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("fleet premium state not found")
	}
	var admiralUntil int64
	if err := rows.Scan(&admiralUntil); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return admiralUntil > r.now().Unix(), nil
}

func (r FleetRepository) loadCommanderActive(ctx context.Context, usersTable string, playerID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT com_until FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("fleet commander state not found")
	}
	var commanderUntil int64
	if err := rows.Scan(&commanderUntil); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return commanderUntil > r.now().Unix(), nil
}

func (r FleetRepository) loadFleetTemplateAccess(ctx context.Context, usersTable string, playerID int) (bool, int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT com_until, `%d` FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchComputer, usersTable), playerID)
	if err != nil {
		return false, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, 0, err
		}
		return false, 0, errors.New("fleet template access not found")
	}
	var commanderUntil int64
	var computer int
	if err := rows.Scan(&commanderUntil, &computer); err != nil {
		return false, 0, err
	}
	if err := rows.Err(); err != nil {
		return false, 0, err
	}
	return commanderUntil > r.now().Unix(), computer + 1, nil
}

func (r FleetRepository) loadFleetTemplates(ctx context.Context, templateTable string, playerID int, limit int) ([]domaingame.FleetTemplate, error) {
	if limit <= 0 {
		return nil, nil
	}
	fleetIDs := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT id, name, date, %s FROM %s WHERE owner_id = ? ORDER BY date DESC, id DESC LIMIT ?", numericColumns(fleetIDs), templateTable),
		playerID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]domaingame.FleetTemplate, 0)
	for rows.Next() {
		template, err := scanFleetTemplateRow(rows, fleetIDs)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return templates, nil
}

func scanFleetTemplateRow(rows Rows, fleetIDs []int) (domaingame.FleetTemplate, error) {
	var id int
	var name string
	var updatedAt int64
	shipValues := make([]int, len(fleetIDs))
	dest := []any{&id, &name, &updatedAt}
	for index := range shipValues {
		dest = append(dest, &shipValues[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return domaingame.FleetTemplate{}, err
	}

	ships := make(domaingame.FleetCounts, len(fleetIDs))
	for index, fleetID := range fleetIDs {
		ships[fleetID] = shipValues[index]
	}
	return domaingame.BuildFleetTemplate(id, name, updatedAt, ships), nil
}

func (r FleetRepository) saveFleetTemplate(ctx context.Context, templateTable string, playerID int, maxTemplates int, templateID int, name string, ships map[int]int) error {
	fleetIDs := domaingame.FleetIDs()
	if templateID > 0 {
		args := []any{domaingame.NormalizeFleetTemplateName(name), r.now().Unix()}
		args = append(args, fleetTemplateValues(fleetIDs, ships)...)
		args = append(args, templateID, playerID)
		_, err := r.execer.ExecContext(
			ctx,
			fmt.Sprintf("UPDATE %s SET name = ?, date = ?, %s WHERE id = ? AND owner_id = ? LIMIT 1", templateTable, numericAssignments(fleetIDs)),
			args...,
		)
		return err
	}

	count, err := r.countFleetTemplates(ctx, templateTable, playerID)
	if err != nil {
		return err
	}
	if count >= maxTemplates {
		return nil
	}
	args := []any{playerID, domaingame.NormalizeFleetTemplateName(name), r.now().Unix()}
	args = append(args, fleetTemplateValues(fleetIDs, ships)...)
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, name, date, %s) VALUES (?, ?, ?, %s)", templateTable, numericColumns(fleetIDs), placeholders(len(fleetIDs))),
		args...,
	)
	return err
}

func (r FleetRepository) countFleetTemplates(ctx context.Context, templateTable string, playerID int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ?", templateTable), playerID)
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

func (r FleetRepository) deleteFleetTemplate(ctx context.Context, templateTable string, playerID int, templateID int) error {
	if templateID <= 0 {
		return nil
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ? AND owner_id = ? LIMIT 1", templateTable), templateID, playerID)
	return err
}

func (r FleetRepository) loadACSEnabled(ctx context.Context, uniTable string) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT acs FROM %s LIMIT 1", uniTable))
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
	var acs int
	if err := rows.Scan(&acs); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return acs > 0, nil
}

func (r FleetRepository) loadFleetSpeedFactor(ctx context.Context, uniTable string) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT fspeed FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 1, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 1, err
		}
		return 1, nil
	}
	var speedFactor int
	if err := rows.Scan(&speedFactor); err != nil {
		return 1, err
	}
	if err := rows.Err(); err != nil {
		return 1, err
	}
	if speedFactor < 1 {
		return 1, nil
	}
	return speedFactor, nil
}

func (r FleetRepository) loadActiveMissions(ctx context.Context, queueTable string, fleetTable string, planetsTable string, usersTable string, unionTable string, playerID int) ([]domaingame.FleetMission, error) {
	fleetIDs := domaingame.FleetIDs()
	resourceIDs := []int{resourceMetal, resourceCrystal, resourceDeuterium}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.sub_id, q.start, q.end, f.mission, f.owner_id, COALESCE(ou.oname, ''), COALESCE(f.union_id, 0), f.start_planet, f.target_planet, %s, %s, COALESCE(o.name, ''), COALESCE(o.g, 0), COALESCE(o.s, 0), COALESCE(o.p, 0), COALESCE(t.name, ''), COALESCE(t.g, 0), COALESCE(t.s, 0), COALESCE(t.p, 0), COALESCE(t.type, ?), COALESCE(u.oname, 'space') FROM %s q JOIN %s f ON f.fleet_id = q.sub_id LEFT JOIN %s ou ON ou.player_id = f.owner_id LEFT JOIN %s o ON o.planet_id = f.start_planet LEFT JOIN %s t ON t.planet_id = f.target_planet LEFT JOIN %s u ON u.player_id = t.owner_id WHERE q.type = ? AND f.mission <> ? AND f.owner_id = ? ORDER BY q.end ASC, q.prio DESC",
			prefixedNumericColumns("f", fleetIDs),
			prefixedNumericColumns("f", resourceIDs),
			queueTable,
			fleetTable,
			usersTable,
			planetsTable,
			planetsTable,
			usersTable,
		),
		legacyPlanetTypeAbandoned,
		queueTypeFleet,
		domaingame.FleetMissionMissile,
		playerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	missions := make([]domaingame.FleetMission, 0)
	unionCache := make(map[int]fleetMissionUnionDetails)
	for rows.Next() {
		mission, err := scanFleetMissionRow(rows, fleetIDs, resourceIDs)
		if err != nil {
			return nil, err
		}
		if mission.UnionID > 0 {
			details, ok := unionCache[mission.UnionID]
			if !ok {
				details, err = r.loadFleetMissionUnionDetails(ctx, unionTable, usersTable, mission.UnionID)
				if err != nil {
					return nil, err
				}
				unionCache[mission.UnionID] = details
			}
			mission.UnionName = details.Name
			mission.UnionPlayers = details.Players
		}
		missions = append(missions, mission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return missions, nil
}

func scanFleetMissionRow(rows Rows, fleetIDs []int, resourceIDs []int) (domaingame.FleetMission, error) {
	var id int
	var departureAt int64
	var arrivalAt int64
	var mission int
	var ownerID int
	var ownerName string
	var unionID int
	var startPlanetID int
	var targetPlanetID int
	shipValues := make([]int, len(fleetIDs))
	var origin domaingame.Coordinates
	var originName string
	var target domaingame.Coordinates
	var targetName string
	var targetType int
	var targetOwner string

	dest := []any{&id, &departureAt, &arrivalAt, &mission, &ownerID, &ownerName, &unionID, &startPlanetID, &targetPlanetID}
	for index := range shipValues {
		dest = append(dest, &shipValues[index])
	}
	resourceValues := make([]int, len(resourceIDs))
	for index := range resourceValues {
		dest = append(dest, &resourceValues[index])
	}
	dest = append(dest,
		&originName,
		&origin.Galaxy,
		&origin.System,
		&origin.Position,
		&targetName,
		&target.Galaxy,
		&target.System,
		&target.Position,
		&targetType,
		&targetOwner,
	)
	if err := rows.Scan(dest...); err != nil {
		return domaingame.FleetMission{}, err
	}

	ships := make(domaingame.FleetCounts, len(fleetIDs))
	for index, fleetID := range fleetIDs {
		ships[fleetID] = shipValues[index]
	}
	loadedResources := make(map[int]int, len(resourceIDs))
	for index, resourceID := range resourceIDs {
		loadedResources[resourceID] = resourceValues[index]
	}
	row := domaingame.BuildFleetMission(id, mission, ships, origin, target, targetType, targetOwner, departureAt, arrivalAt)
	row.LoadedResources = loadedResources
	row.OwnerID = ownerID
	row.OwnerName = ownerName
	row.UnionID = unionID
	row.OriginName = originName
	row.TargetName = targetName
	return row, nil
}

type fleetMissionUnionDetails struct {
	Name    string
	Players []domaingame.FleetUnionPlayer
}

func (r FleetRepository) loadFleetMissionUnionDetails(ctx context.Context, unionTable string, usersTable string, unionID int) (fleetMissionUnionDetails, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(name, ''), COALESCE(players, '') FROM %s WHERE union_id = ? LIMIT 1", unionTable), unionID)
	if err != nil {
		return fleetMissionUnionDetails{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fleetMissionUnionDetails{}, err
		}
		return fleetMissionUnionDetails{}, nil
	}
	var name string
	var players string
	if err := rows.Scan(&name, &players); err != nil {
		return fleetMissionUnionDetails{}, err
	}
	if err := rows.Err(); err != nil {
		return fleetMissionUnionDetails{}, err
	}
	unionPlayers, err := r.loadFleetMissionUnionPlayers(ctx, usersTable, parseFleetMissionUnionPlayerIDs(players))
	if err != nil {
		return fleetMissionUnionDetails{}, err
	}
	return fleetMissionUnionDetails{Name: name, Players: unionPlayers}, nil
}

func (r FleetRepository) loadFleetMissionUnionPlayers(ctx context.Context, usersTable string, playerIDs []int) ([]domaingame.FleetUnionPlayer, error) {
	if len(playerIDs) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(playerIDs))
	order := make(map[int]int, len(playerIDs))
	for index, playerID := range playerIDs {
		args = append(args, playerID)
		order[playerID] = index
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, oname FROM %s WHERE player_id IN (%s)", usersTable, placeholders(len(playerIDs))),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[int]domaingame.FleetUnionPlayer, len(playerIDs))
	for rows.Next() {
		var player domaingame.FleetUnionPlayer
		if err := rows.Scan(&player.ID, &player.Name); err != nil {
			return nil, err
		}
		byID[player.ID] = player
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	players := make([]domaingame.FleetUnionPlayer, 0, len(playerIDs))
	for _, playerID := range playerIDs {
		player, ok := byID[playerID]
		if !ok {
			continue
		}
		if _, known := order[player.ID]; known {
			players = append(players, player)
		}
	}
	return players, nil
}

func parseFleetMissionUnionPlayerIDs(raw string) []int {
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	seen := make(map[int]bool, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func prefixedNumericColumns(prefix string, ids []int) string {
	columns := make([]string, 0, len(ids))
	for _, id := range ids {
		columns = append(columns, fmt.Sprintf("%s.`%d`", prefix, id))
	}
	return strings.Join(columns, ", ")
}

func numericAssignments(ids []int) string {
	assignments := make([]string, 0, len(ids))
	for _, id := range ids {
		assignments = append(assignments, fmt.Sprintf("`%d` = ?", id))
	}
	return strings.Join(assignments, ", ")
}

func placeholders(count int) string {
	values := make([]string, 0, count)
	for range count {
		values = append(values, "?")
	}
	return strings.Join(values, ", ")
}

func fleetTemplateValues(ids []int, ships map[int]int) []any {
	values := make([]any, 0, len(ids))
	for _, id := range ids {
		count := ships[id]
		if id == domaingame.FleetSolarSatellite || count < 0 {
			count = 0
		}
		values = append(values, count)
	}
	return values
}

func fleetCountValues(ids []int, ships map[int]int) []any {
	values := make([]any, 0, len(ids))
	for _, id := range ids {
		count := ships[id]
		if count < 0 {
			count = 0
		}
		values = append(values, count)
	}
	return values
}

func fleetLaunchPlanetType(targetType int) int {
	switch targetType {
	case domaingame.GamePlanetTypeMoon:
		return domaingame.PlanetTypeMoon
	case domaingame.GamePlanetTypeDebris:
		return domaingame.PlanetTypeDebris
	default:
		return domaingame.PlanetTypePlanet
	}
}

func fleetLaunchShips(rows []domaingame.FleetShipCount) domaingame.FleetCounts {
	ships := make(domaingame.FleetCounts, len(rows))
	for _, row := range rows {
		if row.Count > 0 {
			ships[row.ID] = row.Count
		}
	}
	return ships
}

func fleetLaunchResources(rows []domaingame.FleetResourceLoad) map[int]int {
	resources := map[int]int{
		resourceMetal:     0,
		resourceCrystal:   0,
		resourceDeuterium: 0,
	}
	for _, row := range rows {
		if row.Loaded > 0 {
			resources[row.ID] = row.Loaded
		}
	}
	return resources
}
