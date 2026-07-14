package mysqlgame

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestAdminPlanetsUpdateAndDelete(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	recalcRow := append([]any{domaingame.PlanetTypePlanet, 15_000}, buildingLevelRow(map[int]int{
		domaingame.BuildingMetalMine: 9, domaingame.BuildingTerraformer: 2,
	})...)
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, domaingame.PlanetTypePlanet))},
		{rows: fakeRowsFromValues(recalcRow)},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return now }
	issue, err := repository.mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{
		PlayerID: 1, PlanetID: 70, Action: domaingame.AdminActionPlanetsUpdate,
		Planet: &domaingame.AdminPlanetMutation{
			Coordinates: domaingame.Coordinates{Galaxy: -2, System: 33, Position: -8}, Diameter: -15_000, Type: -1, Temperature: -90,
			Resources: map[int]int{700: -100, 701: 200, 702: -300},
			Buildings: map[int]int{domaingame.BuildingMetalMine: 120, domaingame.BuildingTerraformer: -2},
			Fleet:     map[int]int{domaingame.FleetSmallCargo: -4}, Defense: map[int]int{domaingame.DefenseRocketLauncher: -5},
			Production: map[int]float64{1: .5, 212: .7},
		},
	})
	if err != nil || issue == nil || len(runner.execCalls) != 3 {
		t.Fatalf("issue=%+v execs=%+v err=%v", issue, runner.execCalls, err)
	}
	if move := runner.execCalls[0]; !strings.Contains(move.sql, "type IN (?, ?)") || move.args[1] != -2 || move.args[3] != -8 {
		t.Fatalf("unexpected moon move: %+v", move)
	}
	update := runner.execCalls[1]
	if !strings.Contains(update.sql, "`1` = ?") || !strings.Contains(update.sql, "prod212 = ?") || !containsAny(update.args, 99) || !containsAny(update.args, 100) || !containsAny(update.args, 90) {
		t.Fatalf("unexpected planet update: %+v", update)
	}
	fields := runner.execCalls[2]
	if fields.args[0] != 11 || fields.args[1] != 235 {
		t.Fatalf("unexpected field recalculation: %+v", fields)
	}

	home := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(7, 42, 7, 1))}}}}
	issue, err = NewAdminRepositoryWithQueryer(home, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 7, Action: domaingame.AdminActionPlanetsUpdate, Planet: &domaingame.AdminPlanetMutation{Delete: true}})
	if err != nil || len(home.execCalls) != 0 || issue.Result == nil || issue.Result.ItemID != 7 {
		t.Fatalf("home delete issue=%+v execs=%+v err=%v", issue, home.execCalls, err)
	}

	colony := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, 1))}}}}
	issue, err = NewAdminRepositoryWithQueryer(colony, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 70, Action: domaingame.AdminActionPlanetsUpdate, Planet: &domaingame.AdminPlanetMutation{Delete: true}})
	if err != nil || len(colony.execCalls) != 4 || !strings.Contains(colony.execCalls[3].sql, "DELETE FROM `ogame_planets`") || issue.Result.ItemID != 7 {
		t.Fatalf("colony delete issue=%+v execs=%+v err=%v", issue, colony.execCalls, err)
	}
}

