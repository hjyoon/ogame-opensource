package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type AdminRepository interface {
	GetAdmin(context.Context, AdminQuery) (domaingame.Admin, error)
}

type AdminQuery struct {
	PlayerID int
	PlanetID int
	Mode     string
}

type AdminCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Mode            string
}

type AdminResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Admin         domaingame.Admin
	ActionIssue   *domaingame.AdminActionIssue
}

type AdminService struct {
	sessions   SessionLookup
	repository AdminRepository
}

func NewAdminService(sessions SessionLookup, repository AdminRepository) AdminService {
	return AdminService{sessions: sessions, repository: repository}
}

func (s AdminService) GetAdmin(ctx context.Context, command AdminCommand) (AdminResult, error) {
	if s.sessions == nil || s.repository == nil {
		return AdminResult{}, errors.New("admin dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return AdminResult{}, err
	}
	if !session.Authenticated {
		return AdminResult{Authenticated: false, Issues: session.Issues}, nil
	}
	admin, err := s.repository.GetAdmin(ctx, AdminQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Mode:     command.Mode,
	})
	if err != nil {
		return AdminResult{}, err
	}
	var issue *domaingame.AdminActionIssue
	if !admin.CanAccess() {
		issue = domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)
	}
	return AdminResult{Authenticated: true, Admin: admin, ActionIssue: issue}, nil
}
