package mysqlgame

import (
	"context"
	"errors"
	"fmt"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type colonizationTarget struct {
	Type     int
	Galaxy   int
	System   int
	Position int
	Language string
}

func (r FleetRepository) finishColonizationArrival(ctx context.Context, fleetTable string, fleetLogsTable string, queueTable string, planetsTable string, usersTable string, messagesTable string, task fleetQueueTask, fleet recallFleetRow) error {
	target, found, err := r.loadColonizationTarget(ctx, planetsTable, usersTable, fleet)
	if err != nil {
		return err
	}
	if !found || target.Type != legacyPlanetTypeColony {
		return errors.New("colonization target unavailable")
	}
	coordinates := domaingame.Coordinates{Galaxy: target.Galaxy, System: target.System, Position: target.Position}
	occupied, err := r.fleetLaunchColonizeOccupied(ctx, planetsTable, coordinates)
	if err != nil {
		return err
	}

	returning := fleet
	returnTargetID := fleet.TargetPlanetID
	messageState := "fail"
	if !occupied {
		planetCount, err := r.loadColonizedPlanetCount(ctx, planetsTable, fleet.OwnerID)
		if err != nil {
			return err
		}
		if planetCount >= domaingame.MaxColonizedPlanets {
			returnTargetID, err = r.createAbandonedColony(ctx, planetsTable, coordinates, task.End)
			if err != nil {
				return err
			}
			messageState = "max"
		} else {
			returnTargetID, err = r.createFleetColony(ctx, planetsTable, fleet.OwnerID, coordinates, target.Language, task.End)
			if err != nil {
				return err
			}
			messageState = "success"
			if returning.Ships[domaingame.FleetColonyShip] > 0 {
				returning.Ships = copyFleetCounts(returning.Ships)
				returning.Ships[domaingame.FleetColonyShip]--
				colonyShipScore := domaingame.CalculatePlanetScore(nil, domaingame.FleetCounts{domaingame.FleetColonyShip: 1}, nil)
				overview := OverviewRepository{execer: r.execer}
				if err := overview.adjustStats(ctx, usersTable, fleet.OwnerID, colonyShipScore); err != nil {
					return err
				}
				if err := overview.recalcRanks(ctx, usersTable); err != nil {
					return err
				}
			}
		}
		if err := r.deleteColonizationPhantom(ctx, planetsTable, fleet.TargetPlanetID); err != nil {
			return err
		}
	}

	if fleetCountTotal(returning.Ships) > 0 {
		returning.TargetPlanetID = returnTargetID
		returnFleetID, err := r.insertRecallFleet(ctx, fleetTable, fleet.OwnerID, returning, domaingame.FleetMissionColonize+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime))
		if err != nil {
			return err
		}
		if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, returnFleetID, domaingame.FleetMissionColonize+domaingame.FleetMissionReturnOffset, task.End, int64(fleet.FlightTime)); err != nil {
			return err
		}
		contextFleet := fleet
		contextFleet.TargetPlanetID = returnTargetID
		value, found, err := r.loadFleetMessageContext(ctx, usersTable, planetsTable, contextFleet)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("colonization return context unavailable")
		}
		if err := r.insertFleetTransitionLog(ctx, fleetLogsTable, value, returning, domaingame.FleetMissionColonize+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime), 0, task.End); err != nil {
			return err
		}
	}
	if err := r.insertColonizationMessage(ctx, messagesTable, fleet.OwnerID, target, messageState, task.End); err != nil {
		return err
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) loadColonizationTarget(ctx context.Context, planetsTable string, usersTable string, fleet recallFleetRow) (colonizationTarget, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT p.type, p.g, p.s, p.p, COALESCE(u.lang, 'en') FROM %s p JOIN %s u ON u.player_id = ? WHERE p.planet_id = ? LIMIT 1", planetsTable, usersTable), fleet.OwnerID, fleet.TargetPlanetID)
	if err != nil {
		return colonizationTarget{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return colonizationTarget{}, false, err
		}
		return colonizationTarget{}, false, nil
	}
	var target colonizationTarget
	if err := rows.Scan(&target.Type, &target.Galaxy, &target.System, &target.Position, &target.Language); err != nil {
		return colonizationTarget{}, false, err
	}
	return target, true, rows.Err()
}

