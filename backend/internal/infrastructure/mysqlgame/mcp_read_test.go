package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestMCPReadRepositoryListsPlanetsWithLegacyOrdering(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 2, 0})},
		{rows: fakeRowsFromValues(
			[]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3},
			[]any{100, "Moon", domaingame.PlanetTypeMoon, 1, 2, 3},
		)},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	planets, err := repository.ListMCPPlanets(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListMCPPlanets returned error: %v", err)
	}
	if len(planets) != 2 || !planets[0].Current || planets[1].TypeName != "moon" {
		t.Fatalf("unexpected planets: %+v", planets)
	}
	if !strings.Contains(queryer.calls[1].sql, "FROM `uni1_planets`") || !strings.Contains(queryer.calls[1].sql, "ORDER BY name ASC") {
		t.Fatalf("expected legacy planet ordering SQL, got %s", queryer.calls[1].sql)
	}
}

func TestMCPReadRepositoryFallsBackToHomePlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0, 88, 0, 0})},
		{rows: fakeRowsFromValues([]any{88, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	planets, err := repository.ListMCPPlanets(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListMCPPlanets returned error: %v", err)
	}
	if len(planets) != 1 || !planets[0].Current || planets[0].TypeName != "planet" {
		t.Fatalf("unexpected fallback planet: %+v", planets)
	}
}

func TestMCPReadRepositoryErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected nil queryer error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected invalid prefix error")
	}

	wantErr := errors.New("query failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected settings query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected missing player error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 88, 0, 0})}}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected settings scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{99, 88, 0, 0})}}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected settings rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{rows: fakeRowsFromValues([]any{"bad", "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
	}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected planet scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{rows: fakeRowsFromValuesWithErr(wantErr, []any{99, "Unknown", 999, 1, 2, 3})},
	}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet rows error, got %v", err)
	}
}

func TestMCPReadRepositoryConstructorsAndUnknownType(t *testing.T) {
	if repository := NewMCPReadRepository(nil, "uni1_"); repository.prefix != "uni1_" {
		t.Fatalf("unexpected constructor result: %+v", repository)
	}
	if got := mcpPlanetTypeName(999); got != "unknown" {
		t.Fatalf("expected unknown type name, got %q", got)
	}
}
