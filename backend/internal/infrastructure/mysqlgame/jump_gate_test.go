package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestJumpGateRepositoryReadsReadyMoonGate(t *testing.T) {
	now := time.Unix(1_000, 0)
	queryer := &fakeQueryer{results: append(jumpGateOverviewResults(10, domaingame.PlanetTypeMoon),
		fakeQueryResult{rows: fakeRowsFromValues(jumpGateMoonRow(10, 42, "Moon", domaingame.PlanetTypeMoon, 1, 0, map[int]int{domaingame.FleetSmallCargo: 4, domaingame.FleetSolarSatellite: 9}))},
		fakeQueryResult{rows: fakeRowsFromValues(jumpGateTargetRow(20, 42, "Target", 1, 0))},
	)}
	repository := NewJumpGateRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return now })

	jumpGate, err := repository.GetJumpGate(context.Background(), appgame.JumpGateQuery{PlayerID: 42, PlanetID: 10})
	if err != nil {
		t.Fatal(err)
	}

	if jumpGate.Commander != "legor" || jumpGate.Source.ID != 10 || len(jumpGate.Targets) != 1 || jumpGate.Targets[0].ID != 20 {
		t.Fatalf("unexpected jump gate screen: %+v", jumpGate)
	}
	if len(jumpGate.Ships) != 1 || jumpGate.Ships[0].ID != domaingame.FleetSmallCargo || jumpGate.Ships[0].Count != 4 {
		t.Fatalf("expected mobile ships without solar satellites, got %+v", jumpGate.Ships)
	}
	if !strings.Contains(queryer.calls[4].sql, "`43`, gate_until, `202`") || !strings.Contains(queryer.calls[5].sql, "gate_until <= ?") {
		t.Fatalf("expected legacy jump gate columns and ready-target filter, got %+v", queryer.calls)
	}
}

func TestJumpGateRepositoryReadsMCPJumpGateStatus(t *testing.T) {
	now := time.Unix(1_000, 0)
	queryer := &fakeQueryer{results: append(jumpGateOverviewResults(10, domaingame.PlanetTypeMoon),
		fakeQueryResult{rows: fakeRowsFromValues(jumpGateMoonRow(10, 42, "Moon", domaingame.PlanetTypeMoon, 1, 0, map[int]int{domaingame.FleetSmallCargo: 4, domaingame.FleetSolarSatellite: 9}))},
		fakeQueryResult{rows: fakeRowsFromValues(jumpGateTargetRow(20, 42, "Target", 1, 0))},
	)}
	repository := NewJumpGateRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return now })

	status, err := repository.GetMCPJumpGateStatus(context.Background(), 42, domainmcp.JumpGateStatusCommand{PlanetID: 10})
	if err != nil {
		t.Fatal(err)
	}

	if status.PlayerID != 42 || status.Commander != "legor" || status.Planet.ID != 10 || status.Planet.TypeName != "moon" {
		t.Fatalf("unexpected MCP jump gate status: %+v", status)
	}
	if status.Source.ID != 10 || status.Source.TypeName != "moon" || status.Source.GateLevel != 1 {
		t.Fatalf("unexpected MCP source moon: %+v", status.Source)
	}
	if len(status.Targets) != 1 || status.Targets[0].ID != 20 || status.Targets[0].Coordinates.System != 3 {
		t.Fatalf("unexpected MCP targets: %+v", status.Targets)
	}
	if len(status.Ships) != 1 || status.Ships[0].ID != domaingame.FleetSmallCargo || status.Ships[0].Count != 4 {
		t.Fatalf("expected mobile ships without solar satellites, got %+v", status.Ships)
	}
	if status.Issue != nil {
		t.Fatalf("expected ready moon without issue, got %+v", status.Issue)
	}
}

