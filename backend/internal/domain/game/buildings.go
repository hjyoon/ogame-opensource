package game

import "math"

const (
	ResourceMetal     = 700
	ResourceCrystal   = 701
	ResourceDeuterium = 702
	ResourceEnergy    = 703

	PlanetTypeMoon   = 0
	PlanetTypePlanet = 1
)

const (
	BuildingMetalMine       = 1
	BuildingCrystalMine     = 2
	BuildingDeuteriumSynth  = 3
	BuildingSolarPlant      = 4
	BuildingFusionReactor   = 12
	BuildingRoboticsFactory = 14
	BuildingNaniteFactory   = 15
	BuildingShipyard        = 21
	BuildingMetalStorage    = 22
	BuildingCrystalStorage  = 23
	BuildingDeuteriumTank   = 24
	BuildingResearchLab     = 31
	BuildingTerraformer     = 33
	BuildingAllianceDepot   = 34
	BuildingLunarBase       = 41
	BuildingSensorPhalanx   = 42
	BuildingJumpGate        = 43
	BuildingMissileSilo     = 44

	ResearchComputer = 108
	ResearchEnergy   = 113
)

const buildingDurationFactor = 2500

type Buildings struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Items          []BuildingItem
}

type BuildingItem struct {
	ID              int
	Name            string
	Description     string
	Level           int
	NextLevel       int
	Cost            BuildingCost
	DurationSeconds int
	CanBuild        bool
	Action          string
}

type BuildingCost struct {
	Metal     float64
	Crystal   float64
	Deuterium float64
	Energy    float64
}

type BuildingLevels map[int]int
type ResearchLevels map[int]int

type buildingSpec struct {
	id                 int
	name               string
	description        string
	initial            BuildingCost
	factor             float64
	requirements       map[int]int
	allowedPlanetTypes map[int]bool
}

func BuildingIDs() []int {
	return []int{
		BuildingMetalMine,
		BuildingCrystalMine,
		BuildingDeuteriumSynth,
		BuildingSolarPlant,
		BuildingFusionReactor,
		BuildingRoboticsFactory,
		BuildingNaniteFactory,
		BuildingShipyard,
		BuildingMetalStorage,
		BuildingCrystalStorage,
		BuildingDeuteriumTank,
		BuildingResearchLab,
		BuildingTerraformer,
		BuildingAllianceDepot,
		BuildingLunarBase,
		BuildingSensorPhalanx,
		BuildingJumpGate,
		BuildingMissileSilo,
	}
}

func BuildingResearchIDs() []int {
	return []int{ResearchComputer, ResearchEnergy}
}

func BuildBuildings(overview Overview, levels BuildingLevels, research ResearchLevels, speed float64) Buildings {
	if speed <= 0 {
		speed = 1
	}
	robotics := levels[BuildingRoboticsFactory]
	nanites := levels[BuildingNaniteFactory]
	items := make([]BuildingItem, 0, len(buildingCatalog))
	for _, spec := range buildingCatalog {
		if !spec.allowedPlanetTypes[overview.CurrentPlanet.Type] || !requirementsMet(spec.requirements, levels, research) {
			continue
		}
		level := levels[spec.id]
		nextLevel := level + 1
		cost := spec.price(nextLevel)
		canBuild := overview.CurrentPlanet.Fields < overview.CurrentPlanet.MaxFields && cost.enough(overview.CurrentPlanet.Resources)
		action := buildAction(level, nextLevel, canBuild, overview.CurrentPlanet.Fields >= overview.CurrentPlanet.MaxFields)
		items = append(items, BuildingItem{
			ID:              spec.id,
			Name:            spec.name,
			Description:     spec.description,
			Level:           level,
			NextLevel:       nextLevel,
			Cost:            cost,
			DurationSeconds: buildingDuration(cost, robotics, nanites, speed),
			CanBuild:        canBuild,
			Action:          action,
		})
	}
	return Buildings{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Items:          items,
	}
}

func (s buildingSpec) price(level int) BuildingCost {
	if level < 1 {
		level = 1
	}
	multiplier := math.Pow(s.factor, float64(level-1))
	return BuildingCost{
		Metal:     s.initial.Metal * multiplier,
		Crystal:   s.initial.Crystal * multiplier,
		Deuterium: s.initial.Deuterium * multiplier,
		Energy:    s.initial.Energy * multiplier,
	}
}

func buildingDuration(cost BuildingCost, robotics int, nanites int, speed float64) int {
	seconds := math.Floor((((cost.Metal + cost.Crystal) / (buildingDurationFactor * float64(1+robotics))) * math.Pow(0.5, float64(nanites)) * 60 * 60) / speed)
	if seconds < 1 {
		return 1
	}
	return int(seconds)
}

func requirementsMet(requirements map[int]int, buildings BuildingLevels, research ResearchLevels) bool {
	for id, required := range requirements {
		if id >= 100 {
			if research[id] < required {
				return false
			}
			continue
		}
		if buildings[id] < required {
			return false
		}
	}
	return true
}

func (c BuildingCost) enough(resources Resources) bool {
	return resources.Metal >= c.Metal && resources.Crystal >= c.Crystal && resources.Deuterium >= c.Deuterium && c.Energy <= 0
}

