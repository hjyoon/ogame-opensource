package mysqlgame

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func (r AdminRepository) mutateAdminUsers(ctx context.Context, uniTable string, usersTable string, planetsTable string, fleetTable string, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
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
		var err error
		if query.User == nil {
			err = r.updateAdminUserDeletion(ctx, usersTable, targetID, query.Values)
		} else {
			err = r.updateAdminUser(ctx, uniTable, usersTable, targetID, query.User)
		}
		if err != nil {
			return nil, err
		}
	case domaingame.AdminActionUsersCreatePlanet:
		coordinates := adminCoordinatesFromValues(query.Values)
		occupied, err := r.adminPlanetSlotOccupied(ctx, planetsTable, coordinates)
		if err != nil {
			return nil, err
		}
		if !occupied {
			if _, err := r.createAdminUserPlanet(ctx, usersTable, planetsTable, targetID, coordinates); err != nil {
				return nil, err
			}
		}
	case domaingame.AdminActionUsersReactivate:
		mail, err := r.reactivateAdminUser(ctx, uniTable, usersTable, targetID)
		if err != nil {
			return nil, err
		}
		issue := domaingame.AdminIssue(domaingame.AdminIssueActionSaved)
		if mail != nil && query.RemoteAddr != "127.0.0.1" && query.RemoteAddr != "::1" {
			issue.OutboundReactivationMails = []domaingame.AdminReactivationMail{*mail}
		}
		return issue, nil
	case domaingame.AdminActionUsersBotStart:
		if err := r.startAdminUserBot(ctx, targetID); err != nil {
			return nil, err
		}
	case domaingame.AdminActionUsersBotStop:
		queueTable, err := tableName(r.prefix, "queue")
		if err != nil {
			return nil, err
		}
		if _, err := r.mutateAdminBotStop(ctx, queueTable, []int{targetID}); err != nil {
			return nil, err
		}
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) reactivateAdminUser(ctx context.Context, uniTable string, usersTable string, targetID int) (*domaingame.AdminReactivationMail, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(oname, ''), COALESCE(pemail, '') FROM %s WHERE player_id = ? LIMIT 1", usersTable), targetID)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		return nil, rows.Err()
	}
	var character, recipient string
	if err := rows.Scan(&character, &recipient); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	universeRows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(num, 1), COALESCE(lang, 'en'), COALESCE(ext_board, ''), COALESCE(ext_tutorial, '') FROM %s LIMIT 1", uniTable))
	if err != nil {
		return nil, err
	}
	if !universeRows.Next() {
		universeRows.Close()
		return nil, universeRows.Err()
	}
	var universeNumber int
	var language, boardURL, tutorialURL string
	if err := universeRows.Scan(&universeNumber, &language, &boardURL, &tutorialURL); err != nil {
		universeRows.Close()
		return nil, err
	}
	if err := universeRows.Err(); err != nil {
		universeRows.Close()
		return nil, err
	}
	universeRows.Close()
	random := make([]byte, 24)
	randomRead := r.randomRead
	if randomRead == nil {
		return nil, errors.New("admin reactivation random source unavailable")
	}
	if _, err := randomRead(random); err != nil {
		return nil, err
	}
	passwordBytes := make([]byte, 8)
	for index := range passwordBytes {
		passwordBytes[index] = 'a' + random[index]%26
	}
	activationCode := hex.EncodeToString(random[8:])
	password := string(passwordBytes)
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET validatemd = ?, validated = 0, password = ? WHERE player_id = ?", usersTable), activationCode, hashOverviewPassword(password, r.secret), targetID); err != nil {
		return nil, err
	}
	return &domaingame.AdminReactivationMail{
		Character: character, Password: password, Recipient: recipient, ActivationCode: activationCode,
		UniverseNumber: universeNumber, Language: language, BoardURL: boardURL, TutorialURL: tutorialURL,
	}, nil
}

func (r AdminRepository) startAdminUserBot(ctx context.Context, targetID int) error {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return err
	}
	strategyTable, err := tableName(r.prefix, "botstrat")
	if err != nil {
		return err
	}
	start, found, err := r.loadAdminBotStartStrategy(ctx, strategyTable)
	if err != nil || !found || !start.HasStart {
		return err
	}
	now := int(r.now().Unix())
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, 0, ?, ?, ?)", queueTable), targetID, queueTypeAI, start.ID, start.StartBlockID, now, now*2, botQueuePriority)
	return err
}

