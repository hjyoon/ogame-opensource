package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"math"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func (r AdminRepository) mutateAdminUsers(ctx context.Context, usersTable string, planetsTable string, fleetTable string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	targetID := firstPositiveID(query.TargetIDs)
	if targetID <= 0 {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	switch query.Action {
	case domaingame.AdminActionUsersRecalcStats:
		if err := r.recalcAdminUserStats(ctx, usersTable, planetsTable, fleetTable, targetID); err != nil {
			return nil, err
		}
	case domaingame.AdminActionUsersUpdate:
		if err := r.updateAdminUserDeletion(ctx, usersTable, targetID, query.Values); err != nil {
			return nil, err
		}
	case domaingame.AdminActionUsersCreatePlanet:
		coordinates := adminCoordinatesFromValues(query.Values)
		occupied, err := r.adminPlanetSlotOccupied(ctx, planetsTable, coordinates)
		if err != nil {
			return nil, err
		}
		if !occupied {
			if _, err := r.createAdminUserPlanet(ctx, planetsTable, targetID, coordinates); err != nil {
				return nil, err
			}
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func firstPositiveID(ids []int) int {
	for _, id := range ids {
		if id > 0 {
			return id
		}
	}
	return 0
}

func adminCoordinatesFromValues(values map[string]int) domaingame.Coordinates {
	coordinates := domaingame.Coordinates{Galaxy: 1, System: 1, Position: 1}
	if value := values["g"]; value > 0 {
		coordinates.Galaxy = value
	}
	if value := values["s"]; value > 0 {
		coordinates.System = value
	}
	if value := values["p"]; value > 0 {
		coordinates.Position = value
	}
	return coordinates
}

func (r AdminRepository) updateAdminUserDeletion(ctx context.Context, usersTable string, targetID int, values map[string]int) error {
	disable := 0
	disableUntil := 0
	if values["deaktjava"] != 0 {
		disable = 1
		disableUntil = int(r.now().Unix()) + 7*24*60*60
	}
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET disable = ?, disable_until = ? WHERE player_id = ? LIMIT 1", usersTable),
		disable,
		disableUntil,
		targetID,
	)
	return err
}

func (r AdminRepository) adminPlanetSlotOccupied(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates) (bool, error) {
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
	return id > 0, rows.Err()
}

func (r AdminRepository) createAdminUserPlanet(ctx context.Context, planetsTable string, targetID int, coordinates domaingame.Coordinates) (int, error) {
	now := r.now().Unix()
	diameter := adminColonyDiameter(coordinates.Position)
	maxFields := int(math.Floor(math.Pow(float64(diameter)/1000, 2)))
	result, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf(
			"INSERT INTO %s (name, type, g, s, p, owner_id, diameter, temp, fields, maxfields, date, `%d`, `%d`, `%d`, prod1, prod2, prod3, lastpeek, lastakt, gate_until, remove) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, 500, 500, 0, 0, 0, 0, ?, ?, 0, 0)",
			planetsTable,
			resourceMetal,
			resourceCrystal,
			resourceDeuterium,
		),
		"Colony",
		domaingame.PlanetTypePlanet,
		coordinates.Galaxy,
		coordinates.System,
		coordinates.Position,
		targetID,
		diameter,
		adminColonyTemperature(coordinates.Position),
		maxFields,
		now,
		now,
		now,
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("admin create planet id unavailable")
	}
	return int(id), nil
}

func adminColonyDiameter(position int) int {
	switch {
	case position <= 3:
		return 6400
	case position <= 6:
		return 9000
	case position <= 9:
		return 12800
	case position <= 12:
		return 14400
	default:
		return 15600
	}
}

func adminColonyTemperature(position int) int {
	switch {
	case position <= 3:
		return 80 - 2*position
	case position <= 6:
		return 30 - 2*position
	case position <= 9:
		return 10 - 2*position
	case position <= 12:
		return -10 - 2*position
	default:
		return -60 - 2*position
	}
}

func (r AdminRepository) recalcAdminUserStats(ctx context.Context, usersTable string, planetsTable string, fleetTable string, targetID int) error {
	planetScore, err := r.sumAdminUserPlanetScore(ctx, planetsTable, targetID)
	if err != nil {
		return err
	}
	research, err := ResearchRepository{queryer: r.queryer, prefix: r.prefix}.loadResearchLevels(ctx, usersTable, targetID)
	if err != nil {
		return err
	}
	researchPoints, researchLevels := adminResearchScore(research)
	fleetPoints, flyingFleetPoints, err := r.sumAdminUserFlyingFleetScore(ctx, fleetTable, targetID)
	if err != nil {
		return err
	}
	if _, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET score1 = ?, score2 = ?, score3 = ? WHERE player_id = ? AND (banned <> 1 OR admin > 0)", usersTable),
		planetScore.Points+researchPoints+fleetPoints,
		planetScore.FleetPoints+flyingFleetPoints,
		researchLevels,
		targetID,
	); err != nil {
		return err
	}
	return r.overview.recalcRanks(ctx, usersTable)
}

func (r AdminRepository) sumAdminUserPlanetScore(ctx context.Context, planetsTable string, targetID int) (domaingame.PlanetScore, error) {
	buildingIDs := domaingame.BuildingIDs()
	fleetIDs := domaingame.FleetIDs()
	defenseIDs := domaingame.DefenseIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT %s, %s, %s FROM %s WHERE owner_id = ? AND type < ?", numericColumns(buildingIDs), numericColumns(fleetIDs), numericColumns(defenseIDs), planetsTable),
		targetID,
		planetTypeDebris,
	)
	if err != nil {
		return domaingame.PlanetScore{}, err
	}
	defer rows.Close()
	var total domaingame.PlanetScore
	for rows.Next() {
		score, err := scanAdminUserPlanetScore(rows, buildingIDs, fleetIDs, defenseIDs)
		if err != nil {
			return domaingame.PlanetScore{}, err
		}
		total.Points += score.Points
		total.FleetPoints += score.FleetPoints
		total.DefensePoints += score.DefensePoints
	}
	return total, rows.Err()
}

