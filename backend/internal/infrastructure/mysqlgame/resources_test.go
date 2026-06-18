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

func TestResourcesRepositoryReadsLegacyResourceProduction(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 4, 163, 10000.0, 10000.0, 10000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues([]any{10, 0, 0, 10, 0, 3, 1.0, 1.0, 1.0, 1.0, 0.5, 1.0})},
		{rows: fakeRowsFromValues([]any{3, now.Add(-time.Hour).Unix(), now.Add(time.Hour).Unix()})},
		{rows: fakeRowsFromValues([]any{2.0})},
	}}
	repository := NewResourcesRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	resources, err := repository.GetResources(context.Background(), appgame.ResourcesQuery{PlayerID: 42})
	if err != nil {
		t.Fatal(err)
	}

	if resources.Commander != "legor" || resources.CurrentPlanet.ID != 99 {
		t.Fatalf("unexpected resources summary: %+v", resources)
	}
	if resources.Factor != 1 || resources.Natural.Metal != 40 || resources.Totals.Hour.Metal != 1596 {
		t.Fatalf("unexpected production values: %+v", resources)
	}
	satellite := resourceRowByID(t, resources, domaingame.FleetSolarSatellite)
	if satellite.Level != 3 || satellite.Values.Energy != 112.2 {
		t.Fatalf("expected engineer-boosted satellite output, got %+v", satellite)
	}
	if !strings.Contains(queryer.calls[4].sql, "prod1") || !strings.Contains(queryer.calls[4].sql, "prod212") {
		t.Fatalf("expected legacy production columns, got %q", queryer.calls[4].sql)
	}
	if !strings.Contains(queryer.calls[5].sql, "geo_until") || !strings.Contains(queryer.calls[5].sql, "eng_until") {
		t.Fatalf("expected premium columns, got %q", queryer.calls[5].sql)
	}
}

func TestNewResourcesRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewResourcesRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}
	if repository.now == nil {
		t.Fatal("expected default clock")
	}

	withDefaultClock := NewResourcesRepositoryWithQueryer(nil, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected nil clock to default")
	}
}

func TestResourcesRepositoryUpdatesLegacyProductionSettings(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeResourceRunner{fakeQueryer: fakeQueryer{results: append(
		append(resourceOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0})}),
		resourceReadResults(now)...,
	)}}
	repository := NewResourcesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	resources, err := repository.UpdateProduction(context.Background(), appgame.ResourcesUpdateQuery{
		PlayerID: 42,
		Production: domaingame.ProductionFactors{
			domaingame.BuildingMetalMine:      0,
			domaingame.BuildingDeuteriumSynth: 0.4,
			domaingame.BuildingSolarPlant:     1,
			9999:                              0.7,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if resources.Commander != "legor" || resources.CurrentPlanet.ID != 99 {
		t.Fatalf("unexpected resources after update: %+v", resources)
	}
	if !strings.Contains(runner.execSQL, "prod1 = ?") || !strings.Contains(runner.execSQL, "prod3 = ?") || !strings.Contains(runner.execSQL, "prod4 = ?") {
		t.Fatalf("expected legacy production columns, got %q", runner.execSQL)
	}
	if strings.Contains(runner.execSQL, "prod9999") {
		t.Fatalf("unexpected unsupported production column in %q", runner.execSQL)
	}
	if len(runner.execArgs) != 6 || runner.execArgs[0] != 0.0 || runner.execArgs[1] != 0.4 || runner.execArgs[2] != 1.0 || runner.execArgs[3] != 99 || runner.execArgs[4] != 42 {
		t.Fatalf("unexpected update args: %+v", runner.execArgs)
	}
}

func TestResourcesRepositorySkipsProductionUpdateDuringVacation(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeResourceRunner{fakeQueryer: fakeQueryer{results: append(
		append(resourceOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{1})}),
		resourceReadResults(now)...,
	)}}
	repository := NewResourcesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	if _, err := repository.UpdateProduction(context.Background(), appgame.ResourcesUpdateQuery{
		PlayerID:   42,
		Production: domaingame.ProductionFactors{domaingame.BuildingMetalMine: 0.5},
	}); err != nil {
		t.Fatal(err)
	}
	if runner.execSQL != "" {
		t.Fatalf("expected vacation mode to skip production update, got %q", runner.execSQL)
	}
}