func TestMCPJumpGateStatusMapsIssueAndEmptyBranches(t *testing.T) {
	status := mcpJumpGateStatus(42, domaingame.JumpGate{
		Commander: "legor",
		CurrentPlanet: domaingame.PlanetOverview{
			ID:          10,
			Name:        "Moon",
			Type:        domaingame.PlanetTypeMoon,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
		},
		Source: domaingame.JumpGateMoon{
			ID:          10,
			OwnerID:     42,
			Name:        "Moon",
			Type:        domaingame.PlanetTypeMoon,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
			GateLevel:   1,
			GateUntil:   1_100,
		},
		Targets: []domaingame.JumpGateMoon{{
			ID:          20,
			OwnerID:     42,
			Name:        "Target",
			Type:        domaingame.PlanetTypeMoon,
			Coordinates: domaingame.Coordinates{Galaxy: 1, System: 3, Position: 4},
			GateLevel:   1,
		}},
		Ships: []domaingame.JumpGateShip{{ID: domaingame.FleetLightFighter, Name: "Light Fighter", Count: 2}},
		ActionIssue: &domaingame.JumpGateActionIssue{
			Code:    domaingame.JumpGateIssueCooldown,
			Message: "cooldown",
		},
	})

	if status.Issue == nil || status.Issue.Code != domaingame.JumpGateIssueCooldown || status.Issue.Message != "cooldown" {
		t.Fatalf("expected MCP issue mapping, got %+v", status.Issue)
	}
	if len(status.Targets) != 1 || status.Targets[0].TypeName != "moon" || status.Targets[0].GateLevel != 1 {
		t.Fatalf("unexpected target mapping: %+v", status.Targets)
	}
	if len(status.Ships) != 1 || status.Ships[0].ID != domaingame.FleetLightFighter || status.Ships[0].Count != 2 {
		t.Fatalf("unexpected ship mapping: %+v", status.Ships)
	}
	if got := mcpJumpGateIssue(nil); got != nil {
		t.Fatalf("nil issue should stay nil, got %+v", got)
	}
}

func TestNewJumpGateRepositoryConstructorsAndDependencyErrors(t *testing.T) {
	repository := NewJumpGateRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	repository = NewJumpGateRepositoryWithRunner(nil, nil, "ogame_", nil)
	if repository.now == nil {
		t.Fatal("expected default clock")
	}
	if _, err := repository.GetJumpGate(context.Background(), appgame.JumpGateQuery{}); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("expected reader dependency error, got %v", err)
	}
	if _, err := repository.GetMCPJumpGateStatus(context.Background(), 42, domainmcp.JumpGateStatusCommand{}); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("expected MCP reader dependency error, got %v", err)
	}
	if _, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{}); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("expected mutation reader dependency error, got %v", err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", nil)
	if _, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{}); err == nil || !strings.Contains(err.Error(), "writer unavailable") {
		t.Fatalf("expected writer dependency error, got %v", err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{}, &fakeGalaxyRunner{}, "bad-prefix_", nil)
	if _, err := repository.GetJumpGate(context.Background(), appgame.JumpGateQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected prefix error, got %v", err)
	}
}

