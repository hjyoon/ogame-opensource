package mysqlgame

import (
	"context"
	"errors"
	"fmt"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type combatFleetParticipant struct {
	Fleet   recallFleetRow
	Queue   recallQueueRow
	Context fleetMessageContext
	Weapon  int
	Shield  int
	Armour  int
}

func (r FleetRepository) hasHoldingCombatFleet(ctx context.Context, fleetTable string, planetID int) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT EXISTS(SELECT 1 FROM %s WHERE mission = ? AND target_planet = ?)", fleetTable,
	), domaingame.FleetMissionACSHold+domaingame.FleetMissionOrbitingOffset, planetID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, errors.New("holding fleet lookup unavailable")
	}
	var found int
	if err := rows.Scan(&found); err != nil {
		return false, err
	}
	return found != 0, rows.Err()
}

func holdingCombatFleetLimit(acs int) int {
	return max(0, acs*acs-1)
}

func (r FleetRepository) loadHoldingCombatParticipants(
	ctx context.Context,
	fleetTable string,
	queueTable string,
	planetsTable string,
	usersTable string,
	planetID int,
	acs int,
) ([]combatFleetParticipant, error) {
	limit := holdingCombatFleetLimit(acs)
	if limit == 0 {
		return nil, nil
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT fleet_id FROM %s WHERE mission = ? AND target_planet = ? ORDER BY fleet_id LIMIT ?", fleetTable,
	), domaingame.FleetMissionACSHold+domaingame.FleetMissionOrbitingOffset, planetID, limit)
	if err != nil {
		return nil, err
	}
	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	participants := make([]combatFleetParticipant, 0, len(ids))
	for _, id := range ids {
		participant, err := r.loadCombatFleetParticipant(ctx, fleetTable, queueTable, planetsTable, usersTable, id, "holding participant")
		if err != nil {
			return nil, err
		}
		participants = append(participants, participant)
	}
	return participants, nil
}

func (r FleetRepository) loadCombatFleetParticipant(
	ctx context.Context,
	fleetTable string,
	queueTable string,
	planetsTable string,
	usersTable string,
	fleetID int,
	role string,
) (combatFleetParticipant, error) {
	fleet, found, err := r.loadRecallFleetAnyOwner(ctx, fleetTable, fleetID)
	if err != nil {
		return combatFleetParticipant{}, err
	}
	if !found {
		return combatFleetParticipant{}, fmt.Errorf("%s fleet unavailable", role)
	}
	queue, found, err := r.loadRecallQueue(ctx, queueTable, fleetID)
	if err != nil {
		return combatFleetParticipant{}, err
	}
	if !found {
		return combatFleetParticipant{}, fmt.Errorf("%s queue unavailable", role)
	}
	value, found, err := r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
	if err != nil {
		return combatFleetParticipant{}, err
	}
	if !found {
		return combatFleetParticipant{}, fmt.Errorf("%s context unavailable", role)
	}
	weapon, shield, armour, found, err := r.loadCombatPlayerTechnology(ctx, usersTable, fleet.OwnerID)
	if err != nil {
		return combatFleetParticipant{}, err
	}
	if !found {
		return combatFleetParticipant{}, fmt.Errorf("%s technology unavailable", role)
	}
	return combatFleetParticipant{Fleet: fleet, Queue: queue, Context: value, Weapon: weapon, Shield: shield, Armour: armour}, nil
}

func combatDefenderSlots(planetID int, value fleetMessageContext, state unguardedAttackState, holding []combatFleetParticipant) []domaingame.CombatSlot {
	defenders := make([]domaingame.CombatSlot, 0, len(holding)+1)
	defenders = append(defenders, domaingame.CombatSlot{
		ObjectID: planetID, PlayerID: value.TargetOwnerID, Name: value.TargetOwnerName,
		Coords: domaingame.Coordinates{Galaxy: value.TargetGalaxy, System: value.TargetSystem, Position: value.TargetPosition},
		Planet: true, Weapon: state.DefenderWeapon, Shield: state.DefenderShield, Armour: state.DefenderArmour,
		Units: combatUnitCounts(state.DefenderUnits),
	})
	for _, participant := range holding {
		defenders = append(defenders, domaingame.CombatSlot{
			ObjectID: participant.Fleet.ID,
			PlayerID: participant.Fleet.OwnerID,
			Name:     participant.Context.OriginOwnerName,
			Coords: domaingame.Coordinates{
				Galaxy: participant.Context.OriginGalaxy, System: participant.Context.OriginSystem, Position: participant.Context.OriginPosition,
			},
			Weapon: participant.Weapon, Shield: participant.Shield, Armour: participant.Armour,
			Units: combatUnitCounts(participant.Fleet.Ships),
		})
	}
	return defenders
}

func (r FleetRepository) writebackHoldingCombatFleets(ctx context.Context, fleetTable string, queueTable string, holding []combatFleetParticipant, survivors []map[int]int) error {
	for index, participant := range holding {
		units := map[int]int{}
		if index+1 < len(survivors) {
			units = survivors[index+1]
		}
		if combatFleetTotal(domaingame.FleetCounts(units)) == 0 {
			if err := r.removeCompletedFleetTask(ctx, fleetTable, queueTable, participant.Fleet.ID, participant.Queue.TaskID); err != nil {
				return err
			}
			continue
		}
		if err := r.setFleetCombatUnits(ctx, fleetTable, participant.Fleet.ID, units); err != nil {
			return err
		}
	}
	return nil
}

func (r FleetRepository) setFleetCombatUnits(ctx context.Context, fleetTable string, fleetID int, survivors map[int]int) error {
	ids := domaingame.FleetIDs()
	args := make([]any, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, max(0, survivors[id]))
	}
	args = append(args, fleetID)
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET %s WHERE fleet_id = ? LIMIT 1", fleetTable, numericAssignments(ids)), args...)
	return err
}

func combatDefenderOwnerIDs(value fleetMessageContext, holding []combatFleetParticipant) []int {
	owners := make([]int, 0, len(holding)+1)
	owners = append(owners, value.TargetOwnerID)
	for _, participant := range holding {
		owners = append(owners, participant.Fleet.OwnerID)
	}
	return owners
}

func (r FleetRepository) adjustCombatDefenderStats(ctx context.Context, usersTable string, value fleetMessageContext, holding []combatFleetParticipant, losses []domaingame.CombatParticipantLoss) error {
	owners := combatDefenderOwnerIDs(value, holding)
	for index, ownerID := range owners {
		if index >= len(losses) {
			break
		}
		if err := r.adjustCombatStats(ctx, usersTable, ownerID, losses[index]); err != nil {
			return err
		}
	}
	return nil
}
