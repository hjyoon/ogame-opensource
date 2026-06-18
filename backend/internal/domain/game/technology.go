package game

import "sort"

type Technology struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Groups         []TechnologyGroup
}

type TechnologyGroup struct {
	Key   string
	Name  string
	Items []TechnologyItem
}

type TechnologyItem struct {
	ID               int
	Name             string
	Requirements     []TechnologyRequirement
	DetailsAvailable bool
}

type TechnologyRequirement struct {
	ID           int
	Name         string
	Level        int
	CurrentLevel int
	Met          bool
}

func BuildTechnology(overview Overview, levels BuildingLevels, research ResearchLevels) Technology {
	return Technology{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Groups: []TechnologyGroup{
			buildTechnologyGroup("building", "Buildings", []int{
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
				BuildingMissileSilo,
			}, levels, research),
			buildTechnologyGroup("research", "Research", ResearchIDs(), levels, research),
			buildTechnologyGroup("fleet", "Ships", FleetIDs(), levels, research),
			buildTechnologyGroup("defense", "Defense", DefenseIDs(), levels, research),
			buildTechnologyGroup("special", "Lunar Buildings", []int{
				BuildingLunarBase,
				BuildingSensorPhalanx,
				BuildingJumpGate,
			}, levels, research),
		},
	}
}

func buildTechnologyGroup(key string, name string, ids []int, levels BuildingLevels, research ResearchLevels) TechnologyGroup {
	items := make([]TechnologyItem, 0, len(ids))
	for _, id := range ids {
		spec, ok := technologySpecByID(id)
		if !ok {
			continue
		}
		requirements := buildTechnologyRequirements(spec.requirements, levels, research)
		items = append(items, TechnologyItem{
			ID:               id,
			Name:             spec.name,
			Requirements:     requirements,
			DetailsAvailable: len(requirements) > 0,
		})
	}
	return TechnologyGroup{Key: key, Name: name, Items: items}
}

func buildTechnologyRequirements(requirements map[int]int, levels BuildingLevels, research ResearchLevels) []TechnologyRequirement {
	if len(requirements) == 0 {
		return nil
	}
	ids := make([]int, 0, len(requirements))
	for id := range requirements {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	result := make([]TechnologyRequirement, 0, len(ids))
	for _, id := range ids {
		required := requirements[id]
		current := levels[id]
		if isResearchID(id) {
			current = research[id]
		}
		result = append(result, TechnologyRequirement{
			ID:           id,
			Name:         technologyName(id),
			Level:        required,
			CurrentLevel: current,
			Met:          current >= required,
		})
	}
	return result
}

func technologySpecByID(id int) (buildingSpec, bool) {
	for _, spec := range buildingCatalog {
		if spec.id == id {
			return spec, true
		}
	}
	for _, spec := range researchCatalog {
		if spec.id == id {
			return spec, true
		}
	}
	for _, spec := range fleetCatalog {
		if spec.id == id {
			return spec, true
		}
	}
	for _, spec := range defenseCatalog {
		if spec.id == id {
			return spec, true
		}
	}
	return buildingSpec{}, false
}

func technologyName(id int) string {
	if spec, ok := technologySpecByID(id); ok {
		return spec.name
	}
	return ""
}

func isResearchID(id int) bool {
	for _, researchID := range ResearchIDs() {
		if researchID == id {
			return true
		}
	}
	return false
}