func TestJumpGateRepositoryMovesShipsAndSetsCooldown(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(jumpGateMoonRow(10, 42, "Moon", domaingame.PlanetTypeMoon, 1, 0, map[int]int{domaingame.FleetSmallCargo: 5, domaingame.FleetLightFighter: 3, domaingame.FleetSolarSatellite: 7}))},
		{rows: fakeRowsFromValues(jumpGateMoonRow(20, 42, "Target", domaingame.PlanetTypeMoon, 1, 0, map[int]int{domaingame.FleetSmallCargo: 1, domaingame.FleetLightFighter: 1}))},
		{rows: fakeRowsFromValues([]any{128.0})},
		{rows: fakeRowsFromValues(jumpGateOverviewUserRow(10))},
		{rows: fakeRowsFromValues(jumpGateOverviewPlanetRow(10, domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(jumpGateOverviewSwitcherRow(10, domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewJumpGateRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	jumpGate, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{
		PlayerID:     42,
		PlanetID:     10,
		SourceMoonID: 10,
		TargetMoonID: 20,
		Ships: map[int]int{
			domaingame.FleetSmallCargo:     2,
			domaingame.FleetLightFighter:   1,
			domaingame.FleetSolarSatellite: 7,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if jumpGate.ActionIssue == nil || jumpGate.ActionIssue.Code != domaingame.JumpGateIssueMoved {
		t.Fatalf("expected moved issue, got %+v", jumpGate.ActionIssue)
	}
	if jumpGate.Source.Ships[domaingame.FleetSmallCargo] != 3 || jumpGate.Source.Ships[domaingame.FleetLightFighter] != 2 || jumpGate.Source.Ships[domaingame.FleetSolarSatellite] != 7 {
		t.Fatalf("expected source mobile ships moved and satellite unchanged, got %+v", jumpGate.Source.Ships)
	}
	if len(runner.execCalls) != 2 {
		t.Fatalf("expected two moon updates, got %+v", runner.execCalls)
	}
	cooldown := domaingame.JumpGateCooldownUntil(now.Unix(), 128)
	sourceCall := runner.execCalls[0]
	if !strings.Contains(sourceCall.sql, "`202` = `202` - ?") || !strings.Contains(sourceCall.sql, "`204` = `204` - ?") || strings.Contains(sourceCall.sql, "`212`") {
		t.Fatalf("unexpected source update sql: %s", sourceCall.sql)
	}
	if len(sourceCall.args) != 7 || sourceCall.args[0] != 2 || sourceCall.args[1] != 1 || sourceCall.args[2] != cooldown || sourceCall.args[3] != 10 || sourceCall.args[4] != 42 || sourceCall.args[5] != 2 || sourceCall.args[6] != 1 {
		t.Fatalf("unexpected source update args: %+v", sourceCall.args)
	}
	targetCall := runner.execCalls[1]
	if !strings.Contains(targetCall.sql, "`202` = `202` + ?") || !strings.Contains(targetCall.sql, "gate_until = ?") {
		t.Fatalf("unexpected target update sql: %s", targetCall.sql)
	}
	if len(targetCall.args) != 5 || targetCall.args[0] != 2 || targetCall.args[1] != 1 || targetCall.args[2] != cooldown || targetCall.args[3] != 20 || targetCall.args[4] != 42 {
		t.Fatalf("unexpected target update args: %+v", targetCall.args)
	}
}

func TestJumpGateRepositoryHelperEdges(t *testing.T) {
	now := time.Unix(1_000, 0)
	repository := NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, nil, "ogame_", func() time.Time { return now })
	moon, found, err := repository.loadJumpGateMoon(context.Background(), "`ogame_planets`", 10)
	if err != nil || found || moon.ID != 0 {
		t.Fatalf("expected missing moon, found=%v moon=%+v err=%v", found, moon, err)
	}
	moon, found, err = repository.loadJumpGateMoon(context.Background(), "`ogame_planets`", 0)
	if err != nil || found || moon.ID != 0 {
		t.Fatalf("expected zero id no-op, found=%v moon=%+v err=%v", found, moon, err)
	}

	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, nil, "ogame_", func() time.Time { return now })
	speed, err := repository.loadJumpGateFleetSpeed(context.Background(), "`ogame_uni`")
	if err != nil || speed != 1 {
		t.Fatalf("expected default fleet speed, got speed=%v err=%v", speed, err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0.0})}}}, nil, "ogame_", func() time.Time { return now })
	speed, err = repository.loadJumpGateFleetSpeed(context.Background(), "`ogame_uni`")
	if err != nil || speed != 1 {
		t.Fatalf("expected non-positive fleet speed default, got speed=%v err=%v", speed, err)
	}

	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{}, &fakeGalaxyRunner{}, "ogame_", func() time.Time { return now })
	if err := repository.adjustJumpGateShips(context.Background(), "`ogame_planets`", 10, 42, map[int]int{}, -1, 100); err != nil {
		t.Fatalf("empty ship adjustment should be no-op: %v", err)
	}
	runner := &fakeGalaxyRunner{execResults: []sql.Result{fakeSQLResult(0)}}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{}, runner, "ogame_", func() time.Time { return now })
	if err := repository.adjustJumpGateShips(context.Background(), "`ogame_planets`", 10, 42, map[int]int{domaingame.FleetSmallCargo: 1}, -1, 100); err == nil || !strings.Contains(err.Error(), "did not affect one row") {
		t.Fatalf("expected rows-affected race error, got %v", err)
	}
}

