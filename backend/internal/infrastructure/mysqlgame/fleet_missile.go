package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type missilePlanetState struct {
	ID       int
	OwnerID  int
	Type     int
	Name     string
	Galaxy   int
	System   int
	Position int
	Language string
	Weapon   int
	Armour   int
	Defense  domaingame.DefenseCounts
}

func (r FleetRepository) finishMissileArrival(ctx context.Context, fleetTable string, queueTable string, planetsTable string, usersTable string, messagesTable string, task fleetQueueTask, fleet recallFleetRow) error {
	origin, found, err := r.loadMissilePlanet(ctx, planetsTable, usersTable, fleet.StartPlanetID)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("missile origin unavailable")
	}
	target, found, err := r.loadMissilePlanet(ctx, planetsTable, usersTable, fleet.TargetPlanetID)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("missile target unavailable")
	}

	moonAttack := target.Type == domaingame.PlanetTypeMoon
	defendingPlanet := missilePlanetState{Defense: domaingame.DefenseCounts{}}
	if moonAttack {
		defendingPlanet, found, err = r.loadMissileDefendingPlanet(ctx, planetsTable, usersTable, target)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("missile defending planet unavailable")
		}
	}

	result := domaingame.ResolveMissileAttack(domaingame.MissileAttackInput{
		Amount: fleet.MissileAmount, PrimaryDefenseID: fleet.MissileTarget,
		MoonAttack: moonAttack, Target: target.Defense, DefendingPlanet: defendingPlanet.Defense,
		AttackerWeapon: origin.Weapon, DefenderArmour: target.Armour,
	})
	if err := r.updateMissilePlanetDefense(ctx, planetsTable, target.ID, result.Target, task.End, true); err != nil {
		return err
	}
	if moonAttack {
		if err := r.updateMissilePlanetDefense(ctx, planetsTable, defendingPlanet.ID, result.DefendingPlanet, task.End, false); err != nil {
			return err
		}
	}
	if err := (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable); err != nil {
		return err
	}
	target.Defense = result.Target
	defendingPlanet.Defense = result.DefendingPlanet
	if err := r.insertMissileReports(ctx, messagesTable, origin, target, defendingPlanet, moonAttack, fleet.MissileAmount, result.Intercepted, task.End); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) loadMissilePlanet(ctx context.Context, planetsTable string, usersTable string, planetID int) (missilePlanetState, bool, error) {
	ids := domaingame.DefenseIDs()
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT p.planet_id,p.owner_id,p.type,p.name,p.g,p.s,p.p,COALESCE(u.lang,'en'),COALESCE(u.`%d`,0),COALESCE(u.`%d`,0),%s FROM %s p JOIN %s u ON u.player_id=p.owner_id WHERE p.planet_id=? LIMIT 1", domaingame.ResearchWeapon, domaingame.ResearchArmour, prefixedNumericColumns("p", ids), planetsTable, usersTable), planetID)
	if err != nil {
		return missilePlanetState{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return missilePlanetState{}, false, err
		}
		return missilePlanetState{}, false, nil
	}
	state, err := scanMissilePlanet(rows, ids)
	if err != nil {
		return missilePlanetState{}, false, err
	}
	return state, true, rows.Err()
}

func (r FleetRepository) loadMissileDefendingPlanet(ctx context.Context, planetsTable string, usersTable string, moon missilePlanetState) (missilePlanetState, bool, error) {
	ids := domaingame.DefenseIDs()
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT p.planet_id,p.owner_id,p.type,p.name,p.g,p.s,p.p,COALESCE(u.lang,'en'),COALESCE(u.`%d`,0),COALESCE(u.`%d`,0),%s FROM %s p JOIN %s u ON u.player_id=p.owner_id WHERE p.g=? AND p.s=? AND p.p=? AND p.type=? LIMIT 1", domaingame.ResearchWeapon, domaingame.ResearchArmour, prefixedNumericColumns("p", ids), planetsTable, usersTable), moon.Galaxy, moon.System, moon.Position, domaingame.PlanetTypePlanet)
	if err != nil {
		return missilePlanetState{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return missilePlanetState{}, false, err
		}
		return missilePlanetState{}, false, nil
	}
	state, err := scanMissilePlanet(rows, ids)
	if err != nil {
		return missilePlanetState{}, false, err
	}
	return state, true, rows.Err()
}

