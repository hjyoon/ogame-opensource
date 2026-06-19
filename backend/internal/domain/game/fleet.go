package game

import "math"

const (
	FleetMissionAttack         = 1
	FleetMissionACSAttack      = 2
	FleetMissionTransport      = 3
	FleetMissionDeploy         = 4
	FleetMissionACSHold        = 5
	FleetMissionSpy            = 6
	FleetMissionColonize       = 7
	FleetMissionRecycle        = 8
	FleetMissionDestroy        = 9
	FleetMissionExpedition     = 15
	FleetMissionMissile        = 20
	FleetMissionACSAttackHead  = 21
	FleetMissionReturnOffset   = 100
	FleetMissionOrbitingOffset = 200
	FleetMissionCustomOffset   = 1000
)

type Fleet struct {
	Commander       string
	CommanderActive bool
	CurrentPlanet   PlanetOverview
	PlanetSwitcher  []PlanetSummary
	Slots           FleetSlots
	Expeditions     ExpeditionSlots
	Missions        []FleetMission
	Ships           []FleetShipSelection
	TemplateLimit   int
	Templates       []FleetTemplate
	DispatchDraft   *FleetDispatchDraft
}

type FleetSlots struct {
	Used    int
	Max     int
	BaseMax int
	Admiral bool
}

type ExpeditionSlots struct {
	Used int
	Max  int
}

type FleetMission struct {
	ID              int
	Mission         int
	MissionName     string
	StateTitle      string
	StateShort      string
	Ships           []FleetShipCount
	TotalShips      int
	Origin          Coordinates
	Target          Coordinates
	TargetType      int
	TargetOwnerName string
	DepartureAt     int64
	ArrivalAt       int64
	CanRecall       bool
	CanCreateUnion  bool
}

type FleetShipCount struct {
	ID    int
	Name  string
	Count int
}

type FleetShipSelection struct {
	ID          int
	Name        string
	Count       int
	Speed       int
	Cargo       int
	Consumption int
	Selectable  bool
}

type FleetDispatchDraftInput struct {
	Ships      map[int]int
	Target     Coordinates
	TargetType int
	Mission    int
	Speed      int
}

type FleetDispatchDraft struct {
	Ships           []FleetShipCount
	TotalShips      int
	Target          Coordinates
	TargetType      int
	Mission         int
	Speed           int
	Cargo           int
	HasSelection    bool
	MissionOptions  []FleetMissionOption
	Resources       []FleetResourceLoad
	HoldHours       []int
	ExpeditionHours []int
}

type FleetMissionOption struct {
	ID       int
	Name     string
	Selected bool
	Warning  string
}

type FleetResourceLoad struct {
	ID        int
	Name      string
	Available int
}

func BuildFleet(overview Overview, counts FleetCounts, research ResearchLevels, missions []FleetMission, admiral bool, acsEnabled bool) Fleet {
	baseMax := research[ResearchComputer] + 1
	maxFleet := baseMax
	if admiral {
		maxFleet += 2
	}

	normalizedMissions := make([]FleetMission, 0, len(missions))
	expeditions := 0
	for _, mission := range missions {
		baseMission, title, short := fleetMissionDisplay(mission.Mission)
		mission.MissionName = fleetMissionName(baseMission)
		mission.StateTitle = title
		mission.StateShort = short
		mission.TotalShips = fleetTotalShips(mission.Ships)
		mission.CanRecall = mission.Mission < FleetMissionReturnOffset || mission.Mission > FleetMissionOrbitingOffset
		mission.CanCreateUnion = acsEnabled && (mission.Mission == FleetMissionAttack || mission.Mission == FleetMissionACSAttackHead)
		if baseMission == FleetMissionExpedition {
			expeditions++
		}
		normalizedMissions = append(normalizedMissions, mission)
	}

	return Fleet{
		Commander:      overview.Commander,
		CurrentPlanet:  overview.CurrentPlanet,
		PlanetSwitcher: overview.PlanetSwitcher,
		Slots: FleetSlots{
			Used:    len(normalizedMissions),
			Max:     maxFleet,
			BaseMax: baseMax,
			Admiral: admiral,
		},
		Expeditions: ExpeditionSlots{
			Used: expeditions,
			Max:  int(math.Floor(math.Sqrt(float64(research[ResearchExpedition])))),
		},
		Missions: normalizedMissions,
		Ships:    buildFleetShipSelections(counts, research),
	}
}

