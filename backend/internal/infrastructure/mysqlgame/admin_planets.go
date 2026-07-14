package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const adminPlanetTypeCustom = 20001

type adminPlanetMutationTarget struct {
	ID           int
	OwnerID      int
	HomePlanetID int
	Type         int
	Coordinates  domaingame.Coordinates
	Diameter     int
	Temperature  int
	Language     string
}

func (r AdminRepository) mutateAdminPlanets(ctx context.Context, query appgame.AdminMutationQuery) (*domaingame.AdminActionIssue, error) {
	if query.Action == domaingame.AdminActionPlanetsSearch {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	target, found, err := r.loadAdminPlanetMutationTarget(ctx, planetsTable, usersTable, query.PlanetID)
	if err != nil {
		return nil, err
	}
	if !found {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	switch query.Action {
	case domaingame.AdminActionPlanetsUpdate:
		return r.updateAdminPlanet(ctx, planetsTable, target, query.Planet)
	case domaingame.AdminActionPlanetsCreateMoon:
		err = r.createAdminPlanetMoon(ctx, planetsTable, target)
	case domaingame.AdminActionPlanetsCreateDebris:
		err = r.createAdminPlanetDebris(ctx, planetsTable, usersTable, query.PlayerID, target)
	case domaingame.AdminActionPlanetsCooldownGates:
		err = r.setAdminPlanetGate(ctx, planetsTable, target, 0)
	case domaingame.AdminActionPlanetsWarmupGates:
		err = r.setAdminPlanetGate(ctx, planetsTable, target, r.now().Unix()+59*60+59)
	case domaingame.AdminActionPlanetsRecalcFields:
		err = r.recalcAdminPlanetFields(ctx, planetsTable, target.ID)
	case domaingame.AdminActionPlanetsRandomDiameter:
		err = r.randomizeAdminPlanetDiameter(ctx, planetsTable, target)
	}
	if err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) updateAdminPlanet(ctx context.Context, planetsTable string, target adminPlanetMutationTarget, mutation *domaingame.AdminPlanetMutation) (*domaingame.AdminActionIssue, error) {
	if mutation == nil {
		return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
	}
	if mutation.Delete {
		issue := domaingame.AdminIssue(domaingame.AdminIssueActionSaved)
		issue.Result = &domaingame.AdminActionResult{ItemID: target.HomePlanetID}
		if target.HomePlanetID == target.ID {
			return issue, nil
		}
		queueTable, err := tableName(r.prefix, "queue")
		if err != nil {
			return nil, err
		}
		buildQueueTable, err := tableName(r.prefix, "buildqueue")
		if err != nil {
			return nil, err
		}
		if err := r.overview.flushPlanetQueue(ctx, queueTable, buildQueueTable, target.ID); err != nil {
			return nil, err
		}
		if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE planet_id = ?", planetsTable), target.ID); err != nil {
			return nil, err
		}
		return issue, nil
	}
	now := r.now().Unix()
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET lastpeek = ?, g = ?, s = ?, p = ? WHERE g = ? AND s = ? AND p = ? AND type IN (?, ?) LIMIT 1", planetsTable),
		now, mutation.Coordinates.Galaxy, mutation.Coordinates.System, mutation.Coordinates.Position,
		target.Coordinates.Galaxy, target.Coordinates.System, target.Coordinates.Position,
		domaingame.PlanetTypeMoon, domaingame.PlanetTypeDestroyedMoon,
	); err != nil {
		return nil, err
	}
	set := []string{"lastpeek = ?"}
	args := []any{now}
	for _, id := range domaingame.BuildingIDs() {
		if value, ok := mutation.Buildings[id]; ok {
			set = append(set, fmt.Sprintf("`%d` = ?", id))
			args = append(args, adminBoundedLevel(value))
		}
	}
	for _, id := range domaingame.DefenseIDs() {
		if value, ok := mutation.Defense[id]; ok {
			set = append(set, fmt.Sprintf("`%d` = ?", id))
			args = append(args, adminPlanetAbs(value))
		}
	}
	for _, id := range domaingame.FleetIDs() {
		if value, ok := mutation.Fleet[id]; ok {
			set = append(set, fmt.Sprintf("`%d` = ?", id))
			args = append(args, adminPlanetAbs(value))
		}
	}
	for _, id := range []int{domaingame.ResourceMetal, domaingame.ResourceCrystal, domaingame.ResourceDeuterium} {
		if value, ok := mutation.Resources[id]; ok {
			set = append(set, fmt.Sprintf("`%d` = ?", id))
			args = append(args, adminPlanetAbs(value))
		}
	}
	for _, field := range []struct {
		name  string
		value int
	}{{"g", mutation.Coordinates.Galaxy}, {"s", mutation.Coordinates.System}, {"p", mutation.Coordinates.Position}, {"diameter", mutation.Diameter}, {"type", mutation.Type}, {"temp", mutation.Temperature}} {
		set = append(set, field.name+" = ?")
		args = append(args, adminPlanetAbs(field.value))
	}
	for _, id := range []int{1, 2, 3, 4, 12, 212} {
		if value, ok := mutation.Production[id]; ok {
			set = append(set, fmt.Sprintf("prod%d = ?", id))
			args = append(args, value)
		}
	}
	args = append(args, target.ID)
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE planet_id = ?", planetsTable, strings.Join(set, ", ")), args...); err != nil {
		return nil, err
	}
	if err := r.recalcAdminPlanetFields(ctx, planetsTable, target.ID); err != nil {
		return nil, err
	}
	return domaingame.AdminIssue(domaingame.AdminIssueActionSaved), nil
}

