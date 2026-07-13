package game

import "testing"

func TestBattlePlunderMatchesLegacyDistribution(t *testing.T) {
	for _, test := range []struct {
		name      string
		cargo     int
		resources Resources
		want      Resources
	}{
		{name: "balanced cargo", cargo: 900, resources: Resources{Metal: 1000, Crystal: 1000, Deuterium: 1000}, want: Resources{Metal: 300, Crystal: 300, Deuterium: 300}},
		{name: "half resource ceiling", cargo: 10000, resources: Resources{Metal: 1000, Crystal: 600, Deuterium: 200}, want: Resources{Metal: 500, Crystal: 300, Deuterium: 100}},
		{name: "redistributes unused deuterium share", cargo: 900, resources: Resources{Metal: 2000, Crystal: 2000, Deuterium: 0}, want: Resources{Metal: 450, Crystal: 450}},
		{name: "floors fractions", cargo: 10, resources: Resources{Metal: 100, Crystal: 100, Deuterium: 100}, want: Resources{Metal: 3, Crystal: 3, Deuterium: 3}},
		{name: "negative input", cargo: -1, resources: Resources{Metal: -1, Crystal: -2, Deuterium: -3}, want: Resources{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := BattlePlunder(test.cargo, test.resources)
			if got.Metal != test.want.Metal || got.Crystal != test.want.Crystal || got.Deuterium != test.want.Deuterium {
				t.Fatalf("BattlePlunder() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestFleetAvailableCargo(t *testing.T) {
	ships := FleetCounts{FleetSmallCargo: 2, FleetEspionageProbe: 1, 999999: 3}
	if got := FleetAvailableCargo(ships, Resources{Metal: 100, Crystal: 20, Deuterium: 5}, 10); got != 9870 {
		t.Fatalf("FleetAvailableCargo() = %d, want 9870", got)
	}
	if got := FleetAvailableCargo(ships, Resources{Metal: 20000}, 0); got != 0 {
		t.Fatalf("overloaded FleetAvailableCargo() = %d, want 0", got)
	}
}

func TestCombatUnitCatalogMatchesLegacyParameters(t *testing.T) {
	stats, ok := CombatStatsForUnit(FleetDeathstar)
	if !ok || stats.Structure != 9000000 || stats.Shield != 50000 || stats.Attack != 200000 || CombatShortName(FleetDeathstar) != "Deathstar" {
		t.Fatalf("unexpected deathstar combat catalog: %+v, %q", stats, CombatShortName(FleetDeathstar))
	}
	stats, ok = CombatStatsForUnit(DefensePlasmaTurret)
	if !ok || stats.Structure != 100000 || stats.Shield != 300 || stats.Attack != 3000 || CombatShortName(DefensePlasmaTurret) != "Plasma" {
		t.Fatalf("unexpected plasma combat catalog: %+v, %q", stats, CombatShortName(DefensePlasmaTurret))
	}
	if _, ok := CombatStatsForUnit(999999); ok || CombatShortName(999999) != "" {
		t.Fatal("unknown combat units must not resolve")
	}
}
