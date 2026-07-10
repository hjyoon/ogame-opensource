package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestMCPReadRepositoryGetsAccountOverview(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 1, 99, 88})},
		{rows: fakeRowsFromValues([]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues([]any{5})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	overview, err := repository.GetMCPAccountOverview(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetMCPAccountOverview returned error: %v", err)
	}
	if overview.Commander != "legor" || overview.Score.Display != 123 || overview.CurrentPlanet.ID != 99 || overview.PlanetCount != 2 || overview.UnreadMessages != 5 {
		t.Fatalf("unexpected account overview: %+v", overview)
	}
	if !strings.Contains(queryer.calls[0].sql, "FROM `uni1_users`") ||
		!strings.Contains(queryer.calls[1].sql, "FROM `uni1_planets`") ||
		!strings.Contains(queryer.calls[3].sql, "FROM `uni1_messages`") {
		t.Fatalf("unexpected account overview SQL calls: %+v", queryer.calls)
	}
}

func TestMCPReadRepositoryAccountOverviewFallsBackToHomePlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(-1), 2, 0, 88})},
		{rows: fakeRowsFromValues([]any{88, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues([]any{0})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	overview, err := repository.GetMCPAccountOverview(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetMCPAccountOverview returned error: %v", err)
	}
	if overview.CurrentPlanet.ID != 88 || overview.Score.Display != 0 {
		t.Fatalf("unexpected fallback overview: %+v", overview)
	}
}

func TestMCPReadRepositoryGetsPlanetResources(t *testing.T) {
	now := time.Unix(1700000000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 1000, 37, 4, now.Add(time.Hour).Unix(), now.Add(time.Hour).Unix()})},
		{rows: fakeRowsFromValues([]any{
			99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3,
			12345.0, 23456.0, 34567.0,
			40,
			10, 9, 8,
			10, 8, 6, 12, 2, 5,
			1.0, 1.0, 1.0, 1.0, 1.0, 1.0,
		})},
		{rows: fakeRowsFromValues([]any{128.0})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")
	repository.now = func() time.Time { return now }

	resources, err := repository.GetMCPPlanetResources(context.Background(), 42, 0)
	if err != nil {
		t.Fatalf("GetMCPPlanetResources returned error: %v", err)
	}
	if resources.PlayerID != 42 ||
		resources.Planet.ID != 99 ||
		!resources.Planet.Current ||
		resources.Resources.Metal != 12345 ||
		resources.Resources.DarkMatter != 1037 ||
		resources.Capacity.Metal != storageCapacity(10) ||
		resources.Energy.Capacity <= 0 ||
		resources.ProductionPerHour.Metal <= 0 {
		t.Fatalf("unexpected planet resources: %+v", resources)
	}
	if !strings.Contains(queryer.calls[0].sql, "COALESCE(dm, 0)") ||
		!strings.Contains(queryer.calls[1].sql, "COALESCE(`700`, 0)") ||
		!strings.Contains(queryer.calls[2].sql, "FROM `uni1_uni`") {
		t.Fatalf("unexpected resource SQL calls: %+v", queryer.calls)
	}
}

func TestMCPReadRepositoryGetsMoonResourcesWithoutProduction(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 1000, 37, 4, int64(0), int64(0)})},
		{rows: fakeRowsFromValues([]any{
			100, "Moon", domaingame.PlanetTypeMoon, 1, 2, 3,
			100.0, 200.0, 300.0,
			0,
			10, 9, 8,
			10, 8, 6, 12, 2, 5,
			1.0, 1.0, 1.0, 1.0, 1.0, 1.0,
		})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	resources, err := repository.GetMCPPlanetResources(context.Background(), 42, 100)
	if err != nil {
		t.Fatalf("GetMCPPlanetResources returned error: %v", err)
	}
	if resources.Planet.TypeName != "moon" ||
		resources.Planet.Current ||
		resources.Capacity.Metal != 0 ||
		resources.Energy.Capacity != 0 ||
		resources.ProductionPerHour.Metal != 0 ||
		len(queryer.calls) != 2 {
		t.Fatalf("unexpected moon resources: %+v calls=%+v", resources, queryer.calls)
	}
}

func TestMCPReadRepositoryPlanetResourcesErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected nil queryer error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected invalid prefix error")
	}

	wantErr := errors.New("query failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected account query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected missing account error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor", "bad", 88, 0, 0, 0, int64(0), int64(0)})}}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected account scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 0, 0, 0, int64(0), int64(0)})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet resource query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 0, 0, 0, int64(0), int64(0)})},
		{rows: fakeRowsFromValues()},
	}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected missing planet resources error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 0, 0, 0, int64(0), int64(0)})},
		{rows: fakeRowsFromValues([]any{"bad"})},
	}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected planet resource scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 0, 0, 0, int64(0), int64(0)})},
		{rows: fakeRowsFromValues([]any{
			99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3,
			1.0, 2.0, 3.0,
			40,
			0, 0, 0,
			1, 0, 0, 0, 0, 0,
			1.0, 0.0, 0.0, 0.0, 0.0, 0.0,
		})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected universe speed error, got %v", err)
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

func TestMCPReadRepositoryAccountOverviewErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected nil queryer error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected invalid prefix error")
	}

	wantErr := errors.New("query failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected account query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected missing account error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor", "bad", 1, 99, 88})}}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected account scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{"legor", int64(1), 1, 99, 88})}}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected account rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValues()},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected missing current planet error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected current planet query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValues([]any{"bad", "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected current planet scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValuesWithErr(wantErr, []any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected current planet rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValues([]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet count error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValues([]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues([]any{"bad"})},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected unread count scan error")
	}

	emptyCount, err := (MCPReadRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}).singleMCPCount(context.Background(), "SELECT COUNT(*)")
	if err != nil || emptyCount != 0 {
		t.Fatalf("expected empty count fallback, got count=%d err=%v", emptyCount, err)
	}

	if _, err := (MCPReadRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr)}}}}).singleMCPCount(context.Background(), "SELECT COUNT(*)"); !errors.Is(err, wantErr) {
		t.Fatalf("expected empty count rows error, got %v", err)
	}
}
