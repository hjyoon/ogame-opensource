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
)

func TestGalaxyRepositoryReadsLegacyGalaxyScreen(t *testing.T) {
	now := time.Unix(10_000, 0)
	queryer := &fakeQueryer{results: append(galaxyReadPrefixResults(now),
		fakeQueryResult{rows: fakeRowsFromValues(
			galaxyObjectRow(200, "Target", domaingame.PlanetTypePlanet, 4, now.Unix()-60, 0, 0, 7, "enemy", 1000, 12, 5, now.Unix(), 0, 0, 5, "TAG"),
			galaxyObjectRow(201, "Moon", domaingame.PlanetTypeMoon, 4, now.Unix()-30, 0, 0, 7, "enemy", 1000, 12, 5, now.Unix(), 0, 0, 5, "TAG"),
			galaxyObjectRow(202, "", domaingame.PlanetTypeDebris, 4, 0, 200, 100, 0, "", 0, 0, 0, 0, 0, 0, 0, ""),
		)},
	)}
	repository := NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	galaxy, err := repository.GetGalaxy(context.Background(), appgame.GalaxyQuery{
		PlayerID: 42,
	})
	if err != nil {
		t.Fatal(err)
	}

	if galaxy.Commander != "legor" || galaxy.CurrentPlanet.ID != 99 || galaxy.Coordinates.System != 2 || len(galaxy.Rows) != domaingame.GalaxyPositions {
		t.Fatalf("unexpected galaxy summary: %+v", galaxy)
	}
	row := galaxy.Rows[3]
	if row.Planet == nil || row.Planet.Player == nil || row.Planet.Player.Name != "enemy" || row.Planet.ActivityText != "(*)" {
		t.Fatalf("unexpected galaxy planet row: %+v", row.Planet)
	}
	if row.Moon == nil || row.Debris == nil || !row.Debris.Visible || row.Debris.Harvesters != 1 {
		t.Fatalf("unexpected moon/debris rows: %+v", row)
	}
	if !galaxy.Extra.Commander || galaxy.Extra.SpyProbes != 4 || galaxy.Extra.Recyclers != 3 || galaxy.Extra.Missiles != 2 {
		t.Fatalf("unexpected extra info: %+v", galaxy.Extra)
	}
	if galaxy.Slots.Used != 1 || galaxy.Slots.BaseMax != 4 || galaxy.Slots.Max != 6 || !galaxy.Slots.Admiral {
		t.Fatalf("unexpected fleet slots: %+v", galaxy.Slots)
	}
	if !strings.Contains(queryer.calls[5].sql, "`210`, `209`, `503`") ||
		!strings.Contains(queryer.calls[10].sql, "p.`700`, p.`701`") {
		t.Fatalf("expected legacy numeric columns, got %+v", queryer.calls)
	}
}

func TestNewGalaxyRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewGalaxyRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}

	withDefaultClock := NewGalaxyRepositoryWithQueryer(&fakeQueryer{}, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected default clock")
	}
	withRunner := NewGalaxyRepositoryWithQueryer(&fakeGalaxyRunner{}, "ogame_", nil)
	if withRunner.execer == nil {
		t.Fatal("expected queryer execer to be reused")
	}
}

func TestGalaxyRepositoryLaunchMissilesRequiresExecer(t *testing.T) {
	repository := NewGalaxyRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", nil)
	if _, err := repository.LaunchMissiles(context.Background(), appgame.GalaxyMissileLaunchQuery{}); err == nil || !strings.Contains(err.Error(), "mutation unavailable") {
		t.Fatalf("expected mutation unavailable error, got %v", err)
	}
}