func TestJumpGateRepositoryViewIssueBranches(t *testing.T) {
	now := time.Unix(1_000, 0)
	repository := NewJumpGateRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", func() time.Time { return now })
	ready := domaingame.JumpGateMoon{ID: 10, OwnerID: 42, Type: domaingame.PlanetTypeMoon, GateLevel: 1}
	if issue := repository.jumpGateViewIssue(42, ready, true); issue != nil {
		t.Fatalf("expected ready source without issue, got %+v", issue)
	}
	if issue := repository.jumpGateViewIssue(42, ready, false); issue == nil || issue.Code != domaingame.JumpGateIssueSourceMoonMissing {
		t.Fatalf("expected missing source issue, got %+v", issue)
	}
	planet := ready
	planet.Type = domaingame.PlanetTypePlanet
	if issue := repository.jumpGateViewIssue(42, planet, true); issue == nil || issue.Code != domaingame.JumpGateIssueSourceMoonMissing {
		t.Fatalf("expected planet source issue, got %+v", issue)
	}
	foreign := ready
	foreign.OwnerID = 77
	if issue := repository.jumpGateViewIssue(42, foreign, true); issue == nil || issue.Code != domaingame.JumpGateIssueForeignMoon {
		t.Fatalf("expected foreign issue, got %+v", issue)
	}
	noGate := ready
	noGate.GateLevel = 0
	if issue := repository.jumpGateViewIssue(42, noGate, true); issue == nil || issue.Code != domaingame.JumpGateIssueSourceGateMissing {
		t.Fatalf("expected no-gate issue, got %+v", issue)
	}
	cooling := ready
	cooling.GateUntil = 1_100
	if issue := repository.jumpGateViewIssue(42, cooling, true); issue == nil || issue.Code != domaingame.JumpGateIssueCooldown {
		t.Fatalf("expected cooldown issue, got %+v", issue)
	}
}