func (r FleetRepository) loadColonizedPlanetCount(ctx context.Context, planetsTable string, ownerID int) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ? AND type = ?", planetsTable), ownerID, domaingame.PlanetTypePlanet)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, errors.New("colonized planet count unavailable")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, rows.Err()
}

func (r FleetRepository) loadColonySettings(ctx context.Context, table string) (domaingame.ColonySettings, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT t1_a,t1_b,t1_c,t2_a,t2_b,t2_c,t3_a,t3_b,t3_c,t4_a,t4_b,t4_c,t5_a,t5_b,t5_c FROM %s LIMIT 1", table))
	if err != nil {
		return domaingame.ColonySettings{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.ColonySettings{}, err
		}
		return domaingame.ColonySettings{}, errors.New("colonization settings unavailable")
	}
	var settings domaingame.ColonySettings
	destinations := make([]any, 0, 15)
	for index := range settings.Tiers {
		destinations = append(destinations, &settings.Tiers[index].Minimum, &settings.Tiers[index].Maximum, &settings.Tiers[index].Factor)
	}
	if err := rows.Scan(destinations...); err != nil {
		return domaingame.ColonySettings{}, err
	}
	return settings, rows.Err()
}

func (r FleetRepository) createFleetColony(ctx context.Context, planetsTable string, ownerID int, coordinates domaingame.Coordinates, language string, at int64) (int, error) {
	if r.combatRandom == nil {
		return 0, errors.New("colonization random source unavailable")
	}
	colonyTable, err := tableName(r.prefix, "coltab")
	if err != nil {
		return 0, err
	}
	settings, err := r.loadColonySettings(ctx, colonyTable)
	if err != nil {
		return 0, err
	}
	tier := settings.Tiers[domaingame.ColonyTierIndex(coordinates.Position)]
	span := max(1, tier.Maximum-tier.Minimum+1)
	diameter := domaingame.ColonyDiameter(settings, coordinates.Position, tier.Minimum+r.combatRandom(span))
	temperature := domaingame.ColonyTemperature(coordinates.Position, r.combatRandom(10))
	maxFields := domaingame.ColonyMaxFields(diameter)
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,`%d`,`%d`,`%d`,lastpeek,lastakt,gate_until,remove) VALUES (?,?,?,?,?,?,?,?,0,?,?,500,500,0,?,?,0,0)", planetsTable, resourceMetal, resourceCrystal, resourceDeuterium), colonyName(language), domaingame.PlanetTypePlanet, coordinates.Galaxy, coordinates.System, coordinates.Position, ownerID, diameter, temperature, maxFields, at, at, at)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("colonized planet id unavailable")
	}
	return int(id), nil
}

func (r FleetRepository) createAbandonedColony(ctx context.Context, planetsTable string, coordinates domaingame.Coordinates, at int64) (int, error) {
	result, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name,type,g,s,p,owner_id,diameter,temp,fields,maxfields,date,`%d`,`%d`,`%d`,lastpeek,lastakt,gate_until,remove) VALUES (?,?,?,?,?,?,0,0,0,0,?,0,0,0,?,?,0,?)", planetsTable, resourceMetal, resourceCrystal, resourceDeuterium), "Planet abandoned", legacyPlanetTypeAbandoned, coordinates.Galaxy, coordinates.System, coordinates.Position, userSpace, at, at, at, at+24*60*60)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("abandoned colony id unavailable")
	}
	return int(id), nil
}

func (r FleetRepository) deleteColonizationPhantom(ctx context.Context, planetsTable string, planetID int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE planet_id = ? AND type = ? LIMIT 1", planetsTable), planetID, legacyPlanetTypeColony)
	return err
}