func TestResourcesRepositorySkipsEmptyProductionUpdate(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeResourceRunner{fakeQueryer: fakeQueryer{results: append(
		append(resourceOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0})}),
		resourceReadResults(now)...,
	)}}
	repository := NewResourcesRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return now })

	if _, err := repository.UpdateProduction(context.Background(), appgame.ResourcesUpdateQuery{PlayerID: 42}); err != nil {
		t.Fatal(err)
	}
	if runner.execSQL != "" {
		t.Fatalf("expected empty production update to skip SQL, got %q", runner.execSQL)
	}
}

func TestResourcesRepositoryUpdateReturnsErrors(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		runner *fakeResourceRunner
		want   string
	}{
		{
			name:   "unsafe prefix",
			prefix: "bad-prefix_",
			runner: &fakeResourceRunner{},
			want:   "invalid database table prefix",
		},
		{
			name:   "missing execer",
			prefix: "ogame_",
			runner: nil,
			want:   "resource production updater unavailable",
		},
		{
			name:   "overview",
			prefix: "ogame_",
			runner: &fakeResourceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}},
			want:   "overview failed",
		},
		{
			name:   "vacation query",
			prefix: "ogame_",
			runner: &fakeResourceRunner{fakeQueryer: fakeQueryer{results: append(resourceOverviewResults(), fakeQueryResult{err: errors.New("vacation failed")})}},
			want:   "vacation failed",
		},
		{
			name:   "missing vacation",
			prefix: "ogame_",
			runner: &fakeResourceRunner{fakeQueryer: fakeQueryer{results: append(resourceOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues()})}},
			want:   "resource vacation state not found",
		},
		{
			name:   "vacation rows",
			prefix: "ogame_",
			runner: &fakeResourceRunner{fakeQueryer: fakeQueryer{results: append(resourceOverviewResults(), fakeQueryResult{rows: fakeRowsError(errors.New("vacation rows failed"))})}},
			want:   "vacation rows failed",
		},
		{
			name:   "vacation scan",
			prefix: "ogame_",
			runner: &fakeResourceRunner{fakeQueryer: fakeQueryer{results: append(resourceOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})})}},
			want:   "expected int",
		},
		{
			name:   "vacation post rows",
			prefix: "ogame_",
			runner: &fakeResourceRunner{fakeQueryer: fakeQueryer{results: append(resourceOverviewResults(), fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("vacation post rows failed"), []any{0})})}},
			want:   "vacation post rows failed",
		},
		{
			name:   "exec",
			prefix: "ogame_",
			runner: &fakeResourceRunner{
				fakeQueryer: fakeQueryer{results: append(resourceOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0})})},
				execErr:     errors.New("update failed"),
			},
			want: "update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var queryer Queryer
			var execer Execer
			if tt.runner != nil {
				queryer = tt.runner
				execer = tt.runner
			} else {
				queryer = &fakeQueryer{}
			}
			repository := NewResourcesRepositoryWithRunner(queryer, execer, tt.prefix, func() time.Time { return time.Unix(1, 0) })
			_, err := repository.UpdateProduction(context.Background(), appgame.ResourcesUpdateQuery{
				PlayerID:   42,
				Production: domaingame.ProductionFactors{domaingame.BuildingMetalMine: 0.5},
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestResourcesRepositoryReturnsErrors(t *testing.T) {
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
			name:   "overview",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{err: errors.New("overview user failed")},
			}},
			want: "overview user failed",
		},
		{
			name:   "production query",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{{err: errors.New("production query failed")}},
			),
			want: "production query failed",
		},
		{
			name:   "missing production",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{{rows: fakeRowsFromValues()}},
			),
			want: "resource production settings not found",
		},
		{
			name:   "production rows",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{{rows: fakeRowsError(errors.New("production rows failed"))}},
			),
			want: "production rows failed",
		},
		{
			name:   "production scan",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 0, 0, 0, 0, 0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0})}},
			),
			want: "expected int",
		},
		{
			name:   "production post rows",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("production post rows failed"), resourceProductionSettingsRow())}},
			),
			want: "production post rows failed",
		},
		{
			name:   "resource user query",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{
					{rows: fakeRowsFromValues(resourceProductionSettingsRow())},
					{err: errors.New("resource user failed")},
				},
			),
			want: "resource user failed",
		},
		{
			name:   "missing resource user",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{
					{rows: fakeRowsFromValues(resourceProductionSettingsRow())},
					{rows: fakeRowsFromValues()},
				},
			),
			want: "resource user not found",
		},
		{
			name:   "resource user rows",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{
					{rows: fakeRowsFromValues(resourceProductionSettingsRow())},
					{rows: fakeRowsError(errors.New("resource user rows failed"))},
				},
			),
			want: "resource user rows failed",
		},
		{
			name:   "resource user scan",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{
					{rows: fakeRowsFromValues(resourceProductionSettingsRow())},
					{rows: fakeRowsFromValues([]any{"bad", 0, 0})},
				},
			),
			want: "expected int",
		},
		{
			name:   "resource user post rows",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{
					{rows: fakeRowsFromValues(resourceProductionSettingsRow())},
					{rows: fakeRowsFromValuesWithErr(errors.New("resource user post rows failed"), []any{0, 0, 0})},
				},
			),
			want: "resource user post rows failed",
		},
		{
			name:   "speed query",
			prefix: "ogame_",
			queryer: resourceQueryerWithTail(
				[]fakeQueryResult{
					{rows: fakeRowsFromValues(resourceProductionSettingsRow())},
					{rows: fakeRowsFromValues([]any{0, 0, 0})},
					{err: errors.New("speed query failed")},
				},
			),
			want: "speed query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewResourcesRepositoryWithQueryer(tt.queryer, tt.prefix, func() time.Time { return time.Unix(1, 0) })
			_, err := repository.GetResources(context.Background(), appgame.ResourcesQuery{PlayerID: 42})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func resourceQueryerWithTail(tail []fakeQueryResult) *fakeQueryer {
	results := resourceOverviewResults()
	results = append(results, tail...)
	return &fakeQueryer{results: results}
}

func resourceOverviewResults() []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}
}

func resourceReadResults(now time.Time) []fakeQueryResult {
	return append(resourceOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{10, 0, 0, 10, 0, 3, 1.0, 1.0, 1.0, 1.0, 0.5, 1.0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{3, now.Add(-time.Hour).Unix(), now.Add(time.Hour).Unix()})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{2.0})},
	)
}

func resourceProductionSettingsRow() []any {
	return []any{0, 0, 0, 0, 0, 0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
}

func resourceRowByID(t *testing.T, resources domaingame.ResourceProduction, id int) domaingame.ResourceProductionRow {
	t.Helper()
	for _, row := range resources.Rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("resource row %d not found in %+v", id, resources.Rows)
	return domaingame.ResourceProductionRow{}
}

type fakeResourceRunner struct {
	fakeQueryer
	execSQL  string
	execArgs []any
	execErr  error
}

func (f *fakeResourceRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execSQL = query
	f.execArgs = args
	return fakeSQLResult(1), f.execErr
}

type fakeSQLResult int64

func (r fakeSQLResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r fakeSQLResult) RowsAffected() (int64, error) {
	return int64(r), nil
}