func BuildFleetDispatchDraft(fleet Fleet, input FleetDispatchDraftInput) FleetDispatchDraft {
	speed := input.Speed
	if speed < 1 {
		speed = 10
	}
	if speed > 10 {
		speed = 10
	}
	targetType := input.TargetType
	if targetType <= 0 {
		targetType = GamePlanetTypePlanet
	}
	target := input.Target
	if target.Galaxy <= 0 {
		target.Galaxy = fleet.CurrentPlanet.Coordinates.Galaxy
	}
	if target.System <= 0 {
		target.System = fleet.CurrentPlanet.Coordinates.System
	}
	if target.Position <= 0 {
		target.Position = fleet.CurrentPlanet.Coordinates.Position
	}

	ships := make([]FleetShipCount, 0, len(fleet.Ships))
	selectedCounts := make(FleetCounts, len(fleet.Ships))
	total := 0
	cargo := 0
	for _, available := range fleet.Ships {
		if available.ID == FleetSolarSatellite || !available.Selectable {
			continue
		}
		count := input.Ships[available.ID]
		if count < 0 {
			count = 0
		}
		if count > available.Count {
			count = available.Count
		}
		if count <= 0 {
			continue
		}
		ships = append(ships, FleetShipCount{ID: available.ID, Name: available.Name, Count: count})
		selectedCounts[available.ID] = count
		total += count
		if available.ID != FleetEspionageProbe {
			cargo += available.Cargo * count
		}
	}
	missions := fleetDispatchMissionOptions(fleet, selectedCounts, target, targetType, input.Mission)
	selectedMission := input.Mission
	if !missionOptionExists(missions, selectedMission) && len(missions) > 0 {
		selectedMission = missions[0].ID
		missions[0].Selected = true
	}

	return FleetDispatchDraft{
		Ships:           ships,
		TotalShips:      total,
		Target:          target,
		TargetType:      targetType,
		Mission:         selectedMission,
		Speed:           speed,
		Cargo:           cargo,
		HasSelection:    total > 0,
		MissionOptions:  missions,
		Resources:       fleetDispatchResources(fleet.CurrentPlanet.Resources),
		HoldHours:       fleetDispatchHoldHours(missions),
		ExpeditionHours: fleetDispatchExpeditionHours(fleet.Expeditions.Max),
	}
}

func fleetDispatchMissionOptions(fleet Fleet, counts FleetCounts, target Coordinates, targetType int, requested int) []FleetMissionOption {
	ids := make([]int, 0, 5)
	if target.Position >= GalaxyFarSpace {
		ids = append(ids, FleetMissionExpedition)
	} else if targetType == GamePlanetTypeDebris {
		if counts[FleetRecycler] > 0 {
			ids = append(ids, FleetMissionRecycle)
		}
	} else if target == fleet.CurrentPlanet.Coordinates && targetType == gamePlanetTypeFromPlanet(fleet.CurrentPlanet.Type) {
		ids = append(ids, FleetMissionTransport, FleetMissionDeploy)
	} else {
		ids = append(ids, FleetMissionTransport, FleetMissionAttack)
		if counts[FleetColonyShip] > 0 && targetType == GamePlanetTypePlanet {
			ids = append(ids, FleetMissionColonize)
		}
		if counts[FleetDeathstar] > 0 && targetType == GamePlanetTypeMoon {
			ids = append(ids, FleetMissionDestroy)
		}
		if counts[FleetEspionageProbe] > 0 {
			ids = append(ids, FleetMissionSpy)
		}
	}

	options := make([]FleetMissionOption, 0, len(ids))
	for _, id := range ids {
		option := FleetMissionOption{
			ID:       id,
			Name:     fleetMissionName(id),
			Selected: id == requested,
		}
		if id == FleetMissionExpedition {
			option.Warning = "WARNING: Expedition is a very risky mission, not meant to be a save."
		}
		options = append(options, option)
	}
	return options
}