func TestJumpGateRepositoryMutationResultAppendsReadyTarget(t *testing.T) {
	now := time.Unix(1_000, 0)
	queryer := &fakeQueryer{results: append(jumpGateOverviewResults(10, domaingame.PlanetTypeMoon),
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}
	repository := NewJumpGateRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return now })
	jumpGate, err := repository.jumpGateMutationResult(
		context.Background(),
		appgame.JumpGateMutationQuery{PlayerID: 42, PlanetID: 10},
		"`ogame_planets`",
		domaingame.JumpGateMoon{ID: 10, OwnerID: 42, Name: "Moon", Type: domaingame.PlanetTypeMoon, GateLevel: 1},
		domaingame.JumpGateMoon{ID: 20, OwnerID: 42, Name: "Target", Type: domaingame.PlanetTypeMoon, GateLevel: 1, GateUntil: 0},
		domaingame.JumpGateMovedIssue(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(jumpGate.Targets) != 1 || jumpGate.Targets[0].ID != 20 {
		t.Fatalf("expected ready mutation target to be retained, got %+v", jumpGate.Targets)
	}
}

func TestJumpGateRepositoryReadErrorBranches(t *testing.T) {
	now := time.Unix(1_000, 0)
	repository := NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, nil, "ogame_", func() time.Time { return now })
	if _, err := repository.GetJumpGate(context.Background(), appgame.JumpGateQuery{PlayerID: 42, PlanetID: 10}); err == nil || !strings.Contains(err.Error(), "overview failed") {
		t.Fatalf("expected overview error, got %v", err)
	}

	queryer := &fakeQueryer{results: append(jumpGateOverviewResults(10, domaingame.PlanetTypeMoon), fakeQueryResult{err: errors.New("source failed")})}
	repository = NewJumpGateRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return now })
	if _, err := repository.GetJumpGate(context.Background(), appgame.JumpGateQuery{PlayerID: 42, PlanetID: 10}); err == nil || !strings.Contains(err.Error(), "source failed") {
		t.Fatalf("expected source error, got %v", err)
	}

	queryer = &fakeQueryer{results: append(jumpGateOverviewResults(10, domaingame.PlanetTypeMoon),
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}
	repository = NewJumpGateRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return now })
	jumpGate, err := repository.GetJumpGate(context.Background(), appgame.JumpGateQuery{PlayerID: 42, PlanetID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if jumpGate.ActionIssue == nil || jumpGate.ActionIssue.Code != domaingame.JumpGateIssueSourceMoonMissing {
		t.Fatalf("expected missing source issue, got %+v", jumpGate.ActionIssue)
	}

	queryer = &fakeQueryer{results: append(jumpGateOverviewResults(10, domaingame.PlanetTypeMoon),
		fakeQueryResult{rows: fakeRowsFromValues(jumpGateMoonRow(10, 42, "Moon", domaingame.PlanetTypeMoon, 1, 0, nil))},
		fakeQueryResult{err: errors.New("targets failed")},
	)}
	repository = NewJumpGateRepositoryWithRunner(queryer, nil, "ogame_", func() time.Time { return now })
	if _, err := repository.GetJumpGate(context.Background(), appgame.JumpGateQuery{PlayerID: 42, PlanetID: 10}); err == nil || !strings.Contains(err.Error(), "targets failed") {
		t.Fatalf("expected targets error, got %v", err)
	}
}

func TestJumpGateRepositoryMutationErrorBranches(t *testing.T) {
	now := time.Unix(1_000, 0)
	sourceRow := jumpGateMoonRow(10, 42, "Moon", domaingame.PlanetTypeMoon, 1, 0, map[int]int{domaingame.FleetSmallCargo: 5})
	targetRow := jumpGateMoonRow(20, 42, "Target", domaingame.PlanetTypeMoon, 1, 0, nil)
	repository := NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("source failed")}}}, &fakeGalaxyRunner{}, "ogame_", func() time.Time { return now })
	if _, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{PlayerID: 42, PlanetID: 10, SourceMoonID: 10, TargetMoonID: 20}); err == nil || !strings.Contains(err.Error(), "source failed") {
		t.Fatalf("expected source error, got %v", err)
	}

	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(sourceRow)},
		{err: errors.New("target failed")},
	}}, &fakeGalaxyRunner{}, "ogame_", func() time.Time { return now })
	if _, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{PlayerID: 42, PlanetID: 10, SourceMoonID: 10, TargetMoonID: 20}); err == nil || !strings.Contains(err.Error(), "target failed") {
		t.Fatalf("expected target error, got %v", err)
	}

	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(sourceRow)},
		{rows: fakeRowsFromValues(targetRow)},
		{err: errors.New("speed failed")},
	}}, &fakeGalaxyRunner{}, "ogame_", func() time.Time { return now })
	if _, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{PlayerID: 42, PlanetID: 10, SourceMoonID: 10, TargetMoonID: 20, Ships: map[int]int{domaingame.FleetSmallCargo: 1}}); err == nil || !strings.Contains(err.Error(), "speed failed") {
		t.Fatalf("expected speed error, got %v", err)
	}

	runner := &fakeGalaxyRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(sourceRow)},
			{rows: fakeRowsFromValues(targetRow)},
			{rows: fakeRowsFromValues([]any{1.0})},
		}},
		execErrs: []error{errors.New("source update failed")},
	}
	repository = NewJumpGateRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{PlayerID: 42, PlanetID: 10, SourceMoonID: 10, TargetMoonID: 20, Ships: map[int]int{domaingame.FleetSmallCargo: 1}}); err == nil || !strings.Contains(err.Error(), "source update failed") {
		t.Fatalf("expected source update error, got %v", err)
	}

	runner = &fakeGalaxyRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues(sourceRow)},
			{rows: fakeRowsFromValues(targetRow)},
			{rows: fakeRowsFromValues([]any{1.0})},
		}},
		execErrs: []error{nil, errors.New("target update failed")},
	}
	repository = NewJumpGateRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{PlayerID: 42, PlanetID: 10, SourceMoonID: 10, TargetMoonID: 20, Ships: map[int]int{domaingame.FleetSmallCargo: 1}}); err == nil || !strings.Contains(err.Error(), "target update failed") {
		t.Fatalf("expected target update error, got %v", err)
	}

	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(sourceRow)},
		{rows: fakeRowsFromValues(targetRow)},
		{rows: fakeRowsFromValues(jumpGateOverviewUserRow(10))},
		{rows: fakeRowsFromValues(jumpGateOverviewPlanetRow(10, domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(jumpGateOverviewSwitcherRow(10, domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues()},
	}}
	repository = NewJumpGateRepositoryWithRunner(queryer, &fakeGalaxyRunner{}, "ogame_", func() time.Time { return now })
	jumpGate, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{PlayerID: 42, PlanetID: 10, TargetMoonID: 20})
	if err != nil {
		t.Fatal(err)
	}
	if jumpGate.ActionIssue == nil || jumpGate.ActionIssue.Code != domaingame.JumpGateIssueNoShips {
		t.Fatalf("expected source fallback with no-ships issue, got %+v", jumpGate.ActionIssue)
	}
}

