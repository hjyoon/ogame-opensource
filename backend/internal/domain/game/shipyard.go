package game

const defaultShipyardOrderCap = 1000

type Shipyard struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	HasShipyard    bool
	Busy           bool
	Items          []ShipyardItem
}

type ShipyardItem struct {
	ID               int
	Name             string
	Description      string
	Count            int
	Cost             BuildingCost
	DurationSeconds  int
	CanBuild         bool
	MeetsRequirement bool
	MaxBuild         int
	BlockedReason    string
}

type FleetCounts map[int]int

func BuildShipyard(overview Overview, levels BuildingLevels, research ResearchLevels, fleet FleetCounts, speed float64, busy bool, orderCap int) Shipyard {
	if speed <= 0 {
		speed = 1
	}
	hasShipyard := levels[BuildingShipyard] > 0
	items := []ShipyardItem{}
	if hasShipyard {
		items = make([]ShipyardItem, 0, len(fleetCatalog))
		for _, spec := range fleetCatalog {
			count := fleet[spec.id]
			meets := requirementsMet(spec.requirements, levels, research)
			if !meets && count <= 0 {
				continue
			}
			cost := spec.price(1)
			canBuild := meets && !busy && cost.enough(overview.CurrentPlanet.Resources)
			blockedReason := ""
			if !meets {
				blockedReason = "impossibly"
			} else if busy {
				blockedReason = "busy"
			}
			items = append(items, ShipyardItem{
				ID:               spec.id,
				Name:             spec.name,
				Description:      spec.description,
				Count:            count,
				Cost:             cost,
				DurationSeconds:  buildingDuration(cost, levels[BuildingShipyard], levels[BuildingNaniteFactory], speed),
				CanBuild:         canBuild,
				MeetsRequirement: meets,
				MaxBuild:         maxAffordableUnits(overview.CurrentPlanet.Resources, cost, orderCap),
				BlockedReason:    blockedReason,
			})
		}
	}
	return Shipyard{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		HasShipyard:    hasShipyard,
		Busy:           busy,
		Items:          items,
	}
}

func maxAffordableUnits(resources Resources, cost BuildingCost, orderCap int) int {
	if orderCap <= 0 {
		orderCap = defaultShipyardOrderCap
	}
	maximum := orderCap
	if cost.Metal > 0 {
		maximum = min(maximum, int(resources.Metal/cost.Metal))
	}
	if cost.Crystal > 0 {
		maximum = min(maximum, int(resources.Crystal/cost.Crystal))
	}
	if cost.Deuterium > 0 {
		maximum = min(maximum, int(resources.Deuterium/cost.Deuterium))
	}
	if maximum < 0 {
		return 0
	}
	return maximum
}

var fleetCatalog = []buildingSpec{
	{FleetSmallCargo, "Small Cargo", "The small cargo is an agile ship which can quickly transport resources to other planets. The small cargo is an agile ship which can quickly transport resources to other planets.", BuildingCost{Metal: 2000, Crystal: 2000}, 0, map[int]int{BuildingShipyard: 2, ResearchCombustionDrive: 2}, planetOnly},
	{FleetLargeCargo, "Large Cargo", "This cargo ship has a much larger cargo capacity than the small cargo, and is generally faster thanks to an improved drive.", BuildingCost{Metal: 6000, Crystal: 6000}, 0, map[int]int{BuildingShipyard: 4, ResearchCombustionDrive: 6}, planetOnly},
	{FleetLightFighter, "Light Fighter", "This is the first fighting ship all emperors will build. The light fighter is an agile ship, but vulnerable by themselves. In mass numbers, they can become a great threat to any empire. They are the first to accompany small and large cargo to hostile planets with minor defenses.", BuildingCost{Metal: 3000, Crystal: 1000}, 0, map[int]int{BuildingShipyard: 1, ResearchCombustionDrive: 1}, planetOnly},
	{FleetHeavyFighter, "Heavy Fighter", "This fighter is better armoured and has a higher attack strength than the light fighter.", BuildingCost{Metal: 6000, Crystal: 4000}, 0, map[int]int{BuildingShipyard: 3, ResearchArmour: 2, ResearchImpulseDrive: 2}, planetOnly},
	{FleetCruiser, "Cruiser", "Cruisers are armoured almost three times as heavily as heavy fighters and have more than twice the firepower. In addition, they are very fast.", BuildingCost{Metal: 20000, Crystal: 7000, Deuterium: 2000}, 0, map[int]int{BuildingShipyard: 5, ResearchImpulseDrive: 4, ResearchIon: 2}, planetOnly},
	{FleetBattleship, "Battleship", "Battleships form the backbone of a fleet. Their heavy cannons, high speed, and large cargo holds make them opponents to be taken seriously.", BuildingCost{Metal: 45000, Crystal: 15000}, 0, map[int]int{BuildingShipyard: 7, ResearchHyperspaceDrive: 4}, planetOnly},
	{FleetColonyShip, "Colony Ship", "Vacant planets can be colonized with this ship.", BuildingCost{Metal: 10000, Crystal: 20000, Deuterium: 10000}, 0, map[int]int{BuildingShipyard: 4, ResearchImpulseDrive: 3}, planetOnly},
	{FleetRecycler, "Recycler", "Recyclers are the only ships able to harvest debris fields floating in a planets orbit after combat.", BuildingCost{Metal: 10000, Crystal: 6000, Deuterium: 2000}, 0, map[int]int{BuildingShipyard: 4, ResearchCombustionDrive: 6, ResearchShield: 2}, planetOnly},
	{FleetEspionageProbe, "Espionage Probe", "Espionage probes are small, agile drones that provide data on fleets and planets over great distances.", BuildingCost{Crystal: 1000}, 0, map[int]int{BuildingShipyard: 3, ResearchCombustionDrive: 3, ResearchEspionage: 2}, planetOnly},
	{FleetBomber, "Bomber", "The bomber was developed especially to destroy the planetary defenses of a world.", BuildingCost{Metal: 50000, Crystal: 25000, Deuterium: 15000}, 0, map[int]int{ResearchImpulseDrive: 6, BuildingShipyard: 8, ResearchPlasma: 5}, planetOnly},
	{FleetSolarSatellite, "Solar Satellite", "Solar satellites are simple platforms of solar cells, located in a high, stationary orbit. They gather sunlight and transmit it to the ground station via laser.", BuildingCost{Crystal: 2000, Deuterium: 500}, 0, map[int]int{BuildingShipyard: 1}, planetOnly},
	{FleetDestroyer, "Destroyer", "The destroyer is the king of the warships.", BuildingCost{Metal: 60000, Crystal: 50000, Deuterium: 15000}, 0, map[int]int{BuildingShipyard: 9, ResearchHyperspaceDrive: 6, ResearchHyperspace: 5}, planetOnly},
	{FleetDeathstar, "Deathstar", "The destructive power of the deathstar is unsurpassed.", BuildingCost{Metal: 5000000, Crystal: 4000000, Deuterium: 1000000}, 0, map[int]int{BuildingShipyard: 12, ResearchHyperspaceDrive: 7, ResearchHyperspace: 6, ResearchGraviton: 1}, planetOnly},
	{FleetBattlecruiser, "Battlecruiser", "The Battlecruiser is highly specialized in the interception of hostile fleets.", BuildingCost{Metal: 30000, Crystal: 40000, Deuterium: 15000}, 0, map[int]int{ResearchHyperspace: 5, ResearchLaser: 12, ResearchHyperspaceDrive: 5, BuildingShipyard: 8}, planetOnly},
}
