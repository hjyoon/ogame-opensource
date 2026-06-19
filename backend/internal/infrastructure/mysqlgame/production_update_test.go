package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestUpdatePlanetResourcesAccruesLegacyProduction(t *testing.T) {
	runner := &fakeResourceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{
			lastPeek: 1000,
			metal:    1000,
			crystal:  1000,
			deut:     1000,
			levels: map[int]int{
				domaingame.BuildingMetalMine:  1,
				domaingame.BuildingSolarPlant: 1,
			},
		}))},
		{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{1.0})},
	}}}

	err := updatePlanetResources(context.Background(), runner, runner, "ogame_", "`ogame_users`", "`ogame_planets`", 42, 99, 4600, time.Unix(4600, 0))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(runner.execSQL, "lastpeek = ? WHERE planet_id = ? AND owner_id = ? AND lastpeek = ?") {
		t.Fatalf("expected optimistic lastpeek guard, got %q", runner.execSQL)
	}
	if len(runner.execArgs) != 7 {
		t.Fatalf("unexpected exec args: %+v", runner.execArgs)
	}
	if runner.execArgs[0] != 1053.0 || runner.execArgs[1] != 1010.0 || runner.execArgs[2] != 1000.0 {
		t.Fatalf("unexpected accrued resources: %+v", runner.execArgs)
	}
	if runner.execArgs[3] != 4600 || runner.execArgs[6] != 1000 {
		t.Fatalf("expected new and old lastpeek args, got %+v", runner.execArgs)
	}
}

func TestUpdatePlanetResourcesCapsStorageAndKeepsFullResources(t *testing.T) {
	runner := &fakeResourceRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{
			lastPeek: 1000,
			metal:    99_990,
			crystal:  100_000,
			deut:     0,
			levels: map[int]int{
				domaingame.BuildingMetalMine:    10,
				domaingame.BuildingCrystalMine:  10,
				domaingame.BuildingSolarPlant:   20,
				domaingame.BuildingMetalStorage: 0,
			},
		}))},
		{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{1.0})},
	}}}

	err := updatePlanetResources(context.Background(), runner, runner, "ogame_", "`ogame_users`", "`ogame_planets`", 42, 99, 4600, time.Unix(4600, 0))
	if err != nil {
		t.Fatal(err)
	}

	if runner.execArgs[0] != 100_000.0 {
		t.Fatalf("expected metal to cap at storage, got %+v", runner.execArgs)
	}
	if runner.execArgs[1] != 100_000.0 {
		t.Fatalf("expected already-full crystal to stay unchanged, got %+v", runner.execArgs)
	}
}

func TestUpdatePlanetResourcesSkipsLegacyNoopCases(t *testing.T) {
	tests := []struct {
		name      string
		execer    Execer
		planetID  int
		until     int
		results   []fakeQueryResult
		wantCalls int
	}{
		{
			name:     "missing execer",
			execer:   nil,
			planetID: 99,
			until:    4600,
		},
		{
			name:      "missing planet",
			execer:    &fakeResourceRunner{},
			planetID:  99,
			until:     4600,
			results:   []fakeQueryResult{{rows: fakeRowsFromValues()}},
			wantCalls: 1,
		},
		{
			name:     "moon",
			execer:   &fakeResourceRunner{},
			planetID: 99,
			until:    4600,
			results: []fakeQueryResult{{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{
				moon:     true,
				lastPeek: 1000,
			}))}},
			wantCalls: 1,
		},
		{
			name:     "missing user",
			execer:   &fakeResourceRunner{},
			planetID: 99,
			until:    4600,
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000}))},
				{rows: fakeRowsFromValues()},
			},
			wantCalls: 2,
		},
		{
			name:     "space user",
			execer:   &fakeResourceRunner{},
			planetID: 99,
			until:    4600,
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{ownerID: userSpace, lastPeek: 1000}))},
				{rows: fakeRowsFromValues(resourceUpdateUserRow(userSpace, 0, 0, 0))},
			},
			wantCalls: 2,
		},
		{
			name:     "not due",
			execer:   &fakeResourceRunner{},
			planetID: 99,
			until:    1000,
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000}))},
				{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
			},
			wantCalls: 2,
		},
		{
			name:     "invalid input",
			execer:   &fakeResourceRunner{},
			planetID: 0,
			until:    4600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryer := &fakeResourceRunner{fakeQueryer: fakeQueryer{results: tt.results}}
			execer := tt.execer
			if execer == nil && tt.name != "missing execer" {
				execer = queryer
			}
			err := updatePlanetResources(context.Background(), queryer, execer, "ogame_", "`ogame_users`", "`ogame_planets`", 42, tt.planetID, tt.until, time.Unix(4600, 0))
			if err != nil {
				t.Fatal(err)
			}
			if len(queryer.calls) != tt.wantCalls {
				t.Fatalf("expected %d queries, got %+v", tt.wantCalls, queryer.calls)
			}
			if queryer.execSQL != "" {
				t.Fatalf("expected no resource write, got %q", queryer.execSQL)
			}
		})
	}
}

func TestUpdatePlanetResourcesReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
		execErr error
		want    string
	}{
		{
			name:    "planet query",
			results: []fakeQueryResult{{err: errors.New("planet failed")}},
			want:    "planet failed",
		},
		{
			name:    "planet rows",
			results: []fakeQueryResult{{rows: fakeRowsError(errors.New("planet rows failed"))}},
			want:    "planet rows failed",
		},
		{
			name:    "planet scan",
			results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}},
			want:    "unexpected scan destination count",
		},
		{
			name:    "planet post rows",
			results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("planet post rows failed"), resourceUpdatePlanetRow(resourceUpdatePlanetFixture{}))}},
			want:    "planet post rows failed",
		},
		{
			name: "user query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000}))},
				{err: errors.New("user failed")},
			},
			want: "user failed",
		},
		{
			name: "user rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000}))},
				{rows: fakeRowsError(errors.New("user rows failed"))},
			},
			want: "user rows failed",
		},
		{
			name: "user scan",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000}))},
				{rows: fakeRowsFromValues([]any{"bad", 0, 0, 0})},
			},
			want: "expected int",
		},
		{
			name: "user post rows",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000}))},
				{rows: fakeRowsFromValuesWithErr(errors.New("user post rows failed"), resourceUpdateUserRow(42, 0, 0, 0))},
			},
			want: "user post rows failed",
		},
		{
			name: "speed query",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000}))},
				{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
				{err: errors.New("speed failed")},
			},
			want: "speed failed",
		},
		{
			name: "exec",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{lastPeek: 1000}))},
				{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
				{rows: fakeRowsFromValues([]any{1.0})},
			},
			execErr: errors.New("exec failed"),
			want:    "exec failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeResourceRunner{fakeQueryer: fakeQueryer{results: tt.results}, execErr: tt.execErr}
			err := updatePlanetResources(context.Background(), runner, runner, "ogame_", "`ogame_users`", "`ogame_planets`", 42, 99, 4600, time.Unix(4600, 0))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestOverviewRepositoryUpdatesResourcesBeforeRead(t *testing.T) {
	now := time.Unix(4600, 0)
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{
			lastPeek: 1000,
			metal:    1000,
			crystal:  1000,
			deut:     1000,
			levels: map[int]int{
				domaingame.BuildingMetalMine:  1,
				domaingame.BuildingSolarPlant: 1,
			},
		}))},
		{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{1.0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 1053.0, 1010.0, 1000.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")
	repository.updateResources = true
	repository.now = func() time.Time { return now }

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
	if err != nil {
		t.Fatal(err)
	}

	if overview.CurrentPlanet.Resources.Metal != 1053 {
		t.Fatalf("expected overview to read updated resources, got %+v", overview.CurrentPlanet.Resources)
	}
	if len(runner.execs) != 1 || len(runner.execs[0].args) != 7 || runner.execs[0].args[3] != int(now.Unix()) {
		t.Fatalf("expected resource update before read, got execs=%+v", runner.execs)
	}
}

func TestOverviewRepositoryUpdatesFallbackPlanetResources(t *testing.T) {
	now := time.Unix(4600, 0)
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues(resourceUpdatePlanetRow(resourceUpdatePlanetFixture{planetID: 1, lastPeek: 1000}))},
		{rows: fakeRowsFromValues(resourceUpdateUserRow(42, 0, 0, 0))},
		{rows: fakeRowsFromValues([]any{1.0})},
		{rows: fakeRowsFromValues([]any{1, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3, 12800, 19, 1, 163, 20.0, 10.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{1, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")
	repository.updateResources = true
	repository.now = func() time.Time { return now }

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 404))
	if err != nil {
		t.Fatal(err)
	}

	if overview.CurrentPlanet.ID != 1 {
		t.Fatalf("expected fallback home planet, got %+v", overview.CurrentPlanet)
	}
	if len(runner.execs) != 2 || runner.execs[0].args[4] != 1 {
		t.Fatalf("expected home planet resource update before active update, got execs=%+v", runner.execs)
	}
}

func TestRepositoryUpdatePlanetResourceWrappersUseClocks(t *testing.T) {
	if err := (OverviewRepository{}).updatePlanetResources(context.Background(), "`ogame_users`", "`ogame_planets`", 42, 99, 4600); err != nil {
		t.Fatal(err)
	}
	if err := (BuildingsRepository{}).updatePlanetResources(context.Background(), "`ogame_users`", "`ogame_planets`", 42, 99, 4600); err != nil {
		t.Fatal(err)
	}
	if err := (BuildingsRepository{now: func() time.Time { return time.Unix(4600, 0) }}).updatePlanetResources(context.Background(), "`ogame_users`", "`ogame_planets`", 42, 99, 4600); err != nil {
		t.Fatal(err)
	}
	if err := (ResourcesRepository{}).updatePlanetResources(context.Background(), "`ogame_users`", "`ogame_planets`", 42, 99, 4600); err != nil {
		t.Fatal(err)
	}
	if err := (ResourcesRepository{now: func() time.Time { return time.Unix(4600, 0) }}).updatePlanetResources(context.Background(), "`ogame_users`", "`ogame_planets`", 42, 99, 4600); err != nil {
		t.Fatal(err)
	}
}

type resourceUpdatePlanetFixture struct {
	ownerID    int
	planetID   int
	moon       bool
	lastPeek   int
	metal      float64
	crystal    float64
	deut       float64
	levels     map[int]int
	production map[int]float64
}

func resourceUpdatePlanetRow(fixture resourceUpdatePlanetFixture) []any {
	ownerID := fixture.ownerID
	if ownerID == 0 {
		ownerID = 42
	}
	planetID := fixture.planetID
	if planetID == 0 {
		planetID = 99
	}
	planetType := domaingame.PlanetTypePlanet
	if fixture.moon {
		planetType = domaingame.PlanetTypeMoon
	}
	production := fixture.production
	if production == nil {
		production = map[int]float64{
			domaingame.BuildingMetalMine:      1,
			domaingame.BuildingCrystalMine:    1,
			domaingame.BuildingDeuteriumSynth: 1,
			domaingame.BuildingSolarPlant:     1,
			domaingame.BuildingFusionReactor:  1,
			domaingame.FleetSolarSatellite:    1,
		}
	}
	return []any{
		planetID,
		ownerID,
		planetType,
		19,
		fixture.lastPeek,
		fixture.metal,
		fixture.crystal,
		fixture.deut,
		fixture.levels[domaingame.BuildingMetalStorage],
		fixture.levels[domaingame.BuildingCrystalStorage],
		fixture.levels[domaingame.BuildingDeuteriumTank],
		fixture.levels[domaingame.BuildingMetalMine],
		fixture.levels[domaingame.BuildingCrystalMine],
		fixture.levels[domaingame.BuildingDeuteriumSynth],
		fixture.levels[domaingame.BuildingSolarPlant],
		fixture.levels[domaingame.BuildingFusionReactor],
		fixture.levels[domaingame.FleetSolarSatellite],
		production[domaingame.BuildingMetalMine],
		production[domaingame.BuildingCrystalMine],
		production[domaingame.BuildingDeuteriumSynth],
		production[domaingame.BuildingSolarPlant],
		production[domaingame.BuildingFusionReactor],
		production[domaingame.FleetSolarSatellite],
	}
}

func resourceUpdateUserRow(playerID int, energy int, geologistUntil int64, engineerUntil int64) []any {
	return []any{playerID, energy, geologistUntil, engineerUntil}
}
