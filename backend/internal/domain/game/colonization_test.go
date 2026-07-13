package game

import "testing"

func TestColonizationRules(t *testing.T) {
	settings := ColonySettings{Tiers: [5]ColonyTier{
		{Minimum: 50, Maximum: 120, Factor: 72},
		{Minimum: 50, Maximum: 150, Factor: 120},
		{Minimum: 50, Maximum: 120, Factor: 120},
		{Minimum: 50, Maximum: 120, Factor: 96},
		{Minimum: 50, Maximum: 150, Factor: 96},
	}}
	for _, test := range []struct {
		position int
		index    int
		diameter int
		temp     int
	}{
		{position: 3, index: 0, diameter: 3_600, temp: 83},
		{position: 6, index: 1, diameter: 6_000, temp: 27},
		{position: 9, index: 2, diameter: 6_000, temp: 1},
		{position: 12, index: 3, diameter: 4_800, temp: -25},
		{position: 15, index: 4, diameter: 4_800, temp: -81},
	} {
		if index := ColonyTierIndex(test.position); index != test.index {
			t.Fatalf("position %d tier=%d, want %d", test.position, index, test.index)
		}
		if diameter := ColonyDiameter(settings, test.position, 50); diameter != test.diameter {
			t.Fatalf("position %d diameter=%d, want %d", test.position, diameter, test.diameter)
		}
		if temp := ColonyTemperature(test.position, 9); temp != test.temp {
			t.Fatalf("position %d temperature=%d, want %d", test.position, temp, test.temp)
		}
	}
	if diameter := ColonyDiameter(settings, 3, -1); diameter != 3_600 {
		t.Fatalf("minimum diameter clamp failed: %d", diameter)
	}
	if diameter := ColonyDiameter(settings, 15, 999); diameter != 14_400 {
		t.Fatalf("maximum diameter clamp failed: %d", diameter)
	}
	if fields := ColonyMaxFields(12_800); fields != 163 {
		t.Fatalf("max fields=%d, want 163", fields)
	}
	if ColonyMaxFields(-1) != 0 || MaxColonizedPlanets != 9 {
		t.Fatal("unexpected colonization boundary")
	}
}