func buildAction(level int, nextLevel int, canBuild bool, full bool) string {
	if full {
		return "There's no space!"
	}
	if level == 0 {
		return "build"
	}
	if canBuild {
		return "Build level"
	}
	return "Build level"
}

var planetOnly = map[int]bool{PlanetTypePlanet: true}
var moonOnly = map[int]bool{PlanetTypeMoon: true}
var planetAndMoon = map[int]bool{PlanetTypePlanet: true, PlanetTypeMoon: true}

var buildingCatalog = []buildingSpec{
	{BuildingMetalMine, "Metal Mine", "Used in the extraction of metal ore, metal mines are of primary importance to all emerging and established empires.", BuildingCost{Metal: 60, Crystal: 15}, 1.5, nil, planetOnly},
	{BuildingCrystalMine, "Crystal Mine", "Crystals are the main resource used to build electronic circuits and form certain alloy compounds.", BuildingCost{Metal: 48, Crystal: 24}, 1.6, nil, planetOnly},
	{BuildingDeuteriumSynth, "Deuterium Synthesizer", "Deuterium is used as fuel for spaceships and is harvested in the deep sea. Deuterium is a rare substance and is thus relatively expensive.", BuildingCost{Metal: 225, Crystal: 75}, 1.5, nil, planetOnly},
	{BuildingSolarPlant, "Solar Plant", "Solar power plants absorb energy from solar radiation. All mines need energy to operate.", BuildingCost{Metal: 75, Crystal: 30}, 1.5, nil, planetOnly},
	{BuildingFusionReactor, "Fusion Reactor", "The fusion reactor uses deuterium to produce energy.", BuildingCost{Metal: 900, Crystal: 360, Deuterium: 180}, 1.8, map[int]int{BuildingDeuteriumSynth: 5, ResearchEnergy: 3}, planetOnly},
	{BuildingRoboticsFactory, "Robotics Factory", "Robotic factories provide construction robots to aid in the construction of buildings. Each level increases the speed of the upgrade of buildings.", BuildingCost{Metal: 400, Crystal: 120, Deuterium: 200}, 2, nil, planetAndMoon},
	{BuildingNaniteFactory, "Nanite Factory", "This is the ultimate in robotics technology. Each level cuts the construction time for buildings, ships, and defenses.", BuildingCost{Metal: 1000000, Crystal: 500000, Deuterium: 100000}, 2, map[int]int{BuildingRoboticsFactory: 10, ResearchComputer: 10}, planetOnly},
	{BuildingShipyard, "Shipyard", "All types of ships and defensive facilities are built in the planetary shipyard.", BuildingCost{Metal: 400, Crystal: 200, Deuterium: 100}, 2, map[int]int{BuildingRoboticsFactory: 2}, planetAndMoon},
	{BuildingMetalStorage, "Metal Storage", "Provides storage for excess metal.", BuildingCost{Metal: 2000}, 2, nil, planetAndMoon},
	{BuildingCrystalStorage, "Crystal Storage", "Provides storage for excess crystal.", BuildingCost{Metal: 2000, Crystal: 1000}, 2, nil, planetAndMoon},
	{BuildingDeuteriumTank, "Deuterium Tank", "Giant tanks for storing newly-extracted deuterium.", BuildingCost{Metal: 2000, Crystal: 2000}, 2, nil, planetAndMoon},
	{BuildingResearchLab, "Research Lab", "A research lab is required in order to conduct research into new technologies.", BuildingCost{Metal: 200, Crystal: 400, Deuterium: 200}, 2, nil, planetOnly},
	{BuildingTerraformer, "Terraformer", "The terraformer increases the usable surface of planets.", BuildingCost{Crystal: 50000, Deuterium: 100000, Energy: 1000}, 2, map[int]int{BuildingNaniteFactory: 1, ResearchEnergy: 12}, planetOnly},
	{BuildingAllianceDepot, "Alliance Depot", "The alliance depot supplies fuel to friendly fleets in orbit helping with defense.", BuildingCost{Metal: 20000, Crystal: 40000}, 2, nil, planetOnly},
	{BuildingLunarBase, "Lunar Base", "Since the moon has no atmosphere, a lunar base is required to generate habitable space.", BuildingCost{Metal: 20000, Crystal: 40000, Deuterium: 20000}, 2, nil, moonOnly},
	{BuildingSensorPhalanx, "Sensor Phalanx", "Using the sensor phalanx, fleets of other empires can be discovered and observed. The bigger the sensor phalanx array, the larger the range it can scan.", BuildingCost{Metal: 20000, Crystal: 40000, Deuterium: 20000}, 2, map[int]int{BuildingLunarBase: 1}, moonOnly},
	{BuildingJumpGate, "Jump Gate", "Jump gates are huge transceivers capable of sending even the biggest fleet in no time to a distant jump gate.", BuildingCost{Metal: 2000000, Crystal: 4000000, Deuterium: 2000000}, 2, map[int]int{BuildingLunarBase: 1}, moonOnly},
	{BuildingMissileSilo, "Missile Silo", "Missile silos are used to store missiles.", BuildingCost{Metal: 20000, Crystal: 20000, Deuterium: 1000}, 2, map[int]int{BuildingShipyard: 1}, planetOnly},
}
