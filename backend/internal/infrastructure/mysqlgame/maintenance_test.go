package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMaintenanceRepositoryReadsUniverseFreezeState(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1, "fr", "https://board.example.test"})},
	}}
	repository := NewMaintenanceRepositoryWithQueryer(queryer, "ogame_")

	maintenance, err := repository.GetMaintenance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !maintenance.Frozen || maintenance.Language != "fr" || maintenance.BoardURL != "https://board.example.test" {
		t.Fatalf("unexpected maintenance state: %+v", maintenance)
	}
	if !strings.Contains(queryer.calls[0].sql, "FROM `ogame_uni`") || !strings.Contains(queryer.calls[0].sql, "COALESCE(freeze, 0)") {
		t.Fatalf("unexpected maintenance query: %s", queryer.calls[0].sql)
	}
}

func TestMaintenanceRepositoryMapsMCPMaintenance(t *testing.T) {
	repository := NewMaintenanceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{1, "fr", "https://board.example.test"})},
	}}, "ogame_")

	status, err := repository.GetMCPMaintenance(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetMCPMaintenance returned error: %v", err)
	}
	if status.PlayerID != 42 || !status.Frozen || status.Language != "fr" || status.BoardURL != "https://board.example.test" {
		t.Fatalf("unexpected MCP maintenance status: %+v", status)
	}
}

func TestMaintenanceRepositoryDefaultsMissingUniverseToEnglishUnfrozen(t *testing.T) {
	repository := NewMaintenanceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
	}}, "ogame_")

	maintenance, err := repository.GetMaintenance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if maintenance.Frozen || maintenance.Language != "en" || maintenance.BoardURL != "" {
		t.Fatalf("unexpected missing universe state: %+v", maintenance)
	}
}

func TestNewMaintenanceRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewMaintenanceRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestMaintenanceRepositoryErrors(t *testing.T) {
	if _, err := (MaintenanceRepository{}).GetMaintenance(context.Background()); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	repository := NewMaintenanceRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_")
	if _, err := repository.GetMaintenance(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected prefix error, got %v", err)
	}
	repository = NewMaintenanceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("query failed")}}}, "ogame_")
	if _, err := repository.GetMaintenance(context.Background()); err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("expected query error, got %v", err)
	}
	repository = NewMaintenanceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"bad", "en", ""})},
	}}, "ogame_")
	if _, err := repository.GetMaintenance(context.Background()); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected scan error, got %v", err)
	}
	repository = NewMaintenanceRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValuesWithErr(errors.New("rows failed"), []any{0, "en", ""})},
	}}, "ogame_")
	if _, err := repository.GetMaintenance(context.Background()); err == nil || !strings.Contains(err.Error(), "rows failed") {
		t.Fatalf("expected rows error, got %v", err)
	}
}
