package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
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
		{rows: fakeRowsFromValues(overviewEventRow(300, 77, "target", domaingame.FleetMissionTransport, map[int]int{domaingame.FleetSmallCargo: 1}, 2_100, 2_500, 2, 6))},
	}}}
	repository := NewPhalanxRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	phalanx, err := repository.GetPhalanx(context.Background(), appgame.PhalanxQuery{PlayerID: 42, PlanetID: 10, TargetPlanetID: 20})
	if err != nil {
		t.Fatal(err)
	}
	if phalanx.Commander != "legor" || phalanx.Source.ID != 10 || phalanx.Target.ID != 20 {
		t.Fatalf("unexpected phalanx result: %+v", phalanx)
	}
	if phalanx.RemainingDeuterium != 15_000 || len(phalanx.Events) != 2 || phalanx.Events[0].Mission != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset || phalanx.Events[1].ID != 300 {
		t.Fatalf("expected spent deuterium and legacy return/outgoing event order, got %+v", phalanx)
	}
	if len(runner.execCalls) != 1 {
		t.Fatalf("expected one deuterium update, got %+v", runner.execCalls)
	}
	args := runner.execCalls[0].args
	if len(args) != 4 || args[0] != float64(15_000) || args[1] != now.Unix() || args[2] != 10 || args[3] != 42 {
		t.Fatalf("unexpected deuterium update args: %+v", args)
	}
	lastQuery := runner.calls[len(runner.calls)-1].sql
	if !strings.Contains(lastQuery, "f.`202`") || strings.Contains(lastQuery, "SELECT q.sub_id, q.start, q.end, `202`") {
		t.Fatalf("expected phalanx fleet query to use prefixed fleet columns, got %s", lastQuery)
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
	}
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
