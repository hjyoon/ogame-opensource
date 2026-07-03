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

type JumpGateRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
}

func NewJumpGateRepository(db *sql.DB, prefix string) JumpGateRepository {
	runner := SQLQueryer{DB: db}
	return JumpGateRepository{queryer: runner, execer: runner, prefix: prefix, now: time.Now}
}

func NewJumpGateRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, now func() time.Time) JumpGateRepository {
	if now == nil {
		now = time.Now
	}
	return JumpGateRepository{queryer: queryer, execer: execer, prefix: prefix, now: now}
}

func (r JumpGateRepository) GetJumpGate(ctx context.Context, query appgame.JumpGateQuery) (domaingame.JumpGate, error) {
	if r.queryer == nil {
		return domaingame.JumpGate{}, errors.New("jump gate reader unavailable")
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	overview, err := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: query.PlanetID,
	})
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	source, found, err := r.loadJumpGateMoon(ctx, planetsTable, overview.CurrentPlanet.ID)
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	if !found {
		source = domaingame.JumpGateMoon{
			ID:          overview.CurrentPlanet.ID,
			OwnerID:     query.PlayerID,
			Name:        overview.CurrentPlanet.Name,
			Type:        overview.CurrentPlanet.Type,
			Coordinates: overview.CurrentPlanet.Coordinates,
		}
	}
	targets, err := r.loadJumpGateTargets(ctx, planetsTable, query.PlayerID, source.ID)
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	issue := r.jumpGateViewIssue(query.PlayerID, source, found)
	return domaingame.BuildJumpGate(overview, source, targets, issue), nil
}

