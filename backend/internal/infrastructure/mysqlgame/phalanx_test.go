package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestPhalanxRepositoryScansEventsAndSpendsDeuterium(t *testing.T) {
	now := time.Unix(2_000, 0)
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{rows: fakeRowsFromValues(phalanxPlanetRow(10, 42, "Go Smoke Moon", domaingame.PlanetTypeMoon, 6, 3, 20_000.0))},
		{rows: fakeRowsFromValues(phalanxPlanetRow(20, 77, "Go Smoke Target", domaingame.PlanetTypePlanet, 2, 0, 1_000_000.0))},
		{rows: fakeRowsFromValues(phalanxEventRow(300, 77, "target", domaingame.FleetMissionTransport, map[int]int{domaingame.FleetSmallCargo: 1}, 2_100, 2_500, 20, 30, 2, 6))},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	phalanx, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20})
	if err != nil {
		t.Fatal(err)
	}
	if phalanx.Commander != "legor" || phalanx.Source.ID != 10 || phalanx.Target.ID != 20 {
		t.Fatalf("unexpected phalanx result: %+v", phalanx)
	}
	if phalanx.RemainingDeuterium != 15_000 || len(phalanx.Events) != 1 || phalanx.Events[0].Mission != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset {
		t.Fatalf("expected spent deuterium and one legacy return event, got %+v", phalanx)
	}
	if len(runner.execCalls) != 1 {
		t.Fatalf("expected one deuterium update, got %+v", runner.execCalls)
	}
	args := runner.execCalls[0].args
	if len(args) != 4 || args[0] != float64(15_000) || args[1] != now.Unix() || args[2] != 10 || args[3] != 42 {
		t.Fatalf("unexpected deuterium update args: %+v", args)
	}
	eventQuery := ""
	for _, call := range runner.calls {
		if strings.Contains(call.sql, "SELECT q.sub_id") {
			eventQuery = call.sql
			break
		}
	}
	if !strings.Contains(eventQuery, "f.`202`") || strings.Contains(eventQuery, "SELECT q.sub_id, q.start, q.end, `202`") {
		t.Fatalf("expected phalanx fleet query to use prefixed fleet columns, got %s", eventQuery)
	}
}

func TestPhalanxRepositoryMCPPreviewAndExecute(t *testing.T) {
	now := time.Unix(2_000, 0)
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: phalanxSuccessfulReadResults(fakeQueryResult{rows: fakeRowsFromValues()})}}
	repository := NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	preview, err := repository.PreviewMCPPhalanxScan(context.Background(), 42, domainmcp.PhalanxScanCommand{PlanetID: 10, TargetPlanetID: 20})
	if err != nil {
		t.Fatalf("PreviewMCPPhalanxScan returned error: %v", err)
	}
	if preview.PlayerID != 42 || preview.PlanetID != 10 || preview.TargetPlanetID != 20 || preview.Cost != domaingame.PhalanxCost || preview.RemainingDeuterium != 15_000 || preview.Issue != nil {
		t.Fatalf("unexpected MCP preview: %+v", preview)
	}
	if len(preview.Events) != 0 || len(runner.execCalls) != 0 {
		t.Fatalf("preview must not expose events or spend deuterium, preview=%+v exec=%+v", preview, runner.execCalls)
	}

	runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: phalanxSuccessfulReadResults(
		fakeQueryResult{rows: fakeRowsFromValues(phalanxEventRow(300, 77, "target", domaingame.FleetMissionTransport, map[int]int{domaingame.FleetSmallCargo: 1}, 2_100, 2_500, 20, 30, 2, 6))},
	)}}
	repository = NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	scanned, err := repository.ScanMCPPhalanx(context.Background(), 42, domainmcp.PhalanxScanCommand{PlanetID: 10, TargetPlanetID: 20})
	if err != nil {
		t.Fatalf("ScanMCPPhalanx returned error: %v", err)
	}
	if !scanned.Executed || scanned.Issue != nil || len(scanned.Events) != 1 || len(runner.execCalls) != 1 {
		t.Fatalf("expected executed MCP scan with events and one spend, result=%+v exec=%+v", scanned, runner.execCalls)
	}
}

