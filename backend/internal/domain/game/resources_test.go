package game

import (
	"errors"
	"math"
	"testing"
)

func TestNormalizeProductionSettingsMatchesLegacyResourcePost(t *testing.T) {
	settings, err := NormalizeProductionSettings(ProductionPercents{
		BuildingMetalMine:      -250,
		BuildingCrystalMine:    0,
		BuildingDeuteriumSynth: 35,
		BuildingSolarPlant:     100,
		9999:                   70,
	})
	if err != nil {
		t.Fatal(err)
	}

	if settings[BuildingMetalMine] != 0 || settings[BuildingCrystalMine] != 0 || settings[BuildingDeuteriumSynth] != 0.4 || settings[BuildingSolarPlant] != 1 {
		t.Fatalf("unexpected normalized settings: %+v", settings)
	}
	if _, ok := settings[9999]; ok {
		t.Fatalf("expected unsupported producer to be ignored: %+v", settings)
	}

	if _, err := NormalizeProductionSettings(ProductionPercents{BuildingMetalMine: 101}); !errors.Is(err, ErrProductionPercentTooHigh) {
		t.Fatalf("expected >100 percent error, got %v", err)
	}
}

func TestBuildResourceProductionUsesLegacyNaturalAndMineFormula(t *testing.T) {
	overview := resourceOverview(PlanetTypePlanet)
	production := BuildResourceProduction(overview, ResourceProductionInputs{
		Levels: BuildingLevels{
			BuildingMetalMine:  10,
			BuildingSolarPlant: 10,
		},
		ProductionFactors: ProductionFactors{
			BuildingMetalMine:  1,
			BuildingSolarPlant: 1,
		},
		UniverseSpeed: 2,
	})

	metalMine := findResourceRow(t, production, BuildingMetalMine)
	if production.Factor != 1 {
		t.Fatalf("expected full production factor, got %v", production.Factor)
	}
	if production.Natural.Metal != 40 || production.Natural.Crystal != 20 {
		t.Fatalf("unexpected natural production: %+v", production.Natural)
	}
	if metalMine.Percent != 100 || metalMine.Level != 10 {
		t.Fatalf("unexpected metal mine metadata: %+v", metalMine)
	}
	if metalMine.Values.Metal != 1556 || metalMine.Values.EnergyRaw != -260 || metalMine.Values.Energy != -260 {
		t.Fatalf("unexpected metal mine values: %+v", metalMine.Values)
	}
	if production.Totals.Hour.Metal != 1596 || production.Totals.Hour.Energy != 258 {
		t.Fatalf("unexpected hourly totals: %+v", production.Totals.Hour)
	}
	if production.Totals.Day.Metal != 38304 || production.Totals.Day.Energy != 258 {
		t.Fatalf("unexpected daily totals: %+v", production.Totals.Day)
	}
}

func TestBuildResourceProductionAppliesLegacyEnergyShortageFactor(t *testing.T) {
	overview := resourceOverview(PlanetTypePlanet)
	production := BuildResourceProduction(overview, ResourceProductionInputs{
		Levels:            BuildingLevels{BuildingMetalMine: 10},
		ProductionFactors: ProductionFactors{BuildingMetalMine: 1},
		UniverseSpeed:     1,
	})

	metalMine := findResourceRow(t, production, BuildingMetalMine)
	if production.Factor != 0 {
		t.Fatalf("expected production factor to drop to zero, got %v", production.Factor)
	}
	if metalMine.Values.Metal != 0 || metalMine.Values.Energy != 0 || metalMine.Values.EnergyRaw != -260 {
		t.Fatalf("expected row display to keep raw energy consumption, got %+v", metalMine.Values)
	}
	if production.Totals.Hour.Metal != 20 || production.Totals.Hour.Energy != -260 {
		t.Fatalf("unexpected shortage totals: %+v", production.Totals.Hour)
	}
}

func TestBuildResourceProductionAppliesPartialEnergyShortage(t *testing.T) {
	overview := resourceOverview(PlanetTypePlanet)
	production := BuildResourceProduction(overview, ResourceProductionInputs{
		Levels: BuildingLevels{
			BuildingMetalMine:  10,
			BuildingSolarPlant: 5,
		},
		ProductionFactors: ProductionFactors{
			BuildingMetalMine:  1,
			BuildingSolarPlant: 1,
		},
		UniverseSpeed: 1,
	})

	metalMine := findResourceRow(t, production, BuildingMetalMine)
	if production.Factor <= 0 || production.Factor >= 1 {
		t.Fatalf("expected partial production factor, got %v", production.Factor)
	}
	if !nearlyEqual(metalMine.Values.Metal, 481.76, 0.01) || !nearlyEqual(metalMine.Values.Energy, -161, 0.01) {
		t.Fatalf("unexpected partial shortage row: factor=%v row=%+v", production.Factor, metalMine.Values)
	}
}

