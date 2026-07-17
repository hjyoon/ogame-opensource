package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestSpyReportIncludesUnlockedSectionsAndHoldingFleet(t *testing.T) {
	value := fleetMessageContextTestValue()
	state := spyArrivalState{
		TargetEnergyProduction: 321,
		TargetFleet:            domaingame.FleetCounts{domaingame.FleetLightFighter: 3},
		TargetDefense:          map[int]int{domaingame.DefenseRocketLauncher: 4},
		TargetBuildings:        domaingame.BuildingLevels{domaingame.BuildingMetalMine: 5},
		TargetResearch:         domaingame.ResearchLevels{domaingame.ResearchEspionage: 6},
	}
	holding := domaingame.FleetCounts{domaingame.FleetLightFighter: 2, domaingame.FleetCruiser: 1}
	subject, report := spyReport(value, state, holding, domaingame.EspionageResult{ReportLevel: 6, CounterChancePercent: 17}, 1_700_000_000)
	for _, expected := range []string{
		"Information of Away",
		"Resources on Away",
		"Energy:</td><td>321",
		"Fleet     ",
		"Light Fighter</td><td>5",
		"Cruiser</td><td>1",
		"Defence     ",
		"Rocket Launcher</td><td>4",
		"Buildings     ",
		"Metal Mine</td><td>5",
		"Research     ",
		"Espionage Technology</td><td>6",
		"Chance for spy counter:17%",
		"showFleetMenu(2,3,4,1,1)",
	} {
		if !strings.Contains(subject+report, expected) {
			t.Fatalf("missing %q in spy report: %s%s", expected, subject, report)
		}
	}
	if strings.Contains(subject, `\"`) || !strings.Contains(report, `\"`) || !strings.Contains(report, `\'`) {
		t.Fatalf("legacy escaping differs between subject and body: subject=%q report=%q", subject, report)
	}

	_, limited := spyReport(value, state, nil, domaingame.EspionageResult{ReportLevel: 0}, 1_700_000_000)
	for _, hidden := range []string{"Fleet     ", "Defence     ", "Buildings     ", "Research     "} {
		if strings.Contains(limited, hidden) {
			t.Fatalf("locked section %q leaked into report: %s", hidden, limited)
		}
	}
}

func TestSpyReportUsesPlayerLanguage(t *testing.T) {
	state := spyArrivalState{OriginLanguage: "de", TargetFleet: domaingame.FleetCounts{}, TargetDefense: map[int]int{}, TargetBuildings: domaingame.BuildingLevels{}, TargetResearch: domaingame.ResearchLevels{}}
	subject, report := spyReport(fleetMessageContextTestValue(), state, nil, domaingame.EspionageResult{}, 1_700_000_000)
	for _, expected := range []string{"Informationen von Away", "Rohstoffe bei Away", "Metall:", "Chance für Spionageabwehr:0%", "Angreifen"} {
		if !strings.Contains(subject+report, expected) {
			t.Fatalf("missing localized spy text %q: %s%s", expected, subject, report)
		}
	}
}