func missionOptionExists(options []FleetMissionOption, mission int) bool {
	for _, option := range options {
		if option.ID == mission {
			return true
		}
	}
	return false
}

func fleetDispatchResources(resources Resources) []FleetResourceLoad {
	return []FleetResourceLoad{
		{ID: ResourceMetal, Name: "Metal", Available: max(0, int(resources.Metal))},
		{ID: ResourceCrystal, Name: "Crystal", Available: max(0, int(resources.Crystal))},
		{ID: ResourceDeuterium, Name: "Deuterium", Available: max(0, int(resources.Deuterium))},
	}
}

func fleetDispatchExpeditionHours(maxExpeditions int) []int {
	if maxExpeditions < 1 {
		return nil
	}
	hours := make([]int, 0, maxExpeditions)
	for hour := 1; hour <= maxExpeditions; hour++ {
		hours = append(hours, hour)
	}
	return hours
}

func fleetDispatchHoldHours(options []FleetMissionOption) []int {
	if !missionOptionExists(options, FleetMissionACSHold) {
		return nil
	}
	return []int{0, 1, 2, 4, 8, 16, 32}
}

func gamePlanetTypeFromPlanet(planetType int) int {
	if planetType == PlanetTypeMoon {
		return GamePlanetTypeMoon
	}
	return GamePlanetTypePlanet
}

func BuildFleetMission(id int, mission int, ships FleetCounts, origin Coordinates, target Coordinates, targetType int, targetOwner string, departureAt int64, arrivalAt int64) FleetMission {
	shipCounts := make([]FleetShipCount, 0, len(FleetIDs()))
	for _, fleetID := range FleetIDs() {
		count := ships[fleetID]
		if count <= 0 {
			continue
		}
		shipCounts = append(shipCounts, FleetShipCount{
			ID:    fleetID,
			Name:  fleetName(fleetID),
			Count: count,
		})
	}
	return FleetMission{
		ID:              id,
		Mission:         mission,
		Ships:           shipCounts,
		Origin:          origin,
		Target:          target,
		TargetType:      targetType,
		TargetOwnerName: targetOwner,
		DepartureAt:     departureAt,
		ArrivalAt:       arrivalAt,
	}
}

func buildFleetShipSelections(counts FleetCounts, research ResearchLevels) []FleetShipSelection {
	ships := make([]FleetShipSelection, 0, len(FleetIDs()))
	for _, id := range FleetIDs() {
		count := counts[id]
		if count <= 0 {
			continue
		}
		speed := fleetShipSpeed(id, research)
		ships = append(ships, FleetShipSelection{
			ID:          id,
			Name:        fleetName(id),
			Count:       count,
			Speed:       speed,
			Cargo:       fleetShipCargo(id),
			Consumption: fleetShipConsumption(id, research),
			Selectable:  speed > 0,
		})
	}
	return ships
}

func fleetTotalShips(ships []FleetShipCount) int {
	total := 0
	for _, ship := range ships {
		total += ship.Count
	}
	return total
}

func fleetMissionDisplay(mission int) (int, string, string) {
	if mission >= FleetMissionCustomOffset {
		return mission, "Custom task", "(C)"
	}
	if mission >= FleetMissionOrbitingOffset {
		return mission - FleetMissionOrbitingOffset, "On the planet", "(H)"
	}
	if mission >= FleetMissionReturnOffset {
		return mission - FleetMissionReturnOffset, "Fleet Returns home", "(F)"
	}
	return mission, "Going on a mission", "(G)"
}

