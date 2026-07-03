package game

import (
	"fmt"
	"math"
)

const (
	JumpGateIssueSourceMoonMissing = "source_moon_missing"
	JumpGateIssueTargetMoonMissing = "target_moon_missing"
	JumpGateIssueSourceGateMissing = "source_gate_missing"
	JumpGateIssueTargetGateMissing = "target_gate_missing"
	JumpGateIssueForeignMoon       = "foreign_moon"
	JumpGateIssueCooldown          = "cooldown"
	JumpGateIssueNoShips           = "no_ships"
	JumpGateIssueNotEnoughShips    = "not_enough_ships"
	JumpGateIssueMoved             = "moved"
)

type JumpGate struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Source         JumpGateMoon
	Targets        []JumpGateMoon
	Ships          []JumpGateShip
	ActionIssue    *JumpGateActionIssue
}

type JumpGateMoon struct {
	ID          int
	OwnerID     int
	Name        string
	Type        int
	Coordinates Coordinates
	GateLevel   int
	GateUntil   int64
	Ships       FleetCounts
}

type JumpGateShip struct {
	ID    int
	Name  string
	Count int
}

type JumpGateActionIssue struct {
	Code    string
	Message string
}

func BuildJumpGate(overview Overview, source JumpGateMoon, targets []JumpGateMoon, issue *JumpGateActionIssue) JumpGate {
	return JumpGate{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Source:         source,
		Targets:        targets,
		Ships:          JumpGateShips(source.Ships),
		ActionIssue:    issue,
	}
}

func JumpGateMobileFleetIDs() []int {
	ids := make([]int, 0, len(FleetIDs()))
	for _, id := range FleetIDs() {
		if id == FleetSolarSatellite {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func JumpGateShips(counts FleetCounts) []JumpGateShip {
	ships := make([]JumpGateShip, 0, len(JumpGateMobileFleetIDs()))
	for _, id := range JumpGateMobileFleetIDs() {
		count := counts[id]
		if count <= 0 {
			continue
		}
		ships = append(ships, JumpGateShip{ID: id, Name: fleetName(id), Count: count})
	}
	return ships
}

func NormalizeJumpGateSelection(selection map[int]int) map[int]int {
	normalized := make(map[int]int, len(selection))
	for _, id := range JumpGateMobileFleetIDs() {
		amount := selection[id]
		if amount < 0 {
			amount = -amount
		}
		if amount > 0 {
			normalized[id] = amount
		}
	}
	return normalized
}

func JumpGateMoveIssue(playerID int, source JumpGateMoon, sourceFound bool, target JumpGateMoon, targetFound bool, selection map[int]int, now int64) *JumpGateActionIssue {
	if !sourceFound || source.Type != PlanetTypeMoon {
		return jumpGateIssue(JumpGateIssueSourceMoonMissing, "no source moon selected")
	}
	if !targetFound || target.Type != PlanetTypeMoon {
		return jumpGateIssue(JumpGateIssueTargetMoonMissing, "no target moon selected")
	}
	if source.GateLevel <= 0 {
		return jumpGateIssue(JumpGateIssueSourceGateMissing, "no jump gate found at source moon")
	}
	if target.GateLevel <= 0 {
		return jumpGateIssue(JumpGateIssueTargetGateMissing, "no jump gate found at the target moon")
	}
	if source.OwnerID != playerID || target.OwnerID != playerID {
		return jumpGateIssue(JumpGateIssueForeignMoon, "either the source moon or target moon doesn't belong to you")
	}
	if source.ID == target.ID {
		return jumpGateIssue(JumpGateIssueTargetMoonMissing, "no target moon selected")
	}
	if now < source.GateUntil || now < target.GateUntil {
		left := source.GateUntil
		if target.GateUntil > left {
			left = target.GateUntil
		}
		return jumpGateIssue(JumpGateIssueCooldown, JumpGateCooldownMessage(left-now))
	}
	normalized := NormalizeJumpGateSelection(selection)
	if len(normalized) == 0 {
		return jumpGateIssue(JumpGateIssueNoShips, "no ships selected")
	}
	for id, amount := range normalized {
		if amount > source.Ships[id] {
			return jumpGateIssue(JumpGateIssueNotEnoughShips, "not enough ships available")
		}
	}
	return nil
}

func JumpGateCooldownUntil(now int64, fleetSpeed float64) int64 {
	if fleetSpeed <= 0 {
		fleetSpeed = 1
	}
	seconds := int64(math.Floor(3600/fleetSpeed)) - 1
	if seconds < 0 {
		seconds = 0
	}
	return now + seconds
}

func JumpGateCooldownMessage(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("The Jump Gate is in recharge mode!<br>The Gate will be fully recharged for the next jump in %02dmin %02dsec.", seconds/60, seconds%60)
}

func JumpGateMovedIssue() *JumpGateActionIssue {
	return jumpGateIssue(JumpGateIssueMoved, "Jump executed.")
}

func jumpGateIssue(code string, message string) *JumpGateActionIssue {
	return &JumpGateActionIssue{Code: code, Message: message}
}
