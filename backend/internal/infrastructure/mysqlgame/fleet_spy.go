package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type spyArrivalState struct {
	OriginLanguage         string
	TargetLanguage         string
	OriginEspionage        int
	TargetEspionage        int
	OriginTechnocratUntil  int64
	TargetTechnocratUntil  int64
	TargetEngineerUntil    int64
	TargetTemperature      int
	TargetType             int
	TargetFleet            domaingame.FleetCounts
	TargetDefense          map[int]int
	TargetBuildings        domaingame.BuildingLevels
	TargetResearch         domaingame.ResearchLevels
	TargetProduction       domaingame.ProductionFactors
	TargetEnergyProduction int
}

func (r FleetRepository) finishSpyFleetArrival(
	ctx context.Context,
	uniTable string,
	fleetTable string,
	fleetLogsTable string,
	queueTable string,
	planetsTable string,
	usersTable string,
	messagesTable string,
	battleTable string,
	task fleetQueueTask,
	fleet recallFleetRow,
) error {
	messageContext, found, err := r.loadFleetMessageContext(ctx, usersTable, planetsTable, fleet)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("espionage fleet context unavailable")
	}
	state, found, err := r.loadSpyArrivalState(ctx, planetsTable, usersTable, fleet)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("espionage target unavailable")
	}
	holding, err := r.loadSpyHoldingFleet(ctx, uniTable, fleetTable, fleet.TargetPlanetID)
	if err != nil {
		return err
	}

	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	originTechnology := state.OriginEspionage
	if state.OriginTechnocratUntil > now.Unix() {
		originTechnology += 2
	}
	targetTechnology := state.TargetEspionage
	if state.TargetTechnocratUntil > now.Unix() {
		targetTechnology += 2
	}
	result, err := domaingame.ResolveEspionage(domaingame.EspionageInput{
		AttackerTechnology: max(0, originTechnology),
		DefenderTechnology: max(0, targetTechnology),
		AttackerFleet:      fleet.Ships,
		DefenderFleet:      state.TargetFleet,
	}, r.combatRandom)
	if err != nil {
		return err
	}

	if r.legacyEvents {
		subject, report := spyReport(messageContext, state, holding, result, task.End)
		if err := r.insertSpyMessage(ctx, messagesTable, messageContext.OriginOwnerID, domaingame.MessageTypeSpyReport, expeditionLocaleValue(state.OriginLanguage, "FLEET_MESSAGE_FROM"), subject, report, task.End, fleet.TargetPlanetID); err != nil {
			return err
		}
		text := expeditionFormat(
			expeditionLocaleValue(state.TargetLanguage, "FLEET_SPY_OTHER"),
			messageContext.OriginName,
			fleetGalaxyLinkRaw(messageContext.OriginGalaxy, messageContext.OriginSystem, messageContext.OriginPosition),
			messageContext.TargetName,
			fleetGalaxyLinkRaw(messageContext.TargetGalaxy, messageContext.TargetSystem, messageContext.TargetPosition),
			result.CounterChancePercent,
		)
		text = legacyAddSlashes(text)
		if err := r.insertSpyMessage(ctx, messagesTable, messageContext.TargetOwnerID, domaingame.MessageTypeMisc, expeditionLocaleValue(state.TargetLanguage, "FLEET_MESSAGE_OBSERVE"), expeditionLocaleValue(state.TargetLanguage, "FLEET_MESSAGE_SPY"), text, task.End, 0); err != nil {
			return err
		}
	}
	if err := r.updateFleetPlanetActivity(ctx, planetsTable, fleet.TargetPlanetID, task.End); err != nil {
		return err
	}
	if result.Detected {
		return r.finishAttackFleetArrival(ctx, uniTable, fleetTable, fleetLogsTable, queueTable, planetsTable, usersTable, messagesTable, battleTable, task, fleet)
	}

	returnFleetID, err := r.insertRecallFleet(ctx, fleetTable, fleet.OwnerID, fleet, domaingame.FleetMissionSpy+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime))
	if err != nil {
		return err
	}
	if err := r.insertRecallQueue(ctx, queueTable, fleet.OwnerID, returnFleetID, domaingame.FleetMissionSpy+domaingame.FleetMissionReturnOffset, task.End, int64(fleet.FlightTime)); err != nil {
		return err
	}
	if r.legacyEvents {
		if err := r.insertFleetTransitionLog(ctx, fleetLogsTable, messageContext, fleet, domaingame.FleetMissionSpy+domaingame.FleetMissionReturnOffset, int64(fleet.FlightTime), 0, task.End); err != nil {
			return err
		}
	}
	return r.removeCompletedFleetTask(ctx, fleetTable, queueTable, fleet.ID, task.TaskID)
}

