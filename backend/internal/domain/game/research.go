package game

import (
	"math"
	"sort"
)

const (
	maxResearchLevel       = 99
	researchDurationFactor = 1000
)

type Research struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	HasLab         bool
	Active         *ResearchQueue
	Items          []BuildingItem
}

type ResearchQueue struct {
	TaskID           int
	PlanetID         int
	TechID           int
	Level            int
	Start            int
	End              int
	RemainingSeconds int
	Cancelable       bool
}

type ResearchLabLevels map[int]int

func BuildResearchLabLevels(currentLab int, otherLabs []int, research ResearchLevels) ResearchLabLevels {
	labs := append([]int(nil), otherLabs...)
	sort.Sort(sort.Reverse(sort.IntSlice(labs)))
	network := research[ResearchIntergalacticNetwork]
	levels := make(ResearchLabLevels, len(researchCatalog))
	for _, spec := range researchCatalog {
		effective := currentLab
		attached := 0
		for _, lab := range labs {
			if attached >= network {
				break
			}
			if !requirementsMet(spec.requirements, BuildingLevels{BuildingResearchLab: lab}, research) {
				continue
			}
			effective += lab
			attached++
		}
		levels[spec.id] = effective
	}
	return levels
}

func BuildResearch(overview Overview, levels BuildingLevels, research ResearchLevels, labLevels ResearchLabLevels, speed float64, technocrat bool, active *ResearchQueue) Research {
	if speed <= 0 {
		speed = 1
	}
	if technocrat {
		speed *= 1.1
	}
	hasLab := levels[BuildingResearchLab] > 0
	items := []BuildingItem{}
	if hasLab {
		items = make([]BuildingItem, 0, len(researchCatalog))
		for _, spec := range researchCatalog {
			if !requirementsMet(spec.requirements, levels, research) {
				continue
			}
			level := research[spec.id]
			nextLevel := level + 1
			cost := spec.price(nextLevel)
			canResearch := level < maxResearchLevel && cost.enough(overview.CurrentPlanet.Resources)
			action := researchAction(level, nextLevel, canResearch)
			if active != nil {
				canResearch = active.TechID == spec.id && active.Cancelable
				if active.TechID == spec.id {
					action = "Cancel"
				} else {
					action = "-"
				}
			}
			labLevel := labLevels[spec.id]
			if labLevel <= 0 {
				labLevel = levels[BuildingResearchLab]
			}
			items = append(items, BuildingItem{
				ID:              spec.id,
				Name:            spec.name,
				Description:     spec.description,
				Level:           level,
				NextLevel:       nextLevel,
				Cost:            cost,
				DurationSeconds: researchDuration(cost, labLevel, speed),
				CanBuild:        canResearch,
				Action:          action,
			})
		}
	}
	return Research{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		HasLab:         hasLab,
		Active:         active,
		Items:          items,
	}
}