func TestPhalanxRepositoryMCPPreviewEdges(t *testing.T) {
	if _, err := (PhalanxRepository{}).ScanMCPPhalanx(context.Background(), 42, domainmcp.PhalanxScanCommand{TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("expected scan reader error, got %v", err)
	}
	if got := mcpPhalanxIssue(nil); got != nil {
		t.Fatalf("nil phalanx issue should stay nil, got %+v", got)
	}

	repository := NewPhalanxRepositoryWithRunner(&fakeQueryer{}, nil, "bad`", nil)
	if _, err := repository.PreviewMCPPhalanxScan(context.Background(), 42, domainmcp.PhalanxScanCommand{TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected preview table prefix error, got %v", err)
	}

	repository = NewPhalanxRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, nil, "ogame_", nil)
	if _, err := repository.PreviewMCPPhalanxScan(context.Background(), 42, domainmcp.PhalanxScanCommand{TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "overview failed") {
		t.Fatalf("expected preview overview error, got %v", err)
	}

	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{err: errors.New("source failed")},
	}}}
	repository = NewPhalanxRepositoryWithRunner(runner, nil, "ogame_", nil)
	if _, err := repository.PreviewMCPPhalanxScan(context.Background(), 42, domainmcp.PhalanxScanCommand{TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "source failed") {
		t.Fatalf("expected preview source error, got %v", err)
	}

	runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(phalanxPlanetRow(20, 77, "Go Smoke Target", domaingame.PlanetTypePlanet, 2, 0, 1_000_000.0))},
	}}}
	repository = NewPhalanxRepositoryWithRunner(runner, nil, "ogame_", nil)
	preview, err := repository.PreviewMCPPhalanxScan(context.Background(), 42, domainmcp.PhalanxScanCommand{TargetPlanetID: 20})
	if err != nil {
		t.Fatalf("preview source fallback returned error: %v", err)
	}
	if preview.Source.ID != 10 || preview.Issue == nil || preview.Issue.Code != domaingame.PhalanxIssueMissingSensor {
		t.Fatalf("expected source fallback missing sensor issue, got %+v", preview)
	}

	runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{rows: fakeRowsFromValues(phalanxPlanetRow(10, 42, "Go Smoke Moon", domaingame.PlanetTypeMoon, 6, 3, 20_000.0))},
		{err: errors.New("target failed")},
	}}}
	repository = NewPhalanxRepositoryWithRunner(runner, nil, "ogame_", nil)
	if _, err := repository.PreviewMCPPhalanxScan(context.Background(), 42, domainmcp.PhalanxScanCommand{TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "target failed") {
		t.Fatalf("expected preview target error, got %v", err)
	}
}

func TestPhalanxRepositoryReturnsLegacyIssueWithoutSpending(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{rows: fakeRowsFromValues(phalanxPlanetRow(10, 42, "Go Smoke Moon", domaingame.PlanetTypeMoon, 6, 0, 20_000.0))},
		{rows: fakeRowsFromValues(phalanxPlanetRow(20, 77, "Go Smoke Target", domaingame.PlanetTypePlanet, 2, 0, 1_000_000.0))},
	}}}
	repository := NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	phalanx, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20})
	if err != nil {
		t.Fatal(err)
	}
	if phalanx.ActionIssue == nil || phalanx.ActionIssue.Code != domaingame.PhalanxIssueMissingSensor {
		t.Fatalf("expected missing sensor issue, got %+v", phalanx)
	}
	if phalanx.RemainingDeuterium != 20_000 || len(runner.execCalls) != 0 {
		t.Fatalf("expected rejected scan not to spend deuterium, got %+v exec=%+v", phalanx, runner.execCalls)
	}
}