func (r FleetRepository) loadSpyArrivalState(ctx context.Context, planetsTable string, usersTable string, fleet recallFleetRow) (spyArrivalState, bool, error) {
	fleetIDs := domaingame.FleetIDs()
	defenseIDs := domaingame.DefenseIDs()
	buildingIDs := domaingame.BuildingIDs()
	researchIDs := domaingame.ResearchIDs()
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf(
		"SELECT COALESCE(ou.lang, 'en'), COALESCE(tu.lang, 'en'), COALESCE(ou.`%d`, 0), COALESCE(tu.`%d`, 0), COALESCE(ou.tec_until, 0), COALESCE(tu.tec_until, 0), COALESCE(tu.eng_until, 0), COALESCE(tp.temp, 0), COALESCE(tp.type, 1), COALESCE(tp.prod1, 0), COALESCE(tp.prod2, 0), COALESCE(tp.prod3, 0), COALESCE(tp.prod4, 0), COALESCE(tp.prod12, 0), COALESCE(tp.prod212, 0), %s, %s, %s, %s FROM %s tp JOIN %s tu ON tu.player_id = tp.owner_id JOIN %s op ON op.planet_id = ? JOIN %s ou ON ou.player_id = op.owner_id WHERE tp.planet_id = ? LIMIT 1",
		domaingame.ResearchEspionage,
		domaingame.ResearchEspionage,
		prefixedNumericColumns("tp", fleetIDs),
		prefixedNumericColumns("tp", defenseIDs),
		prefixedNumericColumns("tp", buildingIDs),
		prefixedNumericColumns("tu", researchIDs),
		planetsTable,
		usersTable,
		planetsTable,
		usersTable,
	), fleet.StartPlanetID, fleet.TargetPlanetID)
	if err != nil {
		return spyArrivalState{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return spyArrivalState{}, false, rows.Err()
	}

	state := spyArrivalState{
		TargetFleet:     make(domaingame.FleetCounts, len(fleetIDs)),
		TargetDefense:   make(map[int]int, len(defenseIDs)),
		TargetBuildings: make(domaingame.BuildingLevels, len(buildingIDs)),
		TargetResearch:  make(domaingame.ResearchLevels, len(researchIDs)),
	}
	var prodMetal, prodCrystal, prodDeuterium, prodSolar, prodFusion, prodSatellite float64
	fleetCounts := make([]int, len(fleetIDs))
	defenseCounts := make([]int, len(defenseIDs))
	buildingLevels := make([]int, len(buildingIDs))
	researchLevels := make([]int, len(researchIDs))
	destinations := []any{
		&state.OriginLanguage,
		&state.TargetLanguage,
		&state.OriginEspionage,
		&state.TargetEspionage,
		&state.OriginTechnocratUntil,
		&state.TargetTechnocratUntil,
		&state.TargetEngineerUntil,
		&state.TargetTemperature,
		&state.TargetType,
		&prodMetal,
		&prodCrystal,
		&prodDeuterium,
		&prodSolar,
		&prodFusion,
		&prodSatellite,
	}
	for index := range fleetCounts {
		destinations = append(destinations, &fleetCounts[index])
	}
	for index := range defenseCounts {
		destinations = append(destinations, &defenseCounts[index])
	}
	for index := range buildingLevels {
		destinations = append(destinations, &buildingLevels[index])
	}
	for index := range researchLevels {
		destinations = append(destinations, &researchLevels[index])
	}
	if err := rows.Scan(destinations...); err != nil {
		return spyArrivalState{}, false, err
	}
	if err := rows.Err(); err != nil {
		return spyArrivalState{}, false, err
	}
	for index, id := range fleetIDs {
		state.TargetFleet[id] = fleetCounts[index]
	}
	for index, id := range defenseIDs {
		state.TargetDefense[id] = defenseCounts[index]
	}
	for index, id := range buildingIDs {
		state.TargetBuildings[id] = buildingLevels[index]
	}
	for index, id := range researchIDs {
		state.TargetResearch[id] = researchLevels[index]
	}
	state.TargetProduction = domaingame.ProductionFactors{
		domaingame.BuildingMetalMine:      prodMetal,
		domaingame.BuildingCrystalMine:    prodCrystal,
		domaingame.BuildingDeuteriumSynth: prodDeuterium,
		domaingame.BuildingSolarPlant:     prodSolar,
		domaingame.BuildingFusionReactor:  prodFusion,
		domaingame.FleetSolarSatellite:    prodSatellite,
	}
	production := domaingame.BuildResourceProduction(domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{
		Type:        state.TargetType,
		Temperature: state.TargetTemperature,
	}}, domaingame.ResourceProductionInputs{
		Levels:            state.TargetBuildings,
		SolarSatellites:   state.TargetFleet[domaingame.FleetSolarSatellite],
		ProductionFactors: state.TargetProduction,
		EnergyResearch:    state.TargetResearch[domaingame.ResearchEnergy],
		UniverseSpeed:     1,
		Engineer:          state.TargetEngineerUntil > r.now().Unix(),
	})
	state.TargetEnergyProduction = int(production.Totals.Hour.EnergyRaw)
	return state, true, nil
}

