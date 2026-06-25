package game

import (
	"context"
	"errors"
	"testing"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type fakeFeedRepository struct {
	feedQuery     FeedQuery
	itemQuery     FeedItemQuery
	feed          domaingame.Feed
	item          domaingame.FeedItem
	err           error
	feedCalls     int
	feedItemCalls int
}

func (f *fakeFeedRepository) GetFeed(_ context.Context, query FeedQuery) (domaingame.Feed, error) {
	f.feedCalls++
	f.feedQuery = query
	return f.feed, f.err
}

func (f *fakeFeedRepository) GetFeedItem(_ context.Context, query FeedItemQuery) (domaingame.FeedItem, error) {
	f.feedItemCalls++
	f.itemQuery = query
	return f.item, f.err
}

func TestFeedServiceDelegatesQueries(t *testing.T) {
	repository := &fakeFeedRepository{
		feed: domaingame.Feed{FeedID: "abc"},
		item: domaingame.FeedItem{Subject: "Subject", Text: "Text"},
	}
	service := NewFeedService(repository)

	feed, err := service.GetFeed(context.Background(), FeedQuery{FeedID: "abc"})
	if err != nil || feed.FeedID != "abc" || repository.feedQuery.FeedID != "abc" {
		t.Fatalf("unexpected feed result: feed=%+v query=%+v err=%v", feed, repository.feedQuery, err)
	}
	item, err := service.GetFeedItem(context.Background(), FeedItemQuery{FeedID: "abc", MessageID: 7})
	if err != nil || item.Subject != "Subject" || repository.itemQuery.MessageID != 7 {
		t.Fatalf("unexpected feed item result: item=%+v query=%+v err=%v", item, repository.itemQuery, err)
	}
}

func TestFeedServiceSkipsInvalidQueries(t *testing.T) {
	repository := &fakeFeedRepository{}
	service := NewFeedService(repository)

	if feed, err := service.GetFeed(context.Background(), FeedQuery{}); err != nil || feed.FeedID != "" {
		t.Fatalf("empty feed id should return zero feed, got %+v err=%v", feed, err)
	}
	if item, err := service.GetFeedItem(context.Background(), FeedItemQuery{FeedID: "abc"}); err != nil || item.Subject != "" {
		t.Fatalf("invalid item query should return zero item, got %+v err=%v", item, err)
	}
	if repository.feedCalls != 0 || repository.feedItemCalls != 0 {
		t.Fatalf("invalid queries should not call repository: %+v", repository)
	}
}

func TestFeedServiceReportsMissingDependencies(t *testing.T) {
	service := NewFeedService(nil)
	if _, err := service.GetFeed(context.Background(), FeedQuery{FeedID: "abc"}); err == nil {
		t.Fatalf("expected missing dependency error")
	}
	if _, err := service.GetFeedItem(context.Background(), FeedItemQuery{FeedID: "abc", MessageID: 1}); err == nil {
		t.Fatalf("expected missing dependency error")
	}
}

func TestFeedServicePropagatesRepositoryErrors(t *testing.T) {
	repository := &fakeFeedRepository{err: errors.New("feed failed")}
	service := NewFeedService(repository)
	if _, err := service.GetFeed(context.Background(), FeedQuery{FeedID: "abc"}); err == nil {
		t.Fatalf("expected feed error")
	}
	if _, err := service.GetFeedItem(context.Background(), FeedItemQuery{FeedID: "abc", MessageID: 1}); err == nil {
		t.Fatalf("expected feed item error")
	}
}
