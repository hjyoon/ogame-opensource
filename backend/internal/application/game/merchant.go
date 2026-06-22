package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type MerchantRepository interface {
	GetMerchant(context.Context, MerchantQuery) (domaingame.Merchant, error)
	MutateMerchant(context.Context, MerchantMutationQuery) (domaingame.Merchant, *domaingame.MerchantActionIssue, error)
}

type MerchantQuery struct {
	PlayerID int
	PlanetID int
}

type MerchantCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
}

type MerchantMutationQuery struct {
	PlayerID int
	PlanetID int
	Mutation domaingame.MerchantMutation
}

type MerchantMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Mutation        domaingame.MerchantMutation
}

type MerchantResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Merchant      domaingame.Merchant
	ActionIssue   *domaingame.MerchantActionIssue
}

type MerchantService struct {
	sessions   SessionLookup
	repository MerchantRepository
}

func NewMerchantService(sessions SessionLookup, repository MerchantRepository) MerchantService {
	return MerchantService{sessions: sessions, repository: repository}
}

func (s MerchantService) GetMerchant(ctx context.Context, command MerchantCommand) (MerchantResult, error) {
	if s.sessions == nil || s.repository == nil {
		return MerchantResult{}, errors.New("merchant dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return MerchantResult{}, err
	}
	if !session.Authenticated {
		return MerchantResult{Authenticated: false, Issues: session.Issues}, nil
	}
	merchant, err := s.repository.GetMerchant(ctx, MerchantQuery{PlayerID: session.Session.PlayerID, PlanetID: command.PlanetID})
	if err != nil {
		return MerchantResult{}, err
	}
	return MerchantResult{Authenticated: true, Merchant: merchant}, nil
}

func (s MerchantService) MutateMerchant(ctx context.Context, command MerchantMutationCommand) (MerchantResult, error) {
	if s.sessions == nil || s.repository == nil {
		return MerchantResult{}, errors.New("merchant dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return MerchantResult{}, err
	}
	if !session.Authenticated {
		return MerchantResult{Authenticated: false, Issues: session.Issues}, nil
	}
	merchant, issue, err := s.repository.MutateMerchant(ctx, MerchantMutationQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Mutation: command.Mutation,
	})
	if err != nil {
		return MerchantResult{}, err
	}
	return MerchantResult{Authenticated: true, Merchant: merchant, ActionIssue: issue}, nil
}
