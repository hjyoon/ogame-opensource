package game

type Defense struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	HasShipyard    bool
	Busy           bool
	Items          []ShipyardItem
}

type DefenseCounts map[int]int

func BuildDefense(overview Overview, levels BuildingLevels, research ResearchLevels, defense DefenseCounts, speed float64, busy bool, orderCap int) Defense {
	if speed <= 0 {
		speed = 1
	}
	hasShipyard := levels[BuildingShipyard] > 0
	items := []ShipyardItem{}
	if hasShipyard {
		items = make([]ShipyardItem, 0, len(defenseCatalog))
		for _, spec := range defenseCatalog {
			count := defense[spec.id]
			meets := requirementsMet(spec.requirements, levels, research)
			if !meets && count <= 0 {
				continue
			}
			cost := spec.price(1)
			blockedReason := defenseBlockedReason(spec.id, count, meets, busy)
			maxBuild := maxDefenseUnits(overview.CurrentPlanet.Resources, cost, levels, defense, spec.id, orderCap)
			canBuild := blockedReason == "" && maxBuild > 0 && cost.enough(overview.CurrentPlanet.Resources)
			items = append(items, ShipyardItem{
				ID:               spec.id,
				Name:             spec.name,
				Description:      spec.description,
				Count:            count,
				Cost:             cost,
				DurationSeconds:  buildingDuration(cost, levels[BuildingShipyard], levels[BuildingNaniteFactory], speed),
				CanBuild:         canBuild,
				MeetsRequirement: meets,
				MaxBuild:         maxBuild,
				BlockedReason:    blockedReason,
			})
		}
	}
	return Defense{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		HasShipyard:    hasShipyard,
		Busy:           busy,
		Items:          items,
	}
}

func defenseBlockedReason(id int, count int, meets bool, busy bool) string {
	if isShieldDome(id) && count > 0 {
		return "A shield dome can only be built 1 time."
	}
	if !meets {
		return "impossibly"
	}
	if busy {
		return "busy"
	}
	return ""
}

func maxDefenseUnits(resources Resources, cost BuildingCost, levels BuildingLevels, defense DefenseCounts, id int, orderCap int) int {
	if isShieldDome(id) {
		if defense[id] > 0 {
			return 0
		}
		orderCap = 1
	}
	if id == DefenseAntiBallisticMissile || id == DefenseInterplanetaryMissile {
		free := levels[BuildingMissileSilo]*10 - (defense[DefenseAntiBallisticMissile] + 2*defense[DefenseInterplanetaryMissile])
		if free <= 0 {
			return 0
		}
		if id == DefenseInterplanetaryMissile {
			free /= 2
		}
		if free <= 0 {
			return 0
		}
		orderCap = free
	}
	return maxAffordableUnits(resources, cost, orderCap)
}

func isShieldDome(id int) bool {
	return id == DefenseSmallShieldDome || id == DefenseLargeShieldDome
}

var defenseCatalog = []buildingSpec{
	{DefenseRocketLauncher, "Rocket Launcher", "The rocket launcher is a simple, cost-effective defensive option.", BuildingCost{Metal: 2000}, 0, map[int]int{BuildingShipyard: 1}, planetOnly},
	{DefenseLightLaser, "Light Laser", "Concentrated firing at a target with photons can produce significantly greater damage than standard ballistic weapons.", BuildingCost{Metal: 1500, Crystal: 500}, 0, map[int]int{ResearchEnergy: 1, BuildingShipyard: 2, ResearchLaser: 3}, planetOnly},
	{DefenseHeavyLaser, "Heavy Laser", "The heavy laser is the logical development of the light laser.", BuildingCost{Metal: 6000, Crystal: 2000}, 0, map[int]int{ResearchEnergy: 3, BuildingShipyard: 4, ResearchLaser: 6}, planetOnly},
	{DefenseGaussCannon, "Gauss Cannon", "The Gauss Cannon fires projectiles weighing tons at high speeds.", BuildingCost{Metal: 20000, Crystal: 15000, Deuterium: 2000}, 0, map[int]int{BuildingShipyard: 6, ResearchEnergy: 6, ResearchWeapon: 3, ResearchShield: 1}, planetOnly},
	{DefenseIonCannon, "Ion Cannon", "The Ion Cannon fires a continuous beam of accelerating ions, causing considerable damage to objects it strikes.", BuildingCost{Metal: 2000, Crystal: 6000}, 0, map[int]int{BuildingShipyard: 4, ResearchIon: 4}, planetOnly},
	{DefensePlasmaTurret, "Plasma Turret", "Plasma Turrets release the energy of a solar flare and surpass even the destroyer in destructive effect.", BuildingCost{Metal: 50000, Crystal: 50000, Deuterium: 30000}, 0, map[int]int{BuildingShipyard: 8, ResearchPlasma: 7}, planetOnly},
	{DefenseSmallShieldDome, "Small Shield Dome", "The small shield dome covers an entire planet with a field which can absorb a tremendous amount of energy.", BuildingCost{Metal: 10000, Crystal: 10000}, 0, map[int]int{ResearchShield: 2, BuildingShipyard: 1}, planetOnly},
	{DefenseLargeShieldDome, "Large Shield Dome", "The evolution of the small shield dome can employ significantly more energy to withstand attacks.", BuildingCost{Metal: 50000, Crystal: 50000}, 0, map[int]int{ResearchShield: 6, BuildingShipyard: 6}, planetOnly},
	{DefenseAntiBallisticMissile, "Anti-Ballistic Missiles", "Anti-Ballistic Missiles destroy attacking interplanetary missiles", BuildingCost{Metal: 8000, Deuterium: 2000}, 0, map[int]int{BuildingMissileSilo: 2, BuildingShipyard: 1}, planetOnly},
	{DefenseInterplanetaryMissile, "Interplanetary Missiles", "Interplanetary Missiles destroy enemy defenses.", BuildingCost{Metal: 12500, Crystal: 2500, Deuterium: 10000}, 0, map[int]int{BuildingMissileSilo: 4, BuildingShipyard: 1, ResearchImpulseDrive: 1}, planetOnly},
}
