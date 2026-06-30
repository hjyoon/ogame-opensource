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

func TestStatisticsRepositoryReadsLegacyPlayerRankings(t *testing.T) {
	now := time.Unix(123456, 0)
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{250})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{7, 101})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{42, "legor", 7, "TAG", 1, 2, 3, int64(950000000), 101, 120, int64(123400)},
			[]any{43, "allymate", 7, "TAG", 1, 2, 4, int64(900000000), 102, 102, int64(123401)},
		)},
	)}
	repository := NewStatisticsRepositoryWithQueryer(queryer, "ogame_", func() time.Time { return now })

	statistics, err := repository.GetStatistics(context.Background(), appgame.StatisticsQuery{
		PlayerID: 42,
		Type:     "ressources",
		Start:    -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if statistics.Commander != "legor" || statistics.Start != 101 || statistics.Total != 250 || statistics.GeneratedAt != now.Unix() {
		t.Fatalf("unexpected statistics summary: %+v", statistics)
	}
	if len(statistics.Rows) != 2 || !statistics.Rows[0].Own || statistics.Rows[0].DisplayScore(statistics.Type) != 950000 {
		t.Fatalf("unexpected statistics rows: %+v", statistics.Rows)
	}
	if statistics.Rows[0].Alliance == nil || statistics.Rows[0].Alliance.Tag != "TAG" ||
		statistics.Rows[0].SameAlliance || !statistics.Rows[1].SameAlliance || statistics.Rows[1].Alliance == nil {
		t.Fatalf("unexpected alliance mapping: %+v", statistics.Rows)
	}
	if !strings.Contains(queryer.calls[6].sql, "u.score1, u.place1, u.oldplace1") ||
		!strings.Contains(queryer.calls[6].sql, "LEFT JOIN `ogame_ally`") ||
		queryer.calls[6].args[0] != 101 ||
		queryer.calls[6].args[1] != 200 {
		t.Fatalf("expected legacy player statistics query, got %+v", queryer.calls[6])
	}
}

func TestStatisticsRepositoryUsesFleetAndResearchColumns(t *testing.T) {
	tests := []struct {
		name      string
		statType  string
		wantQuery string
	}{
		{name: "fleet", statType: "fleet", wantQuery: "u.score2, u.place2, u.oldplace2"},
		{name: "research", statType: "research", wantQuery: "u.score3, u.place3, u.oldplace3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{2})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{0, 1})},
				fakeQueryResult{rows: fakeRowsFromValues()},
			)}
			repository := NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)

			if _, err := repository.GetStatistics(context.Background(), appgame.StatisticsQuery{PlayerID: 42, Type: tt.statType, Start: 1}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(queryer.calls[6].sql, tt.wantQuery) {
				t.Fatalf("expected %q in query, got %q", tt.wantQuery, queryer.calls[6].sql)
			}
		})
	}
}

func TestStatisticsRepositoryReadsLegacyAllianceRankings(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{2})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{7, 1})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{7, "TAG", int64(950000000), 1, 3, int64(123400), 3},
			[]any{8, "RIV", int64(900000000), 2, 2, int64(123401), 2},
		)},
	)}
	repository := NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)

	statistics, err := repository.GetStatistics(context.Background(), appgame.StatisticsQuery{
		PlayerID: 42,
		Who:      "ally",
		Type:     "ressources",
		Start:    -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if statistics.Who != domaingame.StatisticsWhoAlly || statistics.Start != 1 || statistics.Total != 2 || len(statistics.Rows) != 2 {
		t.Fatalf("unexpected alliance statistics summary: %+v", statistics)
	}
	if statistics.Rows[0].Alliance == nil || statistics.Rows[0].Alliance.Tag != "TAG" || !statistics.Rows[0].Own ||
		statistics.Rows[0].Members != 3 || statistics.Rows[0].DisplayScorePerMember(statistics.Type) != 316667 {
		t.Fatalf("unexpected alliance statistics rows: %+v", statistics.Rows)
	}
	if !strings.Contains(queryer.calls[6].sql, "a.score1, a.place1, a.oldplace1") ||
		!strings.Contains(queryer.calls[6].sql, "LEFT JOIN `ogame_users`") ||
		queryer.calls[6].args[0] != 1 ||
		queryer.calls[6].args[1] != 100 {
		t.Fatalf("expected legacy alliance statistics query, got %+v", queryer.calls[6])
	}
}

func TestNewStatisticsRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewStatisticsRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	withDefaultClock := NewStatisticsRepositoryWithQueryer(&fakeQueryer{}, "ogame_", nil)
	if withDefaultClock.now == nil {
		t.Fatal("expected default clock")
	}
}