func TestAdminPlanetsCreateMoonAndDebris(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, 1))},
		{rows: fakeRowsFromValues()},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return now }
	values := []int{0, 4, 3}
	repository.randomIntN = func(int) int { value := values[0]; values = values[1:]; return value }
	_, err := repository.mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 70, Action: domaingame.AdminActionPlanetsCreateMoon})
	if err != nil || len(runner.execCalls) != 1 {
		t.Fatalf("execs=%+v err=%v", runner.execCalls, err)
	}
	insert := runner.execCalls[0]
	if insert.args[0] != "Mond" || insert.args[1] != domaingame.PlanetTypeMoon || insert.args[6] != int(math.Floor(1000*math.Sqrt(70))) || insert.args[7] != 27 {
		t.Fatalf("unexpected moon insert: %+v", insert)
	}

	debris := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, 1))},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{"fr"})},
	}}}
	repository = NewAdminRepositoryWithQueryer(debris, "ogame_")
	repository.now = func() time.Time { return now }
	_, err = repository.mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlayerID: 1, PlanetID: 70, Action: domaingame.AdminActionPlanetsCreateDebris})
	if err != nil || len(debris.execCalls) != 1 || debris.execCalls[0].args[0] != "Champ de débris" || debris.execCalls[0].args[1] != domaingame.PlanetTypeDebris {
		t.Fatalf("unexpected debris execs=%+v err=%v", debris.execCalls, err)
	}

	existing := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, 1))},
		{rows: fakeRowsFromValues([]any{80})},
	}}}
	_, err = NewAdminRepositoryWithQueryer(existing, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 70, Action: domaingame.AdminActionPlanetsCreateMoon})
	if err != nil || len(existing.execCalls) != 0 {
		t.Fatalf("existing moon execs=%+v err=%v", existing.execCalls, err)
	}
}

func TestAdminPlanetsGateFieldsAndDiameter(t *testing.T) {
	now := time.Unix(2_000, 0)
	warm := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(80, 42, 7, domaingame.PlanetTypeMoon))}}}}
	repository := NewAdminRepositoryWithQueryer(warm, "ogame_")
	repository.now = func() time.Time { return now }
	_, err := repository.mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 80, Action: domaingame.AdminActionPlanetsWarmupGates})
	if err != nil || len(warm.execCalls) != 1 || warm.execCalls[0].args[0] != int64(5_599) {
		t.Fatalf("warm execs=%+v err=%v", warm.execCalls, err)
	}

	cool := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(80, 42, 7, domaingame.PlanetTypeMoon))}}}}
	_, err = NewAdminRepositoryWithQueryer(cool, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 80, Action: domaingame.AdminActionPlanetsCooldownGates})
	if err != nil || cool.execCalls[0].args[0] != int64(0) {
		t.Fatalf("cool execs=%+v err=%v", cool.execCalls, err)
	}

	fieldRow := append([]any{domaingame.PlanetTypeMoon, 8_000}, buildingLevelRow(map[int]int{domaingame.BuildingLunarBase: 3, domaingame.BuildingRoboticsFactory: 2})...)
	fields := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(80, 42, 7, domaingame.PlanetTypeMoon))},
		{rows: fakeRowsFromValues(fieldRow)},
	}}}
	_, err = NewAdminRepositoryWithQueryer(fields, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 80, Action: domaingame.AdminActionPlanetsRecalcFields})
	if err != nil || fields.execCalls[0].args[0] != 5 || fields.execCalls[0].args[1] != 10 {
		t.Fatalf("fields execs=%+v err=%v", fields.execCalls, err)
	}

	diameter := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, domaingame.PlanetTypePlanet))},
		{rows: fakeRowsFromValues([]any{6, 8, 1000, 8, 10, 1000, 10, 12, 1000, 12, 14, 1000, 14, 16, 1000})},
	}}}
	repository = NewAdminRepositoryWithQueryer(diameter, "ogame_")
	repository.randomIntN = func(int) int { return 1 }
	_, err = repository.mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 70, Action: domaingame.AdminActionPlanetsRandomDiameter})
	if err != nil || len(diameter.execCalls) != 1 || diameter.execCalls[0].args[0] != 11_000 {
		t.Fatalf("diameter execs=%+v err=%v", diameter.execCalls, err)
	}
}