func scanMissilePlanet(rows Rows, ids []int) (missilePlanetState, error) {
	state := missilePlanetState{Defense: make(domaingame.DefenseCounts, len(ids))}
	counts := make([]int, len(ids))
	dest := []any{&state.ID, &state.OwnerID, &state.Type, &state.Name, &state.Galaxy, &state.System, &state.Position, &state.Language, &state.Weapon, &state.Armour}
	for index := range counts {
		dest = append(dest, &counts[index])
	}
	if err := rows.Scan(dest...); err != nil {
		return missilePlanetState{}, err
	}
	for index, id := range ids {
		state.Defense[id] = counts[index]
	}
	return state, nil
}

func (r FleetRepository) updateMissilePlanetDefense(ctx context.Context, planetsTable string, planetID int, defense domaingame.DefenseCounts, at int64, activity bool) error {
	ids := domaingame.DefenseIDs()
	set := make([]string, 0, len(ids)+1)
	args := make([]any, 0, len(ids)+2)
	for _, id := range ids {
		set = append(set, fmt.Sprintf("`%d` = ?", id))
		args = append(args, max(0, defense[id]))
	}
	if activity {
		set = append(set, "lastakt = ?")
		args = append(args, at)
	}
	args = append(args, planetID)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE planet_id = ? LIMIT 1", planetsTable, strings.Join(set, ", ")), args...)
	return err
}

func (r FleetRepository) insertMissileReports(ctx context.Context, messagesTable string, origin missilePlanetState, target missilePlanetState, defendingPlanet missilePlanetState, moonAttack bool, amount int, intercepted int, at int64) error {
	defenseText := missileDefenseReport(target.Language, target.Defense, defendingPlanet.Defense, moonAttack)
	defenderLocale := missileLocaleFor(target.Language)
	defenderText := fmt.Sprintf(defenderLocale.DefenderStart, amount) + " " + missilePlanetLink(origin) + "  " + defenderLocale.DefenderHit + " " + missilePlanetLink(target) + " !<br>"
	if intercepted > 0 {
		defenderText += fmt.Sprintf(defenderLocale.Intercepted, intercepted) + "<br>:<br>"
	}
	defenderText += defenseText
	if err := r.insertMissileMessage(ctx, messagesTable, target.OwnerID, defenderLocale.From, defenderLocale.Subject, defenderText, at); err != nil {
		return err
	}

	attackerLocale := missileLocaleFor(origin.Language)
	attackerText := fmt.Sprintf(attackerLocale.AttackerStart, amount) + " " + missilePlanetLink(origin) + " " + attackerLocale.AttackerHit + " " + missilePlanetLink(target) + " !<br>"
	attackerText += missileDefenseReport(origin.Language, target.Defense, defendingPlanet.Defense, moonAttack)
	return r.insertMissileMessage(ctx, messagesTable, origin.OwnerID, attackerLocale.From, attackerLocale.Subject, attackerText, at)
}

func (r FleetRepository) insertMissileMessage(ctx context.Context, messagesTable string, ownerID int, from string, subject string, text string, at int64) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, 0)", messagesTable), ownerID, domaingame.MessageTypeBattleReportLink, from, subject, text, at)
	return err
}

func missilePlanetLink(planet missilePlanetState) string {
	return fmt.Sprintf("%s <a href=# onclick=showGalaxy(%d,%d,%d); >[%d:%d:%d]</a>", planet.Name, planet.Galaxy, planet.System, planet.Position, planet.Galaxy, planet.System, planet.Position)
}