func TestBuildResourceProductionHandlesCrystalAndMissingFactors(t *testing.T) {
	overview := resourceOverview(PlanetTypePlanet)
	production := BuildResourceProduction(overview, ResourceProductionInputs{
		Levels:        BuildingLevels{BuildingCrystalMine: 2},
		UniverseSpeed: 0,
	})

	crystalMine := findResourceRow(t, production, BuildingCrystalMine)
	if crystalMine.Percent != 0 || crystalMine.Values.Crystal != 0 || production.Natural.Metal != 20 {
		t.Fatalf("expected missing production factor and default speed handling, got row=%+v natural=%+v", crystalMine, production.Natural)
	}
}

func TestBuildResourceProductionIncludesPremiumFusionAndSatelliteOutput(t *testing.T) {
	overview := resourceOverview(PlanetTypePlanet)
	production := BuildResourceProduction(overview, ResourceProductionInputs{
		Levels: BuildingLevels{
			BuildingDeuteriumSynth: 1,
			BuildingFusionReactor:  5,
		},
		SolarSatellites: 3,
		ProductionFactors: ProductionFactors{
			BuildingDeuteriumSynth: 1,
			BuildingFusionReactor:  0.5,
			FleetSolarSatellite:    1,
		},
		EnergyResearch: 3,
		UniverseSpeed:  2,
		Geologist:      true,
		Engineer:       true,
	})

	deuteriumSynth := findResourceRow(t, production, BuildingDeuteriumSynth)
	fusion := findResourceRow(t, production, BuildingFusionReactor)
	satellite := findResourceRow(t, production, FleetSolarSatellite)
	if len(deuteriumSynth.BonusIcons) != 1 || deuteriumSynth.BonusIcons[0].Image != "geologe_ikon.gif" {
		t.Fatalf("expected geologist bonus icon on resource mines, got %+v", deuteriumSynth.BonusIcons)
	}
	if len(fusion.BonusIcons) != 1 || fusion.BonusIcons[0].Image != "ingenieur_ikon.gif" {
		t.Fatalf("expected engineer bonus icon on energy producers, got %+v", fusion.BonusIcons)
	}
	if len(satellite.BonusIcons) != 1 || satellite.BonusIcons[0].Image != "ingenieur_ikon.gif" {
		t.Fatalf("expected engineer bonus icon on solar satellites, got %+v", satellite.BonusIcons)
	}
	if !nearlyEqual(deuteriumSynth.Values.Deuterium, 28.1204, 0.0001) {
		t.Fatalf("unexpected deuterium output with geologist and temperature: %+v", deuteriumSynth.Values)
	}
	if fusion.Percent != 50 || fusion.Values.Deuterium != -41 {
		t.Fatalf("unexpected fusion consumption: %+v", fusion)
	}
	if satellite.Values.Energy != 112.2 || satellite.Values.EnergyRaw != 112.2 {
		t.Fatalf("unexpected satellite engineer bonus output: %+v", satellite.Values)
	}
	if production.Totals.Hour.Deuterium != -12 {
		t.Fatalf("expected legacy ceil production minus consumption, got %+v", production.Totals.Hour)
	}
}

func TestBuildResourceProductionDefaultsMoonToNoProduction(t *testing.T) {
	production := BuildResourceProduction(resourceOverview(PlanetTypeMoon), ResourceProductionInputs{
		Levels:            BuildingLevels{BuildingMetalMine: 10},
		ProductionFactors: ProductionFactors{BuildingMetalMine: 1},
	})

	if production.Factor != 0 || len(production.Rows) != 0 || production.Totals.Hour.Metal != 0 {
		t.Fatalf("expected moon to have default production state, got %+v", production)
	}
}

func resourceOverview(planetType int) Overview {
	return Overview{
		Commander: "legor",
		CurrentPlanet: PlanetOverview{
			ID:          99,
			Name:        "Arakis",
			Type:        planetType,
			Temperature: 19,
			Resources: Resources{
				MetalCapacity:     100000,
				CrystalCapacity:   150000,
				DeuteriumCapacity: 200000,
			},
		},
		PlanetSwitcher: []PlanetSummary{{ID: 99, Name: "Arakis", Type: planetType, Current: true}},
	}
}

func findResourceRow(t *testing.T, production ResourceProduction, id int) ResourceProductionRow {
	t.Helper()
	for _, row := range production.Rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("resource row %d not found in %+v", id, production.Rows)
	return ResourceProductionRow{}
}

func nearlyEqual(got float64, want float64, tolerance float64) bool {
	return math.Abs(got-want) <= tolerance
}
