package mysqlgame

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestFleetRepositoryBattleMoonCreation(t *testing.T) {
	ctx := context.Background()
	value := fleetMessageContextTestValue()
	debris := domaingame.Resources{Metal: 1_000_000, Crystal: 1_000_000}

	repository := NewFleetRepositoryWithRunner(&fakeFleetRunner{}, nil, "ogame_", nil)
	result, err := repository.maybeCreateBattleMoon(ctx, "`planets`", "`users`", 100, value, domaingame.Resources{Metal: 99_999})
	if err != nil || result != (battleMoonCreation{}) {
		t.Fatalf("sub-threshold debris must skip moon lookup: %+v err=%v", result, err)
	}

	for _, row := range [][]any{
		{domaingame.PlanetTypePlanet, 50, "en", 1},
		{domaingame.PlanetTypeMoon, 50, "en", 0},
		{domaingame.PlanetTypeDestroyedMoon, 50, "en", 0},
	} {
		runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(row)}}}}
		repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		repository.combatRandom = func(int) int { t.Fatal("suppressed moon creation must not roll"); return 0 }
		result, err = repository.maybeCreateBattleMoon(ctx, "`planets`", "`users`", 100, value, debris)
		if err != nil || result != (battleMoonCreation{}) || len(runner.execCalls) != 0 {
			t.Fatalf("existing/moon target must suppress chance: row=%+v result=%+v err=%v", row, result, err)
		}
	}

	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.PlanetTypePlanet, 50, "en", 0})}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = func(int) int { return 99 }
	result, err = repository.maybeCreateBattleMoon(ctx, "`planets`", "`users`", 100, value, debris)
	if err != nil || result.Chance != 20 || result.Created || len(runner.execCalls) != 0 {
		t.Fatalf("failed 20%% moon roll must retain report chance: %+v err=%v", result, err)
	}

	runner = &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.PlanetTypePlanet, 50, "de", 0})}}}}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", func() time.Time { return time.Unix(1_234, 0) })
	rolls := []int{0, 0, 123, 0}
	repository.combatRandom = func(max int) int {
		roll := rolls[0]
		rolls = rolls[1:]
		return roll % max
	}
	result, err = repository.maybeCreateBattleMoon(ctx, "`planets`", "`users`", 100, value, debris)
	if err != nil || result != (battleMoonCreation{Chance: 20, Created: true}) || len(runner.execCalls) != 1 {
		t.Fatalf("expected moon creation: %+v calls=%+v err=%v", result, runner.execCalls, err)
	}
	insert := runner.execCalls[0]
	if insert.args[0] != "Mond" || insert.args[1] != domaingame.PlanetTypeMoon || insert.args[6] != 8366 || insert.args[7] != 30 || insert.args[8] != int64(1_234) || insert.args[10] != int64(1_234) {
		t.Fatalf("unexpected moon row: %+v", insert)
	}
}

func TestFleetRepositoryBattleMoonCreationErrors(t *testing.T) {
	ctx := context.Background()
	value := fleetMessageContextTestValue()
	debris := domaingame.Resources{Metal: 2_000_000}
	for _, test := range []struct {
		name    string
		result  fakeQueryResult
		wantErr string
	}{
		{name: "query", result: fakeQueryResult{err: errors.New("moon query failed")}, wantErr: "moon query failed"},
		{name: "missing", result: fakeQueryResult{rows: fakeRowsFromValues()}, wantErr: "target unavailable"},
		{name: "empty trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("moon empty trailer failed"))}, wantErr: "moon empty trailer failed"},
		{name: "scan", result: fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}, wantErr: "unexpected scan"},
		{name: "trailer", result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("moon trailer failed"), []any{domaingame.PlanetTypePlanet, 50, "en", 0})}, wantErr: "moon trailer failed"},
	} {
		runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}}
		repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
		if _, err := repository.maybeCreateBattleMoon(ctx, "`planets`", "`users`", 100, value, debris); err == nil || !strings.Contains(err.Error(), test.wantErr) {
			t.Fatalf("%s: expected %q, got %v", test.name, test.wantErr, err)
		}
	}

	runner := &fakeFleetRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.PlanetTypePlanet, 50, "en", 0})}}}}
	repository := NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = nil
	if _, err := repository.maybeCreateBattleMoon(ctx, "`planets`", "`users`", 100, value, debris); err == nil || !strings.Contains(err.Error(), "random source unavailable") {
		t.Fatalf("expected random source error, got %v", err)
	}

	runner = &fakeFleetRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{domaingame.PlanetTypePlanet, 50, "en", 0})}}},
		execErrs:    []error{errors.New("moon insert failed")},
	}
	repository = NewFleetRepositoryWithRunner(runner, runner, "ogame_", nil)
	repository.combatRandom = func(int) int { return 0 }
	if _, err := repository.maybeCreateBattleMoon(ctx, "`planets`", "`users`", 100, value, debris); err == nil || !strings.Contains(err.Error(), "moon insert failed") {
		t.Fatalf("expected moon insert error, got %v", err)
	}
}

func TestBattleMoonReportAndNames(t *testing.T) {
	var report strings.Builder
	appendBattleMoonReport(&report, battleMoonCreation{Chance: 20, Created: true})
	for _, want := range []string{"chance for a moon to be created is 20 %", "form a moon around the planet"} {
		if !strings.Contains(report.String(), want) {
			t.Fatalf("expected %q in moon report: %s", want, report.String())
		}
	}
	for language, want := range map[string]string{
		"en": "Moon", "de": "Mond", "fr": "Lune", "es": "Luna", "it": "Luna",
		"ru": "\u041b\u0443\u043d\u0430", "jp": "\u6708", "unknown": "Moon",
	} {
		if got := battleMoonName(language); got != want {
			t.Fatalf("battleMoonName(%q)=%q, want %q", language, got, want)
		}
	}
	if text := fmt.Sprint(report.String()); text == "" {
		t.Fatal("moon report must not be empty")
	}
}