func TestJumpGateRepositoryMutationResultErrors(t *testing.T) {
	repository := NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, nil, "ogame_", nil)
	_, err := repository.jumpGateMutationResult(context.Background(), appgame.JumpGateMutationQuery{PlayerID: 42}, "`ogame_planets`", domaingame.JumpGateMoon{ID: 10}, domaingame.JumpGateMoon{}, nil)
	if err == nil || !strings.Contains(err.Error(), "overview failed") {
		t.Fatalf("expected overview error, got %v", err)
	}

	queryer := &fakeQueryer{results: append(jumpGateOverviewResults(10, domaingame.PlanetTypeMoon), fakeQueryResult{err: errors.New("targets failed")})}
	repository = NewJumpGateRepositoryWithRunner(queryer, nil, "ogame_", nil)
	_, err = repository.jumpGateMutationResult(context.Background(), appgame.JumpGateMutationQuery{PlayerID: 42}, "`ogame_planets`", domaingame.JumpGateMoon{ID: 10}, domaingame.JumpGateMoon{}, nil)
	if err == nil || !strings.Contains(err.Error(), "targets failed") {
		t.Fatalf("expected targets error, got %v", err)
	}
}

func TestJumpGateRepositoryHelperErrors(t *testing.T) {
	repository := NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("moon query failed")}}}, nil, "ogame_", nil)
	if _, _, err := repository.loadJumpGateMoon(context.Background(), "`ogame_planets`", 10); err == nil || !strings.Contains(err.Error(), "moon query failed") {
		t.Fatalf("expected moon query error, got %v", err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, nil, "ogame_", nil)
	if _, _, err := repository.loadJumpGateMoon(context.Background(), "`ogame_planets`", 10); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected moon scan error, got %v", err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("moon rows failed"), jumpGateMoonRow(10, 42, "Moon", domaingame.PlanetTypeMoon, 1, 0, nil))}}}, nil, "ogame_", nil)
	if _, _, err := repository.loadJumpGateMoon(context.Background(), "`ogame_planets`", 10); err == nil || !strings.Contains(err.Error(), "moon rows failed") {
		t.Fatalf("expected moon rows error, got %v", err)
	}

	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("targets query failed")}}}, nil, "ogame_", nil)
	if _, err := repository.loadJumpGateTargets(context.Background(), "`ogame_planets`", 42, 10); err == nil || !strings.Contains(err.Error(), "targets query failed") {
		t.Fatalf("expected targets query error, got %v", err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, nil, "ogame_", nil)
	if _, err := repository.loadJumpGateTargets(context.Background(), "`ogame_planets`", 42, 10); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected targets scan error, got %v", err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("targets rows failed"), jumpGateTargetRow(20, 42, "Target", 1, 0))}}}, nil, "ogame_", nil)
	if _, err := repository.loadJumpGateTargets(context.Background(), "`ogame_planets`", 42, 10); err == nil || !strings.Contains(err.Error(), "targets rows failed") {
		t.Fatalf("expected targets rows error, got %v", err)
	}

	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("speed query failed")}}}, nil, "ogame_", nil)
	if _, err := repository.loadJumpGateFleetSpeed(context.Background(), "`ogame_uni`"); err == nil || !strings.Contains(err.Error(), "speed query failed") {
		t.Fatalf("expected speed query error, got %v", err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, nil, "ogame_", nil)
	if _, err := repository.loadJumpGateFleetSpeed(context.Background(), "`ogame_uni`"); err == nil || !strings.Contains(err.Error(), "expected float64") {
		t.Fatalf("expected speed scan error, got %v", err)
	}
	repository = NewJumpGateRepositoryWithRunner(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("speed rows failed"), []any{1.0})}}}, nil, "ogame_", nil)
	if _, err := repository.loadJumpGateFleetSpeed(context.Background(), "`ogame_uni`"); err == nil || !strings.Contains(err.Error(), "speed rows failed") {
		t.Fatalf("expected speed rows error, got %v", err)
	}
}