func (r FleetRepository) loadSpyHoldingFleet(ctx context.Context, uniTable string, fleetTable string, planetID int) (domaingame.FleetCounts, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(acs, 0) FROM %s LIMIT 1", uniTable))
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		return nil, errors.New("espionage universe settings unavailable")
	}
	var acs int
	if err := rows.Scan(&acs); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	limit := holdingCombatFleetLimit(acs)
	holding := make(domaingame.FleetCounts, len(domaingame.FleetIDs()))
	if limit == 0 {
		return holding, nil
	}

	ids := domaingame.FleetIDs()
	rows, err = r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE mission = ? AND target_planet = ? ORDER BY fleet_id LIMIT ?", numericColumns(ids), fleetTable), domaingame.FleetMissionACSHold+domaingame.FleetMissionOrbitingOffset, planetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		counts := make([]int, len(ids))
		destinations := make([]any, len(ids))
		for index := range counts {
			destinations[index] = &counts[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		for index, id := range ids {
			holding[id] += counts[index]
		}
	}
	return holding, rows.Err()
}

func (r FleetRepository) insertSpyMessage(ctx context.Context, messagesTable string, ownerID int, messageType int, from string, subject string, text string, at int64, planetID int) error {
	_, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, pm, msgfrom, subj, text, shown, date, planet_id) VALUES (?, ?, ?, ?, ?, 0, ?, ?)", messagesTable), ownerID, messageType, from, subject, text, at, planetID)
	return err
}

