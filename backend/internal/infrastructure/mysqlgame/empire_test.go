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

func TestEmpireRepositoryReadsLegacyEmpire(t *testing.T) {
	now := time.Unix(1000, 0)
	queryer := &fakeQueryer{results: empireReadResults(now, now.Add(time.Hour).Unix())}
	repository := NewEmpireRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	empire, issue, err := repository.GetEmpire(context.Background(), appgame.EmpireQuery{
		PlayerID:   42,
		PlanetID:   99,
		PlanetType: domaingame.EmpirePlanetTypeMoons,
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue != nil {
		t.Fatalf("unexpected issue: %+v", issue)
	}
	if !empire.CommanderActive || empire.PlanetType != domaingame.EmpirePlanetTypeMoons || !empire.MoonEnabled || !empire.HasMoons {
		t.Fatalf("unexpected empire metadata: %+v", empire)
	}
	if len(empire.Planets) != 1 || empire.Planets[0].ID != 100 || empire.Planets[0].Type != domaingame.PlanetTypeMoon {
		t.Fatalf("unexpected empire planets: %+v", empire.Planets)
	}
	if row := findEmpireLevelRowInfra(t, empire.Buildings, domaingame.BuildingLunarBase); row.Total != 3 {
		t.Fatalf("expected lunar base row, got %+v", row)
	}
	if row := findEmpireLevelRowInfra(t, empire.Research, domaingame.ResearchComputer); row.Total != 4 {
		t.Fatalf("expected computer research row, got %+v", row)
	}
	if row := findEmpireCountRowInfra(t, empire.Fleet, domaingame.FleetSmallCargo); row.Total != 8 {
		t.Fatalf("expected small cargo row, got %+v", row)
	}
	if row := findEmpireCountRowInfra(t, empire.Defense, domaingame.DefenseRocketLauncher); row.Total != 6 {
		t.Fatalf("expected rocket launcher row, got %+v", row)
	}
	if !strings.Contains(queryer.calls[9].sql, "type = ?") || queryer.calls[9].args[1] != domaingame.PlanetTypeMoon {
		t.Fatalf("expected moon planet query, got %+v", queryer.calls[9])
	}
}

func TestEmpireRepositoryReturnsCommanderIssue(t *testing.T) {
	now := time.Unix(1000, 0)
	queryer := &fakeQueryer{results: empireReadResults(now, now.Add(-time.Hour).Unix())}
	repository := NewEmpireRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	empire, issue, err := repository.GetEmpire(context.Background(), appgame.EmpireQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if empire.CommanderActive || issue == nil || issue.Code != domaingame.EmpireIssueCommanderRequired {
		t.Fatalf("expected commander issue, got empire=%+v issue=%+v", empire, issue)
	}
	if empire.PlanetType != domaingame.EmpirePlanetTypePlanets {
		t.Fatalf("expected default planet type, got %d", empire.PlanetType)
	}
}

func TestEmpireRepositoryFlushesDueQueuesWhenWritable(t *testing.T) {
	now := time.Unix(1000, 0)
	runner := &fakeOptionsRunner{fakeQueryer: fakeQueryer{results: append([]fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1.0, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{1.0, 1000, 0})},
		{rows: fakeRowsFromValues()},
	}, empireReadResults(now, now.Add(time.Hour).Unix())...)}}
	repository := NewEmpireRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	empire, issue, err := repository.GetEmpire(context.Background(), appgame.EmpireQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}
	if issue != nil || !empire.CommanderActive {
		t.Fatalf("unexpected empire result: empire=%+v issue=%+v", empire, issue)
	}
	if len(runner.calls) < 14 || !strings.Contains(runner.calls[0].sql, "SELECT speed, freeze") ||
		!strings.Contains(runner.calls[2].sql, "SELECT speed, max_werf, freeze") {
		t.Fatalf("expected queue flush queries before empire read, got %+v", runner.calls[:4])
	}
}

func TestNewEmpireRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewEmpireRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}
	repository = NewEmpireRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_", nil)
	if repository.now == nil {
		t.Fatal("expected default clock")
	}
	repository = NewEmpireRepositoryWithQueryer(&fakeOptionsRunner{}, "ogame_", nil)
	if repository.execer == nil {
		t.Fatal("expected queryer execer to be reused")
	}
	if repository.currentTime().IsZero() {
		t.Fatal("expected default current time")
	}
	if _, _, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_", time.Now).GetEmpire(context.Background(), appgame.EmpireQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected bad prefix error, got %v", err)
	}
}

func TestEmpireRepositoryGetEmpireErrors(t *testing.T) {
	now := time.Unix(1000, 0)
	tests := []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{
			name:    "overview",
			results: []fakeQueryResult{{err: errors.New("overview failed")}},
			want:    "overview failed",
		},
		{
			name:    "user",
			results: append(empireOverviewResults(), fakeQueryResult{err: errors.New("user failed")}),
			want:    "user failed",
		},
		{
			name:    "moon config",
			results: append(empireOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), int64(0), int64(0), 0, 0, 0})}, fakeQueryResult{err: errors.New("moon config failed")}),
			want:    "moon config failed",
		},
		{
			name: "moon count",
			results: append(empireOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), int64(0), int64(0), 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
				fakeQueryResult{err: errors.New("moon count failed")},
			),
			want: "moon count failed",
		},
		{
			name: "research",
			results: append(empireOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), int64(0), int64(0), 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
				fakeQueryResult{err: errors.New("research failed")},
			),
			want: "research failed",
		},
		{
			name: "speed",
			results: append(empireOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), int64(0), int64(0), 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
				fakeQueryResult{rows: fakeRowsFromValues(empireResearchRow())},
				fakeQueryResult{err: errors.New("speed failed")},
			),
			want: "speed failed",
		},
		{
			name: "planets",
			results: append(empireOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{now.Add(time.Hour).Unix(), int64(0), int64(0), 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
				fakeQueryResult{rows: fakeRowsFromValues(empireResearchRow())},
				fakeQueryResult{rows: fakeRowsFromValues([]any{1.0})},
				fakeQueryResult{err: errors.New("planets failed")},
			),
			want: "planets failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: tt.results}, "ogame_", func() time.Time { return now }).GetEmpire(context.Background(), appgame.EmpireQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestEmpireRepositoryHelperErrors(t *testing.T) {
	now := time.Unix(1000, 0)
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "missing user",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", func() time.Time { return now }).loadEmpireUser(context.Background(), "`ogame_users`", 42)
				return err
			},
			want: "empire user not found",
		},
		{
			name: "user rows",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("user rows failed"), []any{int64(0), int64(0), int64(0), 0, 0, 0})}}}, "ogame_", func() time.Time { return now }).loadEmpireUser(context.Background(), "`ogame_users`", 42)
				return err
			},
			want: "user rows failed",
		},
		{
			name: "user scan",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{int64(0)})}}}, "ogame_", func() time.Time { return now }).loadEmpireUser(context.Background(), "`ogame_users`", 42)
				return err
			},
			want: "scan",
		},
		{
			name: "moon query",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("moon query failed")}}}, "ogame_", func() time.Time { return now }).loadMoonEnabled(context.Background())
				return err
			},
			want: "moon query failed",
		},
		{
			name: "moon scan",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{})}}}, "ogame_", func() time.Time { return now }).loadMoonEnabled(context.Background())
				return err
			},
			want: "scan",
		},
		{
			name: "moon rows",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("moon rows failed"), []any{1})}}}, "ogame_", func() time.Time { return now }).loadMoonEnabled(context.Background())
				return err
			},
			want: "moon rows failed",
		},
		{
			name: "has moon rows",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("has moon rows failed"), []any{1})}}}, "ogame_", func() time.Time { return now }).loadHasMoons(context.Background(), "`ogame_planets`", 42)
				return err
			},
			want: "has moon rows failed",
		},
		{
			name: "has moon query",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("has moon query failed")}}}, "ogame_", func() time.Time { return now }).loadHasMoons(context.Background(), "`ogame_planets`", 42)
				return err
			},
			want: "has moon query failed",
		},
		{
			name: "has moon scan",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{})}}}, "ogame_", func() time.Time { return now }).loadHasMoons(context.Background(), "`ogame_planets`", 42)
				return err
			},
			want: "scan",
		},
		{
			name: "planet rows",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("planet rows failed"), empirePlanetRow())}}}, "ogame_", func() time.Time { return now }).loadEmpirePlanets(context.Background(), "`ogame_planets`", 42, domaingame.EmpirePlanetTypePlanets, empireUser{}, 1)
				return err
			},
			want: "planet rows failed",
		},
		{
			name: "planet scan",
			run: func() error {
				_, err := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}, "ogame_", func() time.Time { return now }).loadEmpirePlanets(context.Background(), "`ogame_planets`", 42, domaingame.EmpirePlanetTypePlanets, empireUser{}, 1)
				return err
			},
			want: "scan",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestEmpireRepositoryHelpersDefaultEmptyRows(t *testing.T) {
	repository := NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", nil)
	enabled, err := repository.loadMoonEnabled(context.Background())
	if err != nil || enabled {
		t.Fatalf("expected empty moon config to be disabled, got enabled=%v err=%v", enabled, err)
	}
	repository = NewEmpireRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_", nil)
	hasMoons, err := repository.loadHasMoons(context.Background(), "`ogame_planets`", 42)
	if err != nil || hasMoons {
		t.Fatalf("expected empty moon count to be false, got hasMoons=%v err=%v", hasMoons, err)
	}
}

func empireOverviewResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 1000.0, 2000.0, 3000.0, 10000, 10000, 10000})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
	}
}

func empireReadResults(now time.Time, commanderUntil int64) []fakeQueryResult {
	return append(empireOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{commanderUntil, now.Add(time.Hour).Unix(), now.Add(time.Hour).Unix(), 7, 1, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
		fakeQueryResult{rows: fakeRowsFromValues(empireResearchRow())},
		fakeQueryResult{rows: fakeRowsFromValues([]any{128.0})},
		fakeQueryResult{rows: fakeRowsFromValues(empirePlanetRow())},
	)
}

func empireResearchRow() []any {
	row := make([]any, 0, len(domaingame.ResearchIDs()))
	for _, id := range domaingame.ResearchIDs() {
		value := 0
		if id == domaingame.ResearchComputer {
			value = 4
		}
		row = append(row, value)
	}
	return row
}

func empirePlanetRow() []any {
	row := []any{
		100, "Luna", domaingame.PlanetTypeMoon, 1, 2, 3, 5, 120, -20,
		10.0, 20.0, 30.0,
		1.0, 1.0, 1.0, 1.0, 1.0, 1.0,
	}
	for _, id := range empirePlanetLevelIDs() {
		value := 0
		switch id {
		case domaingame.BuildingLunarBase:
			value = 3
		case domaingame.FleetSmallCargo:
			value = 8
		case domaingame.DefenseRocketLauncher:
			value = 6
		}
		row = append(row, value)
	}
	return row
}

func findEmpireLevelRowInfra(t *testing.T, rows []domaingame.EmpireLevelRow, id int) domaingame.EmpireLevelRow {
	t.Helper()
	for _, row := range rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing level row %d in %+v", id, rows)
	return domaingame.EmpireLevelRow{}
}

func findEmpireCountRowInfra(t *testing.T, rows []domaingame.EmpireCountRow, id int) domaingame.EmpireCountRow {
	t.Helper()
	for _, row := range rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing count row %d in %+v", id, rows)
	return domaingame.EmpireCountRow{}
}