func TestPhalanxRepositoryHandlesReaderAndRowEdges(t *testing.T) {
	constructed := NewPhalanxRepository(nil, "ogame_")
	if constructed.prefix != "ogame_" || constructed.queryer == nil || constructed.execer == nil {
		t.Fatalf("unexpected constructed repository: %+v", constructed)
	}

	if _, err := (PhalanxRepository{}).GetPhalanx(context.Background(), appgame.PhalanxQuery{}); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("expected missing reader error, got %v", err)
	}
	if _, err := (PhalanxRepository{}).PreviewMCPPhalanxScan(context.Background(), 42, domainmcp.PhalanxScanCommand{TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("expected missing MCP reader error, got %v", err)
	}
	repository := NewPhalanxRepositoryWithRunner(&fakeQueryer{}, nil, "bad`", nil)
	if _, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected table prefix error, got %v", err)
	}

	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewPhalanxRepositoryWithRunner(queryer, nil, "ogame_", nil)
	if planet, found, err := repository.loadPhalanxPlanet(context.Background(), "ogame_planets", 99); err != nil || found || planet.ID != 0 {
		t.Fatalf("expected missing phalanx planet, planet=%+v found=%v err=%v", planet, found, err)
	}
	if planet, found, err := repository.loadPhalanxPlanet(context.Background(), "ogame_planets", 0); err != nil || found || planet.ID != 0 {
		t.Fatalf("expected zero id to skip loading, planet=%+v found=%v err=%v", planet, found, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("planet rows failed"))}}}
	repository = NewPhalanxRepositoryWithRunner(queryer, nil, "ogame_", nil)
	if _, _, err := repository.loadPhalanxPlanet(context.Background(), "ogame_planets", 99); err == nil || !strings.Contains(err.Error(), "planet rows failed") {
		t.Fatalf("expected row error, got %v", err)
	}
}

func TestPhalanxRepositoryRequiresWriterForSuccessfulScan(t *testing.T) {
	queryer := &fakeQueryer{results: phalanxSuccessfulReadResults(fakeQueryResult{rows: fakeRowsFromValues()})}
	repository := NewPhalanxRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return time.Unix(2_000, 0) })

	if _, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "writer unavailable") {
		t.Fatalf("expected writer error, got %v", err)
	}
}

func TestPhalanxRepositoryPropagatesEventAndUpdateErrors(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: phalanxSuccessfulReadResults(fakeQueryResult{err: errors.New("events failed")})}}
	repository := NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if _, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "events failed") {
		t.Fatalf("expected event error, got %v", err)
	}

	runner = &fakeOverviewRunner{
		fakeQueryer: fakeQueryer{results: phalanxSuccessfulReadResults(fakeQueryResult{rows: fakeRowsFromValues()})},
		execErr:     errors.New("update failed"),
	}
	repository = NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if _, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected update error, got %v", err)
	}
}

func TestPhalanxEventVisibilityMatchesLegacyRules(t *testing.T) {
	target := domaingame.PhalanxPlanet{ID: 20, OwnerID: 77}
	mission := func(id int, owner int, kind int, start int, end int) overviewEventScan {
		return overviewEventScan{
			Mission: domaingame.FleetMission{
				ID: id, OwnerID: owner, Mission: kind, DepartureAt: 100, ArrivalAt: 200,
				Origin: domaingame.Coordinates{Position: 2}, Target: domaingame.Coordinates{Position: 6},
			},
			FlightTime: 100, DeployTime: 300, StartPlanetID: start, TargetPlanetID: end,
		}
	}

	outbound := phalanxNonUnionMissions(mission(1, 77, domaingame.FleetMissionTransport, 20, 30), target)
	if len(outbound) != 1 || outbound[0].Mission != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset || outbound[0].ArrivalAt != 300 || outbound[0].Target.Position != 2 {
		t.Fatalf("outbound transport=%+v", outbound)
	}
	if got := phalanxNonUnionMissions(mission(2, 77, domaingame.FleetMissionDeploy, 20, 30), target); len(got) != 0 {
		t.Fatalf("outbound deploy must be hidden: %+v", got)
	}
	if got := phalanxNonUnionMissions(mission(3, 88, domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset, 30, 20), target); len(got) != 0 {
		t.Fatalf("return departing the scanned destination must be hidden: %+v", got)
	}
	inbound := phalanxNonUnionMissions(mission(4, 88, domaingame.FleetMissionAttack, 30, 20), target)
	if len(inbound) != 1 || inbound[0].Mission != domaingame.FleetMissionAttack || inbound[0].ArrivalAt != 200 {
		t.Fatalf("inbound attack=%+v", inbound)
	}
	hold := phalanxNonUnionMissions(mission(5, 88, domaingame.FleetMissionACSHold, 30, 20), target)
	if len(hold) != 2 || hold[0].Mission != domaingame.FleetMissionACSHold+domaingame.FleetMissionOrbitingOffset || hold[0].ArrivalAt != 500 || hold[1].Mission != domaingame.FleetMissionACSHold {
		t.Fatalf("foreign ACS hold=%+v", hold)
	}
	missile := phalanxNonUnionMissions(mission(6, 77, domaingame.FleetMissionMissile, 20, 30), target)
	if len(missile) != 1 || missile[0].Mission != domaingame.FleetMissionMissile || missile[0].ArrivalAt != 300 {
		t.Fatalf("outbound missile=%+v", missile)
	}
}

