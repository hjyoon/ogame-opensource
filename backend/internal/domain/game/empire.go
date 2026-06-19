package game

import "math"

const (
	EmpirePlanetTypePlanets = 1
	EmpirePlanetTypeMoons   = 3

	EmpireIssueCommanderRequired = "commander_required"
)

type Empire struct {
	Commander       string
	CommanderActive bool
	CurrentPlanet   PlanetOverview
	PlanetSwitcher  []PlanetSummary
	PlanetType      int
	MoonEnabled     bool
	HasMoons        bool
	Planets         []EmpirePlanet
	Resources       []EmpireResourceRow
	Buildings       []EmpireLevelRow
	Research        []EmpireLevelRow
	Fleet           []EmpireCountRow
	Defense         []EmpireCountRow
}

type EmpirePlanet struct {
	ID          int
	Name        string
	Type        int
	Coordinates Coordinates
	Fields      int
	MaxFields   int
	Resources   Resources
	Production  EmpireProduction
	Levels      BuildingLevels
	BuildQueue  map[int][]EmpireBuildQueueEntry
	Fleet       FleetCounts
	Defense     DefenseCounts
}

type EmpireBuildQueueEntry struct {
	ListID   int
	Level    int
	Active   bool
	Demolish bool
}

type EmpireProduction struct {
	MetalHourly     int
	CrystalHourly   int
	DeuteriumHourly int
	EnergyBalance   int
	EnergyCapacity  int
}

type EmpireResourceRow struct {
	ID         int
	Name       string
	Values     []EmpireResourceValue
	Total      int
	Production int
}

type EmpireResourceValue struct {
	PlanetID   int
	Amount     int
	Production int
}

type EmpireLevelRow struct {
	ID      int
	Name    string
	Values  []EmpireLevelValue
	Total   int
	Average float64
}

type EmpireLevelValue struct {
	PlanetID int
	Level    int
	Queue    []EmpireBuildQueueEntry
}

type EmpireCountRow struct {
	ID     int
	Name   string
	Values []EmpireCountValue
	Total  int
}

type EmpireCountValue struct {
	PlanetID int
	Count    int
}

type EmpireActionIssue struct {
	Code    string
	Message string
}

func BuildEmpire(
	overview Overview,
	commanderActive bool,
	planetType int,
	moonEnabled bool,
	hasMoons bool,
	planets []EmpirePlanet,
	research ResearchLevels,
) Empire {
	planetType = NormalizeEmpirePlanetType(planetType, moonEnabled)
	return Empire{
		Commander:       overview.Commander,
		CommanderActive: commanderActive,
		CurrentPlanet:   overview.CurrentPlanet,
		PlanetSwitcher:  overview.PlanetSwitcher,
		PlanetType:      planetType,
		MoonEnabled:     moonEnabled,
		HasMoons:        hasMoons,
		Planets:         planets,
		Resources:       buildEmpireResourceRows(planets),
		Buildings:       buildEmpireLevelRows(BuildingIDs(), planets, nil, true),
		Research:        buildEmpireLevelRows(ResearchIDs(), planets, research, false),
		Fleet:           buildEmpireCountRows(FleetIDs(), planets, "fleet"),
		Defense:         buildEmpireCountRows(DefenseIDs(), planets, "defense"),
	}
}

func NormalizeEmpirePlanetType(planetType int, moonEnabled bool) int {
	if moonEnabled && planetType == EmpirePlanetTypeMoons {
		return EmpirePlanetTypeMoons
	}
	return EmpirePlanetTypePlanets
}

func EmpireActionIssueFor(code string) *EmpireActionIssue {
	message := map[string]string{
		EmpireIssueCommanderRequired: "Commander is required to view the Empire overview.",
	}[code]
	if message == "" {
		message = "The Empire overview could not be displayed."
	}
	return &EmpireActionIssue{Code: code, Message: message}
}

func TechnologyName(id int) string {
	return technologyName(id)
}

func buildEmpireResourceRows(planets []EmpirePlanet) []EmpireResourceRow {
	return []EmpireResourceRow{
		buildEmpireResourceRow(ResourceMetal, "Metal", planets, func(planet EmpirePlanet) (int, int) {
			return floorResource(planet.Resources.Metal), planet.Production.MetalHourly
		}),
		buildEmpireResourceRow(ResourceCrystal, "Crystal", planets, func(planet EmpirePlanet) (int, int) {
			return floorResource(planet.Resources.Crystal), planet.Production.CrystalHourly
		}),
		buildEmpireResourceRow(ResourceDeuterium, "Deuterium", planets, func(planet EmpirePlanet) (int, int) {
			return floorResource(planet.Resources.Deuterium), planet.Production.DeuteriumHourly
		}),
		buildEmpireResourceRow(ResourceEnergy, "Energy", planets, func(planet EmpirePlanet) (int, int) {
			return planet.Production.EnergyBalance, planet.Production.EnergyCapacity
		}),
	}
}

func buildEmpireResourceRow(id int, name string, planets []EmpirePlanet, value func(EmpirePlanet) (int, int)) EmpireResourceRow {
	values := make([]EmpireResourceValue, 0, len(planets))
	total := 0
	production := 0
	for _, planet := range planets {
		amount, hourly := value(planet)
		total += amount
		production += hourly
		values = append(values, EmpireResourceValue{PlanetID: planet.ID, Amount: amount, Production: hourly})
	}
	if id != ResourceEnergy && len(planets) > 0 {
		production = int(math.Ceil(float64(production) / float64(len(planets))))
	}
	return EmpireResourceRow{ID: id, Name: name, Values: values, Total: total, Production: production}
}

func buildEmpireLevelRows(ids []int, planets []EmpirePlanet, research ResearchLevels, skipZero bool) []EmpireLevelRow {
	rows := make([]EmpireLevelRow, 0, len(ids))
	for _, id := range ids {
		values := make([]EmpireLevelValue, 0, len(planets))
		total := 0
		for _, planet := range planets {
			level := planet.Levels[id]
			queue := planet.BuildQueue[id]
			if research != nil {
				level = research[id]
				queue = nil
			}
			total += level
			values = append(values, EmpireLevelValue{PlanetID: planet.ID, Level: level, Queue: queue})
		}
		if total == 0 && skipZero {
			continue
		}
		if research != nil && total == 0 {
			continue
		}
		average := 0.0
		if len(planets) > 0 {
			average = math.Round((float64(total)/float64(len(planets)))*100) / 100
		}
		if research != nil && len(planets) > 0 {
			total = values[0].Level
			average = float64(total)
		}
		rows = append(rows, EmpireLevelRow{ID: id, Name: TechnologyName(id), Values: values, Total: total, Average: average})
	}
	return rows
}

func buildEmpireCountRows(ids []int, planets []EmpirePlanet, source string) []EmpireCountRow {
	rows := make([]EmpireCountRow, 0, len(ids))
	for _, id := range ids {
		values := make([]EmpireCountValue, 0, len(planets))
		total := 0
		for _, planet := range planets {
			count := planet.Fleet[id]
			if source == "defense" {
				count = planet.Defense[id]
			}
			total += count
			values = append(values, EmpireCountValue{PlanetID: planet.ID, Count: count})
		}
		if total == 0 {
			continue
		}
		rows = append(rows, EmpireCountRow{ID: id, Name: TechnologyName(id), Values: values, Total: total})
	}
	return rows
}

func floorResource(value float64) int {
	if value <= 0 {
		return 0
	}
	return int(math.Floor(value))
}
