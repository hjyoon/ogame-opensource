package game

import (
	"math"
	"sort"
)

type Technology struct {
	Commander      string
	CurrentPlanet  PlanetOverview
	PlanetSwitcher []PlanetSummary
	Groups         []TechnologyGroup
	Details        *TechnologyDetails
	Info           *TechnologyInfo
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

type TechnologyDetails struct {
	Target   TechnologyItem
	Levels   []TechnologyDetailsLevel
	Demolish *TechnologyDemolish
}

type TechnologyDetailsLevel struct {
	Step         int
	Requirements []TechnologyRequirement
}

type TechnologyDemolish struct {
	Level           int
	Cost            BuildingCost
	DurationSeconds int
}

type TechnologyInfo struct {
	ID            int
	Name          string
	Description   string
	Level         int
	Kind          string
	Rows          []TechnologyInfoRow
	Unit          *TechnologyUnitInfo
	AllianceDepot *TechnologyAllianceDepotInfo
	Demolish      *TechnologyDemolish
}

type TechnologyAllianceDepotInfo struct {
	AvailableDeuterium int
	Capacity           int
}

type TechnologyUnitInfo struct {
	Structure                int
	Shield                   int
	Attack                   int
	Cargo                    int
	BaseSpeed                int
	AlternateBaseSpeed       int
	BaseConsumption          int
	AlternateBaseConsumption int
	DefenseRepair            int
	RapidFireOut             []TechnologyRapidFire
	RapidFireIn              []TechnologyRapidFire
}

type TechnologyRapidFire struct {
	ID    int
	Name  string
	Count int
}

type TechnologyInfoRow struct {
	Level                int
	Current              bool
	Production           int
	ProductionDifference int
	Energy               int
	EnergyDifference     int
	Storage              int
	StorageDifference    int
	DeuteriumConsumption int
	DeuteriumDifference  int
	Radius               int
}

var legacyTechnologyRequirementOrder = map[int][]int{
	BuildingFusionReactor:        {BuildingDeuteriumSynth, ResearchEnergy},
	BuildingNaniteFactory:        {BuildingRoboticsFactory, ResearchComputer},
	BuildingShipyard:             {BuildingRoboticsFactory},
	BuildingTerraformer:          {BuildingNaniteFactory, ResearchEnergy},
	BuildingMissileSilo:          {BuildingShipyard},
	ResearchEspionage:            {BuildingResearchLab},
	ResearchComputer:             {BuildingResearchLab},
	ResearchWeapon:               {BuildingResearchLab},
	ResearchShield:               {ResearchEnergy, BuildingResearchLab},
	ResearchArmour:               {BuildingResearchLab},
	ResearchEnergy:               {BuildingResearchLab},
	ResearchHyperspace:           {ResearchEnergy, ResearchShield, BuildingResearchLab},
	ResearchCombustionDrive:      {ResearchEnergy},
	ResearchImpulseDrive:         {ResearchEnergy, BuildingResearchLab},
	ResearchHyperspaceDrive:      {ResearchHyperspace},
	ResearchLaser:                {ResearchEnergy},
	ResearchIon:                  {BuildingResearchLab, ResearchLaser, ResearchEnergy},
	ResearchPlasma:               {ResearchEnergy, ResearchLaser, ResearchIon},
	ResearchIntergalacticNetwork: {BuildingResearchLab, ResearchComputer, ResearchHyperspace},
	ResearchExpedition:           {ResearchEspionage, ResearchImpulseDrive},
	ResearchGraviton:             {BuildingResearchLab},
	FleetSmallCargo:              {BuildingShipyard, ResearchCombustionDrive},
	FleetLargeCargo:              {BuildingShipyard, ResearchCombustionDrive},
	FleetLightFighter:            {BuildingShipyard, ResearchCombustionDrive},
	FleetHeavyFighter:            {BuildingShipyard, ResearchArmour, ResearchImpulseDrive},
	FleetCruiser:                 {BuildingShipyard, ResearchImpulseDrive, ResearchIon},
	FleetBattleship:              {BuildingShipyard, ResearchHyperspaceDrive},
	FleetColonyShip:              {BuildingShipyard, ResearchImpulseDrive},
	FleetRecycler:                {BuildingShipyard, ResearchCombustionDrive, ResearchShield},
	FleetEspionageProbe:          {BuildingShipyard, ResearchCombustionDrive, ResearchEspionage},
	FleetBomber:                  {ResearchImpulseDrive, BuildingShipyard, ResearchPlasma},
	FleetSolarSatellite:          {BuildingShipyard},
	FleetDestroyer:               {BuildingShipyard, ResearchHyperspaceDrive, ResearchHyperspace},
	FleetDeathstar:               {BuildingShipyard, ResearchHyperspaceDrive, ResearchHyperspace, ResearchGraviton},
	FleetBattlecruiser:           {ResearchHyperspace, ResearchLaser, ResearchHyperspaceDrive, BuildingShipyard},
	DefenseRocketLauncher:        {BuildingShipyard},
	DefenseLightLaser:            {ResearchEnergy, BuildingShipyard, ResearchLaser},
	DefenseHeavyLaser:            {ResearchEnergy, BuildingShipyard, ResearchLaser},
	DefenseGaussCannon:           {BuildingShipyard, ResearchEnergy, ResearchWeapon, ResearchShield},
	DefenseIonCannon:             {BuildingShipyard, ResearchIon},
	DefensePlasmaTurret:          {BuildingShipyard, ResearchPlasma},
	DefenseSmallShieldDome:       {ResearchShield, BuildingShipyard},
	DefenseLargeShieldDome:       {ResearchShield, BuildingShipyard},
	DefenseAntiBallisticMissile:  {BuildingMissileSilo, BuildingShipyard},
	DefenseInterplanetaryMissile: {BuildingMissileSilo, BuildingShipyard, ResearchImpulseDrive},
	BuildingSensorPhalanx:        {BuildingLunarBase},
	BuildingJumpGate:             {BuildingLunarBase, ResearchHyperspace},
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

func BuildTechnologyDetails(id int, levels BuildingLevels, research ResearchLevels) (TechnologyDetails, bool) {
	return BuildTechnologyDetailsWithSpeed(id, levels, research, 1)
}

func BuildTechnologyDetailsWithSpeed(id int, levels BuildingLevels, research ResearchLevels, speed float64) (TechnologyDetails, bool) {
	spec, ok := technologySpecByID(id)
	if !ok {
		return TechnologyDetails{}, false
	}

	tree := newTechnologyRequirementsByDepth()
	maxDepth := walkTechnologyRequirements(id, spec.requirements, 0, tree)
	detailLevels := make([]TechnologyDetailsLevel, 0, maxDepth)
	step := 1
	for depth := maxDepth - 1; depth >= 0; depth-- {
		requirements := buildTechnologyRequirementsOrdered(tree.requirements[depth], tree.order[depth], levels, research)
		if len(requirements) == 0 {
			continue
		}
		detailLevels = append(detailLevels, TechnologyDetailsLevel{
			Step:         step,
			Requirements: requirements,
		})
		step++
	}

	return TechnologyDetails{
		Target: TechnologyItem{
			ID:               id,
			Name:             spec.name,
			Requirements:     buildTechnologyRequirements(id, spec.requirements, levels, research),
			DetailsAvailable: len(spec.requirements) > 0,
		},
		Levels:   detailLevels,
		Demolish: buildTechnologyDemolish(id, levels, speed),
	}, true
}

func BuildTechnologyInfoWithSpeed(id int, planet PlanetOverview, levels BuildingLevels, research ResearchLevels, speed float64) (TechnologyInfo, bool) {
	return BuildTechnologyInfoWithSettings(id, planet, levels, research, speed, 0)
}

func BuildTechnologyInfoWithSettings(id int, planet PlanetOverview, levels BuildingLevels, research ResearchLevels, speed float64, defenseRepair int) (TechnologyInfo, bool) {
	spec, ok := technologySpecByID(id)
	if !ok {
		return TechnologyInfo{}, false
	}
	if speed <= 0 {
		speed = 1
	}
	level := levels[id]
	if isResearchID(id) {
		level = research[id]
	}
	info := TechnologyInfo{
		ID:          id,
		Name:        spec.name,
		Description: legacyTechnologyLongDescription(id, spec.description),
		Level:       level,
		Kind:        technologyInfoKind(id),
		Demolish:    buildTechnologyDemolish(id, levels, speed),
	}
	info.Rows = buildTechnologyInfoRows(id, level, planet, research, speed)
	info.Unit = buildTechnologyUnitInfo(id, defenseRepair)
	info.AllianceDepot = buildTechnologyAllianceDepotInfo(id, level, planet)
	return info, true
}

func technologyInfoKind(id int) string {
	if isFleetID(id) {
		return "fleet"
	}
	if isLegacyDefenseInfoID(id) {
		return "defense"
	}
	switch id {
	case BuildingMetalMine, BuildingCrystalMine, BuildingDeuteriumSynth:
		return "mine"
	case BuildingSolarPlant:
		return "solar"
	case BuildingFusionReactor:
		return "fusion"
	case BuildingMetalStorage, BuildingCrystalStorage, BuildingDeuteriumTank:
		return "storage"
	case BuildingAllianceDepot:
		return "alliance-depot"
	case BuildingSensorPhalanx:
		return "phalanx"
	default:
		return "description"
	}
}

func buildTechnologyAllianceDepotInfo(id int, level int, planet PlanetOverview) *TechnologyAllianceDepotInfo {
	if id != BuildingAllianceDepot {
		return nil
	}
	capacity := 10000 * int(math.Pow(2, float64(max(0, level))))
	available := 0
	if level > 0 {
		available = min(int(math.Floor(max(0, planet.Resources.Deuterium))), capacity)
	}
	return &TechnologyAllianceDepotInfo{AvailableDeuterium: available, Capacity: capacity}
}

func buildTechnologyUnitInfo(id int, defenseRepair int) *TechnologyUnitInfo {
	if !isFleetID(id) && !isLegacyDefenseInfoID(id) {
		return nil
	}
	stats, ok := CombatStatsForUnit(id)
	if !ok {
		return nil
	}
	info := &TechnologyUnitInfo{
		Structure:     stats.Structure,
		Shield:        stats.Shield,
		Attack:        stats.Attack,
		DefenseRepair: defenseRepair,
		RapidFireOut:  technologyRapidFireOut(id),
		RapidFireIn:   technologyRapidFireIn(id),
	}
	if params, fleet := fleetUnitParams[id]; fleet {
		info.Cargo = params.cargo
		info.BaseSpeed = params.speed
		info.BaseConsumption = params.consumption
		switch id {
		case FleetSmallCargo:
			info.AlternateBaseSpeed = params.speed + 5000
			info.AlternateBaseConsumption = params.consumption * 2
		case FleetBomber:
			info.AlternateBaseSpeed = params.speed + 1000
		}
	}
	return info
}

func technologyRapidFireOut(id int) []TechnologyRapidFire {
	result := make([]TechnologyRapidFire, 0)
	for _, targetID := range combatUnitOrder {
		count := combatRapidFire[id][targetID]
		if count > 1 {
			result = append(result, TechnologyRapidFire{ID: targetID, Name: technologyName(targetID), Count: count})
		}
	}
	return result
}

func technologyRapidFireIn(id int) []TechnologyRapidFire {
	result := make([]TechnologyRapidFire, 0)
	for _, sourceID := range combatUnitOrder {
		count := combatRapidFire[sourceID][id]
		if count > 1 {
			result = append(result, TechnologyRapidFire{ID: sourceID, Name: technologyName(sourceID), Count: count})
		}
	}
	return result
}

func buildTechnologyInfoRows(id int, currentLevel int, planet PlanetOverview, research ResearchLevels, speed float64) []TechnologyInfoRow {
	switch id {
	case BuildingMetalMine, BuildingCrystalMine, BuildingDeuteriumSynth:
		start := currentLevel - 2
		if start <= 0 {
			start = 1
		}
		currentProduction, currentEnergy, currentDeuterium := technologyInfoProductionRaw(id, currentLevel, planet, research, speed)
		rows := make([]TechnologyInfoRow, 0, 15)
		for level := start; level < start+15; level++ {
			production, energy, deuterium := technologyInfoProductionRaw(id, level, planet, research, speed)
			rows = append(rows, TechnologyInfoRow{
				Level:                level,
				Current:              level == currentLevel,
				Production:           legacyRoundInt(production),
				ProductionDifference: legacyRoundInt(production - currentProduction),
				Energy:               legacyRoundInt(energy),
				EnergyDifference:     legacyRoundInt(energy - currentEnergy),
				DeuteriumConsumption: legacyRoundInt(deuterium),
				DeuteriumDifference:  legacyRoundInt(deuterium - currentDeuterium),
			})
		}
		return rows
	case BuildingSolarPlant, BuildingFusionReactor:
		start := currentLevel - 2
		if start <= 0 {
			start = 1
		}
		_, currentEnergy, currentDeuterium := technologyInfoProductionRaw(id, currentLevel, planet, research, speed)
		rows := make([]TechnologyInfoRow, 0, 15)
		for level := start; level < start+15; level++ {
			_, energy, deuterium := technologyInfoProductionRaw(id, level, planet, research, speed)
			rows = append(rows, TechnologyInfoRow{
				Level:                level,
				Current:              level == currentLevel,
				Energy:               legacyRoundInt(energy),
				EnergyDifference:     legacyRoundInt(energy - currentEnergy),
				DeuteriumConsumption: legacyRoundInt(deuterium),
				DeuteriumDifference:  legacyRoundInt(deuterium - currentDeuterium),
			})
		}
		return rows
	case BuildingMetalStorage, BuildingCrystalStorage, BuildingDeuteriumTank:
		start := currentLevel
		currentStorage := technologyInfoStorage(currentLevel)
		rows := make([]TechnologyInfoRow, 0, 15)
		for level := start; level < start+15; level++ {
			storage := technologyInfoStorage(level)
			rows = append(rows, TechnologyInfoRow{
				Level:             level,
				Current:           level == currentLevel,
				Storage:           storage / 1000,
				StorageDifference: (storage - currentStorage) / 1000,
			})
		}
		return rows
	case BuildingSensorPhalanx:
		start := currentLevel - 3
		if start <= 0 {
			start = 1
		}
		rows := make([]TechnologyInfoRow, 0, 8)
		for level := start; level < currentLevel+5; level++ {
			rows = append(rows, TechnologyInfoRow{
				Level:   level,
				Current: level == currentLevel,
				Radius:  level*level - 1,
			})
		}
		return rows
	default:
		return nil
	}
}

func technologyInfoProduction(id int, level int, planet PlanetOverview, research ResearchLevels, speed float64) (int, int, int) {
	production, energy, deuterium := technologyInfoProductionRaw(id, level, planet, research, speed)
	return legacyRoundInt(production), legacyRoundInt(energy), legacyRoundInt(deuterium)
}

func technologyInfoProductionRaw(id int, level int, planet PlanetOverview, research ResearchLevels, speed float64) (float64, float64, float64) {
	if level <= 0 {
		return 0, 0, 0
	}
	switch id {
	case BuildingMetalMine:
		production := math.Floor(30*float64(level)*math.Pow(1.1, float64(level))) * speed
		energy := -math.Ceil(10 * float64(level) * math.Pow(1.1, float64(level)))
		return production, energy, 0
	case BuildingCrystalMine:
		production := math.Floor(20*float64(level)*math.Pow(1.1, float64(level))) * speed
		energy := -math.Ceil(10 * float64(level) * math.Pow(1.1, float64(level)))
		return production, energy, 0
	case BuildingDeuteriumSynth:
		temperatureFactor := 1.28 - 0.002*float64(planet.Temperature+40)
		production := math.Floor(10*float64(level)*math.Pow(1.1, float64(level))) * temperatureFactor * speed
		energy := -math.Ceil(20 * float64(level) * math.Pow(1.1, float64(level)))
		return production, energy, 0
	case BuildingSolarPlant:
		energy := math.Floor(20 * float64(level) * math.Pow(1.1, float64(level)))
		return 0, energy, 0
	case BuildingFusionReactor:
		energy := math.Floor(30 * float64(level) * math.Pow(1.05+float64(research[ResearchEnergy])*0.01, float64(level)))
		deuterium := -math.Ceil(10 * float64(level) * math.Pow(1.1, float64(level)))
		return 0, energy, deuterium
	default:
		return 0, 0, 0
	}
}

func legacyRoundInt(value float64) int {
	return int(math.Round(value))
}

func technologyInfoStorage(level int) int {
	return 100000 + 50000*int(math.Ceil(math.Pow(1.6, float64(level))-1))
}

func legacyTechnologyLongDescription(id int, fallback string) string {
	if description, ok := legacyTechnologyLongDescriptions[id]; ok {
		return description
	}
	return fallback
}

//go:generate go run ../../../cmd/gentechlong -input ../../../../game/loca/en_en/techlong.php -output legacy_technology_descriptions_generated.go

func buildTechnologyDemolish(id int, levels BuildingLevels, speed float64) *TechnologyDemolish {
	level := levels[id]
	if level <= 0 || !isBuildingID(id) || !BuildingCanDemolish(id) {
		return nil
	}
	cost, ok := BuildingCostForLevel(id, level-1)
	if !ok {
		return nil
	}
	duration := BuildingDurationForCost(cost, levels[BuildingRoboticsFactory], levels[BuildingNaniteFactory], speed)
	return &TechnologyDemolish{
		Level:           level,
		Cost:            cost,
		DurationSeconds: duration,
	}
}

func buildTechnologyGroup(key string, name string, ids []int, levels BuildingLevels, research ResearchLevels) TechnologyGroup {
	items := make([]TechnologyItem, 0, len(ids))
	for _, id := range ids {
		spec, ok := technologySpecByID(id)
		if !ok {
			continue
		}
		requirements := buildTechnologyRequirements(id, spec.requirements, levels, research)
		items = append(items, TechnologyItem{
			ID:               id,
			Name:             spec.name,
			Requirements:     requirements,
			DetailsAvailable: len(requirements) > 0,
		})
	}
	return TechnologyGroup{Key: key, Name: name, Items: items}
}

type technologyRequirementsByDepth struct {
	requirements map[int]map[int]int
	order        map[int][]int
}

func newTechnologyRequirementsByDepth() *technologyRequirementsByDepth {
	return &technologyRequirementsByDepth{
		requirements: map[int]map[int]int{},
		order:        map[int][]int{},
	}
}

func walkTechnologyRequirements(targetID int, requirements map[int]int, depth int, tree *technologyRequirementsByDepth) int {
	if len(requirements) == 0 {
		return depth
	}
	if tree.requirements[depth] == nil {
		tree.requirements[depth] = map[int]int{}
	}
	ids := legacyTechnologyRequirementIDs(targetID, requirements)

	maxDepth := depth + 1
	for _, id := range ids {
		level := requirements[id]
		if _, exists := tree.requirements[depth][id]; !exists {
			tree.order[depth] = append(tree.order[depth], id)
		}
		if tree.requirements[depth][id] < level {
			tree.requirements[depth][id] = level
		}
	}
	for _, id := range ids {
		spec, ok := technologySpecByID(id)
		if !ok {
			continue
		}
		if childDepth := walkTechnologyRequirements(id, spec.requirements, depth+1, tree); childDepth > maxDepth {
			maxDepth = childDepth
		}
	}
	return maxDepth
}

func buildTechnologyRequirements(targetID int, requirements map[int]int, levels BuildingLevels, research ResearchLevels) []TechnologyRequirement {
	return buildTechnologyRequirementsOrdered(requirements, legacyTechnologyRequirementIDs(targetID, requirements), levels, research)
}

func buildTechnologyRequirementsOrdered(requirements map[int]int, ids []int, levels BuildingLevels, research ResearchLevels) []TechnologyRequirement {
	if len(requirements) == 0 {
		return nil
	}
	if len(ids) == 0 {
		ids = make([]int, 0, len(requirements))
		for id := range requirements {
			ids = append(ids, id)
		}
		sort.Ints(ids)
	}

	result := make([]TechnologyRequirement, 0, len(ids))
	seen := map[int]bool{}
	for _, id := range ids {
		required, ok := requirements[id]
		if !ok {
			continue
		}
		seen[id] = true
		result = append(result, technologyRequirement(id, required, levels, research))
	}
	missing := make([]int, 0)
	for id := range requirements {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	sort.Ints(missing)
	for _, id := range missing {
		result = append(result, technologyRequirement(id, requirements[id], levels, research))
	}
	return result
}

func technologyRequirement(id int, required int, levels BuildingLevels, research ResearchLevels) TechnologyRequirement {
	current := levels[id]
	if isResearchID(id) {
		current = research[id]
	}
	return TechnologyRequirement{
		ID:           id,
		Name:         technologyName(id),
		Level:        required,
		CurrentLevel: current,
		Met:          current >= required,
	}
}

func legacyTechnologyRequirementIDs(targetID int, requirements map[int]int) []int {
	if len(requirements) == 0 {
		return nil
	}
	legacyIDs, ok := legacyTechnologyRequirementOrder[targetID]
	if !ok {
		ids := make([]int, 0, len(requirements))
		for id := range requirements {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		return ids
	}
	ids := make([]int, 0, len(requirements))
	seen := map[int]bool{}
	for _, id := range legacyIDs {
		if _, exists := requirements[id]; exists {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	missing := make([]int, 0)
	for _, id := range ids {
		seen[id] = true
	}
	for id := range requirements {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	sort.Ints(missing)
	return append(ids, missing...)
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

func isFleetID(id int) bool {
	for _, fleetID := range FleetIDs() {
		if fleetID == id {
			return true
		}
	}
	return false
}

func isLegacyDefenseInfoID(id int) bool {
	return id >= DefenseRocketLauncher && id <= DefenseLargeShieldDome
}

func isBuildingID(id int) bool {
	for _, buildingID := range BuildingIDs() {
		if buildingID == id {
			return true
		}
	}
	return false
}