func TestFinishSpyFleetArrivalCreatesMessagesAndReturnFleet(t *testing.T) {
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
		{rows: fakeRowsFromValues(spyArrivalTestRow(
			map[int]int{domaingame.FleetSolarSatellite: 2},
			map[int]int{domaingame.DefenseRocketLauncher: 4},
			map[int]int{domaingame.BuildingMetalMine: 5, domaingame.BuildingSolarPlant: 5},
			map[int]int{domaingame.ResearchEnergy: 3},
		))},
		{rows: fakeRowsFromValues([]any{0})},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	repository.legacyEvents = true
	repository.combatRandom = func(int) int { return 0 }
	fleet := recallFleetRow{
		ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionSpy,
		StartPlanetID: 99, TargetPlanetID: 100, FlightTime: 300, Fuel: 50,
		Ships: domaingame.FleetCounts{domaingame.FleetEspionageProbe: 2},
	}
	err := repository.finishSpyFleetArrival(
		context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`",
		"`ogame_planets`", "`ogame_users`", "`ogame_messages`", "`ogame_battledata`",
		fleetQueueTask{TaskID: 55, OwnerID: 42, FleetID: 123, End: 2_000}, fleet,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.execCalls) != 8 {
		t.Fatalf("unexpected spy transition writes: %+v", runner.execCalls)
	}
	if runner.execCalls[0].args[1] != domaingame.MessageTypeSpyReport || runner.execCalls[0].args[6] != 100 {
		t.Fatalf("unexpected spy report persistence: %+v", runner.execCalls[0])
	}
	if runner.execCalls[1].args[1] != domaingame.MessageTypeMisc || runner.execCalls[1].args[6] != 0 {
		t.Fatalf("unexpected observation persistence: %+v", runner.execCalls[1])
	}
	if !strings.Contains(runner.execCalls[3].sql, "INSERT INTO `ogame_fleet`") || runner.execCalls[3].args[6] != domaingame.FleetMissionSpy+domaingame.FleetMissionReturnOffset || runner.execCalls[3].args[5] != 25 {
		t.Fatalf("unexpected return fleet: %+v", runner.execCalls[3])
	}
	if runner.execCalls[4].args[2] != 1 || runner.execCalls[4].args[6] != int64(2_300) {
		t.Fatalf("unexpected return queue: %+v", runner.execCalls[4])
	}
	if !strings.Contains(runner.execCalls[6].sql, "DELETE FROM `ogame_fleet`") || !strings.Contains(runner.execCalls[7].sql, "DELETE FROM `ogame_queue`") {
		t.Fatalf("original spy task was not removed: %+v", runner.execCalls)
	}
}

func TestLoadSpyHoldingFleetUsesLegacyACSLimit(t *testing.T) {
	row := make([]any, 0, len(domaingame.FleetIDs()))
	for _, id := range domaingame.FleetIDs() {
		count := 0
		if id == domaingame.FleetCruiser {
			count = 3
		}
		row = append(row, count)
	}
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues(row, row)},
	}}
	repository := NewFleetRepositoryWithQueryer(queryer, "ogame_", time.Now)
	holding, err := repository.loadSpyHoldingFleet(context.Background(), "`ogame_uni`", "`ogame_fleet`", 100)
	if err != nil {
		t.Fatal(err)
	}
	if holding[domaingame.FleetCruiser] != 6 || len(queryer.calls) != 2 || queryer.calls[1].args[2] != 3 {
		t.Fatalf("unexpected holding fleet: counts=%+v calls=%+v", holding, queryer.calls)
	}
}

func TestLoadSpyHoldingFleetPropagatesUniverseRowErrors(t *testing.T) {
	for _, rows := range []*fakeRows{
		fakeRowsFromValuesWithErr(errors.New("universe rows failed")),
		fakeRowsFromValuesWithErr(errors.New("universe trailing rows failed"), []any{0}),
	} {
		repository := NewFleetRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: rows}}}, "ogame_", time.Now)
		if _, err := repository.loadSpyHoldingFleet(context.Background(), "`ogame_uni`", "`ogame_fleet`", 100); err == nil || !strings.Contains(err.Error(), "universe") {
			t.Fatalf("expected universe row error, got %v", err)
		}
	}
}

func TestFinishSpyFleetArrivalHandsDetectedProbeToCombat(t *testing.T) {
	stateRow := spyArrivalTestRow(
		map[int]int{domaingame.FleetLightFighter: 100},
		nil,
		nil,
		nil,
	)
	stateRow[2], stateRow[3] = 2, 2
	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
		{rows: fakeRowsFromValues(stateRow)},
		{rows: fakeRowsFromValues([]any{0})},
		{err: errors.New("combat handoff reached")},
	}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	repository.legacyEvents = true
	rolls := []int{50, 49}
	repository.combatRandom = func(int) int {
		roll := rolls[0]
		rolls = rolls[1:]
		return roll
	}
	fleet := recallFleetRow{
		ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionSpy,
		StartPlanetID: 99, TargetPlanetID: 100, FlightTime: 300,
		Ships: domaingame.FleetCounts{domaingame.FleetEspionageProbe: 5},
	}
	err := repository.finishSpyFleetArrival(
		context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`",
		"`ogame_planets`", "`ogame_users`", "`ogame_messages`", "`ogame_battledata`",
		fleetQueueTask{TaskID: 55, OwnerID: 42, FleetID: 123, End: 2_000}, fleet,
	)
	if err == nil || !strings.Contains(err.Error(), "combat handoff reached") {
		t.Fatalf("detected spy did not reach combat engine: %v", err)
	}
	if len(runner.execCalls) != 3 || len(rolls) != 0 {
		t.Fatalf("detected spy writes/rolls differ: writes=%+v rolls=%v", runner.execCalls, rolls)
	}
}