func TestGalaxyRepositoryLaunchesMissiles(t *testing.T) {
	now := time.Unix(10_000, 0)
	runner := &fakeGalaxyRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{0})},
			{rows: fakeRowsFromValues(galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2))},
			{rows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 4, 5, int64(10000), 0, 0, 0, now.Unix()))},
		}},
		execResults: []sql.Result{
			galaxySQLResult{rows: 1},
			galaxySQLResult{rows: 1},
			galaxySQLResult{id: 123, rows: 1},
			galaxySQLResult{rows: 1},
			galaxySQLResult{rows: 1},
		},
	}
	repository := NewGalaxyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	issue, err := repository.LaunchMissiles(context.Background(), appgame.GalaxyMissileLaunchQuery{
		PlayerID:        42,
		PlanetID:        0,
		TargetPlanetID:  77,
		Amount:          2,
		TargetDefenseID: domaingame.DefenseRocketLauncher,
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.GalaxyIssueRocketLaunched || issue.Message != "Start of rocket 2!" {
		t.Fatalf("unexpected launch issue: %+v", issue)
	}
	if len(runner.execCalls) != 5 {
		t.Fatalf("expected delete/update/fleet/log/queue execs, got %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[1].sql, "`503` = `503` - ?") || runner.execCalls[1].args[0] != 2 || runner.execCalls[1].args[2] != 99 {
		t.Fatalf("expected IPM reservation, got %+v", runner.execCalls[1])
	}
	if !strings.Contains(runner.execCalls[2].sql, "ipm_amount, ipm_target") || runner.execCalls[2].args[3] != domaingame.FleetMissionMissile {
		t.Fatalf("expected missile fleet insert, got %+v", runner.execCalls[2])
	}
	if !strings.Contains(runner.execCalls[3].sql, "fleetlogs") || runner.execCalls[3].args[17] != 2 || runner.execCalls[3].args[18] != domaingame.DefenseRocketLauncher {
		t.Fatalf("expected missile fleet log insert, got %+v", runner.execCalls[3])
	}
}

func TestGalaxyRepositoryLaunchMissileValidationIssues(t *testing.T) {
	now := time.Unix(10_000, 0)
	tests := []struct {
		name          string
		amount        int
		targetRows    *fakeRows
		originRow     []any
		targetDefense int
		want          string
	}{
		{
			name:       "missing target",
			amount:     1,
			targetRows: fakeRowsFromValues(),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2),
			want:       domaingame.GalaxyIssueRocketNoTarget,
		},
		{
			name:       "zero amount",
			amount:     0,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 4, 5, int64(10000), 0, 0, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2),
			want:       domaingame.GalaxyIssueRocketNoRockets,
		},
		{
			name:       "not enough",
			amount:     6,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 4, 5, int64(10000), 0, 0, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2),
			want:       domaingame.GalaxyIssueRocketNotEnough,
		},
		{
			name:       "weak drive",
			amount:     1,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 20, 5, int64(10000), 0, 0, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 1),
			want:       domaingame.GalaxyIssueRocketWeakDrive,
		},
		{
			name:       "weak drive overrides not enough",
			amount:     6,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 20, 5, int64(10000), 0, 0, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 1),
			want:       domaingame.GalaxyIssueRocketWeakDrive,
		},
		{
			name:       "self",
			amount:     1,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 42, 1, 4, 5, int64(10000), 0, 0, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2),
			want:       domaingame.GalaxyIssueRocketSelf,
		},
		{
			name:       "origin vacation",
			amount:     1,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 4, 5, int64(10000), 0, 0, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 1, 0, now.Unix(), 2),
			want:       domaingame.GalaxyIssueRocketVacationSelf,
		},
		{
			name:       "target vacation",
			amount:     1,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 4, 5, int64(10000), 0, 1, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2),
			want:       domaingame.GalaxyIssueRocketVacationOther,
		},
		{
			name:       "admin target",
			amount:     1,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 4, 5, int64(10000), 1, 0, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2),
			want:       domaingame.GalaxyIssueRocketAdmin,
		},
		{
			name:       "noob protected target",
			amount:     1,
			targetRows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 4, 5, int64(1000), 0, 0, 0, now.Unix())),
			originRow:  galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(100000), 0, 0, 0, now.Unix(), 2),
			want:       domaingame.GalaxyIssueRocketNoob,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{0})},
				{rows: fakeRowsFromValues(tt.originRow)},
				{rows: tt.targetRows},
			}}}
			repository := NewGalaxyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
			issue, err := repository.LaunchMissiles(context.Background(), appgame.GalaxyMissileLaunchQuery{
				PlayerID:        42,
				PlanetID:        99,
				TargetPlanetID:  77,
				Amount:          tt.amount,
				TargetDefenseID: tt.targetDefense,
			})
			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.want {
				t.Fatalf("expected %s, got %+v", tt.want, issue)
			}
			if len(runner.execCalls) != 0 {
				t.Fatalf("validation failure should not mutate, got %+v", runner.execCalls)
			}
		})
	}
}