func (r AdminRepository) loadAdminPlanetMutationTarget(ctx context.Context, planetsTable string, usersTable string, planetID int) (adminPlanetMutationTarget, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT p.planet_id, p.owner_id, COALESCE(u.hplanetid, 0), p.type, p.g, p.s, p.p, p.diameter, p.temp, COALESCE(u.lang, 'en') FROM %s p LEFT JOIN %s u ON u.player_id = p.owner_id WHERE p.planet_id = ? LIMIT 1", planetsTable, usersTable), planetID)
	if err != nil {
		return adminPlanetMutationTarget{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return adminPlanetMutationTarget{}, false, rows.Err()
	}
	var target adminPlanetMutationTarget
	if err := rows.Scan(&target.ID, &target.OwnerID, &target.HomePlanetID, &target.Type, &target.Coordinates.Galaxy, &target.Coordinates.System, &target.Coordinates.Position, &target.Diameter, &target.Temperature, &target.Language); err != nil {
		return adminPlanetMutationTarget{}, false, err
	}
	return target, true, rows.Err()
}

func (r AdminRepository) createAdminPlanetMoon(ctx context.Context, planetsTable string, target adminPlanetMutationTarget) error {
	if target.Type <= domaingame.PlanetTypeMoon || target.Type >= domaingame.PlanetTypeDebris {
		return nil
	}
	exists, err := r.adminRelatedPlanetExists(ctx, planetsTable, target.Coordinates, []int{domaingame.PlanetTypeMoon, domaingame.PlanetTypeDestroyedMoon})
	if err != nil || exists {
		return err
	}
	random := r.randomIntN
	if random == nil {
		random = randomAdminIntN
	}
	diameter := int(math.Floor(1000 * math.Sqrt(float64(10+random(11)+3*20))))
	_ = random(10)
	temperature := target.Temperature - (20 + random(11))
	now := r.now().Unix()
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, type, g, s, p, owner_id, diameter, temp, fields, maxfields, date, `%d`, `%d`, `%d`, lastpeek, lastakt, gate_until, remove) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 1, ?, 0, 0, 0, ?, ?, 0, 0)", planetsTable, resourceMetal, resourceCrystal, resourceDeuterium),
		battleMoonName(target.Language), domaingame.PlanetTypeMoon, target.Coordinates.Galaxy, target.Coordinates.System, target.Coordinates.Position, target.OwnerID, diameter, temperature, now, now, now)
	return err
}

