package game

type PlanetScore struct {
	Points      int64
	FleetPoints int64
}

func CalculatePlanetScore(buildings BuildingLevels, fleet FleetCounts, defense DefenseCounts) PlanetScore {
	var score PlanetScore
	for _, spec := range buildingCatalog {
		level := buildings[spec.id]
		for current := 1; current <= level; current++ {
			score.Points += scoreCost(spec.price(current))
		}
	}
	for _, spec := range fleetCatalog {
		count := fleet[spec.id]
		if count <= 0 {
			continue
		}
		score.Points += scoreCost(spec.price(1)) * int64(count)
		score.FleetPoints += int64(count)
	}
	for _, spec := range defenseCatalog {
		count := defense[spec.id]
		if count <= 0 {
			continue
		}
		score.Points += scoreCost(spec.price(1)) * int64(count)
	}
	return score
}

func BuildingScoreForLevel(id int, level int) (int64, bool) {
	cost, ok := BuildingCostForLevel(id, level)
	if !ok {
		return 0, false
	}
	return scoreCost(cost), true
}

func ResearchScoreForLevel(id int, level int) (int64, bool) {
	cost, ok := ResearchCostForLevel(id, level)
	if !ok {
		return 0, false
	}
	return scoreCost(cost), true
}

func UnitScoreForCount(id int, count int) (points int64, fleetPoints int64, ok bool) {
	if count <= 0 {
		return 0, 0, false
	}
	for _, spec := range fleetCatalog {
		if spec.id == id {
			return scoreCost(spec.price(1)) * int64(count), int64(count), true
		}
	}
	for _, spec := range defenseCatalog {
		if spec.id == id {
			return scoreCost(spec.price(1)) * int64(count), 0, true
		}
	}
	return 0, 0, false
}

func scoreCost(cost BuildingCost) int64 {
	return int64(cost.Metal + cost.Crystal + cost.Deuterium)
}