func TestAdminPlanetSearchAndHelpers(t *testing.T) {
	row := []any{70, "Alpha", int64(100), 1, 2, 3, float64(4), float64(5), float64(6), 42, "owner", int64(10), int64(20), 1, 0, 1, 0}
	for _, searchType := range []string{"playername", "planetname", "allytag", "invalid"} {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(row)}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		rows, err := repository.loadAdminPlanetSearchRows(context.Background(), domaingame.AdminPlanetSearch{Type: searchType, Text: "Al"})
		if err != nil || len(rows) != 1 || rows[0].Owner == nil || !rows[0].Owner.Vacation || !rows[0].Owner.NoAttack || runner.calls[0].args[0] != "Al%" {
			t.Fatalf("type=%s rows=%+v calls=%+v err=%v", searchType, rows, runner.calls, err)
		}
	}
	if adminPlanetAbs(-3) != 3 || adminPlanetAbs(4) != 4 || !adminIsGamePlanet(domaingame.PlanetTypeAbandoned) || adminIsGamePlanet(domaingame.PlanetTypeMoon) || adminIsGamePlanet(adminPlanetTypeCustom) {
		t.Fatal("unexpected admin planet helpers")
	}
	for language, want := range map[string]string{"de": "Trümmerfeld", "es": "Campo de escombros", "fr": "Champ de débris", "it": "Campo detriti", "ru": "Поле обломков", "jp": "デブリフィールド", "en": "Debris Field"} {
		if got := adminDebrisName(language); got != want {
			t.Fatalf("adminDebrisName(%q)=%q want %q", language, got, want)
		}
	}
}

func TestAdminPlanetMutationNoopsAndErrors(t *testing.T) {
	search, err := NewAdminRepositoryWithQueryer(&fakeGalaxyRunner{}, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{Action: domaingame.AdminActionPlanetsSearch})
	if err != nil || search == nil {
		t.Fatalf("search issue=%+v err=%v", search, err)
	}
	missing := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	issue, err := NewAdminRepositoryWithQueryer(missing, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 999, Action: domaingame.AdminActionPlanetsUpdate})
	if err != nil || issue == nil || len(missing.execCalls) != 0 {
		t.Fatalf("missing issue=%+v err=%v", issue, err)
	}
	if _, err := NewAdminRepositoryWithQueryer(&fakeGalaxyRunner{}, "bad-prefix_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{Action: domaingame.AdminActionPlanetsUpdate}); err == nil {
		t.Fatal("expected invalid prefix error")
	}
	failed := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("target failed")}}}}
	if _, err := NewAdminRepositoryWithQueryer(failed, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 1, Action: domaingame.AdminActionPlanetsUpdate}); err == nil || !strings.Contains(err.Error(), "target failed") {
		t.Fatalf("expected target error, got %v", err)
	}
	nilMutation := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, 1))}}}}
	if _, err := NewAdminRepositoryWithQueryer(nilMutation, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 70, Action: domaingame.AdminActionPlanetsUpdate}); err != nil || len(nilMutation.execCalls) != 0 {
		t.Fatalf("nil mutation execs=%+v err=%v", nilMutation.execCalls, err)
	}
}

func TestAdminRepositoryReadsPlanetSearchState(t *testing.T) {
	searchRow := []any{70, "Alpha", int64(100), 1, 2, 3, float64(4), float64(5), float64(6), 42, "owner", int64(10), int64(20), 0, 0, 0, 0}
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues(searchRow)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")
	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{
		PlayerID: 42, PlanetID: 99, Mode: "Planets", PlanetSearch: &domaingame.AdminPlanetSearch{Type: "planetname", Text: "Al"},
	})
	if err != nil || !admin.PlanetSearchAttempted || admin.PlanetSearchBlank || len(admin.PlanetSearchRows) != 1 {
		t.Fatalf("admin=%+v err=%v", admin, err)
	}

	blankQueryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}
	admin, err = NewAdminRepositoryWithQueryer(blankQueryer, "ogame_").GetAdmin(context.Background(), appgame.AdminQuery{
		PlayerID: 42, PlanetID: 99, Mode: "Planets", PlanetSearch: &domaingame.AdminPlanetSearch{Type: "planetname", Text: " "},
	})
	if err != nil || !admin.PlanetSearchAttempted || !admin.PlanetSearchBlank || len(blankQueryer.results) != 0 {
		t.Fatalf("blank admin=%+v remaining=%d err=%v", admin, len(blankQueryer.results), err)
	}
}

