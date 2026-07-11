package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestPrangerRepositoryReadsBanEntries(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(
			[]any{int64(300), "Admin", "Player", int64(900), "Reason"},
			[]any{int64(200), "System", "Other", int64(800), "Second"},
		)},
	}}
	repository := NewPrangerRepositoryWithQueryer(queryer, "ogame_")

	pranger, err := repository.GetPranger(context.Background(), appgame.PrangerQuery{Universe: 7, From: 50, Internal: true})
	if err != nil {
		t.Fatal(err)
	}
	if pranger.Universe != 7 || pranger.From != 50 || !pranger.Internal || len(pranger.Entries) != 2 {
		t.Fatalf("unexpected pranger result: %+v", pranger)
	}
	if first := pranger.Entries[0]; first.BanWhen != 300 || first.AdminName != "Admin" || first.UserName != "Player" || first.BanUntil != 900 || first.Reason != "Reason" {
		t.Fatalf("unexpected first entry: %+v", first)
	}
	call := queryer.calls[0]
	if !strings.Contains(call.sql, "FROM `ogame_pranger`") || !strings.Contains(call.sql, "ORDER BY ban_when DESC") {
		t.Fatalf("unexpected pranger query: %s", call.sql)
	}
	if len(call.args) != 2 || call.args[0] != domaingame.PrangerPageLimit || call.args[1] != 50 {
		t.Fatalf("unexpected query args: %+v", call.args)
	}
}

func TestPrangerRepositoryMapsMCPPranger(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{int64(300), "Admin", "Player", int64(900), "Reason"})},
	}}
	repository := NewPrangerRepositoryWithQueryer(queryer, "ogame_")

	pranger, err := repository.GetMCPPranger(context.Background(), 42, domainmcp.PrangerCommand{Universe: 7, From: 50})
	if err != nil {
		t.Fatal(err)
	}
	if pranger.PlayerID != 42 || pranger.Universe != 7 || pranger.From != 50 || !pranger.HasPrevious || pranger.Previous != 0 || pranger.Limit != domaingame.PrangerPageLimit || len(pranger.Entries) != 1 {
		t.Fatalf("unexpected mcp pranger: %+v", pranger)
	}
	if pranger.Entries[0].UserName != "Player" || pranger.Entries[0].Reason != "Reason" {
		t.Fatalf("unexpected mcp pranger entry: %+v", pranger.Entries[0])
	}
}

func TestNewPrangerRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewPrangerRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestPrangerRepositoryErrors(t *testing.T) {
	if _, err := (PrangerRepository{}).GetPranger(context.Background(), appgame.PrangerQuery{}); err == nil || !strings.Contains(err.Error(), "reader unavailable") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	repository := NewPrangerRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_")
	if _, err := repository.GetPranger(context.Background(), appgame.PrangerQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected prefix error, got %v", err)
	}
	repository = NewPrangerRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("query failed")}}}, "ogame_")
	if _, err := repository.GetPranger(context.Background(), appgame.PrangerQuery{}); err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("expected query error, got %v", err)
	}
	repository = NewPrangerRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"bad", "Admin", "Player", int64(900), "Reason"})},
	}}, "ogame_")
	if _, err := repository.GetPranger(context.Background(), appgame.PrangerQuery{}); err == nil || !strings.Contains(err.Error(), "expected int64") {
		t.Fatalf("expected scan error, got %v", err)
	}
	repository = NewPrangerRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValuesWithErr(errors.New("rows failed"), []any{int64(1), "Admin", "Player", int64(2), "Reason"})},
	}}, "ogame_")
	if _, err := repository.GetPranger(context.Background(), appgame.PrangerQuery{}); err == nil || !strings.Contains(err.Error(), "rows failed") {
		t.Fatalf("expected rows error, got %v", err)
	}
}
