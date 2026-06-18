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
	if repository.now == nil {
		t.Fatal("expected default clock")
	}

	withDefaultClock := NewResourcesRepositoryWithQueryer(nil, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected nil clock to default")
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
	results := []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0})},
		{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{1, "Home", domaingame.PlanetTypePlanet, 1, 1, 1})},
		{rows: fakeRowsFromValues([]any{1})},
	}
	results = append(results, tail...)
	return &fakeQueryer{results: results}
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