func TestAdminPlanetMutationFailureBoundaries(t *testing.T) {
	t.Run("moon move", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, 1))}}}, execErrs: []error{errors.New("move failed")}}
		_, err := NewAdminRepositoryWithQueryer(runner, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 70, Action: domaingame.AdminActionPlanetsUpdate, Planet: &domaingame.AdminPlanetMutation{}})
		if err == nil || !strings.Contains(err.Error(), "move failed") {
			t.Fatalf("expected moon move error, got %v", err)
		}
	})
	t.Run("delete flush and row", func(t *testing.T) {
		for index, want := range []string{"shipyard flush failed", "building event flush failed", "build queue flush failed", "delete failed"} {
			runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(adminPlanetMutationTargetRow(70, 42, 7, 1))}}}}
			runner.execErrs = make([]error, index+1)
			runner.execErrs[index] = errors.New(want)
			_, err := NewAdminRepositoryWithQueryer(runner, "ogame_").mutateAdminPlanets(context.Background(), appgame.AdminMutationQuery{PlanetID: 70, Action: domaingame.AdminActionPlanetsUpdate, Planet: &domaingame.AdminPlanetMutation{Delete: true}})
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("index=%d expected %q, got %v", index, want, err)
			}
		}
	})
	t.Run("target scan", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}}
		if _, _, err := NewAdminRepositoryWithQueryer(runner, "ogame_").loadAdminPlanetMutationTarget(context.Background(), "`planets`", "`users`", 1); err == nil {
			t.Fatal("expected target scan error")
		}
	})
	t.Run("creation guards", func(t *testing.T) {
		target := adminPlanetMutationTarget{ID: 70, Type: domaingame.PlanetTypeMoon}
		repository := NewAdminRepositoryWithQueryer(&fakeGalaxyRunner{}, "ogame_")
		if err := repository.createAdminPlanetMoon(context.Background(), "`planets`", target); err != nil {
			t.Fatal(err)
		}
		if err := repository.createAdminPlanetDebris(context.Background(), "`planets`", "`users`", 1, target); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("gate guard", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		if err := NewAdminRepositoryWithQueryer(runner, "ogame_").setAdminPlanetGate(context.Background(), "`planets`", adminPlanetMutationTarget{Type: 1}, 3); err != nil || len(runner.execCalls) != 0 {
			t.Fatalf("execs=%+v err=%v", runner.execCalls, err)
		}
	})
	t.Run("related empty", func(t *testing.T) {
		repository := NewAdminRepositoryWithQueryer(&fakeGalaxyRunner{}, "ogame_")
		exists, err := repository.adminRelatedPlanetExists(context.Background(), "`planets`", domaingame.Coordinates{}, nil)
		if err != nil || exists {
			t.Fatalf("exists=%v err=%v", exists, err)
		}
	})
	t.Run("language empty", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
		if _, err := NewAdminRepositoryWithQueryer(runner, "ogame_").loadAdminUserLanguage(context.Background(), "`users`", 1); err == nil || !strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("expected unavailable language, got %v", err)
		}
	})
	t.Run("recalc empty", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
		if err := NewAdminRepositoryWithQueryer(runner, "ogame_").recalcAdminPlanetFields(context.Background(), "`planets`", 1); err != nil {
			t.Fatalf("empty recalculation should be a noop: %v", err)
		}
	})
	t.Run("diameter guard", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		if err := NewAdminRepositoryWithQueryer(runner, "ogame_").randomizeAdminPlanetDiameter(context.Background(), "`planets`", adminPlanetMutationTarget{Type: domaingame.PlanetTypeDebris}); err != nil || len(runner.execCalls) != 0 {
			t.Fatalf("execs=%+v err=%v", runner.execCalls, err)
		}
	})
}

