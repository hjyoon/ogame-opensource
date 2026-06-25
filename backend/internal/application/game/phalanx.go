package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type PhalanxRepository interface {
	GetPhalanx(context.Context, PhalanxQuery) (domaingame.Phalanx, error)
}

type PhalanxQuery struct {
	PlayerID       int
	PlanetID       int
	TargetPlanetID int
}

type PhalanxCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	TargetPlanetID  int
}

type PhalanxResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Phalanx       domaingame.Phalanx
}

type PhalanxService struct {
	sessions   SessionLookup
	repository PhalanxRepository
}

func NewPhalanxService(sessions SessionLookup, repository PhalanxRepository) PhalanxService {
	return PhalanxService{sessions: sessions, repository: repository}
}

func (s PhalanxService) GetPhalanx(ctx context.Context, command PhalanxCommand) (PhalanxResult, error) {
	if s.sessions == nil || s.repository == nil {
		return PhalanxResult{}, errors.New("phalanx dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return PhalanxResult{}, err
	}
	if !session.Authenticated {
		return PhalanxResult{Authenticated: false, Issues: session.Issues}, nil
	}

	phalanx, err := s.repository.GetPhalanx(ctx, PhalanxQuery{
		PlayerID:       session.Session.PlayerID,
		PlanetID:       command.PlanetID,
		TargetPlanetID: command.TargetPlanetID,
	})
	if err != nil {
		return PhalanxResult{}, err
	}
	return PhalanxResult{Authenticated: true, Phalanx: phalanx}, nil
}