func missileDefenseReport(language string, defense domaingame.DefenseCounts, defendingPlanet domaingame.DefenseCounts, moonAttack bool) string {
	locale := missileLocaleFor(language)
	text := "<table width=400><tr><td class=c colspan=4>" + locale.Title + "</td></tr>"
	shown := 0
	ids := domaingame.DefenseIDs()
	for index := len(ids) - 1; index >= 0; index-- {
		id := ids[index]
		if shown%2 == 0 {
			text += "</tr>"
		}
		count := defense[id]
		if count <= 0 {
			continue
		}
		if moonAttack && id == domaingame.DefenseAntiBallisticMissile {
			count = defendingPlanet[id]
		}
		text += fmt.Sprintf("<td>%s</td><td>%s</td>", missileDefenseName(language, id), fleetLegacyNumber(float64(count)))
		shown++
	}
	return text + "</table><br>\n"
}

type missileLocale struct {
	From          string
	Subject       string
	Title         string
	DefenderStart string
	DefenderHit   string
	Intercepted   string
	AttackerStart string
	AttackerHit   string
}

func missileLocaleFor(language string) missileLocale {
	english := missileLocale{"Fleet Command", "Missile attack", "Defeated Defense", "%d missile out of the total number of missiles fired from the planet", "managed to get to your planet", "%d missile(s) was destroyed by your interceptor missiles.", "%d missile(s) from your planet.", "hit the planet"}
	switch language {
	case "de":
		return missileLocale{"Flottenkommando", "Raketenangriff", "Besiegte Verteidigung", "%d Rakete von der Gesamtzahl der vom Planeten abgefeuerten Raketen", "es geschafft, auf euren Planeten zu gelangen", "%d Rakete(n) wurde durch Ihre Abfangraketen zerstört.", "%d Rakete(n) von Ihrem Planeten.", "den Planeten treffen"}
	case "fr", "ru":
		return missileLocale{"Командование флотом", "Ракетная атака", "Поражённая оборона", "%d ракетам из общего числа выпущенных ракет с планеты", "удалось попасть на Вашу планету", "%d ракет(-ы) было уничтожено Вашими ракетами-перехватчиками", "%d ракет(ы) с Вашей планеты", "ударили по планете"}
	case "it":
		return missileLocale{"Comando della flotta", "Attacco missilistico", "Difese rimaste", "%d missili interplanetari partiti dal pianeta", "hanno colpito il tuo pianeta", "%d missili interplanetari sono stati distrutti dai tuoi missili anti-balistici", "%d missili dal vostro pianeta.", "colpire il pianeta"}
	default:
		return english
	}
}

func missileDefenseName(language string, id int) string {
	names := map[int]string{401: "Rocket Launcher", 402: "Light Laser", 403: "Heavy Laser", 404: "Gauss Cannon", 405: "Ion Cannon", 406: "Plasma Turret", 407: "Small Shield Dome", 408: "Large Shield Dome", 502: "Anti-Ballistic Missiles", 503: "Interplanetary Missiles"}
	switch language {
	case "de":
		names = map[int]string{401: "Raketenwerfer", 402: "Leichtes Lasergeschütz", 403: "Schweres Lasergeschütz", 404: "Gaußkanone", 405: "Ionengeschütz", 406: "Plasmawerfer", 407: "Kleine Schildkuppel", 408: "Große Schildkuppel", 502: "Abfangrakete", 503: "Interplanetarrakete"}
	case "fr":
		names = map[int]string{401: "Lanceur de Missile", 402: "Laser Léger", 403: "Laser Lourd", 404: "Canon de Gauss", 405: "Canon à Ion", 406: "Tourelle Plasma", 407: "Petit Dome", 408: "Grand Dome", 502: "Missiles anti-balistiques", 503: "Missiles interplanétaires"}
	case "it":
		names = map[int]string{401: "Lanciamissili", 402: "Laser leggero", 403: "Laser pesante", 404: "Cannone gauss", 405: "Cannone ionico", 406: "Cannone al plasma", 407: "Cupola scudo", 408: "Cupola scudo potenziata", 502: "Missili anti-balistici", 503: "Missili interplanetari"}
	case "ru":
		names = map[int]string{401: "Ракетная установка", 402: "Лёгкий лазер", 403: "Тяжёлый лазер", 404: "Пушка Гаусса", 405: "Ионное орудие", 406: "Плазменное орудие", 407: "Малый щитовой купол", 408: "Большой щитовой купол", 502: "Ракета-перехватчик", 503: "Межпланетная ракета"}
	}
	return names[id]
}