func researchDuration(cost BuildingCost, labLevel int, speed float64) int {
	seconds := mathFloorPositive((((cost.Metal + cost.Crystal) / (researchDurationFactor * float64(1+labLevel))) * 60 * 60) / speed)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func researchAction(level int, nextLevel int, canResearch bool) string {
	if level >= maxResearchLevel {
		return "Maximum level reached."
	}
	if level == 0 {
		return "research"
	}
	_ = nextLevel
	return "Research level"
}

func ResearchCostForLevel(id int, level int) (BuildingCost, bool) {
	for _, spec := range researchCatalog {
		if spec.id == id {
			return spec.price(level), true
		}
	}
	return BuildingCost{}, false
}

func ResearchDurationForLevel(id int, level int, labLevel int, speed float64) (int, bool) {
	cost, ok := ResearchCostForLevel(id, level)
	if !ok {
		return 0, false
	}
	return researchDuration(cost, labLevel, speed), true
}

func ResearchRequirementsMet(id int, buildings BuildingLevels, research ResearchLevels) bool {
	for _, spec := range researchCatalog {
		if spec.id == id {
			return requirementsMet(spec.requirements, buildings, research)
		}
	}
	return false
}

func mathFloorPositive(value float64) int {
	if value < 1 {
		return 0
	}
	return int(math.Floor(value))
}

var researchCatalog = []buildingSpec{
	{ResearchEspionage, "Espionage Technology", "Information about other planets and moons can be gained using this technology.", BuildingCost{Metal: 200, Crystal: 1000, Deuterium: 200}, 2, map[int]int{BuildingResearchLab: 3}, planetOnly},
	{ResearchComputer, "Computer Technology", "More fleets can be commanded by increasing computer capacities. Each level of computer technology increases the maximum number of fleets by one.", BuildingCost{Crystal: 400, Deuterium: 600}, 2, map[int]int{BuildingResearchLab: 1}, planetOnly},
	{ResearchWeapon, "Weapons Technology", "Weapons technology makes weapons systems more efficient. Each level of weapons technology increases the weapon strength of units by 10 % of the base value.", BuildingCost{Metal: 800, Crystal: 200}, 2, map[int]int{BuildingResearchLab: 4}, planetOnly},
	{ResearchShield, "Shielding Technology", "Shielding technology makes the shields on ships and defensive facilities more efficient. Each level of shield technology increases the strength of the shields by 10 % of the base value.", BuildingCost{Metal: 200, Crystal: 600}, 2, map[int]int{ResearchEnergy: 3, BuildingResearchLab: 6}, planetOnly},
	{ResearchArmour, "Armour Technology", "Special alloys improve the armour on ships and defensive structures. The effectiveness of the armour can be increased by 10 % per level.", BuildingCost{Metal: 1000}, 2, map[int]int{BuildingResearchLab: 2}, planetOnly},
	{ResearchEnergy, "Energy Technology", "The command of different types of energy is necessary for many new technologies.", BuildingCost{Crystal: 800, Deuterium: 400}, 2, map[int]int{BuildingResearchLab: 1}, planetOnly},
	{ResearchHyperspace, "Hyperspace Technology", "By integrating the 4th and 5th dimensions it is now possible to research a new kind of drive that is more economical and efficient.", BuildingCost{Crystal: 4000, Deuterium: 2000}, 2, map[int]int{ResearchEnergy: 5, ResearchShield: 5, BuildingResearchLab: 7}, planetOnly},
	{ResearchCombustionDrive, "Combustion Drive", "The development of this drive makes some ships faster, although each level increases speed by only 10 % of the base value.", BuildingCost{Metal: 400, Deuterium: 600}, 2, map[int]int{ResearchEnergy: 1}, planetOnly},
	{ResearchImpulseDrive, "Impulse Drive", "The impulse drive is based on the reaction principle. Further development of this drive makes some ships faster, although each level increases speed by only 20 % of the base value.", BuildingCost{Metal: 2000, Crystal: 4000, Deuterium: 600}, 2, map[int]int{ResearchEnergy: 1, BuildingResearchLab: 2}, planetOnly},
	{ResearchHyperspaceDrive, "Hyperspace Drive", "Hyperspace drive warps space around a ship. The development of this drive makes some ships faster, although each level increases speed by only 30 % of the base value.", BuildingCost{Metal: 10000, Crystal: 20000, Deuterium: 6000}, 2, map[int]int{ResearchHyperspace: 3}, planetOnly},
	{ResearchLaser, "Laser Technology", "Focusing light produces a beam that causes damage when it strikes an object.", BuildingCost{Metal: 200, Crystal: 100}, 2, map[int]int{ResearchEnergy: 2}, planetOnly},
	{ResearchIon, "Ion Technology", "A deadly beam of accelerated ions. This causes enormous damage when striking an object.", BuildingCost{Metal: 1000, Crystal: 300, Deuterium: 100}, 2, map[int]int{BuildingResearchLab: 4, ResearchLaser: 5, ResearchEnergy: 4}, planetOnly},
	{ResearchPlasma, "Plasma Technology", "A further development of ion technology which accelerates high-energy plasma.", BuildingCost{Metal: 2000, Crystal: 4000, Deuterium: 1000}, 2, map[int]int{ResearchEnergy: 8, ResearchLaser: 10, ResearchIon: 5}, planetOnly},
	{ResearchIntergalacticNetwork, "Intergalactic Research Network", "Researchers on different planets communicate via this network.", BuildingCost{Metal: 240000, Crystal: 400000, Deuterium: 160000}, 2, map[int]int{BuildingResearchLab: 10, ResearchComputer: 8, ResearchHyperspace: 8}, planetOnly},
	{ResearchExpedition, "Expedition Technology", "Ships can be equipped with a research module which allows a data back up of information collected during an expedition of unexplored regions.", BuildingCost{Metal: 4000, Crystal: 8000, Deuterium: 4000}, 2, map[int]int{ResearchEspionage: 4, ResearchImpulseDrive: 3}, planetOnly},
	{ResearchGraviton, "Graviton Technology", "Firing a concentrated charge of graviton particles can create an artificial gravity field, which can destroy ships or even moons.", BuildingCost{Energy: 300000}, 3, map[int]int{BuildingResearchLab: 12}, planetOnly},
}