func TestPhalanxUnionEventsAreGrouped(t *testing.T) {
	runner := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{9})},
		{rows: fakeRowsFromValues(overviewEventRow(300, 88, "attacker", domaingame.FleetMissionACSAttack, map[int]int{domaingame.FleetSmallCargo: 1}, 2_100, 2_500, 2, 6))},
	}}
	repository := NewPhalanxRepositoryWithRunner(runner, nil, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	events, err := repository.loadPhalanxUnionEvents(context.Background(), "`queue`", "`fleet`", "`planets`", "`users`", domaingame.PhalanxPlanet{ID: 20, OwnerID: 77}, domaingame.FleetIDs())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].UnionID != 9 || len(events[0].GroupMissions) != 1 {
		t.Fatalf("grouped union events=%+v", events)
	}
}

func TestPhalanxRepositoryPropagatesOverviewAndPlanetLoadErrors(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}}
	repository := NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if _, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "overview failed") {
		t.Fatalf("expected overview error, got %v", err)
	}

	runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{err: errors.New("source failed")},
	}}}
	repository = NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if _, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "source failed") {
		t.Fatalf("expected source load error, got %v", err)
	}

	runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{rows: fakeRowsFromValues(phalanxPlanetRow(10, 42, "Go Smoke Moon", domaingame.PlanetTypeMoon, 6, 3, 20_000.0))},
		{err: errors.New("target failed")},
	}}}
	repository = NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if _, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20}); err == nil || !strings.Contains(err.Error(), "target failed") {
		t.Fatalf("expected target load error, got %v", err)
	}
}

func TestPhalanxRepositoryHandlesMissingSourceAndTargetRows(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(phalanxPlanetRow(20, 77, "Go Smoke Target", domaingame.PlanetTypePlanet, 2, 0, 1_000_000.0))},
	}}}
	repository := NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	phalanx, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20})
	if err != nil {
		t.Fatal(err)
	}
	if phalanx.Source.ID != 10 || phalanx.ActionIssue == nil || phalanx.ActionIssue.Code != domaingame.PhalanxIssueMissingSensor {
		t.Fatalf("expected overview source fallback with missing sensor issue, got %+v", phalanx)
	}

	runner = &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{rows: fakeRowsFromValues(phalanxPlanetRow(10, 42, "Go Smoke Moon", domaingame.PlanetTypeMoon, 6, 3, 20_000.0))},
		{rows: fakeRowsFromValues()},
	}}}
	repository = NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	phalanx, err = repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20})
	if err != nil {
		t.Fatal(err)
	}
	if phalanx.Target.ID != 20 || phalanx.ActionIssue == nil || phalanx.ActionIssue.Code != domaingame.PhalanxIssueForbidden {
		t.Fatalf("expected missing target to be rejected, got %+v", phalanx)
	}
}

