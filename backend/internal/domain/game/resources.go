package game

import (
	"errors"
	"math"
)

const FleetSolarSatellite = 212

var ErrProductionPercentTooHigh = errors.New("production percent cannot exceed 100")

type ResourceProduction struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Factor         float64
	Natural        ResourceProductionValues
	Rows           []ResourceProductionRow
	Storage        ResourceProductionValues
	Totals         ResourceProductionTotals
}

type ResourceProductionRow struct {
	ID      int
	Name    string
	Level   int
	Percent int
	Values  ResourceProductionValues
}

type ResourceProductionValues struct {
	Metal        float64
	Crystal      float64
	Deuterium    float64
	Energy       float64
	EnergyRaw    float64
	EnergyStored bool
}

type ResourceProductionTotals struct {
	Hour ResourceProductionValues
	Day  ResourceProductionValues
	Week ResourceProductionValues
}

type ResourceProductionInputs struct {
	Levels            BuildingLevels
	SolarSatellites   int
	ProductionFactors ProductionFactors
	EnergyResearch    int
	UniverseSpeed     float64
	Geologist         bool
	Engineer          bool
}

type ProductionFactors map[int]float64
type ProductionPercents map[int]int

type producerOutput struct {
	id      int
	name    string
	level   int
	percent int
	values  ResourceProductionValues
}

func ResourceProducerIDs() []int {
	return []int{
		BuildingMetalMine,
		BuildingCrystalMine,
		BuildingDeuteriumSynth,
		BuildingSolarPlant,
		BuildingFusionReactor,
		FleetSolarSatellite,
	}
}

func NormalizeProductionSettings(settings ProductionPercents) (ProductionFactors, error) {
	normalized := make(ProductionFactors, len(settings))
	for id, percent := range settings {
		if !resourceProducerID(id) {
			continue
		}
		factor, err := NormalizeProductionPercent(percent)
		if err != nil {
			return nil, err
		}
		normalized[id] = factor
	}
	return normalized, nil
}

func NormalizeProductionPercent(percent int) (float64, error) {
	if percent > 100 {
		return 0, ErrProductionPercentTooHigh
	}
	if percent < 0 {
		percent = 0
	}
	return float64(int(math.Round(float64(percent)/10))*10) / 100, nil
}

func BuildResourceProduction(overview Overview, inputs ResourceProductionInputs) ResourceProduction {
	speed := inputs.UniverseSpeed
	if speed <= 0 {
		speed = 1
	}
	production := ResourceProduction{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Storage: ResourceProductionValues{
			Metal:        float64(overview.CurrentPlanet.Resources.MetalCapacity),
			Crystal:      float64(overview.CurrentPlanet.Resources.CrystalCapacity),
			Deuterium:    float64(overview.CurrentPlanet.Resources.DeuteriumCapacity),
			EnergyStored: false,
		},
	}
	if overview.CurrentPlanet.Type != PlanetTypePlanet {
		return production
	}

	rows := resourceProducerOutputs(overview.CurrentPlanet, inputs, speed)
	energyBalance, energyConsumption := energyTotals(rows)
	production.Factor = 1
	if energyBalance < 0 && energyConsumption > 0 {
		production.Factor = math.Max(0, 1-math.Abs(energyBalance)/energyConsumption)
	}

	production.Natural = ResourceProductionValues{
		Metal:     20 * speed,
		Crystal:   10 * speed,
		EnergyRaw: 0,
	}
	if inputs.Geologist {
		for index := range rows {
			if rows[index].values.Metal > 0 {
				rows[index].values.Metal *= 1.1
			}
			if rows[index].values.Crystal > 0 {
				rows[index].values.Crystal *= 1.1
			}
			if rows[index].values.Deuterium > 0 {
				rows[index].values.Deuterium *= 1.1
			}
		}
	}
	for index := range rows {
		if rows[index].values.Metal > 0 {
			rows[index].values.Metal *= production.Factor
		}
		if rows[index].values.Crystal > 0 {
			rows[index].values.Crystal *= production.Factor
		}
		if rows[index].values.Deuterium > 0 {
			rows[index].values.Deuterium *= production.Factor
		}
		if rows[index].values.Energy < 0 {
			rows[index].values.Energy *= production.Factor
		}
		production.Rows = append(production.Rows, ResourceProductionRow{
			ID:      rows[index].id,
			Name:    rows[index].name,
			Level:   rows[index].level,
			Percent: rows[index].percent,
			Values:  rows[index].values,
		})
	}

	hour := ResourceProductionValues{
		Metal:     production.Natural.Metal,
		Crystal:   production.Natural.Crystal,
		Energy:    energyBalance,
		EnergyRaw: energyBalance,
	}
	for _, row := range rows {
		hour.Metal += ceilPositive(row.values.Metal)
		hour.Crystal += ceilPositive(row.values.Crystal)
		hour.Deuterium += ceilPositive(row.values.Deuterium)
		if row.values.Metal < 0 {
			hour.Metal += math.Floor(row.values.Metal)
		}
		if row.values.Crystal < 0 {
			hour.Crystal += math.Floor(row.values.Crystal)
		}
		if row.values.Deuterium < 0 {
			hour.Deuterium += math.Floor(row.values.Deuterium)
		}
	}
	hour.Metal = math.Floor(hour.Metal)
	hour.Crystal = math.Floor(hour.Crystal)
	hour.Deuterium = math.Floor(hour.Deuterium)
	production.Totals = ResourceProductionTotals{
		Hour: hour,
		Day:  productionTotalsForHours(hour, 24),
		Week: productionTotalsForHours(hour, 24*7),
	}
	return production
}

