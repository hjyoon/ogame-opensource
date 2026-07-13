package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"math"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type battleMoonCreation struct {
	Chance  int
	Created bool
}

type battleMoonTarget struct {
	Type     int
	Temp     int
	Language string
	HasMoon  bool
}

func (r FleetRepository) maybeCreateBattleMoon(ctx context.Context, planetsTable string, usersTable string, targetID int, value fleetMessageContext, debris domaingame.Resources) (battleMoonCreation, error) {
	chance := domaingame.BattleMoonChance(debris)
	if chance == 0 {
		return battleMoonCreation{}, nil
	}
	target, found, err := r.loadBattleMoonTarget(ctx, planetsTable, usersTable, targetID, value)
	if err != nil {
		return battleMoonCreation{}, err
	}
	if !found {
		return battleMoonCreation{}, errors.New("battle moon target unavailable")
	}
	if target.Type == domaingame.PlanetTypeMoon || target.Type == domaingame.PlanetTypeDestroyedMoon || target.HasMoon {
		return battleMoonCreation{}, nil
	}
	if r.combatRandom == nil {
		return battleMoonCreation{}, errors.New("battle moon random source unavailable")
	}
	result := battleMoonCreation{Chance: chance}
	if 1+r.combatRandom(100) > chance {
		return result, nil
	}
	diameter := domaingame.BattleMoonDiameter(chance, 10+r.combatRandom(11))
	_ = r.combatRandom(math.MaxInt32)
	temperature := target.Temp - (20 + r.combatRandom(11))
	at := r.now().Unix()
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (name, type, g, s, p, owner_id, diameter, temp, fields, maxfields, date, `%d`, `%d`, `%d`, lastpeek, lastakt, gate_until, remove) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 1, ?, 0, 0, 0, ?, ?, 0, 0)",
		planetsTable, resourceMetal, resourceCrystal, resourceDeuterium,
	), battleMoonName(target.Language), domaingame.PlanetTypeMoon, value.TargetGalaxy, value.TargetSystem, value.TargetPosition, value.TargetOwnerID, diameter, temperature, at, at, at)
	if err != nil {
		return battleMoonCreation{}, err
	}
	result.Created = true
	return result, nil
}

func (r FleetRepository) loadBattleMoonTarget(ctx context.Context, planetsTable string, usersTable string, targetID int, value fleetMessageContext) (battleMoonTarget, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT p.type, p.temp, COALESCE(u.lang, 'en'), EXISTS(SELECT 1 FROM %s m WHERE m.g = ? AND m.s = ? AND m.p = ? AND m.type IN (?, ?)) FROM %s p JOIN %s u ON u.player_id = p.owner_id WHERE p.planet_id = ? LIMIT 1",
		planetsTable, planetsTable, usersTable,
	), value.TargetGalaxy, value.TargetSystem, value.TargetPosition, domaingame.PlanetTypeMoon, domaingame.PlanetTypeDestroyedMoon, targetID)
	if err != nil {
		return battleMoonTarget{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return battleMoonTarget{}, false, err
		}
		return battleMoonTarget{}, false, nil
	}
	var target battleMoonTarget
	var hasMoon int
	if err := rows.Scan(&target.Type, &target.Temp, &target.Language, &hasMoon); err != nil {
		return battleMoonTarget{}, false, err
	}
	target.HasMoon = hasMoon != 0
	return target, true, rows.Err()
}

func battleMoonName(language string) string {
	switch language {
	case "de":
		return "Mond"
	case "fr":
		return "Lune"
	case "es", "it":
		return "Luna"
	case "ru":
		return "\u041b\u0443\u043d\u0430"
	case "jp":
		return "\u6708"
	default:
		return "Moon"
	}
}