func TestAdminPlanetDatabaseFailurePropagation(t *testing.T) {
	target := adminPlanetMutationTarget{ID: 70, OwnerID: 42, Type: 1, Temperature: 50, Coordinates: domaingame.Coordinates{Galaxy: 1, System: 2, Position: 8}, Language: "en"}
	t.Run("update row", func(t *testing.T) {
		runner := &fakeGalaxyRunner{execErrs: []error{nil, errors.New("update failed")}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.updateAdminPlanet(context.Background(), "`planets`", target, &domaingame.AdminPlanetMutation{}); err == nil || !strings.Contains(err.Error(), "update failed") {
			t.Fatalf("expected update error, got %v", err)
		}
	})
	t.Run("update recalc", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("recalc failed")}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.updateAdminPlanet(context.Background(), "`planets`", target, &domaingame.AdminPlanetMutation{}); err == nil || !strings.Contains(err.Error(), "recalc failed") {
			t.Fatalf("expected recalc error, got %v", err)
		}
	})
	t.Run("moon related and insert", func(t *testing.T) {
		for _, test := range []struct {
			results []fakeQueryResult
			execErr error
			want    string
		}{
			{results: []fakeQueryResult{{err: errors.New("related failed")}}, want: "related failed"},
			{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}, execErr: errors.New("moon insert failed"), want: "moon insert failed"},
		} {
			runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: test.results}, execErrs: []error{test.execErr}}
			repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
			repository.randomIntN = nil
			if err := repository.createAdminPlanetMoon(context.Background(), "`planets`", target); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		}
	})
	t.Run("debris language and insert", func(t *testing.T) {
		for _, test := range []struct {
			results []fakeQueryResult
			execErr error
			want    string
		}{
			{results: []fakeQueryResult{{err: errors.New("related failed")}}, want: "related failed"},
			{results: []fakeQueryResult{{rows: fakeRowsFromValues()}, {err: errors.New("language failed")}}, want: "language failed"},
			{results: []fakeQueryResult{{rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues([]any{"en"})}}, execErr: errors.New("debris insert failed"), want: "debris insert failed"},
		} {
			runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: test.results}, execErrs: []error{test.execErr}}
			if err := NewAdminRepositoryWithQueryer(runner, "ogame_").createAdminPlanetDebris(context.Background(), "`planets`", "`users`", 1, target); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		}
	})
	t.Run("related rows", func(t *testing.T) {
		for _, result := range []fakeQueryResult{
			{err: errors.New("query failed")},
			{rows: fakeRowsFromValues([]any{"bad"})},
			{rows: fakeRowsFromValuesWithErr(errors.New("trailer failed"), []any{1})},
		} {
			runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{result}}}
			if _, err := NewAdminRepositoryWithQueryer(runner, "ogame_").adminRelatedPlanetExists(context.Background(), "`planets`", target.Coordinates, []int{0}); err == nil {
				t.Fatalf("expected related row error for %+v", result)
			}
		}
	})
	t.Run("language rows", func(t *testing.T) {
		for _, result := range []fakeQueryResult{
			{err: errors.New("query failed")},
			{rows: fakeRowsFromValues([]any{1, 2})},
			{rows: fakeRowsFromValuesWithErr(errors.New("trailer failed"), []any{"en"})},
			{rows: fakeRowsError(errors.New("empty trailer failed"))},
		} {
			runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{result}}}
			if _, err := NewAdminRepositoryWithQueryer(runner, "ogame_").loadAdminUserLanguage(context.Background(), "`users`", 1); err == nil {
				t.Fatalf("expected language row error for %+v", result)
			}
		}
	})
	t.Run("gate exec", func(t *testing.T) {
		runner := &fakeGalaxyRunner{execErrs: []error{errors.New("gate failed")}}
		if err := NewAdminRepositoryWithQueryer(runner, "ogame_").setAdminPlanetGate(context.Background(), "`planets`", adminPlanetMutationTarget{ID: 1, Type: 0}, 3); err == nil {
			t.Fatal("expected gate error")
		}
	})
}