func TestJumpGateRepositoryRejectsInvalidMovesWithoutWriting(t *testing.T) {
	now := time.Unix(1_000, 0)
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(jumpGateMoonRow(10, 42, "Moon", domaingame.PlanetTypeMoon, 0, 0, map[int]int{domaingame.FleetSmallCargo: 5}))},
		{rows: fakeRowsFromValues(jumpGateMoonRow(20, 42, "Target", domaingame.PlanetTypeMoon, 1, 0, nil))},
		{rows: fakeRowsFromValues(jumpGateOverviewUserRow(10))},
		{rows: fakeRowsFromValues(jumpGateOverviewPlanetRow(10, domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(jumpGateOverviewSwitcherRow(10, domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewJumpGateRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	jumpGate, err := repository.Jump(context.Background(), appgame.JumpGateMutationQuery{
		PlayerID:     42,
		PlanetID:     10,
		SourceMoonID: 10,
		TargetMoonID: 20,
		Ships:        map[int]int{domaingame.FleetSmallCargo: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jumpGate.ActionIssue == nil || jumpGate.ActionIssue.Code != domaingame.JumpGateIssueSourceGateMissing {
		t.Fatalf("expected missing source gate issue, got %+v", jumpGate.ActionIssue)
	}
	if len(runner.execCalls) != 0 {
		t.Fatalf("invalid jump should not write, got %+v", runner.execCalls)
	}
}

func jumpGateOverviewResults(planetID int, planetType int) []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues(jumpGateOverviewUserRow(planetID))},
		{rows: fakeRowsFromValues(jumpGateOverviewPlanetRow(planetID, planetType))},
		{rows: fakeRowsFromValues(jumpGateOverviewSwitcherRow(planetID, planetType))},
		{rows: fakeRowsFromValues([]any{1})},
	}
}

func jumpGateOverviewUserRow(planetID int) []any {
	return []any{"legor", int64(0), 0, planetID, 1, 0, 0, 0}
}

func jumpGateOverviewPlanetRow(planetID int, planetType int) []any {
	return []any{planetID, "Moon", planetType, 1, 2, 3, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0}
}

func jumpGateOverviewSwitcherRow(planetID int, planetType int) []any {
	return []any{planetID, "Moon", planetType, 1, 2, 3}
}

func jumpGateMoonRow(planetID int, ownerID int, name string, planetType int, gateLevel int, gateUntil int64, ships map[int]int) []any {
	row := []any{planetID, ownerID, name, planetType, 1, 2, 3, gateLevel, gateUntil}
	for _, id := range domaingame.FleetIDs() {
		row = append(row, ships[id])
	}
	return row
}

func jumpGateTargetRow(planetID int, ownerID int, name string, gateLevel int, gateUntil int64) []any {
	return []any{planetID, ownerID, name, domaingame.PlanetTypeMoon, 1, 3, 4, gateLevel, gateUntil}
}