func (r AdminRepository) updateAdminUserDeletion(ctx context.Context, usersTable string, targetID int, values map[string]int) error {
	disable := values["deaktjava"] != 0
	disableUntil := 0
	if disable {
		disableUntil = int(r.now().Unix()) + 7*adminCronDaySeconds
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET disable = ?, disable_until = ? WHERE player_id = ? LIMIT 1", usersTable), boolInt(disable), disableUntil, targetID)
	return err
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

func (r AdminRepository) updateAdminUser(ctx context.Context, uniTable string, usersTable string, targetID int, mutation *domaingame.AdminUserMutation) error {
	if mutation == nil {
		return nil
	}
	now := int(r.now().Unix())
	speed, err := r.loadAdminUniverseSpeed(ctx, uniTable)
	if err != nil {
		return err
	}
	if speed <= 0 {
		speed = 1
	}
	disableUntil := 0
	if mutation.Disable {
		disableUntil = now + 7*adminCronDaySeconds
	}
	vacationUntil := 0
	if mutation.Vacation {
		vacationUntil = now + int(float64(2*adminCronDaySeconds)/speed)
	}
	set := make([]string, 0, len(domaingame.ResearchIDs())+20)
	args := make([]any, 0, len(domaingame.ResearchIDs())+24)
	for _, id := range domaingame.ResearchIDs() {
		set = append(set, fmt.Sprintf("`%d` = ?", id))
		args = append(args, adminBoundedLevel(mutation.Research[id]))
	}
	set = append(set,
		"disable = ?", "disable_until = ?", "vacation = ?", "vacation_until = ?",
		"pemail = ?", "email = ?", "admin = ?", "validated = ?", "sniff = ?", "debug = ?",
		"dm = ?", "dmfree = ?", "sortby = ?", "sortorder = ?", "skin = ?", "useskin = ?",
		"deact_ip = ?", "maxspy = ?", "maxfleetmsg = ?",
	)
	args = append(args,
		boolInt(mutation.Disable), disableUntil, boolInt(mutation.Vacation), vacationUntil,
		mutation.PermanentEmail, mutation.Email,
		mutation.AdminLevel, boolInt(mutation.Validated), boolInt(mutation.Sniff), boolInt(mutation.Debug),
		mutation.DarkMatter, mutation.DarkMatterFree, mutation.SortBy, mutation.SortOrder, mutation.Skin,
		boolInt(mutation.UseSkin), boolInt(mutation.DeactivateIP), mutation.MaxSpy, mutation.MaxFleetMsg,
	)
	args = append(args, targetID)
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE player_id = ?", usersTable, strings.Join(set, ", ")), args...); err != nil {
		return err
	}
	return r.updateAdminUserOfficers(ctx, usersTable, targetID, mutation.OfficerDays, now)
}

func (r AdminRepository) loadAdminUniverseSpeed(ctx context.Context, uniTable string) (float64, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(speed, 0) FROM %s LIMIT 1", uniTable))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var speed float64
	if err := rows.Scan(&speed); err != nil {
		return 0, err
	}
	return speed, rows.Err()
}

func adminBoundedLevel(value int) int {
	if value < 0 {
		value = -value
		if value < 0 {
			return 99
		}
	}
	if value > 99 {
		return 99
	}
	return value
}

func (r AdminRepository) updateAdminUserOfficers(ctx context.Context, usersTable string, targetID int, days map[int]int, now int) error {
	for _, officerID := range []int{domaingame.OfficerCommander, domaingame.OfficerAdmiral, domaingame.OfficerEngineer, domaingame.OfficerGeologist, domaingame.OfficerTechnocrat} {
		deltaDays, present := days[officerID]
		if !present {
			continue
		}
		column, ok := officerTimerColumn(officerID)
		if !ok {
			continue
		}
		seconds := deltaDays * adminCronDaySeconds
		statement := fmt.Sprintf("UPDATE %s SET %s = CASE WHEN ? < 0 THEN 0 WHEN ? >= %s THEN ? + ? ELSE %s + ? END WHERE player_id = ?", usersTable, column, column, column)
		if _, err := r.execer.ExecContext(ctx, statement, seconds, now, now, seconds, seconds, targetID); err != nil {
			return err
		}
	}
	return nil
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

func (r AdminRepository) createAdminUserPlanet(ctx context.Context, usersTable string, planetsTable string, targetID int, coordinates domaingame.Coordinates) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(lang, 'en') FROM %s WHERE player_id = ? LIMIT 1", usersTable), targetID)
	if err != nil {
		return 0, err
	}
	if !rows.Next() {
		rows.Close()
		return 0, errors.New("admin create planet user unavailable")
	}
	var language string
	if err := rows.Scan(&language); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	random := r.randomIntN
	if random == nil {
		random = randomAdminIntN
	}
	now := r.now().Unix()
	planetID, err := (FleetRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, combatRandom: random}).createFleetColony(ctx, planetsTable, targetID, coordinates, language, now)
	if err != nil {
		return 0, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET prod1 = 0, prod2 = 0, prod3 = 0 WHERE planet_id = ?", planetsTable), planetID); err != nil {
		return 0, err
	}
	return planetID, nil
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
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return err
	}
	fleetPoints, flyingFleetPoints, err := r.sumAdminUserFlyingFleetScore(ctx, fleetTable, queueTable, targetID)
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
		total.FleetCostPoints += score.FleetCostPoints
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

func (r AdminRepository) sumAdminUserFlyingFleetScore(ctx context.Context, fleetTable string, queueTable string, targetID int) (points int64, fleetPoints int64, err error) {
	// Legacy RecalcStats reads nonexistent ship-prefixed keys from flying fleets,
	// so only queued interplanetary missiles contribute to this score path.
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(f.ipm_amount, 0) FROM %s q JOIN %s f ON f.fleet_id = q.sub_id WHERE q.type = ? AND q.owner_id = ?", queueTable, fleetTable),
		queueTypeFleet,
		targetID,
	)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var missiles int
		if err := rows.Scan(&missiles); err != nil {
			return 0, 0, err
		}
		unitPoints, _, ok := domaingame.UnitScoreForCount(domaingame.DefenseInterplanetaryMissile, missiles)
		if ok {
			points += unitPoints
		}
	}
	return points, 0, rows.Err()
}