func (r JumpGateRepository) Jump(ctx context.Context, query appgame.JumpGateMutationQuery) (domaingame.JumpGate, error) {
	if r.queryer == nil {
		return domaingame.JumpGate{}, errors.New("jump gate reader unavailable")
	}
	if r.execer == nil {
		return domaingame.JumpGate{}, errors.New("jump gate writer unavailable")
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	uniTable, err := tableName(r.prefix, "uni")
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	sourceID := query.SourceMoonID
	if sourceID <= 0 {
		sourceID = query.PlanetID
	}
	source, sourceFound, err := r.loadJumpGateMoon(ctx, planetsTable, sourceID)
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	target, targetFound, err := r.loadJumpGateMoon(ctx, planetsTable, query.TargetMoonID)
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	now := r.now().Unix()
	selection := domaingame.NormalizeJumpGateSelection(query.Ships)
	if issue := domaingame.JumpGateMoveIssue(query.PlayerID, source, sourceFound, target, targetFound, selection, now); issue != nil {
		return r.jumpGateMutationResult(ctx, query, planetsTable, source, target, issue)
	}
	fleetSpeed, err := r.loadJumpGateFleetSpeed(ctx, uniTable)
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	cooldown := domaingame.JumpGateCooldownUntil(now, fleetSpeed)
	if err := r.adjustJumpGateShips(ctx, planetsTable, source.ID, query.PlayerID, selection, -1, cooldown); err != nil {
		return domaingame.JumpGate{}, err
	}
	if err := r.adjustJumpGateShips(ctx, planetsTable, target.ID, query.PlayerID, selection, 1, cooldown); err != nil {
		return domaingame.JumpGate{}, err
	}
	source.GateUntil = cooldown
	target.GateUntil = cooldown
	for id, amount := range selection {
		source.Ships[id] -= amount
		target.Ships[id] += amount
	}
	return r.jumpGateMutationResult(ctx, query, planetsTable, source, target, domaingame.JumpGateMovedIssue())
}

func (r JumpGateRepository) jumpGateViewIssue(playerID int, source domaingame.JumpGateMoon, found bool) *domaingame.JumpGateActionIssue {
	if !found || source.Type != domaingame.PlanetTypeMoon {
		return &domaingame.JumpGateActionIssue{Code: domaingame.JumpGateIssueSourceMoonMissing, Message: "no source moon selected"}
	}
	if source.OwnerID != playerID {
		return &domaingame.JumpGateActionIssue{Code: domaingame.JumpGateIssueForeignMoon, Message: "either the source moon or target moon doesn't belong to you"}
	}
	if source.GateLevel <= 0 {
		return &domaingame.JumpGateActionIssue{Code: domaingame.JumpGateIssueSourceGateMissing, Message: "no jump gate found at source moon"}
	}
	if now := r.now().Unix(); now < source.GateUntil {
		return &domaingame.JumpGateActionIssue{Code: domaingame.JumpGateIssueCooldown, Message: domaingame.JumpGateCooldownMessage(source.GateUntil - now)}
	}
	return nil
}

func (r JumpGateRepository) jumpGateMutationResult(ctx context.Context, query appgame.JumpGateMutationQuery, planetsTable string, source domaingame.JumpGateMoon, target domaingame.JumpGateMoon, issue *domaingame.JumpGateActionIssue) (domaingame.JumpGate, error) {
	overview, err := NewOverviewRepositoryWithRunner(r.queryer, r.execer, r.prefix).GetOverview(ctx, appgame.OverviewQuery{
		PlayerID: query.PlayerID,
		PlanetID: source.ID,
	})
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	targets, err := r.loadJumpGateTargets(ctx, planetsTable, query.PlayerID, source.ID)
	if err != nil {
		return domaingame.JumpGate{}, err
	}
	if target.ID > 0 && target.OwnerID == query.PlayerID && target.Type == domaingame.PlanetTypeMoon && target.GateLevel > 0 && target.ID != source.ID && target.GateUntil <= r.now().Unix() {
		hasTarget := false
		for _, existing := range targets {
			if existing.ID == target.ID {
				hasTarget = true
				break
			}
		}
		if !hasTarget {
			targets = append(targets, target)
		}
	}
	return domaingame.BuildJumpGate(overview, source, targets, issue), nil
}

func (r JumpGateRepository) loadJumpGateMoon(ctx context.Context, planetsTable string, planetID int) (domaingame.JumpGateMoon, bool, error) {
	if planetID <= 0 {
		return domaingame.JumpGateMoon{}, false, nil
	}
	fleetIDs := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, owner_id, name, type, g, s, p, `%d`, gate_until, %s FROM %s WHERE planet_id = ? LIMIT 1", domaingame.BuildingJumpGate, numericColumns(fleetIDs), planetsTable),
		planetID,
	)
	if err != nil {
		return domaingame.JumpGateMoon{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.JumpGateMoon{}, false, err
		}
		return domaingame.JumpGateMoon{}, false, nil
	}
	moon := domaingame.JumpGateMoon{Ships: make(domaingame.FleetCounts, len(fleetIDs))}
	values := make([]int, len(fleetIDs))
	dest := []any{
		&moon.ID,
		&moon.OwnerID,
		&moon.Name,
		&moon.Type,
		&moon.Coordinates.Galaxy,
		&moon.Coordinates.System,
		&moon.Coordinates.Position,
		&moon.GateLevel,
		&moon.GateUntil,
	}
	for index := range values {
		dest = append(dest, &values[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return domaingame.JumpGateMoon{}, false, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.JumpGateMoon{}, false, err
	}
	for index, id := range fleetIDs {
		moon.Ships[id] = values[index]
	}
	return moon, true, nil
}

func (r JumpGateRepository) loadJumpGateTargets(ctx context.Context, planetsTable string, playerID int, sourceID int) ([]domaingame.JumpGateMoon, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, owner_id, name, type, g, s, p, `%d`, gate_until FROM %s WHERE owner_id = ? AND type = ? AND `%d` > 0 AND gate_until <= ? AND planet_id <> ? ORDER BY name ASC, planet_id ASC", domaingame.BuildingJumpGate, planetsTable, domaingame.BuildingJumpGate),
		playerID,
		domaingame.PlanetTypeMoon,
		r.now().Unix(),
		sourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := make([]domaingame.JumpGateMoon, 0)
	for rows.Next() {
		var moon domaingame.JumpGateMoon
		if err := rows.Scan(
			&moon.ID,
			&moon.OwnerID,
			&moon.Name,
			&moon.Type,
			&moon.Coordinates.Galaxy,
			&moon.Coordinates.System,
			&moon.Coordinates.Position,
			&moon.GateLevel,
			&moon.GateUntil,
		); err != nil {
			return nil, err
		}
		targets = append(targets, moon)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return targets, nil
}

func (r JumpGateRepository) loadJumpGateFleetSpeed(ctx context.Context, uniTable string) (float64, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT fspeed FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 1, nil
	}
	var speed float64
	if err := rows.Scan(&speed); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if speed <= 0 {
		return 1, nil
	}
	return speed, nil
}

func (r JumpGateRepository) adjustJumpGateShips(ctx context.Context, planetsTable string, planetID int, playerID int, ships map[int]int, direction int, cooldown int64) error {
	ids := make([]int, 0, len(domaingame.JumpGateMobileFleetIDs()))
	args := make([]any, 0, len(ships)+3)
	assignments := make([]string, 0, len(ships)+1)
	for _, id := range domaingame.JumpGateMobileFleetIDs() {
		amount := ships[id]
		if amount <= 0 {
			continue
		}
		ids = append(ids, id)
		operator := "+"
		if direction < 0 {
			operator = "-"
		}
		assignments = append(assignments, fmt.Sprintf("`%d` = `%d` %s ?", id, id, operator))
		args = append(args, amount)
	}
	if len(ids) == 0 {
		return nil
	}
	assignments = append(assignments, "gate_until = ?")
	args = append(args, cooldown, planetID, playerID)
	if direction < 0 {
		for _, id := range ids {
			args = append(args, ships[id])
		}
	}
	where := "planet_id = ? AND owner_id = ?"
	if direction < 0 {
		where += jumpGateEnoughShipsWhere(ids)
	}
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE %s LIMIT 1", planetsTable, strings.Join(assignments, ", "), where), args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("jump gate race: ship update did not affect one row")
	}
	return nil
}

func jumpGateEnoughShipsWhere(ids []int) string {
	clauses := make([]string, 0, len(ids))
	for _, id := range ids {
		clauses = append(clauses, fmt.Sprintf("`%d` >= ?", id))
	}
	return " AND " + strings.Join(clauses, " AND ")
}