func (r AdminRepository) createAdminPlanetDebris(ctx context.Context, planetsTable string, usersTable string, actorID int, target adminPlanetMutationTarget) error {
	if target.Type <= domaingame.PlanetTypeMoon || target.Type >= domaingame.PlanetTypeDebris {
		return nil
	}
	exists, err := r.adminRelatedPlanetExists(ctx, planetsTable, target.Coordinates, []int{domaingame.PlanetTypeDebris})
	if err != nil || exists {
		return err
	}
	language, err := r.loadAdminUserLanguage(ctx, usersTable, actorID)
	if err != nil {
		return err
	}
	now := r.now().Unix()
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, type, g, s, p, owner_id, diameter, temp, fields, maxfields, date, `%d`, `%d`, `%d`, lastpeek, lastakt, gate_until, remove) VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, 0, ?, 0, 0, 0, ?, ?, 0, 0)", planetsTable, resourceMetal, resourceCrystal, resourceDeuterium),
		adminDebrisName(language), domaingame.PlanetTypeDebris, target.Coordinates.Galaxy, target.Coordinates.System, target.Coordinates.Position, target.OwnerID, now, now, now)
	return err
}

func (r AdminRepository) adminRelatedPlanetExists(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates, types []int) (bool, error) {
	if len(types) == 0 {
		return false, nil
	}
	placeholders := make([]string, len(types))
	args := []any{coordinates.Galaxy, coordinates.System, coordinates.Position}
	for index, planetType := range types {
		placeholders[index] = "?"
		args = append(args, planetType)
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT planet_id FROM %s WHERE g = ? AND s = ? AND p = ? AND type IN (%s) LIMIT 1", planetsTable, strings.Join(placeholders, ", ")), args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	var id int
	if err := rows.Scan(&id); err != nil {
		return false, err
	}
	return id > 0, rows.Err()
}

func (r AdminRepository) loadAdminUserLanguage(ctx context.Context, usersTable string, playerID int) (string, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(lang, 'en') FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", err
		}
		return "", errors.New("admin planet user language unavailable")
	}
	var language string
	if err := rows.Scan(&language); err != nil {
		return "", err
	}
	return language, rows.Err()
}

func (r AdminRepository) setAdminPlanetGate(ctx context.Context, planetsTable string, target adminPlanetMutationTarget, until int64) error {
	if target.Type != domaingame.PlanetTypeMoon {
		return nil
	}
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET gate_until = ? WHERE planet_id = ?", planetsTable), until, target.ID)
	return err
}

func (r AdminRepository) recalcAdminPlanetFields(ctx context.Context, planetsTable string, planetID int) error {
	ids := domaingame.BuildingIDs()
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT type, diameter, %s FROM %s WHERE planet_id = ? LIMIT 1", numericColumns(ids), planetsTable), planetID)
	if err != nil {
		return err
	}
	if !rows.Next() {
		rows.Close()
		return rows.Err()
	}
	var planetType, diameter int
	values := make([]int, len(ids))
	dest := []any{&planetType, &diameter}
	dest = appendIntDest(dest, values)
	if err := rows.Scan(dest...); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	fields := 0
	levels := make(map[int]int, len(ids))
	for index, id := range ids {
		fields += values[index]
		levels[id] = values[index]
	}
	maxFields := int(math.Floor(math.Pow(float64(diameter)/1000, 2)))
	if planetType == domaingame.PlanetTypeMoon || planetType == domaingame.PlanetTypeDestroyedMoon {
		maxFields = 1
	}
	maxFields += 5*levels[domaingame.BuildingTerraformer] + 3*levels[domaingame.BuildingLunarBase]
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET fields = ?, maxfields = ? WHERE planet_id = ?", planetsTable), fields, maxFields, planetID)
	return err
}