func TestAdminPlanetReadAndRecalculationFailures(t *testing.T) {
	badLevelRow := append([]any{1, 10_000}, buildingLevelRow(nil)...)
	badLevelRow = badLevelRow[:len(badLevelRow)-1]
	for _, test := range []struct {
		result  fakeQueryResult
		execErr error
	}{
		{result: fakeQueryResult{err: errors.New("query failed")}},
		{result: fakeQueryResult{rows: fakeRowsFromValues(badLevelRow)}},
		{result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("trailer failed"), append([]any{1, 10_000}, buildingLevelRow(nil)...))}},
		{result: fakeQueryResult{rows: fakeRowsFromValues(append([]any{1, 10_000}, buildingLevelRow(nil)...))}, execErr: errors.New("fields update failed")},
	} {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}, execErrs: []error{test.execErr}}
		if err := NewAdminRepositoryWithQueryer(runner, "ogame_").recalcAdminPlanetFields(context.Background(), "`planets`", 1); err == nil {
			t.Fatalf("expected recalc error for %+v", test)
		}
	}

	for _, test := range []struct {
		result  fakeQueryResult
		execErr error
	}{
		{result: fakeQueryResult{err: errors.New("settings failed")}},
		{result: fakeQueryResult{rows: fakeRowsFromValues([]any{6, 8, 1000, 8, 10, 1000, 10, 12, 1000, 12, 14, 1000, 14, 16, 1000})}, execErr: errors.New("diameter update failed")},
	} {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{test.result}}, execErrs: []error{test.execErr}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		repository.randomIntN = nil
		if err := repository.randomizeAdminPlanetDiameter(context.Background(), "`planets`", adminPlanetMutationTarget{ID: 1, Type: 1, Coordinates: domaingame.Coordinates{Position: 8}}); err == nil {
			t.Fatalf("expected diameter error for %+v", test)
		}
	}

	for _, result := range []fakeQueryResult{{err: errors.New("search failed")}, {rows: fakeRowsFromValues([]any{"bad"})}} {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{result}}}
		if _, err := NewAdminRepositoryWithQueryer(runner, "ogame_").loadAdminPlanetSearchRows(context.Background(), domaingame.AdminPlanetSearch{Type: "planetname", Text: "A"}); err == nil {
			t.Fatalf("expected search error for %+v", result)
		}
	}
}

func TestAdminParentPlanetRows(t *testing.T) {
	coordinates := domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{70, "Parent", int64(10), 1, 2, 3, float64(4), float64(5), float64(6)})}}}}
	parent, err := NewAdminRepositoryWithQueryer(runner, "ogame_").loadAdminParentPlanet(context.Background(), "`planets`", coordinates)
	if err != nil || parent == nil || parent.ID != 70 || parent.Resources.Crystal != 5 {
		t.Fatalf("parent=%+v err=%v", parent, err)
	}
	for _, result := range []fakeQueryResult{
		{err: errors.New("query failed")},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{"bad"})},
		{rows: fakeRowsFromValuesWithErr(errors.New("trailer failed"), []any{70, "Parent", int64(10), 1, 2, 3, float64(4), float64(5), float64(6)})},
	} {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{result}}}
		parent, err := NewAdminRepositoryWithQueryer(runner, "ogame_").loadAdminParentPlanet(context.Background(), "`planets`", coordinates)
		if result.rows != nil && len(result.rows.values) == 0 && result.rows.err == nil {
			if err != nil || parent != nil {
				t.Fatalf("empty parent=%+v err=%v", parent, err)
			}
		} else if err == nil {
			t.Fatalf("expected parent error for %+v", result)
		}
	}
}

func adminPlanetMutationTargetRow(id int, ownerID int, homeID int, planetType int) []any {
	return []any{id, ownerID, homeID, planetType, 1, 2, 8, 15_000, 50, "de"}
}

func containsAny(values []any, wanted any) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
