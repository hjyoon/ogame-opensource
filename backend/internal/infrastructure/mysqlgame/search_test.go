package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestSearchRepositoryReadsLegacyPlayerNameResults(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{7})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{42, "legor", 7, "TAG", 99, "Arakis", 1, 2, 3, 101},
			[]any{43, "target", 7, "TAG", 100, "Colony", 1, 2, 4, 102},
		)},
	)}
	repository := NewSearchRepositoryWithQueryer(queryer, "ogame_")

	search, err := repository.GetSearch(context.Background(), appgame.SearchQuery{PlayerID: 42, Type: "playername", Text: "leg"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Commander != "legor" || search.Type != domaingame.SearchTypePlayerName || search.Text != "leg" || len(search.PlayerRows) != 2 {
		t.Fatalf("unexpected search summary: %+v", search)
	}
	if !search.PlayerRows[0].Own || search.PlayerRows[0].Alliance == nil || search.PlayerRows[0].Alliance.Tag != "TAG" ||
		search.PlayerRows[0].Coordinates.Galaxy != 1 || search.PlayerRows[1].PlanetName != "Colony" || !search.PlayerRows[1].SameAlliance {
		t.Fatalf("unexpected player rows: %+v", search.PlayerRows)
	}
	if !strings.Contains(queryer.calls[5].sql, "u.oname LIKE ?") || queryer.calls[5].args[0] != "%leg%" || queryer.calls[5].args[1] != domaingame.SearchLimit+1 {
		t.Fatalf("expected legacy player search query, got %+v", queryer.calls[5])
	}
}

func TestSearchRepositoryReadsLegacyPlanetNameResults(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{43, "target", 7, "TAG", 100, "Arakis", 1, 2, 4, 102})},
	)}
	repository := NewSearchRepositoryWithQueryer(queryer, "ogame_")

	search, err := repository.GetSearch(context.Background(), appgame.SearchQuery{PlayerID: 42, Type: "planetname", Text: "Ara"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Type != domaingame.SearchTypePlanetName || len(search.PlayerRows) != 1 || search.PlayerRows[0].PlanetName != "Arakis" {
		t.Fatalf("unexpected planet search rows: %+v", search)
	}
	if !strings.Contains(queryer.calls[5].sql, "p.name LIKE ?") {
		t.Fatalf("expected legacy planet search query, got %+v", queryer.calls[5])
	}
}

func TestSearchRepositoryReadsAllianceResults(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{7, "TAG", "The Alliance", 3, int64(950000000), 1})},
	)}
	repository := NewSearchRepositoryWithQueryer(queryer, "ogame_")

	search, err := repository.GetSearch(context.Background(), appgame.SearchQuery{PlayerID: 42, Type: "allytag", Text: "TA"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Type != domaingame.SearchTypeAllianceTag || len(search.AllianceRows) != 1 || !search.AllianceRows[0].Own ||
		search.AllianceRows[0].Members != 3 || search.AllianceRows[0].DisplayScore() != 950000 {
		t.Fatalf("unexpected alliance search rows: %+v", search.AllianceRows)
	}
	if !strings.Contains(queryer.calls[4].sql, "a.tag LIKE ?") {
		t.Fatalf("expected legacy alliance search query, got %+v", queryer.calls[4])
	}
}

func TestNewSearchRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewSearchRepository(nil, "ogame_")
	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
}

