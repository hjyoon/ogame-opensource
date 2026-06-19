package game

import (
	"context"
	"errors"
	"strings"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type MessagesRepository interface {
	GetMessages(context.Context, MessagesQuery) (domaingame.Messages, error)
}

type MessagesQuery struct {
	PlayerID       int
	PlanetID       int
	TargetPlayerID int
	PublicSession  string
}

type MessagesCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	TargetPlayerID  int
}

type MessagesResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Messages      domaingame.Messages
}

type MessagesService struct {
	sessions   SessionLookup
	repository MessagesRepository
}

func NewMessagesService(sessions SessionLookup, repository MessagesRepository) MessagesService {
	return MessagesService{sessions: sessions, repository: repository}
}

func (s MessagesService) GetMessages(ctx context.Context, command MessagesCommand) (MessagesResult, error) {
	if s.sessions == nil || s.repository == nil {
		return MessagesResult{}, errors.New("messages dependencies unavailable")
	}

	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return MessagesResult{}, err
	}
	if !session.Authenticated {
		return MessagesResult{Authenticated: false, Issues: session.Issues}, nil
	}

	messages, err := s.repository.GetMessages(ctx, MessagesQuery{
		PlayerID:       session.Session.PlayerID,
		PlanetID:       command.PlanetID,
		TargetPlayerID: command.TargetPlayerID,
		PublicSession:  command.PublicSession,
	})
	if err != nil {
		return MessagesResult{}, err
	}
	messages = replaceMessagePublicSession(messages, command.PublicSession)
	return MessagesResult{Authenticated: true, Messages: messages}, nil
}

func replaceMessagePublicSession(messages domaingame.Messages, publicSession string) domaingame.Messages {
	if publicSession == "" {
		return messages
	}
	for index := range messages.Rows {
		messages.Rows[index].From = strings.ReplaceAll(messages.Rows[index].From, "{PUBLIC_SESSION}", publicSession)
		messages.Rows[index].Subject = strings.ReplaceAll(messages.Rows[index].Subject, "{PUBLIC_SESSION}", publicSession)
		messages.Rows[index].Text = strings.ReplaceAll(messages.Rows[index].Text, "{PUBLIC_SESSION}", publicSession)
	}
	return messages
}
