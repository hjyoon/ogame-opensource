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
	ExpeditionLevel int
	SpeedFactor     int
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
	OwnerID         int
	OwnerName       string
	Foreign         bool
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

type FleetDispatchValidationInput struct {
	Ships           map[int]int
	Resources       map[int]int
	Target          Coordinates
	TargetType      int
	Mission         int
	Speed           int
	HoldHours       int
	ExpeditionHours int
	UnionID         int
}

type FleetDispatchDraft struct {
	Ships           []FleetShipCount
	TotalShips      int
	Target          Coordinates
	TargetType      int
	Mission         int
	Speed           int
	Cargo           int
	Distance        int
	DurationSeconds int
	MaxSpeed        int
	FuelConsumption int
	SpeedFactor     int
	RemainingCargo  int
	Ready           bool
	HasSelection    bool
	MissionOptions  []FleetMissionOption
	Resources       []FleetResourceLoad
	HoldHours       []int
	ExpeditionHours []int
}

type FleetActionIssue struct {
	Code    string
	Message string
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
	Requested int
	Loaded    int
}

const (
	FleetIssueNoShips       = "no_ships"
	FleetIssueSamePlanet    = "same_planet"
	FleetIssueMaxFleet      = "max_fleet"
	FleetIssueInvalidOrder  = "invalid_order"
	FleetIssueInvalidTarget = "invalid_target"
	FleetIssueNoFuel        = "no_fuel"
	FleetIssueNoCargo       = "no_cargo"
	FleetIssueExpLimit      = "expedition_limit"
	FleetIssueExpRequired   = "expedition_required"
	FleetIssueFrozen        = "frozen"
	FleetIssueLaunchRace    = "launch_race"
)

func BuildFleet(overview Overview, counts FleetCounts, research ResearchLevels, missions []FleetMission, admiral bool, acsEnabled bool) Fleet {
	baseMax := research[ResearchComputer] + 1
	maxFleet := baseMax
	if admiral {
		maxFleet += 2
	}

	normalizedMissions := normalizeFleetMissions(missions, acsEnabled)
	expeditions := 0
	for _, mission := range normalizedMissions {
		baseMission, _, _ := fleetMissionDisplay(mission.Mission)
		if baseMission == FleetMissionExpedition {
			expeditions++
		}
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
		ExpeditionLevel: research[ResearchExpedition],
		SpeedFactor:     1,
		Missions:        normalizedMissions,
		Ships:           buildFleetShipSelections(counts, research),
	}
}

func BuildOverviewEvents(missions []FleetMission) []FleetMission {
	return normalizeFleetMissions(missions, false)
}

