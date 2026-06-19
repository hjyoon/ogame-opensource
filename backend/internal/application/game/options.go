package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type OptionsRepository interface {
	GetOptions(context.Context, OptionsQuery) (domaingame.Options, error)
	UpdateOptions(context.Context, OptionsUpdateQuery) (domaingame.Options, *domaingame.OptionsActionIssue, error)
}

type OptionsQuery struct {
	PlayerID int
	PlanetID int
}

type OptionsCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type OptionsUpdateQuery struct {
	PlayerID int
	PlanetID int
	Mutation domaingame.OptionsMutation
}

type OptionsUpdateCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Mutation        domaingame.OptionsMutation
}

type OptionsResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Options       domaingame.Options
	ActionIssue   *domaingame.OptionsActionIssue
}

type OptionsService struct {
	sessions   SessionLookup
	repository OptionsRepository
}

func NewOptionsService(sessions SessionLookup, repository OptionsRepository) OptionsService {
	return OptionsService{sessions: sessions, repository: repository}
}

func (s OptionsService) GetOptions(ctx context.Context, command OptionsCommand) (OptionsResult, error) {
	if s.sessions == nil || s.repository == nil {
		return OptionsResult{}, errors.New("options dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return OptionsResult{}, err
	}
	if !session.Authenticated {
		return OptionsResult{Authenticated: false, Issues: session.Issues}, nil
	}

	options, err := s.repository.GetOptions(ctx, OptionsQuery{PlayerID: session.Session.PlayerID, PlanetID: command.PlanetID})
	if err != nil {
		return OptionsResult{}, err
	}
	return OptionsResult{Authenticated: true, Options: options}, nil
}

func (s OptionsService) UpdateOptions(ctx context.Context, command OptionsUpdateCommand) (OptionsResult, error) {
	if s.sessions == nil || s.repository == nil {
		return OptionsResult{}, errors.New("options dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return OptionsResult{}, err
	}
	if !session.Authenticated {
		return OptionsResult{Authenticated: false, Issues: session.Issues}, nil
	}

	options, issue, err := s.repository.UpdateOptions(ctx, OptionsUpdateQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Mutation: command.Mutation,
	})
	if err != nil {
		return OptionsResult{}, err
	}
	return OptionsResult{Authenticated: true, Options: options, ActionIssue: issue}, nil
}