func TestPhalanxRepositoryRowScanEdges(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{{err: errors.New("planet query failed")}}}
	repository := NewPhalanxRepositoryWithRunner(queryer, nil, "ogame_", nil)
	if _, _, err := repository.loadPhalanxPlanet(context.Background(), "ogame_planets", 99); err == nil || !strings.Contains(err.Error(), "planet query failed") {
		t.Fatalf("expected query error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewPhalanxRepositoryWithRunner(queryer, nil, "ogame_", nil)
	if _, _, err := repository.loadPhalanxPlanet(context.Background(), "ogame_planets", 99); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("planet post scan failed"), phalanxPlanetRow(99, 42, "Moon", domaingame.PlanetTypeMoon, 6, 3, 20_000.0))}}}
	repository = NewPhalanxRepositoryWithRunner(queryer, nil, "ogame_", nil)
	if _, _, err := repository.loadPhalanxPlanet(context.Background(), "ogame_planets", 99); err == nil || !strings.Contains(err.Error(), "planet post scan failed") {
		t.Fatalf("expected post scan row error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewPhalanxRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if _, err := repository.loadPhalanxEvents(context.Background(), "ogame_queue", "ogame_fleet", "ogame_planets", "ogame_users", domaingame.PhalanxPlanet{ID: 20, OwnerID: 77}); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected event scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("event rows failed"))}}}
	repository = NewPhalanxRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return time.Unix(2_000, 0) })
	if _, err := repository.loadPhalanxEvents(context.Background(), "ogame_queue", "ogame_fleet", "ogame_planets", "ogame_users", domaingame.PhalanxPlanet{ID: 20, OwnerID: 77}); err == nil || !strings.Contains(err.Error(), "event rows failed") {
		t.Fatalf("expected event rows error, got %v", err)
	}
}

func phalanxOverviewUserRow(activePlanetID int) []any {
	return []any{"legor", int64(0), 0, activePlanetID, 1, 0, 0, 0, 0, 0, 0, 0, int64(0), int64(0), int64(0), int64(0), int64(0)}
}

func phalanxSuccessfulReadResults(eventResult fakeQueryResult) []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues(phalanxOverviewUserRow(10))},
		{rows: fakeRowsFromValues(phalanxOverviewPlanetRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxPlanetSwitchRow(10, "Go Smoke Moon", domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(phalanxUniverseRow())},
		{rows: fakeRowsFromValues(phalanxPlanetRow(10, 42, "Go Smoke Moon", domaingame.PlanetTypeMoon, 6, 3, 20_000.0))},
		{rows: fakeRowsFromValues(phalanxPlanetRow(20, 77, "Go Smoke Target", domaingame.PlanetTypePlanet, 2, 0, 1_000_000.0))},
		eventResult,
		{rows: fakeRowsFromValues()},
	}
}

func phalanxEventRow(id int, ownerID int, ownerName string, mission int, ships map[int]int, start int64, end int64, startPlanetID int, targetPlanetID int, originPosition int, targetPosition int) []any {
	row := overviewEventRow(id, ownerID, ownerName, mission, ships, start, end, originPosition, targetPosition)
	row[10] = startPlanetID
	row[11] = targetPlanetID
	return row
}

func phalanxOverviewPlanetRow(id int, name string, planetType int) []any {
	return []any{
		id, name, planetType, 1, 1, 6, 8888, -2, 2, 4,
		1_000_000.0, 1_000_000.0, 20_000.0,
		0, 0, 0,
		0, 0, 0, 0, 0, 0,
		0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
	}
}

func phalanxPlanetSwitchRow(id int, name string, planetType int) []any {
	return []any{id, name, planetType, 1, 1, 6}
}

func phalanxUniverseRow() []any {
	return []any{1, 0, "", "", int64(0), "", ""}
}

func phalanxPlanetRow(id int, ownerID int, name string, planetType int, position int, phalanxLevel int, deuterium float64) []any {
	return []any{id, ownerID, name, planetType, 1, 1, position, phalanxLevel, deuterium}
}
