package game

import "testing"

func TestJumpGateMoveIssueMatchesLegacyValidationOrder(t *testing.T) {
	source := JumpGateMoon{
		ID:        10,
		OwnerID:   42,
		Type:      PlanetTypeMoon,
		GateLevel: 1,
		GateUntil: 0,
		Ships:     FleetCounts{FleetSmallCargo: 2},
	}
	target := JumpGateMoon{
		ID:        20,
		OwnerID:   42,
		Type:      PlanetTypeMoon,
		GateLevel: 1,
		GateUntil: 0,
		Ships:     FleetCounts{},
	}

	if issue := JumpGateMoveIssue(42, JumpGateMoon{Type: PlanetTypePlanet}, true, target, true, map[int]int{FleetSmallCargo: 1}, 100); issue == nil || issue.Code != JumpGateIssueSourceMoonMissing {
		t.Fatalf("expected source moon issue, got %+v", issue)
	}
	if issue := JumpGateMoveIssue(42, source, true, JumpGateMoon{Type: PlanetTypePlanet}, true, map[int]int{FleetSmallCargo: 1}, 100); issue == nil || issue.Code != JumpGateIssueTargetMoonMissing {
		t.Fatalf("expected target moon issue, got %+v", issue)
	}
	sourceNoGate := source
	sourceNoGate.GateLevel = 0
	if issue := JumpGateMoveIssue(42, sourceNoGate, true, target, true, map[int]int{FleetSmallCargo: 1}, 100); issue == nil || issue.Code != JumpGateIssueSourceGateMissing {
		t.Fatalf("expected source gate issue, got %+v", issue)
	}
	targetNoGate := target
	targetNoGate.GateLevel = 0
	if issue := JumpGateMoveIssue(42, source, true, targetNoGate, true, map[int]int{FleetSmallCargo: 1}, 100); issue == nil || issue.Code != JumpGateIssueTargetGateMissing {
		t.Fatalf("expected target gate issue, got %+v", issue)
	}
	foreign := target
	foreign.OwnerID = 77
	if issue := JumpGateMoveIssue(42, source, true, foreign, true, map[int]int{FleetSmallCargo: 1}, 100); issue == nil || issue.Code != JumpGateIssueForeignMoon {
		t.Fatalf("expected foreign moon issue, got %+v", issue)
	}
	same := source
	if issue := JumpGateMoveIssue(42, source, true, same, true, map[int]int{FleetSmallCargo: 1}, 100); issue == nil || issue.Code != JumpGateIssueTargetMoonMissing {
		t.Fatalf("expected same-moon target issue, got %+v", issue)
	}
	cooling := target
	cooling.GateUntil = 200
	if issue := JumpGateMoveIssue(42, source, true, cooling, true, map[int]int{FleetSmallCargo: 1}, 100); issue == nil || issue.Code != JumpGateIssueCooldown {
		t.Fatalf("expected cooldown issue, got %+v", issue)
	}
	if issue := JumpGateMoveIssue(42, source, true, target, true, map[int]int{FleetSolarSatellite: 5}, 100); issue == nil || issue.Code != JumpGateIssueNoShips {
		t.Fatalf("expected satellite-only no-ships issue, got %+v", issue)
	}
	if issue := JumpGateMoveIssue(42, source, true, target, true, map[int]int{FleetSmallCargo: 3}, 100); issue == nil || issue.Code != JumpGateIssueNotEnoughShips {
		t.Fatalf("expected not-enough issue, got %+v", issue)
	}
	if issue := JumpGateMoveIssue(42, source, true, target, true, map[int]int{FleetSmallCargo: -2}, 100); issue != nil {
		t.Fatalf("expected negative selection to normalize like legacy abs(int), got %+v", issue)
	}
}

func TestJumpGateCooldownUsesFleetSpeed(t *testing.T) {
	if got := JumpGateCooldownUntil(1_000, 1); got != 4_599 {
		t.Fatalf("expected 1x cooldown at 4599, got %d", got)
	}
	if got := JumpGateCooldownUntil(1_000, 128); got != 1_027 {
		t.Fatalf("expected 128x cooldown at 1027, got %d", got)
	}
	if got := JumpGateCooldownUntil(1_000, 0); got != 4_599 {
		t.Fatalf("expected non-positive speed to default, got %d", got)
	}
	if got := JumpGateCooldownUntil(1_000, 10_000); got != 1_000 {
		t.Fatalf("expected extreme speed to clamp cooldown at now, got %d", got)
	}
	if got := JumpGateCooldownMessage(65); got != "The Jump Gate is in recharge mode!<br>The Gate will be fully recharged for the next jump in 01min 05sec." {
		t.Fatalf("unexpected cooldown message: %s", got)
	}
	if got := JumpGateCooldownMessage(-1); got != "The Jump Gate is in recharge mode!<br>The Gate will be fully recharged for the next jump in 00min 00sec." {
		t.Fatalf("unexpected negative cooldown message: %s", got)
	}
}