func normalizeFleetMissions(missions []FleetMission, acsEnabled bool) []FleetMission {
	normalized := make([]FleetMission, 0, len(missions))
	for _, mission := range missions {
		baseMission, title, short := fleetMissionDisplay(mission.Mission)
		mission.MissionName = fleetMissionName(baseMission)
		mission.StateTitle = title
		mission.StateShort = short
		mission.TotalShips = fleetTotalShips(mission.Ships)
		mission.CanRecall = !mission.Foreign && (mission.Mission < FleetMissionReturnOffset || mission.Mission > FleetMissionOrbitingOffset)
		mission.CanCreateUnion = !mission.Foreign && acsEnabled && (mission.Mission == FleetMissionAttack || mission.Mission == FleetMissionACSAttackHead)
		normalized = append(normalized, mission)
	}
	return normalized
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
	speedFactor := fleet.SpeedFactor
	if speedFactor <= 0 {
		speedFactor = 1
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
	distance := fleetFlightDistance(fleet.CurrentPlanet.Coordinates, target)
	maxSpeed := fleetDispatchMaxSpeed(fleet.Ships, selectedCounts)
	durationSeconds := 0
	fuelConsumption := 0
	if total > 0 && maxSpeed > 0 {
		durationSeconds = fleetFlightTime(distance, maxSpeed, speed, speedFactor)
		fuelConsumption = fleetFlightConsumption(fleet.Ships, selectedCounts, distance, durationSeconds, speedFactor, 0)
	}

	return FleetDispatchDraft{
		Ships:           ships,
		TotalShips:      total,
		Target:          target,
		TargetType:      targetType,
		Mission:         selectedMission,
		Speed:           speed,
		Cargo:           cargo,
		Distance:        distance,
		DurationSeconds: durationSeconds,
		MaxSpeed:        maxSpeed,
		FuelConsumption: fuelConsumption,
		SpeedFactor:     speedFactor,
		HasSelection:    total > 0,
		MissionOptions:  missions,
		Resources:       fleetDispatchResources(fleet.CurrentPlanet.Resources),
		HoldHours:       fleetDispatchHoldHours(missions),
		ExpeditionHours: fleetDispatchExpeditionHours(fleet.ExpeditionLevel),
	}
}

func BuildFleetDispatchValidation(fleet Fleet, input FleetDispatchValidationInput) (FleetDispatchDraft, *FleetActionIssue) {
	draft := BuildFleetDispatchDraft(fleet, FleetDispatchDraftInput{
		Ships:      input.Ships,
		Target:     input.Target,
		TargetType: input.TargetType,
		Mission:    input.Mission,
		Speed:      input.Speed,
	})
	selectedCounts := fleetCountsFromShipCounts(draft.Ships)
	holdHours := NormalizeFleetHoldHours(input.Mission, input.HoldHours, input.ExpeditionHours, fleet.ExpeditionLevel)
	fleetFuel, probeFuel := fleetFlightConsumptionParts(fleet.Ships, selectedCounts, draft.Distance, draft.DurationSeconds, draft.SpeedFactor, holdHours)
	draft.FuelConsumption = fleetFuel + probeFuel
	cargoSpace := draft.Cargo - fleetFuel
	resourceRows, remainingCargo := fleetDispatchResourcePlan(fleet.CurrentPlanet.Resources, input.Resources, cargoSpace)
	draft.Resources = resourceRows
	draft.RemainingCargo = remainingCargo

	if !draft.HasSelection {
		return draft, FleetActionIssueFor(FleetIssueNoShips)
	}
	if draft.Target == fleet.CurrentPlanet.Coordinates && draft.TargetType == gamePlanetTypeFromPlanet(fleet.CurrentPlanet.Type) {
		return draft, FleetActionIssueFor(FleetIssueSamePlanet)
	}
	if fleet.Slots.Max > 0 && fleet.Slots.Used >= fleet.Slots.Max {
		return draft, FleetActionIssueFor(FleetIssueMaxFleet)
	}
	if input.Mission <= 0 || !missionOptionExists(draft.MissionOptions, input.Mission) {
		return draft, FleetActionIssueFor(FleetIssueInvalidOrder)
	}
	if input.Mission == FleetMissionExpedition {
		if draft.TargetType != GamePlanetTypePlanet || draft.Target.Position != GalaxyFarSpace {
			return draft, FleetActionIssueFor(FleetIssueInvalidTarget)
		}
		if fleet.Expeditions.Max <= 0 || fleet.Expeditions.Used >= fleet.Expeditions.Max {
			return draft, FleetActionIssueFor(FleetIssueExpLimit)
		}
		if !fleetHasMannedShips(selectedCounts) {
			return draft, FleetActionIssueFor(FleetIssueExpRequired)
		}
	}
	if int(fleet.CurrentPlanet.Resources.Deuterium) < draft.FuelConsumption {
		return draft, FleetActionIssueFor(FleetIssueNoFuel)
	}
	if cargoSpace < 0 {
		return draft, FleetActionIssueFor(FleetIssueNoCargo)
	}
	draft.Ready = true
	return draft, nil
}

func FleetActionIssueFor(code string) *FleetActionIssue {
	message := map[string]string{
		FleetIssueNoShips:       "No ships have been selected.",
		FleetIssueSamePlanet:    "Origin and target are the same planet.",
		FleetIssueMaxFleet:      "Maximum fleet size has been reached.",
		FleetIssueInvalidOrder:  "No suitable mission.",
		FleetIssueInvalidTarget: "Target not found.",
		FleetIssueNoFuel:        "Not enough deuterium.",
		FleetIssueNoCargo:       "Not enough cargo capacity.",
		FleetIssueExpLimit:      "Maximum number of expeditions has been reached.",
		FleetIssueExpRequired:   "An expedition needs at least one crewed ship.",
		FleetIssueFrozen:        "The universe is currently frozen.",
		FleetIssueLaunchRace:    "Selected ships or resources are no longer available.",
	}[code]
	if message == "" {
		message = "Fleet dispatch failed."
	}
	return &FleetActionIssue{Code: code, Message: message}
}

func NormalizeFleetHoldHours(mission int, holdHours int, expeditionHours int, expeditionLevel int) int {
	switch mission {
	case FleetMissionExpedition:
		hours := expeditionHours
		if hours < 1 {
			hours = 1
		}
		if expeditionLevel > 0 && hours > expeditionLevel {
			hours = expeditionLevel
		}
		return hours
	case FleetMissionACSHold:
		hours := holdHours
		if hours < 0 {
			hours = 0
		}
		if hours > 32 {
			hours = 32
		}
		return hours
	default:
		return 0
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
	rows, _ := fleetDispatchResourcePlan(resources, nil, 0)
	return rows
}

func fleetDispatchResourcePlan(resources Resources, requested map[int]int, capacity int) ([]FleetResourceLoad, int) {
	remaining := max(0, capacity)
	rows := []FleetResourceLoad{
		{ID: ResourceMetal, Name: "Metal", Available: max(0, int(resources.Metal))},
		{ID: ResourceCrystal, Name: "Crystal", Available: max(0, int(resources.Crystal))},
		{ID: ResourceDeuterium, Name: "Deuterium", Available: max(0, int(resources.Deuterium))},
	}
	for index := range rows {
		request := 0
		if requested != nil {
			request = requested[rows[index].ID]
		}
		if request < 0 {
			request = 0
		}
		if request > rows[index].Available {
			request = rows[index].Available
		}
		rows[index].Requested = request
		rows[index].Loaded = min(request, remaining)
		remaining -= rows[index].Loaded
	}
	return rows, remaining
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

func fleetFlightDistance(origin Coordinates, target Coordinates) int {
	if origin.Galaxy == target.Galaxy {
		if origin.System == target.System {
			if origin.Position == target.Position {
				return 5
			}
			return int(math.Abs(float64(target.Position-origin.Position)))*5 + 1000
		}
		return int(math.Abs(float64(target.System-origin.System)))*5*19 + 2700
	}
	return int(math.Abs(float64(target.Galaxy-origin.Galaxy))) * 20000
}

func fleetDispatchMaxSpeed(ships []FleetShipSelection, counts FleetCounts) int {
	maxSpeed := 0
	for _, ship := range ships {
		if counts[ship.ID] <= 0 || ship.Speed <= 0 {
			continue
		}
		if maxSpeed == 0 || ship.Speed < maxSpeed {
			maxSpeed = ship.Speed
		}
	}
	return maxSpeed
}

func fleetFlightTime(distance int, slowestSpeed int, speed int, speedFactor int) int {
	if distance <= 0 || slowestSpeed <= 0 {
		return 0
	}
	if speed < 1 {
		speed = 10
	}
	if speed > 10 {
		speed = 10
	}
	if speedFactor <= 0 {
		speedFactor = 1
	}
	seconds := (35000/float64(speed*10)*math.Sqrt(float64(distance*10)/float64(slowestSpeed)) + 10) / float64(speedFactor)
	return int(math.Round(seconds))
}

func fleetFlightConsumption(ships []FleetShipSelection, counts FleetCounts, distance int, flightTime int, speedFactor int, holdHours int) int {
	fleetFuel, probeFuel := fleetFlightConsumptionParts(ships, counts, distance, flightTime, speedFactor, holdHours)
	return fleetFuel + probeFuel
}

func fleetFlightConsumptionParts(ships []FleetShipSelection, counts FleetCounts, distance int, flightTime int, speedFactor int, holdHours int) (int, int) {
	if distance <= 0 || flightTime <= 0 {
		return 0, 0
	}
	if speedFactor <= 0 {
		speedFactor = 1
	}
	denominator := float64(flightTime*speedFactor - 10)
	if denominator <= 0 {
		return 0, 0
	}
	fleetFuel := 0
	probeFuel := 0
	for _, ship := range ships {
		amount := counts[ship.ID]
		if amount <= 0 || ship.Speed <= 0 || ship.Consumption <= 0 {
			continue
		}
		fleetSpeed := 35000 / denominator * math.Sqrt(float64(distance*10)/float64(ship.Speed))
		baseConsumption := float64(amount * ship.Consumption)
		consumption := baseConsumption * float64(distance) / 35000 * math.Pow(fleetSpeed/10+1, 2)
		consumption += float64(holdHours*amount*ship.Consumption) / 10
		if ship.ID == FleetEspionageProbe {
			probeFuel += int(consumption)
		} else {
			fleetFuel += int(consumption)
		}
	}
	return fleetFuel, probeFuel
}

func fleetCountsFromShipCounts(ships []FleetShipCount) FleetCounts {
	counts := make(FleetCounts, len(ships))
	for _, ship := range ships {
		counts[ship.ID] = ship.Count
	}
	return counts
}

func fleetHasMannedShips(counts FleetCounts) bool {
	for id, count := range counts {
		if count > 0 && id != FleetEspionageProbe {
			return true
		}
	}
	return false
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
