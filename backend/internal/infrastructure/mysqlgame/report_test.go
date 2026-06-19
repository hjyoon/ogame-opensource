package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestReportRepositoryReadsOwnedReport(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, domaingame.MessageTypeBattleReportText, "<table>battle</table>", 0, 7})},
	}}
	repository := NewReportRepositoryWithQueryer(queryer, "ogame_")

	report, err := repository.GetReport(context.Background(), appgame.ReportQuery{PlayerID: 42, ReportID: 11})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Allowed || report.Title != domaingame.ReportTitleBattle || report.Text != "<table>battle</table>" {
		t.Fatalf("unexpected owned report: %+v", report)
	}
	if !strings.Contains(queryer.calls[0].sql, "FROM `ogame_messages`") || queryer.calls[0].args[0] != 42 || queryer.calls[0].args[1] != 11 {
		t.Fatalf("unexpected report query: %+v", queryer.calls[0])
	}
}

func TestReportRepositoryAllowsSameAllianceSpyReportOnly(t *testing.T) {
	repository := NewReportRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{77, domaingame.MessageTypeSpyReport, "spy", 5, 5})},
	}}, "ogame_")
	report, err := repository.GetReport(context.Background(), appgame.ReportQuery{PlayerID: 42, ReportID: 12})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Allowed || report.Text != "spy" || report.Title != domaingame.ReportTitleSpy {
		t.Fatalf("expected allied spy access, got %+v", report)
	}

	repository = NewReportRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{77, domaingame.MessageTypeBattleReportText, "battle", 5, 5})},
	}}, "ogame_")
	report, err = repository.GetReport(context.Background(), appgame.ReportQuery{PlayerID: 42, ReportID: 13})
	if err != nil {
		t.Fatal(err)
	}
	if report.Allowed || report.Text != "" {
		t.Fatalf("expected allied battle text to stay hidden, got %+v", report)
	}
}

func TestReportRepositoryReturnsEmptyDeniedReportForMissingID(t *testing.T) {
	repository := NewReportRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
	}}, "ogame_")

	report, err := repository.GetReport(context.Background(), appgame.ReportQuery{PlayerID: 42, ReportID: 99})
	if err != nil {
		t.Fatal(err)
	}
	if report.Allowed || report.ID != 99 || report.Text != "" {
		t.Fatalf("unexpected missing report: %+v", report)
	}
}

func TestReportRepositoryReturnsQueryAndScanErrors(t *testing.T) {
	repository := NewReportRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("report query failed")}}}, "ogame_")
	if _, err := repository.GetReport(context.Background(), appgame.ReportQuery{PlayerID: 42, ReportID: 11}); err == nil || !strings.Contains(err.Error(), "report query failed") {
		t.Fatalf("expected query error, got %v", err)
	}

	repository = NewReportRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"bad", domaingame.MessageTypeSpyReport, "spy", 5, 5})},
	}}, "ogame_")
	if _, err := repository.GetReport(context.Background(), appgame.ReportQuery{PlayerID: 42, ReportID: 11}); err == nil || !strings.Contains(err.Error(), "expected int") {
		t.Fatalf("expected scan error, got %v", err)
	}

	repository = NewReportRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValuesWithErr(errors.New("report rows failed"), []any{42, domaingame.MessageTypeSpyReport, "spy", 5, 5})},
	}}, "ogame_")
	if _, err := repository.GetReport(context.Background(), appgame.ReportQuery{PlayerID: 42, ReportID: 11}); err == nil || !strings.Contains(err.Error(), "report rows failed") {
		t.Fatalf("expected rows error, got %v", err)
	}
}