func resourceProducerOutputs(planet PlanetOverview, inputs ResourceProductionInputs, speed float64) []producerOutput {
	levels := inputs.Levels
	factors := inputs.ProductionFactors
	energyBonus := 1.0
	if inputs.Engineer {
		energyBonus = 1.1
	}
	rows := make([]producerOutput, 0, len(ResourceProducerIDs()))

	metalLevel := levels[BuildingMetalMine]
	if metalLevel > 0 {
		rawMetal := math.Floor(30*float64(metalLevel)*math.Pow(1.1, float64(metalLevel))*factorFor(factors, BuildingMetalMine)) * speed
		rawEnergy := math.Ceil(10 * float64(metalLevel) * math.Pow(1.1, float64(metalLevel)) * factorFor(factors, BuildingMetalMine))
		rows = append(rows, producerOutput{
			id:      BuildingMetalMine,
			name:    "Metal Mine",
			level:   metalLevel,
			percent: percentFor(factors, BuildingMetalMine),
			values:  ResourceProductionValues{Metal: rawMetal, Energy: -rawEnergy, EnergyRaw: -rawEnergy},
		})
	}

	crystalLevel := levels[BuildingCrystalMine]
	if crystalLevel > 0 {
		rawCrystal := math.Floor(20*float64(crystalLevel)*math.Pow(1.1, float64(crystalLevel))*factorFor(factors, BuildingCrystalMine)) * speed
		rawEnergy := math.Ceil(10 * float64(crystalLevel) * math.Pow(1.1, float64(crystalLevel)) * factorFor(factors, BuildingCrystalMine))
		rows = append(rows, producerOutput{
			id:      BuildingCrystalMine,
			name:    "Crystal Mine",
			level:   crystalLevel,
			percent: percentFor(factors, BuildingCrystalMine),
			values:  ResourceProductionValues{Crystal: rawCrystal, Energy: -rawEnergy, EnergyRaw: -rawEnergy},
		})
	}

	deuteriumLevel := levels[BuildingDeuteriumSynth]
	if deuteriumLevel > 0 {
		temperatureFactor := 1.28 - 0.002*float64(planet.Temperature+40)
		rawDeuterium := math.Floor(10*float64(deuteriumLevel)*math.Pow(1.1, float64(deuteriumLevel))*factorFor(factors, BuildingDeuteriumSynth)) * temperatureFactor * speed
		rawEnergy := math.Ceil(20 * float64(deuteriumLevel) * math.Pow(1.1, float64(deuteriumLevel)) * factorFor(factors, BuildingDeuteriumSynth))
		rows = append(rows, producerOutput{
			id:      BuildingDeuteriumSynth,
			name:    "Deuterium Synthesizer",
			level:   deuteriumLevel,
			percent: percentFor(factors, BuildingDeuteriumSynth),
			values:  ResourceProductionValues{Deuterium: rawDeuterium, Energy: -rawEnergy, EnergyRaw: -rawEnergy},
		})
	}

	solarLevel := levels[BuildingSolarPlant]
	if solarLevel > 0 {
		rawEnergy := math.Floor(20*float64(solarLevel)*math.Pow(1.1, float64(solarLevel))*factorFor(factors, BuildingSolarPlant)) * energyBonus
		rows = append(rows, producerOutput{
			id:      BuildingSolarPlant,
			name:    "Solar Plant",
			level:   solarLevel,
			percent: percentFor(factors, BuildingSolarPlant),
			values:  ResourceProductionValues{Energy: rawEnergy, EnergyRaw: rawEnergy},
		})
	}

	fusionLevel := levels[BuildingFusionReactor]
	if fusionLevel > 0 {
		rawEnergy := math.Floor(30*float64(fusionLevel)*math.Pow(1.05+float64(inputs.EnergyResearch)*0.01, float64(fusionLevel))*factorFor(factors, BuildingFusionReactor)) * energyBonus
		rawDeuterium := math.Ceil(10 * float64(fusionLevel) * math.Pow(1.1, float64(fusionLevel)) * factorFor(factors, BuildingFusionReactor))
		rows = append(rows, producerOutput{
			id:      BuildingFusionReactor,
			name:    "Fusion Reactor",
			level:   fusionLevel,
			percent: percentFor(factors, BuildingFusionReactor),
			values:  ResourceProductionValues{Deuterium: -rawDeuterium, Energy: rawEnergy, EnergyRaw: rawEnergy},
		})
	}

	if inputs.SolarSatellites > 0 {
		satelliteOutput := math.Max(1, math.Floor((float64(planet.Temperature+40)/4)+20))
		rawEnergy := satelliteOutput * float64(inputs.SolarSatellites) * factorFor(factors, FleetSolarSatellite) * energyBonus
		rows = append(rows, producerOutput{
			id:      FleetSolarSatellite,
			name:    "Solar Satellite",
			level:   inputs.SolarSatellites,
			percent: percentFor(factors, FleetSolarSatellite),
			values:  ResourceProductionValues{Energy: rawEnergy, EnergyRaw: rawEnergy},
		})
	}

	return rows
}

func energyTotals(rows []producerOutput) (float64, float64) {
	production := 0.0
	consumption := 0.0
	for _, row := range rows {
		if row.values.Energy > 0 {
			production += math.Ceil(row.values.Energy)
		}
		if row.values.EnergyRaw < 0 {
			consumption += math.Ceil(math.Abs(row.values.EnergyRaw))
		}
	}
	return math.Floor(production - consumption), consumption
}

func productionTotalsForHours(hour ResourceProductionValues, hours float64) ResourceProductionValues {
	return ResourceProductionValues{
		Metal:     hour.Metal * hours,
		Crystal:   hour.Crystal * hours,
		Deuterium: hour.Deuterium * hours,
		Energy:    hour.Energy,
		EnergyRaw: hour.EnergyRaw,
	}
}

func factorFor(factors ProductionFactors, id int) float64 {
	if factors == nil {
		return 0
	}
	return factors[id]
}

func percentFor(factors ProductionFactors, id int) int {
	return int(math.Round(factorFor(factors, id) * 100))
}

func ceilPositive(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Ceil(value)
}

func resourceProducerID(id int) bool {
	for _, candidate := range ResourceProducerIDs() {
		if candidate == id {
			return true
		}
	}
	return false
}
