package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type JumpGateRepository interface {
	GetJumpGate(context.Context, JumpGateQuery) (domaingame.JumpGate, error)
	Jump(context.Context, JumpGateMutationQuery) (domaingame.JumpGate, error)
}

type JumpGateQuery struct {
	PlayerID int
	PlanetID int
}

type JumpGateMutationQuery struct {
	PlayerID     int
	PlanetID     int
	SourceMoonID int
	TargetMoonID int
	Ships        map[int]int
}

type JumpGateCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type JumpGateMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	SourceMoonID    int
	TargetMoonID    int
	Ships           map[int]int
}

type JumpGateResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	JumpGate      domaingame.JumpGate
}

type JumpGateService struct {
	sessions   SessionLookup
	repository JumpGateRepository
}

func NewJumpGateService(sessions SessionLookup, repository JumpGateRepository) JumpGateService {
	return JumpGateService{sessions: sessions, repository: repository}
}

func (s JumpGateService) GetJumpGate(ctx context.Context, command JumpGateCommand) (JumpGateResult, error) {
	if s.sessions == nil || s.repository == nil {
		return JumpGateResult{}, errors.New("jump gate dependencies unavailable")
	}
	session, err := s.lookupSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return JumpGateResult{}, err
	}
	if !session.Authenticated {
		return JumpGateResult{Authenticated: false, Issues: session.Issues}, nil
	}
	jumpGate, err := s.repository.GetJumpGate(ctx, JumpGateQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
	})
	if err != nil {
		return JumpGateResult{}, err
	}
	return JumpGateResult{Authenticated: true, JumpGate: jumpGate}, nil
}

func (s JumpGateService) Jump(ctx context.Context, command JumpGateMutationCommand) (JumpGateResult, error) {
	if s.sessions == nil || s.repository == nil {
		return JumpGateResult{}, errors.New("jump gate dependencies unavailable")
	}
	session, err := s.lookupSession(ctx, command.PublicSession, command.PrivateSessions, command.RemoteAddr)
	if err != nil {
		return JumpGateResult{}, err
	}
	if !session.Authenticated {
		return JumpGateResult{Authenticated: false, Issues: session.Issues}, nil
	}
	jumpGate, err := s.repository.Jump(ctx, JumpGateMutationQuery{
		PlayerID:     session.Session.PlayerID,
		PlanetID:     command.PlanetID,
		SourceMoonID: command.SourceMoonID,
		TargetMoonID: command.TargetMoonID,
		Ships:        command.Ships,
	})
	if err != nil {
		return JumpGateResult{}, err
	}
	return JumpGateResult{Authenticated: true, JumpGate: jumpGate}, nil
}

func (s JumpGateService) lookupSession(ctx context.Context, publicSession string, privateSessions map[string]string, remoteAddr string) (domainpublicsite.SessionAuthentication, error) {
	return s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   publicSession,
		PrivateSessions: privateSessions,
		RemoteAddr:      remoteAddr,
	})
}
