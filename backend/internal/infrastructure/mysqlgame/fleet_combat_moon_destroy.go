package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type moonDestructionResolution struct {
	Code           int
	ReturnTargetID int
}

type moonDestructionTarget struct {
	Type     int
	Diameter int
}

func (r FleetRepository) finishMoonDestruction(
	ctx context.Context,
	fleetTable string,
	queueTable string,
	planetsTable string,
	usersTable string,
	messagesTable string,
	task fleetQueueTask,
	fleet recallFleetRow,
	value fleetMessageContext,
	survivors domaingame.FleetCounts,
	attackerWon bool,
) (moonDestructionResolution, error) {
	if fleet.Mission != domaingame.FleetMissionDestroy || !attackerWon || survivors[domaingame.FleetDeathstar] <= 0 {
		return moonDestructionResolution{}, nil
	}
	if r.combatRandom == nil {
		return moonDestructionResolution{}, errors.New("moon destruction random source unavailable")
	}
	target, found, err := r.loadMoonDestructionTarget(ctx, planetsTable, fleet.TargetPlanetID)
	if err != nil {
		return moonDestructionResolution{}, err
	}
	if !found {
		return moonDestructionResolution{}, nil
	}
	if target.Type != domaingame.PlanetTypeMoon && target.Type != domaingame.PlanetTypeDestroyedMoon {
		return moonDestructionResolution{}, errors.New("only moons can be destroyed")
	}

	odds := domaingame.MoonDestructionChances(target.Diameter, survivors[domaingame.FleetDeathstar])
	code := domaingame.ResolveMoonDestruction(odds, 1+r.combatRandom(999), 1+r.combatRandom(999))
	resolution := moonDestructionResolution{Code: code}
	if code&domaingame.MoonDestroyMoon != 0 {
		resolution.ReturnTargetID, err = r.destroyBattleMoon(ctx, fleetTable, queueTable, planetsTable, usersTable, fleet.TargetPlanetID, fleet.ID, value, task.End)
		if err != nil {
			return moonDestructionResolution{}, err
		}
	}
	if code&domaingame.MoonDestroyFleet != 0 {
		score := domaingame.CalculatePlanetScore(nil, fleet.Ships, nil)
		if err := (OverviewRepository{execer: r.execer}).adjustStats(ctx, usersTable, fleet.OwnerID, score); err != nil {
			return moonDestructionResolution{}, err
		}
		if err := (OverviewRepository{execer: r.execer}).recalcRanks(ctx, usersTable); err != nil {
			return moonDestructionResolution{}, err
		}
	}
	attackerText, defenderText := moonDestructionMessages(code, value, odds)
	attackerText = strings.ReplaceAll(attackerText, "'", "\\'")
	defenderText = strings.ReplaceAll(defenderText, "'", "\\'")
	if err := r.insertFleetMessage(ctx, messagesTable, value.OriginOwnerID, "Fleet Command", "Moon attack", attackerText, task.End); err != nil {
		return moonDestructionResolution{}, err
	}
	if err := r.insertFleetMessage(ctx, messagesTable, value.TargetOwnerID, "Fleet Command", "Moon quakes", defenderText, task.End); err != nil {
		return moonDestructionResolution{}, err
	}
	return resolution, nil
}

func (r FleetRepository) loadMoonDestructionTarget(ctx context.Context, planetsTable string, planetID int) (moonDestructionTarget, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT type, diameter FROM %s WHERE planet_id = ? LIMIT 1", planetsTable), planetID)
	if err != nil {
		return moonDestructionTarget{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return moonDestructionTarget{}, false, err
		}
		return moonDestructionTarget{}, false, nil
	}
	var target moonDestructionTarget
	if err := rows.Scan(&target.Type, &target.Diameter); err != nil {
		return moonDestructionTarget{}, false, err
	}
	if err := rows.Err(); err != nil {
		return moonDestructionTarget{}, false, err
	}
	return target, true, nil
}

func (r FleetRepository) destroyBattleMoon(
	ctx context.Context,
	fleetTable string,
	queueTable string,
	planetsTable string,
	usersTable string,
	moonID int,
	destroyingFleetID int,
	value fleetMessageContext,
	when int64,
) (int, error) {
	overview := OverviewRepository{queryer: r.queryer, execer: r.execer}
	score, err := overview.loadPlanetScore(ctx, planetsTable, moonID)
	if err != nil {
		return 0, err
	}
	planetID, ownerID, found, err := r.loadMoonUnderlyingPlanet(ctx, planetsTable, value)
	if err != nil || !found {
		return 0, err
	}
	foreignFleetIDs, err := r.loadMoonForeignFleetIDs(ctx, fleetTable, moonID, destroyingFleetID, ownerID)
	if err != nil {
		return 0, err
	}
	for _, fleetID := range foreignFleetIDs {
		recaller := r
		recaller.now = func() time.Time { return time.Unix(when, 0) }
		if err := recaller.recallFleet(ctx, fleetID, recaller.loadRecallFleetAnyOwner, 0); err != nil {
			return 0, err
		}
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET start_planet = ? WHERE start_planet = ?", fleetTable), planetID, moonID); err != nil {
		return 0, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET target_planet = ? WHERE target_planet = ?", fleetTable), planetID, moonID); err != nil {
		return 0, err
	}
	if err := overview.applyPlanetScoreRemoval(ctx, usersTable, score); err != nil {
		return 0, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return 0, err
	}
	if err := overview.flushPlanetQueue(ctx, queueTable, buildQueueTable, moonID); err != nil {
		return 0, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE planet_id = ? LIMIT 1", planetsTable), moonID); err != nil {
		return 0, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET aktplanet = ? WHERE player_id = ? LIMIT 1", usersTable), planetID, ownerID); err != nil {
		return 0, err
	}
	return planetID, nil
}

