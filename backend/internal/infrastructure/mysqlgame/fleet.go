package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const (
	queueTypeFleet            = "Fleet"
	legacyPlanetTypeAbandoned = 10004
)

type FleetRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

func NewFleetRepository(db *sql.DB, prefix string) FleetRepository {
	runner := SQLQueryer{DB: db}
	return FleetRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
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
	missions, err := r.loadActiveMissions(ctx, queueTable, fleetTable, planetsTable, usersTable, query.PlayerID)
	if err != nil {
		return domaingame.Fleet{}, err
	}

	fleet := domaingame.BuildFleet(overview, counts, research, missions, admiral, acsEnabled)
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

func (r FleetRepository) loadActiveMissions(ctx context.Context, queueTable string, fleetTable string, planetsTable string, usersTable string, playerID int) ([]domaingame.FleetMission, error) {
	fleetIDs := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT q.sub_id, q.start, q.end, f.mission, f.start_planet, f.target_planet, %s, COALESCE(o.g, 0), COALESCE(o.s, 0), COALESCE(o.p, 0), COALESCE(t.g, 0), COALESCE(t.s, 0), COALESCE(t.p, 0), COALESCE(t.type, ?), COALESCE(u.oname, 'space') FROM %s q JOIN %s f ON f.fleet_id = q.sub_id LEFT JOIN %s o ON o.planet_id = f.start_planet LEFT JOIN %s t ON t.planet_id = f.target_planet LEFT JOIN %s u ON u.player_id = t.owner_id WHERE q.type = ? AND f.mission <> ? AND f.owner_id = ? ORDER BY q.end ASC, q.prio DESC",
			prefixedNumericColumns("f", fleetIDs),
			queueTable,
			fleetTable,
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
	for rows.Next() {
		mission, err := scanFleetMissionRow(rows, fleetIDs)
		if err != nil {
			return nil, err
		}
		missions = append(missions, mission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return missions, nil
}

func scanFleetMissionRow(rows Rows, fleetIDs []int) (domaingame.FleetMission, error) {
	var id int
	var departureAt int64
	var arrivalAt int64
	var mission int
	var startPlanetID int
	var targetPlanetID int
	shipValues := make([]int, len(fleetIDs))
	var origin domaingame.Coordinates
	var target domaingame.Coordinates
	var targetType int
	var targetOwner string

	dest := []any{&id, &departureAt, &arrivalAt, &mission, &startPlanetID, &targetPlanetID}
	for index := range shipValues {
		dest = append(dest, &shipValues[index])
	}
	dest = append(dest,
		&origin.Galaxy,
		&origin.System,
		&origin.Position,
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
	return domaingame.BuildFleetMission(id, mission, ships, origin, target, targetType, targetOwner, departureAt, arrivalAt), nil
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
