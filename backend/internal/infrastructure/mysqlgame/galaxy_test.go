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

	withDefaultClock := NewGalaxyRepositoryWithQueryer(&fakeQueryer{}, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected default clock")
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