func scanAdminUserPlanetScore(rows Rows, buildingIDs []int, fleetIDs []int, defenseIDs []int) (domaingame.PlanetScore, error) {
	buildingValues := make([]int, len(buildingIDs))
	fleetValues := make([]int, len(fleetIDs))
	defenseValues := make([]int, len(defenseIDs))
	dest := make([]any, 0, len(buildingIDs)+len(fleetIDs)+len(defenseIDs))
	dest = appendIntDest(dest, buildingValues)
	dest = appendIntDest(dest, fleetValues)
	dest = appendIntDest(dest, defenseValues)
	if err := rows.Scan(dest...); err != nil {
		return domaingame.PlanetScore{}, err
	}
	buildings := make(domaingame.BuildingLevels, len(buildingIDs))
	for index, id := range buildingIDs {
		buildings[id] = buildingValues[index]
	}
	fleet := make(domaingame.FleetCounts, len(fleetIDs))
	for index, id := range fleetIDs {
		fleet[id] = fleetValues[index]
	}
	defense := make(domaingame.DefenseCounts, len(defenseIDs))
	for index, id := range defenseIDs {
		defense[id] = defenseValues[index]
	}
	return domaingame.CalculatePlanetScore(buildings, fleet, defense), nil
}

func adminResearchScore(research domaingame.ResearchLevels) (points int64, levels int64) {
	for _, id := range domaingame.ResearchIDs() {
		level := research[id]
		if level <= 0 {
			continue
		}
		levels += int64(level)
		for current := 1; current <= level; current++ {
			score, ok := domaingame.ResearchScoreForLevel(id, current)
			if ok {
				points += score
			}
		}
	}
	return points, levels
}

func (r AdminRepository) sumAdminUserFlyingFleetScore(ctx context.Context, fleetTable string, targetID int) (points int64, fleetPoints int64, err error) {
	fleetIDs := domaingame.FleetIDs()
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT %s, COALESCE(ipm_amount, 0) FROM %s WHERE owner_id = ?", numericColumns(fleetIDs), fleetTable),
		targetID,
	)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		rowPoints, rowFleetPoints, err := scanAdminUserFlyingFleetScore(rows, fleetIDs)
		if err != nil {
			return 0, 0, err
		}
		points += rowPoints
		fleetPoints += rowFleetPoints
	}
	return points, fleetPoints, rows.Err()
}

func scanAdminUserFlyingFleetScore(rows Rows, fleetIDs []int) (points int64, fleetPoints int64, err error) {
	fleetValues := make([]int, len(fleetIDs))
	var missiles int
	dest := make([]any, 0, len(fleetIDs)+1)
	dest = appendIntDest(dest, fleetValues)
	dest = append(dest, &missiles)
	if err := rows.Scan(dest...); err != nil {
		return 0, 0, err
	}
	for index, id := range fleetIDs {
		unitPoints, unitFleetPoints, ok := domaingame.UnitScoreForCount(id, fleetValues[index])
		if ok {
			points += unitPoints
			fleetPoints += unitFleetPoints
		}
	}
	unitPoints, _, ok := domaingame.UnitScoreForCount(domaingame.DefenseInterplanetaryMissile, missiles)
	if ok {
		points += unitPoints
	}
	return points, fleetPoints, nil
}