func TestFinishSpyFleetArrivalStopsAtPersistenceBoundaries(t *testing.T) {
	queryError := func(message string) fakeQueryResult { return fakeQueryResult{err: errors.New(message)} }
	standardResults := func() []fakeQueryResult {
		return []fakeQueryResult{
			{rows: fakeRowsFromValues(fleetMessageContextTestRow())},
			{rows: fakeRowsFromValues(spyArrivalTestRow(nil, nil, nil, nil))},
			{rows: fakeRowsFromValues([]any{0})},
		}
	}
	for _, test := range []struct {
		name       string
		results    []fakeQueryResult
		execErrors []error
		nilRandom  bool
		want       string
	}{
		{name: "context query", results: []fakeQueryResult{queryError("context failed")}, want: "context failed"},
		{name: "context missing", results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, want: "context unavailable"},
		{name: "state query", results: []fakeQueryResult{{rows: fakeRowsFromValues(fleetMessageContextTestRow())}, queryError("state failed")}, want: "state failed"},
		{name: "state missing", results: []fakeQueryResult{{rows: fakeRowsFromValues(fleetMessageContextTestRow())}, {rows: fakeRowsFromValues()}}, want: "target unavailable"},
		{name: "universe query", results: append(standardResults()[:2], queryError("universe failed")), want: "universe failed"},
		{name: "random", results: standardResults(), nilRandom: true, want: domaingame.ErrEspionageRandomUnavailable.Error()},
		{name: "report message", results: standardResults(), execErrors: []error{errors.New("report failed")}, want: "report failed"},
		{name: "observation message", results: standardResults(), execErrors: []error{nil, errors.New("observation failed")}, want: "observation failed"},
		{name: "activity", results: standardResults(), execErrors: []error{nil, nil, errors.New("activity failed")}, want: "activity failed"},
		{name: "return fleet", results: standardResults(), execErrors: []error{nil, nil, nil, errors.New("return failed")}, want: "return failed"},
		{name: "return queue", results: standardResults(), execErrors: []error{nil, nil, nil, nil, errors.New("queue failed")}, want: "queue failed"},
		{name: "transition log", results: standardResults(), execErrors: []error{nil, nil, nil, nil, nil, errors.New("log failed")}, want: "log failed"},
		{name: "fleet cleanup", results: standardResults(), execErrors: []error{nil, nil, nil, nil, nil, nil, errors.New("fleet cleanup failed")}, want: "fleet cleanup failed"},
		{name: "queue cleanup", results: standardResults(), execErrors: []error{nil, nil, nil, nil, nil, nil, nil, errors.New("queue cleanup failed")}, want: "queue cleanup failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: test.results}, execErrs: test.execErrors}
			repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
			repository.legacyEvents = true
			if test.nilRandom {
				repository.combatRandom = nil
			} else {
				repository.combatRandom = func(int) int { return 0 }
			}
			fleet := recallFleetRow{
				ID: 123, OwnerID: 42, Mission: domaingame.FleetMissionSpy,
				StartPlanetID: 99, TargetPlanetID: 100, FlightTime: 300,
				Ships: domaingame.FleetCounts{domaingame.FleetEspionageProbe: 2},
			}
			err := repository.finishSpyFleetArrival(
				context.Background(), "`ogame_uni`", "`ogame_fleet`", "`ogame_fleetlogs`", "`ogame_queue`",
				"`ogame_planets`", "`ogame_users`", "`ogame_messages`", "`ogame_battledata`",
				fleetQueueTask{TaskID: 55, OwnerID: 42, FleetID: 123, End: 2_000}, fleet,
			)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestSpyGamePlanetTypeMatchesLegacyConversion(t *testing.T) {
	for _, test := range []struct {
		planetType int
		want       int
	}{
		{domaingame.PlanetTypePlanet, domaingame.GamePlanetTypePlanet},
		{domaingame.PlanetTypeMoon, domaingame.GamePlanetTypeMoon},
		{domaingame.PlanetTypeDebris, domaingame.GamePlanetTypeDebris},
		{domaingame.PlanetTypeDestroyedMoon, domaingame.GamePlanetTypeMoon},
		{20001, 20001},
	} {
		if got := spyGamePlanetType(test.planetType); got != test.want {
			t.Fatalf("spyGamePlanetType(%d)=%d, want %d", test.planetType, got, test.want)
		}
	}
}

func spyArrivalTestRow(fleet map[int]int, defense map[int]int, buildings map[int]int, research map[int]int) []any {
	row := []any{"en", "en", 8, 3, int64(0), int64(0), int64(0), 40, domaingame.PlanetTypePlanet, float64(1), float64(1), float64(1), float64(1), float64(1), float64(1)}
	for _, id := range domaingame.FleetIDs() {
		row = append(row, fleet[id])
	}
	for _, id := range domaingame.DefenseIDs() {
		row = append(row, defense[id])
	}
	for _, id := range domaingame.BuildingIDs() {
		row = append(row, buildings[id])
	}
	for _, id := range domaingame.ResearchIDs() {
		row = append(row, research[id])
	}
	return row
}