func spyReport(value fleetMessageContext, state spyArrivalState, holding domaingame.FleetCounts, result domaingame.EspionageResult, at int64) (string, string) {
	language := state.OriginLanguage
	targetLink := fleetGalaxyLinkRaw(value.TargetGalaxy, value.TargetSystem, value.TargetPosition)
	subject := fmt.Sprintf("\n<span class=\"espionagereport\">\n%s\n%s", expeditionFormat(expeditionLocaleValue(language, "SPY_SUBJ"), value.TargetName), targetLink)
	var report strings.Builder
	fmt.Fprintf(&report, "<table width=400><tr><td class=c colspan=4>%s %s %s</td></tr>\n", expeditionFormat(expeditionLocaleValue(language, "SPY_RESOURCES"), value.TargetName), targetLink, expeditionFormat(expeditionLocaleValue(language, "SPY_PLAYER"), value.TargetOwnerName, time.Unix(at, 0).Format("01-02 15:04:05")))
	fmt.Fprintf(&report, "</div></font></TD></TR><tr><td>%s</td><td>%s</td>\n", expeditionLocaleValue(language, "SPY_M"), fleetLegacyNumber(value.TargetMetal))
	fmt.Fprintf(&report, "<td>%s</td><td>%s</td></tr>\n", expeditionLocaleValue(language, "SPY_K"), fleetLegacyNumber(value.TargetCrystal))
	fmt.Fprintf(&report, "<tr><td>%s</td><td>%s</td>\n", expeditionLocaleValue(language, "SPY_D"), fleetLegacyNumber(value.TargetDeuterium))
	fmt.Fprintf(&report, "<td>%s</td><td>%s</td></tr>\n", expeditionLocaleValue(language, "SPY_E"), fleetLegacyNumber(float64(state.TargetEnergyProduction)))
	report.WriteString("</table>\n")
	report.WriteString("<table width=400><tr><td class=c colspan=4>     </td></tr>\n")
	fmt.Fprintf(&report, "<TR><TD colspan=4><div onmouseover='return overlib(\"&lt;font color=white&gt;%s&lt;/font&gt;\", STICKY, MOUSEOFF, DELAY, 750, CENTER, WIDTH, 100, OFFSETX, -130, OFFSETY, -10);' onmouseout='return nd();'></TD></TR></table>\n", expeditionLocaleValue(language, "SPY_ACTIVITY"))

	if result.ReportLevel > 0 {
		counts := make(map[int]int, len(domaingame.FleetIDs()))
		for _, id := range domaingame.FleetIDs() {
			counts[id] = state.TargetFleet[id] + holding[id]
		}
		writeSpyReportSection(&report, language, expeditionLocaleValue(language, "SPY_FLEET"), domaingame.FleetIDs(), counts)
	}
	if result.ReportLevel > 1 {
		writeSpyReportSection(&report, language, expeditionLocaleValue(language, "SPY_DEFENSE"), domaingame.DefenseIDs(), state.TargetDefense)
	}
	if result.ReportLevel > 3 {
		writeSpyReportSection(&report, language, expeditionLocaleValue(language, "SPY_BUILDINGS"), domaingame.BuildingIDs(), mapFromBuildingLevels(state.TargetBuildings))
	}
	if result.ReportLevel > 5 {
		writeSpyReportSection(&report, language, expeditionLocaleValue(language, "SPY_RESEARCH"), domaingame.ResearchIDs(), mapFromResearchLevels(state.TargetResearch))
	}
	fmt.Fprintf(&report, "<center>%s</center>\n", expeditionFormat(expeditionLocaleValue(language, "SPY_COUNTER"), result.CounterChancePercent))
	fmt.Fprintf(&report, "<center><a href='#' onclick='showFleetMenu(%d,%d,%d,%d,1);'>%s</a></center>\n", value.TargetGalaxy, value.TargetSystem, value.TargetPosition, spyGamePlanetType(value.TargetType), expeditionLocaleValue(language, "SPY_ATTACK"))
	return subject, legacyAddSlashes(report.String())
}

func writeSpyReportSection(report *strings.Builder, language string, title string, ids []int, counts map[int]int) {
	fmt.Fprintf(report, "<table width=400><tr><td class=c colspan=4>%s     </td></tr>\n", title)
	count := 0
	for _, id := range ids {
		if counts[id] <= 0 {
			continue
		}
		if count%2 == 0 {
			report.WriteString("</tr>\n")
		}
		name := expeditionLocaleValue(language, fmt.Sprintf("NAME_%d", id))
		if strings.HasPrefix(name, "NAME_") {
			name = domaingame.TechnologyName(id)
		}
		fmt.Fprintf(report, "<td>%s</td><td>%s</td>\n", name, fleetLegacyNumber(float64(counts[id])))
		count++
	}
	report.WriteString("</table>\n")
}

func mapFromBuildingLevels(levels domaingame.BuildingLevels) map[int]int {
	return mapFromTechnologyLevels(levels)
}

func mapFromResearchLevels(levels domaingame.ResearchLevels) map[int]int {
	return mapFromTechnologyLevels(levels)
}

func mapFromTechnologyLevels[T ~map[int]int](levels T) map[int]int {
	result := make(map[int]int, len(levels))
	for id, level := range levels {
		result[id] = level
	}
	return result
}

func spyGamePlanetType(planetType int) int {
	if planetType >= 20001 {
		return planetType
	}
	switch planetType {
	case domaingame.PlanetTypeMoon, domaingame.PlanetTypeDestroyedMoon:
		return domaingame.GamePlanetTypeMoon
	case domaingame.PlanetTypeDebris:
		return domaingame.GamePlanetTypeDebris
	default:
		return domaingame.GamePlanetTypePlanet
	}
}

func fleetGalaxyLinkRaw(galaxy int, system int, position int) string {
	return fmt.Sprintf(`<a onclick="showGalaxy(%d,%d,%d);" href="#">[%d:%d:%d]</a>`, galaxy, system, position, galaxy, system, position)
}

func legacyAddSlashes(value string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		`"`, `\"`,
		"\x00", `\0`,
	).Replace(value)
}