func TestSearchRepositoryHandlesValidationAndResultMessages(t *testing.T) {
	queryer := &fakeQueryer{results: shipyardOverviewResults()}
	repository := NewSearchRepositoryWithQueryer(queryer, "ogame_")
	search, err := repository.GetSearch(context.Background(), appgame.SearchQuery{PlayerID: 42, Text: ""})
	if err != nil {
		t.Fatal(err)
	}
	if search.Message != "" || search.Text != "" || len(search.PlayerRows) != 0 || len(search.AllianceRows) != 0 || len(queryer.calls) != 4 {
		t.Fatalf("empty search should render form only, got search=%+v calls=%d", search, len(queryer.calls))
	}

	queryer = &fakeQueryer{results: shipyardOverviewResults()}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	search, err = repository.GetSearch(context.Background(), appgame.SearchQuery{PlayerID: 42, Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Message != "Too few characters! Please enter at least 2 characters." || len(queryer.calls) != 4 {
		t.Fatalf("expected short text message without search query, got search=%+v calls=%d", search, len(queryer.calls))
	}

	queryer = &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0})}, fakeQueryResult{rows: fakeRowsFromValues()})}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	search, err = repository.GetSearch(context.Background(), appgame.SearchQuery{PlayerID: 42, Text: "zz"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Message != "no entries found" {
		t.Fatalf("expected empty result message, got %+v", search)
	}

	values := make([][]any, 0, domaingame.SearchLimit+1)
	for i := 0; i < domaingame.SearchLimit+1; i++ {
		values = append(values, []any{i + 1, "TAG", "Alliance", 1, int64(1), 0})
	}
	queryer = &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues(values...)})}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	search, err = repository.GetSearch(context.Background(), appgame.SearchQuery{PlayerID: 42, Type: "allyname", Text: "Alliance"})
	if err != nil {
		t.Fatal(err)
	}
	if search.Message != "more than 25 entries found" || len(search.AllianceRows) != domaingame.SearchLimit {
		t.Fatalf("expected alliance over limit truncation, got message=%q rows=%d", search.Message, len(search.AllianceRows))
	}
}

func TestSearchRepositoryReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		query   appgame.SearchQuery
		want    string
	}{
		{name: "unsafe prefix", prefix: "bad-prefix_", queryer: &fakeQueryer{}, want: "invalid database table prefix"},
		{name: "overview", prefix: "ogame_", queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview failed")}}}, want: "overview failed"},
		{name: "own alliance", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("own alliance failed")})}, query: appgame.SearchQuery{Text: "leg"}, want: "own alliance failed"},
		{name: "player rows", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0})}, fakeQueryResult{err: errors.New("player failed")})}, query: appgame.SearchQuery{Text: "leg"}, want: "player failed"},
		{name: "planet rows", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{rows: fakeRowsFromValues([]any{0})}, fakeQueryResult{err: errors.New("planet failed")})}, query: appgame.SearchQuery{Type: "planetname", Text: "Ara"}, want: "planet failed"},
		{name: "alliance rows", prefix: "ogame_", queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("ally failed")})}, query: appgame.SearchQuery{Type: "allytag", Text: "TA"}, want: "ally failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewSearchRepositoryWithQueryer(tt.queryer, tt.prefix)
			_, err := repository.GetSearch(context.Background(), tt.query)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestSearchRepositoryLoaderScanEdges(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "legor", 0, "", 99, "Arakis", 1, 2, 3, 1})}}}
	repository := NewSearchRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadPlayerNameSearchRows(context.Background(), "ogame_users", "ogame_planets", "ogame_ally", 42, 0, "leg"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected player scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("player rows failed"), []any{42, "legor", 0, "", 99, "Arakis", 1, 2, 3, 1})}}}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadPlayerNameSearchRows(context.Background(), "ogame_users", "ogame_planets", "ogame_ally", 42, 0, "leg"); err == nil || !strings.Contains(err.Error(), "player rows failed") {
		t.Fatalf("expected player rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	if allianceID, err := repository.loadViewerAllianceID(context.Background(), "ogame_users", 42); err != nil || allianceID != 0 {
		t.Fatalf("empty own alliance should return zero, got alliance=%d err=%v", allianceID, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadViewerAllianceID(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected own alliance scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("own alliance rows failed"), []any{7})}}}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadViewerAllianceID(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "own alliance rows failed") {
		t.Fatalf("expected own alliance rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("own alliance empty rows failed"))}}}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadViewerAllianceID(context.Background(), "ogame_users", 42); err == nil || !strings.Contains(err.Error(), "own alliance empty rows failed") {
		t.Fatalf("expected own alliance empty rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{7, "TAG", "Alliance", "bad", int64(1), 0})}}}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadAllianceSearchRows(context.Background(), "ogame_ally", "ogame_users", 42, "TA", "tag"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected alliance scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("ally rows failed"), []any{7, "TAG", "Alliance", 1, int64(1), 0})}}}
	repository = NewSearchRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadAllianceSearchRows(context.Background(), "ogame_ally", "ogame_users", 42, "TA", "tag"); err == nil || !strings.Contains(err.Error(), "ally rows failed") {
		t.Fatalf("expected alliance rows error, got %v", err)
	}
}
