package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type GalaxyRepository interface {
	GetGalaxy(context.Context, GalaxyQuery) (domaingame.Galaxy, error)
	LaunchMissiles(context.Context, GalaxyMissileLaunchQuery) (*domaingame.GalaxyActionIssue, error)
	DispatchInstantFleet(context.Context, GalaxyInstantDispatchQuery) (*domaingame.GalaxyActionIssue, error)
}

type GalaxyQuery struct {
	PlayerID    int
	PlanetID    int
	Coordinates domaingame.Coordinates
}

type GalaxyCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Coordinates     domaingame.Coordinates
}

type GalaxyMissileLaunchQuery struct {
	PlayerID        int
	PlanetID        int
	Coordinates     domaingame.Coordinates
	TargetPlanetID  int
	Amount          int
	TargetDefenseID int
}

type GalaxyMissileLaunchCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Coordinates     domaingame.Coordinates
	TargetPlanetID  int
	Amount          int
	TargetDefenseID int
}

type GalaxyInstantDispatchQuery struct {
	PlayerID    int
	PlanetID    int
	Coordinates domaingame.Coordinates
	Target      domaingame.Coordinates
	TargetType  int
	Mission     int
	Amount      int
}

type GalaxyInstantDispatchCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Coordinates     domaingame.Coordinates
	Target          domaingame.Coordinates
	TargetType      int
	Mission         int
	Amount          int
}

type GalaxyResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Galaxy        domaingame.Galaxy
	ActionIssue   *domaingame.GalaxyActionIssue
}

type GalaxyService struct {
	sessions   SessionLookup
	repository GalaxyRepository
}

func NewGalaxyService(sessions SessionLookup, repository GalaxyRepository) GalaxyService {
	return GalaxyService{sessions: sessions, repository: repository}
}

func (s GalaxyService) GetGalaxy(ctx context.Context, command GalaxyCommand) (GalaxyResult, error) {
	if s.sessions == nil || s.repository == nil {
		return GalaxyResult{}, errors.New("galaxy dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	if !session.Authenticated {
		return GalaxyResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	galaxy, err := s.repository.GetGalaxy(ctx, GalaxyQuery{
		PlayerID:    session.Session.PlayerID,
		PlanetID:    command.PlanetID,
		Coordinates: command.Coordinates,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	return GalaxyResult{
		Authenticated: true,
		Galaxy:        galaxy,
	}, nil
}

func (s GalaxyService) DispatchInstantFleet(ctx context.Context, command GalaxyInstantDispatchCommand) (GalaxyResult, error) {
	if s.sessions == nil || s.repository == nil {
		return GalaxyResult{}, errors.New("galaxy dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	if !session.Authenticated {
		return GalaxyResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	issue, err := s.repository.DispatchInstantFleet(ctx, GalaxyInstantDispatchQuery{
		PlayerID:    session.Session.PlayerID,
		PlanetID:    command.PlanetID,
		Coordinates: command.Coordinates,
		Target:      command.Target,
		TargetType:  command.TargetType,
		Mission:     command.Mission,
		Amount:      command.Amount,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	galaxy, err := s.repository.GetGalaxy(ctx, GalaxyQuery{
		PlayerID:    session.Session.PlayerID,
		PlanetID:    command.PlanetID,
		Coordinates: command.Coordinates,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	return GalaxyResult{
		Authenticated: true,
		Galaxy:        galaxy,
		ActionIssue:   issue,
	}, nil
}

func (s GalaxyService) LaunchMissiles(ctx context.Context, command GalaxyMissileLaunchCommand) (GalaxyResult, error) {
	if s.sessions == nil || s.repository == nil {
		return GalaxyResult{}, errors.New("galaxy dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	if !session.Authenticated {
		return GalaxyResult{
			Authenticated: false,
			Issues:        session.Issues,
		}, nil
	}

	issue, err := s.repository.LaunchMissiles(ctx, GalaxyMissileLaunchQuery{
		PlayerID:        session.Session.PlayerID,
		PlanetID:        command.PlanetID,
		Coordinates:     command.Coordinates,
		TargetPlanetID:  command.TargetPlanetID,
		Amount:          command.Amount,
		TargetDefenseID: command.TargetDefenseID,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	galaxy, err := s.repository.GetGalaxy(ctx, GalaxyQuery{
		PlayerID:    session.Session.PlayerID,
		PlanetID:    command.PlanetID,
		Coordinates: command.Coordinates,
	})
	if err != nil {
		return GalaxyResult{}, err
	}
	return GalaxyResult{
		Authenticated: true,
		Galaxy:        galaxy,
		ActionIssue:   issue,
	}, nil
}