func TestGalaxyRepositoryLaunchMissileFrozenAndRaceIssues(t *testing.T) {
	now := time.Unix(10_000, 0)
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	repository := NewGalaxyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	issue, err := repository.LaunchMissiles(context.Background(), appgame.GalaxyMissileLaunchQuery{PlayerID: 42, Amount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.GalaxyIssueRocketFrozen {
		t.Fatalf("expected frozen issue, got %+v", issue)
	}
	if len(runner.execCalls) != 0 {
		t.Fatalf("frozen universe should not mutate, got %+v", runner.execCalls)
	}

	runner = &fakeGalaxyRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{0})},
			{rows: fakeRowsFromValues(galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2))},
			{rows: fakeRowsFromValues(galaxyMissileTargetRow(77, 7, 1, 4, 5, int64(10000), 0, 0, 0, now.Unix()))},
		}},
		execResults: []sql.Result{
			galaxySQLResult{rows: 1},
			galaxySQLResult{rows: 0},
		},
	}
	repository = NewGalaxyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	issue, err = repository.LaunchMissiles(context.Background(), appgame.GalaxyMissileLaunchQuery{
		PlayerID:       42,
		TargetPlanetID: 77,
		Amount:         1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue == nil || issue.Code != domaingame.GalaxyIssueRocketLaunchRace {
		t.Fatalf("expected launch race issue, got %+v", issue)
	}
	if len(runner.execCalls) != 2 {
		t.Fatalf("expected delete and reserve execs, got %+v", runner.execCalls)
	}
}

func TestGalaxyRepositoryReturnsErrors(t *testing.T) {
	now := time.Unix(10_000, 0)
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:    "unsafe prefix",
			prefix:  "bad-prefix_",
			queryer: &fakeQueryer{},
			want:    "invalid database table prefix",
		},
		{
			name:    "overview",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}},
			want:    "overview failed",
		},
		{
			name:    "viewer",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})},
			want:    "galaxy viewer not found",
		},
		{
			name:    "units",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{int64(1000), 0, 0, 1, now.Add(time.Hour).Unix()})}, fakeQueryResult{err: errors.New("units failed")})},
			want:    "units failed",
		},
		{
			name:    "research",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(galaxyViewerPrefixResults(now), fakeQueryResult{err: errors.New("research failed")})},
			want:    "research failed",
		},
		{
			name:    "admiral",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(galaxyViewerPrefixResults(now), fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(nil))}, fakeQueryResult{err: errors.New("admiral failed")})},
			want:    "admiral failed",
		},
		{
			name:    "active fleets",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(galaxyPremiumPrefixResults(now), fakeQueryResult{err: errors.New("fleet count failed")})},
			want:    "fleet count failed",
		},
		{
			name:    "bounds",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(galaxyPremiumPrefixResults(now), fakeQueryResult{rows: fakeRowsFromValues([]any{0})}, fakeQueryResult{err: errors.New("bounds failed")})},
			want:    "bounds failed",
		},
		{
			name:    "objects",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(galaxyReadPrefixResults(now), fakeQueryResult{err: errors.New("objects failed")})},
			want:    "objects failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewGalaxyRepositoryWithQueryer(tt.queryer, tt.prefix, func() time.Time { return now })
			_, err := repository.GetGalaxy(context.Background(), appgame.GalaxyQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestGalaxyRepositoryLoadersHandleRowsAndScanEdges(t *testing.T) {
	now := time.Unix(10_000, 0)
	repository := NewGalaxyRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })

	if _, err := repository.loadGalaxyViewer(context.Background(), "ogame_users", 42); err == nil {
		t.Fatal("expected viewer query error")
	}

	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("viewer rows failed"), []any{int64(1), 0, 0, 1, int64(0)})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadGalaxyViewer(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "viewer rows failed") {
		t.Fatalf("expected viewer rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 0, 0, 1, int64(0)})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadGalaxyViewer(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "expected int64") {
		t.Fatalf("expected viewer scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, _, _, err := repository.loadGalaxyUnits(context.Background(), "ogame_planets", 42, 99); err == nil || !strings.Contains(err.Error(), "galaxy current planet units not found") {
		t.Fatalf("expected missing units error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("units rows failed"), []any{1, 2, 3})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, _, _, err := repository.loadGalaxyUnits(context.Background(), "ogame_planets", 42, 99); err == nil || !strings.Contains(err.Error(), "units rows failed") {
		t.Fatalf("expected units rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 2, 3})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, _, _, err := repository.loadGalaxyUnits(context.Background(), "ogame_planets", 42, 99); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected units scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("bounds rows failed"), []any{9, 499})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadGalaxyBounds(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "bounds rows failed") {
		t.Fatalf("expected bounds rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{err: errors.New("bounds query failed")}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadGalaxyBounds(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "bounds query failed") {
		t.Fatalf("expected bounds query error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if bounds, err := repository.loadGalaxyBounds(context.Background(), "ogame_uni"); err != nil || bounds.Galaxies != 9 || bounds.Systems != 499 {
		t.Fatalf("empty bounds should default, got %+v err=%v", bounds, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0, 0})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if bounds, err := repository.loadGalaxyBounds(context.Background(), "ogame_uni"); err != nil || bounds.Galaxies != 9 || bounds.Systems != 499 {
		t.Fatalf("nonpositive bounds should default, got %+v err=%v", bounds, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 499})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadGalaxyBounds(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected bounds scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("active rows failed"), []any{1})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadActiveFleetCount(context.Background(), "ogame_queue", "ogame_fleet", 42); err == nil || !strings.Contains(err.Error(), "active rows failed") {
		t.Fatalf("expected active rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if count, err := repository.loadActiveFleetCount(context.Background(), "ogame_queue", "ogame_fleet", 42); err != nil || count != 0 {
		t.Fatalf("empty active fleet count should default to zero, got %d err=%v", count, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadActiveFleetCount(context.Background(), "ogame_queue", "ogame_fleet", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected active fleet scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadGalaxyObjects(context.Background(), "ogame_planets", "ogame_users", "ogame_ally", domaingame.Coordinates{Galaxy: 1, System: 2}); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
		t.Fatalf("expected object scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("object rows failed"))}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, err := repository.loadGalaxyObjects(context.Background(), "ogame_planets", "ogame_users", "ogame_ally", domaingame.Coordinates{Galaxy: 1, System: 2}); err == nil || !strings.Contains(err.Error(), "object rows failed") {
		t.Fatalf("expected object rows error, got %v", err)
	}

	coordinates := clampCoordinatesForRepository(domaingame.Coordinates{Galaxy: -1, System: 999}, domaingame.GalaxyBounds{Galaxies: 9, Systems: 499})
	if coordinates.Galaxy != 1 || coordinates.System != 499 {
		t.Fatalf("unexpected repository coordinate clamp: %+v", coordinates)
	}
	coordinates = clampCoordinatesForRepository(domaingame.Coordinates{Galaxy: 99, System: -1}, domaingame.GalaxyBounds{Galaxies: 9, Systems: 499})
	if coordinates.Galaxy != 9 || coordinates.System != 1 {
		t.Fatalf("unexpected repository inverse coordinate clamp: %+v", coordinates)
	}
}

func TestGalaxyRepositoryMissileLoadersAndMutatorsHandleEdges(t *testing.T) {
	now := time.Unix(10_000, 0)
	repository := NewGalaxyRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadGalaxyMissileOrigin(context.Background(), "ogame_planets", "ogame_users", 42, 99); err == nil {
		t.Fatal("expected origin query error")
	}

	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, found, err := repository.loadGalaxyMissileOrigin(context.Background(), "ogame_planets", "ogame_users", 42, 99); err != nil || found {
		t.Fatalf("empty origin should return not found without error, found=%v err=%v", found, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadGalaxyMissileOrigin(context.Background(), "ogame_planets", "ogame_users", 42, 99); err == nil {
		t.Fatal("expected origin scan error")
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("origin rows failed"), galaxyMissileOriginRow(99, 42, 1, 2, 3, 5, int64(10000), 0, 0, 0, now.Unix(), 2))}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadGalaxyMissileOrigin(context.Background(), "ogame_planets", "ogame_users", 42, 99); err == nil || !strings.Contains(err.Error(), "origin rows failed") {
		t.Fatalf("expected origin rows error, got %v", err)
	}

	repository = NewGalaxyRepositoryWithQueryer(&fakeQueryer{}, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadGalaxyMissileTarget(context.Background(), "ogame_planets", "ogame_users", 77); err == nil {
		t.Fatal("expected target query error")
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, found, err := repository.loadGalaxyMissileTarget(context.Background(), "ogame_planets", "ogame_users", 77); err != nil || found {
		t.Fatalf("empty target should return not found without error, found=%v err=%v", found, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewGalaxyRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })
	if _, _, err := repository.loadGalaxyMissileTarget(context.Background(), "ogame_planets", "ogame_users", 77); err == nil {
		t.Fatal("expected target scan error")
	}

	runner := &fakeGalaxyRunner{execErrs: []error{errors.New("reserve failed")}}
	repository = NewGalaxyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.reserveGalaxyMissiles(context.Background(), "ogame_planets", 42, 99, 1, int(now.Unix())); err == nil || !strings.Contains(err.Error(), "reserve failed") {
		t.Fatalf("expected reserve exec error, got %v", err)
	}

	runner = &fakeGalaxyRunner{execResults: []sql.Result{galaxySQLResultWithRowsErr{err: errors.New("rows affected failed")}}}
	repository = NewGalaxyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.reserveGalaxyMissiles(context.Background(), "ogame_planets", 42, 99, 1, int(now.Unix())); err == nil || !strings.Contains(err.Error(), "rows affected failed") {
		t.Fatalf("expected rows affected error, got %v", err)
	}

	runner = &fakeGalaxyRunner{execErrs: []error{errors.New("fleet insert failed")}}
	repository = NewGalaxyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.insertGalaxyMissileFleet(context.Background(), "ogame_fleet", galaxyMissilePlanet{ID: 99, OwnerID: 42}, galaxyMissilePlanet{ID: 77}, 1, 0, 30); err == nil || !strings.Contains(err.Error(), "fleet insert failed") {
		t.Fatalf("expected fleet insert error, got %v", err)
	}

	runner = &fakeGalaxyRunner{execResults: []sql.Result{galaxySQLResult{id: 0, rows: 1}}}
	repository = NewGalaxyRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })
	if _, err := repository.insertGalaxyMissileFleet(context.Background(), "ogame_fleet", galaxyMissilePlanet{ID: 99, OwnerID: 42}, galaxyMissilePlanet{ID: 77}, 1, 0, 30); err == nil || !strings.Contains(err.Error(), "fleet id unavailable") {
		t.Fatalf("expected fleet id error, got %v", err)
	}
}

func galaxyViewerPrefixResults(now time.Time) []fakeQueryResult {
	return append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{int64(10000), 0, domaingame.GalaxyActionSpy | domaingame.GalaxyActionMessage | domaingame.GalaxyActionBuddy | domaingame.GalaxyActionMissile, 4, now.Add(time.Hour).Unix()})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{4, 3, 2})},
	)
}

func galaxyPremiumPrefixResults(now time.Time) []fakeQueryResult {
	return append(galaxyViewerPrefixResults(now),
		fakeQueryResult{rows: fakeRowsFromValues(allResearchLevelRow(map[int]int{domaingame.ResearchComputer: 3}))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix()})},
	)
}

func galaxyReadPrefixResults(now time.Time) []fakeQueryResult {
	return append(galaxyPremiumPrefixResults(now),
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{9, 499})},
	)
}

func galaxyObjectRow(id int, name string, planetType int, position int, lastActivity int64, metal float64, crystal float64, ownerID int, ownerName string, ownerScore int64, ownerRank int, allyID int, lastClick int64, vacation int, banned int, rowAllyID int, tag string) []any {
	return []any{
		id,
		name,
		planetType,
		1,
		2,
		position,
		12800,
		19,
		lastActivity,
		metal,
		crystal,
		ownerID,
		ownerName,
		ownerScore,
		ownerRank,
		allyID,
		lastClick,
		vacation,
		banned,
		0,
		rowAllyID,
		tag,
	}
}

func galaxyMissileOriginRow(id int, ownerID int, galaxy int, system int, position int, missiles int, score int64, admin int, vacation int, banned int, lastClick int64, impulseDrive int) []any {
	return []any{id, ownerID, domaingame.PlanetTypePlanet, galaxy, system, position, missiles, score, admin, vacation, banned, lastClick, impulseDrive}
}

func galaxyMissileTargetRow(id int, ownerID int, galaxy int, system int, position int, score int64, admin int, vacation int, banned int, lastClick int64) []any {
	return []any{id, ownerID, domaingame.PlanetTypePlanet, galaxy, system, position, score, admin, vacation, banned, lastClick}
}

type fakeGalaxyRunner struct {
	fakeQueryer
	execCalls   []fakeGalaxyExecCall
	execErrs    []error
	execResults []sql.Result
}

type fakeGalaxyExecCall struct {
	sql  string
	args []any
}

func (f *fakeGalaxyRunner) ExecContext(_ context.Context, sqlText string, args ...any) (sql.Result, error) {
	f.execCalls = append(f.execCalls, fakeGalaxyExecCall{sql: sqlText, args: args})
	result := sql.Result(galaxySQLResult{rows: 1})
	if len(f.execResults) > 0 {
		result = f.execResults[0]
		f.execResults = f.execResults[1:]
	}
	var err error
	if len(f.execErrs) > 0 {
		err = f.execErrs[0]
		f.execErrs = f.execErrs[1:]
	}
	return result, err
}

type galaxySQLResult struct {
	id   int64
	rows int64
}

func (r galaxySQLResult) LastInsertId() (int64, error) {
	return r.id, nil
}

func (r galaxySQLResult) RowsAffected() (int64, error) {
	return r.rows, nil
}

type galaxySQLResultWithRowsErr struct {
	err error
}

func (r galaxySQLResultWithRowsErr) LastInsertId() (int64, error) {
	return 0, nil
}

func (r galaxySQLResultWithRowsErr) RowsAffected() (int64, error) {
	return 0, r.err
}