func (r FleetRepository) loadMoonUnderlyingPlanet(ctx context.Context, planetsTable string, value fleetMessageContext) (int, int, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT planet_id, owner_id FROM %s WHERE g = ? AND s = ? AND p = ? AND type = ? LIMIT 1", planetsTable), value.TargetGalaxy, value.TargetSystem, value.TargetPosition, domaingame.PlanetTypePlanet)
	if err != nil {
		return 0, 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, false, err
		}
		return 0, 0, false, nil
	}
	var planetID, ownerID int
	if err := rows.Scan(&planetID, &ownerID); err != nil {
		return 0, 0, false, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, false, err
	}
	return planetID, ownerID, true, nil
}

func (r FleetRepository) loadMoonForeignFleetIDs(ctx context.Context, fleetTable string, moonID int, destroyingFleetID int, ownerID int) ([]int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT fleet_id FROM %s WHERE owner_id <> ? AND target_planet = ? AND fleet_id <> ? ORDER BY fleet_id", fleetTable), ownerID, moonID, destroyingFleetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int{}
	for rows.Next() {
		var fleetID int
		if err := rows.Scan(&fleetID); err != nil {
			return nil, err
		}
		ids = append(ids, fleetID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func moonDestructionMessages(code int, value fleetMessageContext, odds domaingame.MoonDestructionOdds) (string, string) {
	origin := fmt.Sprintf("[%d:%d:%d]", value.OriginGalaxy, value.OriginSystem, value.OriginPosition)
	target := fmt.Sprintf("[%d:%d:%d]", value.TargetGalaxy, value.TargetSystem, value.TargetPosition)
	moonChance := int(math.Floor(odds.MoonPercent))
	fleetChance := int(math.Floor(odds.FleetPercent))
	switch code {
	case domaingame.MoonDestroyMoon:
		return fmt.Sprintf("The fleet from planet %s %s reaches the planet's moon at %s .\nThe death star weapons shoot a series of graviton charges at the moon, which cause a powerful concussion and destroy the satellite. All structures on the moon are destroyed. A complete success. The fleet returns to their home planet to drink to the occasion.\n<br>Chance of destroying the moon: %d %%. Chance to destroy the death star:%d %%", value.OriginName, origin, target, moonChance, fleetChance),
			fmt.Sprintf("The fleet from planet %s %s reaches your planet's moon at %s.\nAn ever-increasing vibration shakes this satellite. The moon begins to warp and eventually explodes into millions of pieces. It was a heavy blow to your empire. The enemy fleet is turning back.\n<br>Chance of destroying the moon: %d %%. Chance to destroy the death star:%d %%", value.OriginName, origin, target, moonChance, fleetChance)
	case domaingame.MoonDestroyFleet:
		return fmt.Sprintf("The fleet from planet %s %s reaches the planet's moon at %s . The Death Star aims its graviton cannon at the satellite. A slight vibration shakes the moon's surface. But something's not right. The graviton cannon is causing the Death Star to vibrate. It's starting to recoil. The Death Star explodes into millions of pieces. The resulting shockwave destroys your entire fleet. I've had enough...\n<br>Chance of destroying the moon: %d %%. Chance to destroy the death star:%d %%", value.OriginName, origin, target, moonChance, fleetChance),
			fmt.Sprintf("Fleet from planet %s %s reaches your planet's moon at %s.\nThe slight tremors on your moon indicate a failed attack on the lunar structure. Suddenly they stop. A giant explosion shakes space. The attacking fleet disappears from radar screens. It's a mess...\n<br>Chance of destroying the moon: %d %%. Chance to destroy the death star:%d %%", value.OriginName, origin, target, moonChance, fleetChance)
	case domaingame.MoonDestroyMoon | domaingame.MoonDestroyFleet:
		return fmt.Sprintf("The fleet from planet %s %s reaches the moon orbiting planet %s . Your Death Star is aiming its graviton cannon at the satellite. The tremors on the moon's surface are increasing. The moon begins to deform and rupture. Giant pieces of debris are flying at your fleet. It's too late to retreat. Your entire fleet is destroyed in a hail of debris. What a bummer...\n<br>Chance of destroying the moon: %d %%. Chance to destroy the death star: %d%%.", value.OriginName, origin, target, moonChance, fleetChance),
			fmt.Sprintf("Fleet from planet %s %s reaches your planet's moon at %s.\nIncreasing tremors shake the satellite. The moon begins to deform and eventually breaks into millions of pieces. Suddenly, the enemy fleet disappears from your radar screens. Something's wrong there, they must have gotten nailed by the debris...\n<br>Chance of destroying the moon: %d %%. Chance to destroy the death star:%d %%.", value.OriginName, origin, target, moonChance, fleetChance)
	default:
		return fmt.Sprintf("The fleet from %s %s reaches the planet's moon at %s .\nThe moon structure wasn't weakened enough, the fleet is heading back.\n<br>Chance of destroying the moon: %d %%. Chance to destroy the death star:%d %%;", value.OriginName, origin, target, moonChance, fleetChance),
			fmt.Sprintf("The fleet from planet %s %s reaches your planet's moon at %s.\nA slight tremor on your moon indicates a failed attack on a lunar structure; the attacking fleet, having failed to complete the mission, returns back to %s %s.\n<br>Chance of destroying the moon: %d %%. Chance to destroy the death star:%d %%;", value.OriginName, origin, target, value.OriginName, origin, moonChance, fleetChance)
	}
}