func (r AdminRepository) randomizeAdminPlanetDiameter(ctx context.Context, planetsTable string, target adminPlanetMutationTarget) error {
	if !adminIsGamePlanet(target.Type) {
		return nil
	}
	colonyTable, err := tableName(r.prefix, "coltab")
	if err != nil {
		return err
	}
	settings, err := (FleetRepository{queryer: r.queryer, prefix: r.prefix}).loadColonySettings(ctx, colonyTable)
	if err != nil {
		return err
	}
	tier := settings.Tiers[domaingame.ColonyTierIndex(target.Coordinates.Position)]
	random := r.randomIntN
	if random == nil {
		random = randomAdminIntN
	}
	span := max(1, tier.Maximum-tier.Minimum+1)
	diameter := domaingame.ColonyDiameter(settings, target.Coordinates.Position, tier.Minimum+random(span))
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET diameter = ? WHERE planet_id = ?", planetsTable), diameter, target.ID)
	return err
}

func (r AdminRepository) loadAdminPlanetSearchRows(ctx context.Context, search domaingame.AdminPlanetSearch) ([]domaingame.AdminPlanetRow, error) {
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return nil, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return nil, err
	}
	allyTable, err := tableName(r.prefix, "ally")
	if err != nil {
		return nil, err
	}
	where := "1 = 0"
	switch search.Type {
	case "playername":
		where = "u.oname LIKE ?"
	case "planetname":
		where = "p.name LIKE ?"
	case "allytag":
		where = "u.ally_id <> 0 AND a.tag LIKE ?"
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT p.planet_id, COALESCE(p.name, ''), COALESCE(p.date, 0), COALESCE(p.g, 0), COALESCE(p.s, 0), COALESCE(p.p, 0), COALESCE(p.`%d`, 0), COALESCE(p.`%d`, 0), COALESCE(p.`%d`, 0), COALESCE(u.player_id, 0), COALESCE(u.oname, ''), COALESCE(u.regdate, 0), COALESCE(u.lastclick, 0), COALESCE(u.vacation, 0), COALESCE(u.banned, 0), COALESCE(u.noattack, 0), COALESCE(u.disable, 0) FROM %s p LEFT JOIN %s u ON u.player_id = p.owner_id LEFT JOIN %s a ON a.ally_id = u.ally_id WHERE %s", resourceMetal, resourceCrystal, resourceDeuterium, planetsTable, usersTable, allyTable, where), search.Text+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domaingame.AdminPlanetRow, 0)
	for rows.Next() {
		var row domaingame.AdminPlanetRow
		var owner domaingame.AdminUserRow
		var vacation, banned, noattack, disable int
		if err := rows.Scan(&row.ID, &row.Name, &row.Date, &row.Coordinates.Galaxy, &row.Coordinates.System, &row.Coordinates.Position,
			&row.Resources.Metal, &row.Resources.Crystal, &row.Resources.Deuterium,
			&owner.PlayerID, &owner.Name, &owner.RegDate, &owner.LastClick, &vacation, &banned, &noattack, &disable); err != nil {
			return nil, err
		}
		if owner.PlayerID != 0 {
			owner.Vacation, owner.Banned, owner.NoAttack, owner.Disable = vacation != 0, banned != 0, noattack != 0, disable != 0
			row.Owner = &owner
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func adminPlanetAbs(value int) int {
	if value >= 0 {
		return value
	}
	value = -value
	if value < 0 {
		return int(^uint(0) >> 1)
	}
	return value
}

func adminIsGamePlanet(planetType int) bool {
	return planetType < adminPlanetTypeCustom && planetType != domaingame.PlanetTypeMoon && planetType != domaingame.PlanetTypeDestroyedMoon && planetType != domaingame.PlanetTypeDebris
}

func adminDebrisName(language string) string {
	switch language {
	case "de":
		return "Trümmerfeld"
	case "es":
		return "Campo de escombros"
	case "fr":
		return "Champ de débris"
	case "it":
		return "Campo detriti"
	case "ru":
		return "Поле обломков"
	case "jp":
		return "デブリフィールド"
	default:
		return "Debris Field"
	}
}
