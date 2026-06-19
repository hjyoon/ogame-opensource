package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

func TestReportServiceReturnsReportForAuthenticatedSession(t *testing.T) {
	sessions := &fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}
	repository := &fakeReportRepository{report: domaingame.NewReport(11, domaingame.MessageTypeSpyReport, "body {PUBLIC_SESSION}", true)}
	service := NewReportService(sessions, repository)

	result, err := service.GetReport(context.Background(), ReportCommand{
		PublicSession:   "public",
		PrivateSessions: map[string]string{"private": "token"},
		RemoteAddr:      "203.0.113.9",
		ReportID:        11,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authenticated || result.Report.Text != "body public" || result.Report.Title != domaingame.ReportTitleSpy {
		t.Fatalf("unexpected report result: %+v", result)
	}
	if repository.query.PlayerID != 42 || repository.query.ReportID != 11 || sessions.command.RemoteAddr != "203.0.113.9" {
		t.Fatalf("unexpected query/session: query=%+v session=%+v", repository.query, sessions.command)
	}
}

func TestReportServiceReturnsUnauthenticatedSessionIssues(t *testing.T) {
	issue := domainpublicsite.SessionIssue{Code: "missing", Message: "missing session"}
	service := NewReportService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{Issues: []domainpublicsite.SessionIssue{issue}}}, &fakeReportRepository{})

	result, err := service.GetReport(context.Background(), ReportCommand{PublicSession: "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticated || len(result.Issues) != 1 || result.Issues[0].Code != "missing" {
		t.Fatalf("expected unauthenticated issue, got %+v", result)
	}
}

func TestReportServiceReturnsDependencyAndRepositoryErrors(t *testing.T) {
	if _, err := (ReportService{}).GetReport(context.Background(), ReportCommand{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if _, err := NewReportService(&fakeSessionLookup{err: errors.New("session failed")}, &fakeReportRepository{}).GetReport(context.Background(), ReportCommand{}); err == nil || !strings.Contains(err.Error(), "session failed") {
		t.Fatalf("expected session error, got %v", err)
	}
	if _, err := NewReportService(&fakeSessionLookup{result: domainpublicsite.SessionAuthentication{
		Authenticated: true,
		Session:       domainpublicsite.GameSession{PlayerID: 42},
	}}, &fakeReportRepository{err: errors.New("report failed")}).GetReport(context.Background(), ReportCommand{}); err == nil || !strings.Contains(err.Error(), "report failed") {
		t.Fatalf("expected repository error, got %v", err)
	}
}

type fakeReportRepository struct {
	report domaingame.Report
	query  ReportQuery
	err    error
}

func (f *fakeReportRepository) GetReport(_ context.Context, query ReportQuery) (domaingame.Report, error) {
	f.query = query
	if f.err != nil {
		return domaingame.Report{}, f.err
	}
	return f.report, nil
}
