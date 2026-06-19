package game

import (
	"context"
	"errors"
	"strings"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type ReportRepository interface {
	GetReport(context.Context, ReportQuery) (domaingame.Report, error)
}

type ReportQuery struct {
	PlayerID      int
	ReportID      int
	PublicSession string
}

type ReportCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	ReportID        int
}

type ReportResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Report        domaingame.Report
}

type ReportService struct {
	sessions   SessionLookup
	repository ReportRepository
}

func NewReportService(sessions SessionLookup, repository ReportRepository) ReportService {
	return ReportService{sessions: sessions, repository: repository}
}

func (s ReportService) GetReport(ctx context.Context, command ReportCommand) (ReportResult, error) {
	if s.sessions == nil || s.repository == nil {
		return ReportResult{}, errors.New("report dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return ReportResult{}, err
	}
	if !session.Authenticated {
		return ReportResult{Authenticated: false, Issues: session.Issues}, nil
	}

	report, err := s.repository.GetReport(ctx, ReportQuery{
		PlayerID:      session.Session.PlayerID,
		ReportID:      command.ReportID,
		PublicSession: command.PublicSession,
	})
	if err != nil {
		return ReportResult{}, err
	}
	if command.PublicSession != "" {
		report.Text = strings.ReplaceAll(report.Text, "{PUBLIC_SESSION}", command.PublicSession)
	}
	return ReportResult{Authenticated: true, Report: report}, nil
}