func TestStatisticsRepositoryReturnsErrors(t *testing.T) {
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
			name:    "total",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("total failed")})},
			want:    "total failed",
		},
		{
			name:   "own place",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{2})},
				fakeQueryResult{err: errors.New("place failed")},
			)},
			want: "place failed",
		},
		{
			name:   "rows",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{2})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{0, 1})},
				fakeQueryResult{err: errors.New("rows failed")},
			)},
			want: "rows failed",
		},
		{
			name:    "alliance total",
			prefix:  "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(), fakeQueryResult{err: errors.New("ally total failed")})},
			want:    "ally total failed",
		},
		{
			name:   "alliance own place",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{2})},
				fakeQueryResult{err: errors.New("ally place failed")},
			)},
			want: "ally place failed",
		},
		{
			name:   "alliance rows",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: append(shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{2})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{7, 1})},
				fakeQueryResult{err: errors.New("ally rows failed")},
			)},
			want: "ally rows failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewStatisticsRepositoryWithQueryer(tt.queryer, tt.prefix, time.Now)
			who := ""
			if strings.HasPrefix(tt.name, "alliance") {
				who = "ally"
			}
			_, err := repository.GetStatistics(context.Background(), appgame.StatisticsQuery{PlayerID: 42, Who: who})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestStatisticsRepositoryLoadersHandleRowsAndScanEdges(t *testing.T) {
	repository := NewStatisticsRepositoryWithQueryer(&fakeQueryer{}, "ogame_", time.Now)
	if _, err := repository.loadStatisticsTotal(context.Background(), "ogame_uni"); err == nil {
		t.Fatal("expected total query error")
	}

	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if total, err := repository.loadStatisticsTotal(context.Background(), "ogame_uni"); err != nil || total != 0 {
		t.Fatalf("empty total should return zero, got total=%d err=%v", total, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("total empty rows failed"))}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadStatisticsTotal(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "total empty rows failed") {
		t.Fatalf("expected empty total rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadStatisticsTotal(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected total scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("total rows failed"), []any{1})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadStatisticsTotal(context.Background(), "ogame_uni"); err == nil || !strings.Contains(err.Error(), "total rows failed") {
		t.Fatalf("expected total rows error, got %v", err)
	}

	repository = NewStatisticsRepositoryWithQueryer(&fakeQueryer{}, "ogame_", time.Now)
	if _, err := repository.loadAllianceStatisticsTotal(context.Background(), "ogame_ally"); err == nil {
		t.Fatal("expected alliance total query error")
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if total, err := repository.loadAllianceStatisticsTotal(context.Background(), "ogame_ally"); err != nil || total != 0 {
		t.Fatalf("empty alliance total should return zero, got total=%d err=%v", total, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("alliance total empty rows failed"))}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadAllianceStatisticsTotal(context.Background(), "ogame_ally"); err == nil || !strings.Contains(err.Error(), "alliance total empty rows failed") {
		t.Fatalf("expected empty alliance total rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadAllianceStatisticsTotal(context.Background(), "ogame_ally"); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected alliance total scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("alliance total rows failed"), []any{1})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadAllianceStatisticsTotal(context.Background(), "ogame_ally"); err == nil || !strings.Contains(err.Error(), "alliance total rows failed") {
		t.Fatalf("expected alliance total rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 1})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadOwnStatisticsPlace(context.Background(), "ogame_users", 42, domaingame.StatisticsTypeResources); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected place scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if place, err := repository.loadOwnStatisticsPlace(context.Background(), "ogame_users", 42, domaingame.StatisticsTypeResources); err != nil || place != 0 {
		t.Fatalf("empty own place should return zero, got place=%d err=%v", place, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("place empty rows failed"))}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadOwnStatisticsPlace(context.Background(), "ogame_users", 42, domaingame.StatisticsTypeResources); err == nil || !strings.Contains(err.Error(), "place empty rows failed") {
		t.Fatalf("expected empty place rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("place rows failed"), []any{0, 1})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadOwnStatisticsPlace(context.Background(), "ogame_users", 42, domaingame.StatisticsTypeResources); err == nil || !strings.Contains(err.Error(), "place rows failed") {
		t.Fatalf("expected place rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if allyID, place, err := repository.loadOwnAllianceStatistics(context.Background(), "ogame_users", "ogame_ally", 42, domaingame.StatisticsTypeResources); err != nil || allyID != 0 || place != 0 {
		t.Fatalf("empty own alliance should return zeroes, got ally=%d place=%d err=%v", allyID, place, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("own alliance empty rows failed"))}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, _, err := repository.loadOwnAllianceStatistics(context.Background(), "ogame_users", "ogame_ally", 42, domaingame.StatisticsTypeResources); err == nil || !strings.Contains(err.Error(), "own alliance empty rows failed") {
		t.Fatalf("expected empty own alliance rows error, got %v", err)
	}

	repository = NewStatisticsRepositoryWithQueryer(&fakeQueryer{}, "ogame_", time.Now)
	if _, _, err := repository.loadOwnAllianceStatistics(context.Background(), "ogame_users", "ogame_ally", 42, domaingame.StatisticsTypeResources); err == nil {
		t.Fatal("expected own alliance query error")
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 1})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, _, err := repository.loadOwnAllianceStatistics(context.Background(), "ogame_users", "ogame_ally", 42, domaingame.StatisticsTypeResources); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected own alliance scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("own alliance rows failed"), []any{7, 1})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, _, err := repository.loadOwnAllianceStatistics(context.Background(), "ogame_users", "ogame_ally", 42, domaingame.StatisticsTypeResources); err == nil || !strings.Contains(err.Error(), "own alliance rows failed") {
		t.Fatalf("expected own alliance rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "legor", 0, "", 1, 2, 3, "bad", 1, 1, int64(0)})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadPlayerStatisticsRows(context.Background(), "ogame_users", "ogame_planets", "ogame_ally", 42, 0, domaingame.StatisticsTypeResources, 1); err == nil || !strings.Contains(err.Error(), "expected int64") {
		t.Fatalf("expected player row scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("rows failed"), []any{42, "legor", 0, "", 1, 2, 3, int64(1), 1, 1, int64(0)})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadPlayerStatisticsRows(context.Background(), "ogame_users", "ogame_planets", "ogame_ally", 42, 0, domaingame.StatisticsTypeResources, 1); err == nil || !strings.Contains(err.Error(), "rows failed") {
		t.Fatalf("expected rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{7, "TAG", "bad", 1, 1, int64(0), 3})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadAllianceStatisticsRows(context.Background(), "ogame_ally", "ogame_users", 7, domaingame.StatisticsTypeResources, 1); err == nil || !strings.Contains(err.Error(), "expected int64") {
		t.Fatalf("expected alliance row scan error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("ally rows failed"), []any{7, "TAG", int64(1), 1, 1, int64(0), 3})}}}
	repository = NewStatisticsRepositoryWithQueryer(queryer, "ogame_", time.Now)
	if _, err := repository.loadAllianceStatisticsRows(context.Background(), "ogame_ally", "ogame_users", 7, domaingame.StatisticsTypeResources, 1); err == nil || !strings.Contains(err.Error(), "ally rows failed") {
		t.Fatalf("expected alliance rows error, got %v", err)
	}
}