func (r FleetRepository) insertColonizationMessage(ctx context.Context, messagesTable string, ownerID int, target colonizationTarget, state string, at int64) error {
	message := colonizationMessageFor(target.Language)
	text := fmt.Sprintf(message.Prefix, fleetGalaxyLink(target.Galaxy, target.System, target.Position))
	switch state {
	case "success":
		text += message.Success
	case "max":
		text += message.Maximum
	default:
		text += message.Failure
	}
	return r.insertFleetMessage(ctx, messagesTable, ownerID, message.From, message.Subject, text, at)
}

type colonizationMessage struct {
	From    string
	Subject string
	Prefix  string
	Maximum string
	Success string
	Failure string
}

func colonizationMessageFor(language string) colonizationMessage {
	english := colonizationMessage{
		From: "Settlers", Subject: "Settlers' report",
		Prefix:  "\nFleet reaches set coordinates\n%s\n",
		Maximum: ", and establishes that this planet is suitable for colonization. Shortly after the planet\\'s exploration begins, there is a report of unrest on the main planet, as the empire becomes too large and the people move back.\n",
		Success: ", finds a new planet there and immediately begins to explore it.\n",
		Failure: ", but finds no planet suitable for colonization. The settlers return in a depressed state.\n",
	}
	switch language {
	case "de":
		return colonizationMessage{
			From: "Siedler", Subject: "Bericht der Siedler",
			Prefix:  "\nDie Flotte erreicht die angegebenen Koordinaten\n%s\n",
			Maximum: ", und stellt fest, dass dieser Planet für eine Kolonisierung geeignet ist. Kurz nach Beginn der Erkundung des Planeten wird von Unruhen auf dem Hauptplaneten berichtet, da das Imperium zu groß wird und sich die Menschen zurückziehen.\n",
			Success: ", findet dort einen neuen Planeten und beginnt sofort, ihn zu erforschen.\n",
			Failure: ", findet aber keinen für die Kolonisierung geeigneten Planeten. Die Siedler kehren deprimiert zurück.\n",
		}
	case "fr", "ru":
		return colonizationMessage{
			From: "Поселенцы", Subject: "Доклад поселенцев",
			Prefix:  "\nФлот достигает заданных координат\n%s\n",
			Maximum: ", и устанавливает, что эта планета пригодна для колонизации. Вскоре после начала освоения планеты поступает сообщение о беспорядках на главной планете, так как империя становится слишком большой и люди возвращаются обратно.\n",
			Success: ", находит там новую планету и сразу же начинает её освоение.\n",
			Failure: ", но не находит там пригодной для колонизации планеты. В подавленном состоянии поселенцы возвращаются обратно.\n",
		}
	case "it":
		return colonizationMessage{
			From: "Comando della flotta", Subject: "Rapporto colonizzazione",
			Prefix:  "\nLa tua flotta arriva alle coordinate\n%s\n",
			Maximum: ", ma non &egrave; stato possibile colonizzare. Hai gi&agrave; raggiunto il limite massimo di colonie.\n",
			Success: "e i coloni preparano subito il terreno per costruire.\n",
			Failure: ", ma non &egrave; stato possibile completare la missione, la posizione &egrave; gi&agrave; occupata. I coloni ritornano alla base.\n",
		}
	default:
		return english
	}
}

func fleetCountTotal(counts domaingame.FleetCounts) int {
	total := 0
	for _, count := range counts {
		total += max(0, count)
	}
	return total
}

func colonyName(language string) string {
	switch language {
	case "de":
		return "Kolonie"
	case "fr":
		return "Colonie"
	case "es", "it":
		return "Colonia"
	case "ru":
		return "\u041a\u043e\u043b\u043e\u043d\u0438\u044f"
	case "jp":
		return "\u30b3\u30ed\u30cb\u30fc"
	default:
		return "Colony"
	}
}