func fleetMissionName(mission int) string {
	switch mission {
	case FleetMissionAttack, FleetMissionACSAttackHead:
		return "Attack"
	case FleetMissionACSAttack:
		return "Joint attack"
	case FleetMissionTransport:
		return "Transport"
	case FleetMissionDeploy:
		return "Station"
	case FleetMissionACSHold:
		return "Defend"
	case FleetMissionSpy:
		return "Espionage"
	case FleetMissionColonize:
		return "Colonise"
	case FleetMissionRecycle:
		return "Recycle"
	case FleetMissionDestroy:
		return "Destroy"
	case FleetMissionExpedition:
		return "Expedition"
	case FleetMissionMissile:
		return "Missile Attack"
	default:
		return "Custom task"
	}
}

func fleetName(id int) string {
	for _, spec := range fleetCatalog {
		if spec.id == id {
			return spec.name
		}
	}
	return ""
}

type fleetUnitParam struct {
	cargo       int
	speed       int
	consumption int
}

var fleetUnitParams = map[int]fleetUnitParam{
	FleetSmallCargo:     {cargo: 5000, speed: 5000, consumption: 10},
	FleetLargeCargo:     {cargo: 25000, speed: 7500, consumption: 50},
	FleetLightFighter:   {cargo: 50, speed: 12500, consumption: 20},
	FleetHeavyFighter:   {cargo: 100, speed: 10000, consumption: 75},
	FleetCruiser:        {cargo: 800, speed: 15000, consumption: 300},
	FleetBattleship:     {cargo: 1500, speed: 10000, consumption: 500},
	FleetColonyShip:     {cargo: 7500, speed: 2500, consumption: 1000},
	FleetRecycler:       {cargo: 20000, speed: 2000, consumption: 300},
	FleetEspionageProbe: {cargo: 5, speed: 100000000, consumption: 1},
	FleetBomber:         {cargo: 500, speed: 4000, consumption: 1000},
	FleetSolarSatellite: {cargo: 0, speed: 0, consumption: 0},
	FleetDestroyer:      {cargo: 2000, speed: 5000, consumption: 1000},
	FleetDeathstar:      {cargo: 1000000, speed: 100, consumption: 1},
	FleetBattlecruiser:  {cargo: 750, speed: 10000, consumption: 250},
}

func fleetShipSpeed(id int, research ResearchLevels) int {
	params := fleetUnitParams[id]
	base := float64(params.speed)
	combustion := float64(research[ResearchCombustionDrive])
	impulse := float64(research[ResearchImpulseDrive])
	hyper := float64(research[ResearchHyperspaceDrive])

	switch id {
	case FleetSmallCargo:
		if research[ResearchImpulseDrive] >= 5 {
			return int((base + 5000) * (1 + 0.2*impulse))
		}
		return int(base * (1 + 0.1*combustion))
	case FleetBomber:
		if research[ResearchHyperspaceDrive] >= 8 {
			return int((base + 1000) * (1 + 0.3*hyper))
		}
		return int(base * (1 + 0.2*impulse))
	case FleetLargeCargo, FleetLightFighter, FleetRecycler, FleetEspionageProbe, FleetSolarSatellite:
		return int(base * (1 + 0.1*combustion))
	case FleetHeavyFighter, FleetCruiser, FleetColonyShip:
		return int(base * (1 + 0.2*impulse))
	case FleetBattleship, FleetDestroyer, FleetDeathstar, FleetBattlecruiser:
		return int(base * (1 + 0.3*hyper))
	default:
		return int(base)
	}
}

func fleetShipCargo(id int) int {
	return fleetUnitParams[id].cargo
}

func fleetShipConsumption(id int, research ResearchLevels) int {
	consumption := fleetUnitParams[id].consumption
	if id == FleetSmallCargo && research[ResearchImpulseDrive] >= 5 {
		consumption *= 2
	}
	return consumption
}
