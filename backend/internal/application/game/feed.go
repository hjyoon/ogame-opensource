package game

import (
	"context"
	"errors"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type FeedRepository interface {
	GetFeed(context.Context, FeedQuery) (domaingame.Feed, error)
	GetFeedItem(context.Context, FeedItemQuery) (domaingame.FeedItem, error)
}

type FeedQuery struct {
	FeedID string
}

type FeedItemQuery struct {
	FeedID    string
	MessageID int
}

type FeedService struct {
	repository FeedRepository
}

func NewFeedService(repository FeedRepository) FeedService {
	return FeedService{repository: repository}
}

func (s FeedService) GetFeed(ctx context.Context, query FeedQuery) (domaingame.Feed, error) {
	if s.repository == nil {
		return domaingame.Feed{}, errors.New("feed dependencies unavailable")
	}
	if query.FeedID == "" {
		return domaingame.Feed{}, nil
	}
	return s.repository.GetFeed(ctx, query)
}

func (s FeedService) GetFeedItem(ctx context.Context, query FeedItemQuery) (domaingame.FeedItem, error) {
	if s.repository == nil {
		return domaingame.FeedItem{}, errors.New("feed dependencies unavailable")
	}
	if query.FeedID == "" || query.MessageID <= 0 {
		return domaingame.FeedItem{}, nil
	}
	return s.repository.GetFeedItem(ctx, query)
}
