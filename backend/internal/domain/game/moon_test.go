package game

import "testing"

func TestBattleMoonCreationRules(t *testing.T) {
	for _, test := range []struct {
		debris Resources
		want   int
	}{
		{debris: Resources{}, want: 0},
		{debris: Resources{Metal: -1, Crystal: -1}, want: 0},
		{debris: Resources{Metal: 99_999}, want: 0},
		{debris: Resources{Metal: 100_000}, want: 1},
		{debris: Resources{Metal: 1_000_000, Crystal: 1_000_000}, want: 20},
		{debris: Resources{Metal: 9_000_000}, want: 20},
	} {
		if got := BattleMoonChance(test.debris); got != test.want {
			t.Fatalf("BattleMoonChance(%+v)=%d, want %d", test.debris, got, test.want)
		}
	}
	if got := BattleMoonDiameter(20, 10); got != 8366 {
		t.Fatalf("unexpected minimum 20%% moon diameter: %d", got)
	}
	if BattleMoonDiameter(-1, 0) != 3162 || BattleMoonDiameter(0, 99) != 4472 {
		t.Fatal("moon diameter inputs must use legacy size bounds")
	}
}

func TestMoonDestructionRules(t *testing.T) {
	odds := MoonDestructionChances(10_000, 1)
	if odds.MoonPercent != 0 || odds.FleetPercent != 50 {
		t.Fatalf("unexpected 10km moon odds: %+v", odds)
	}
	clamped := MoonDestructionChances(1, 100)
	if clamped.MoonPercent != 99.9 || clamped.FleetPercent != 0.5 {
		t.Fatalf("unexpected clamped odds: %+v", clamped)
	}
	zero := MoonDestructionChances(-1, -1)
	if zero.MoonPercent != 0 || zero.FleetPercent != 0 {
		t.Fatalf("negative inputs must clamp to zero: %+v", zero)
	}
	for _, test := range []struct {
		moonRoll  int
		fleetRoll int
		want      int
	}{
		{moonRoll: 999, fleetRoll: 999, want: 0},
		{moonRoll: 1, fleetRoll: 999, want: MoonDestroyMoon},
		{moonRoll: 999, fleetRoll: 1, want: MoonDestroyFleet},
		{moonRoll: 1, fleetRoll: 1, want: MoonDestroyMoon | MoonDestroyFleet},
		{moonRoll: 0, fleetRoll: 1000, want: 0},
	} {
		if got := ResolveMoonDestruction(clamped, test.moonRoll, test.fleetRoll); got != test.want {
			t.Fatalf("ResolveMoonDestruction(%d,%d)=%d, want %d", test.moonRoll, test.fleetRoll, got, test.want)
		}
	}
}
