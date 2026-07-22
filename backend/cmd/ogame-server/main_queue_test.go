package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/hjyoon/ogame-opensource/backend/internal/config"
)

func TestDueQueueWorkerStaysDisabledWithoutUniverseDatabase(t *testing.T) {
	done := startDueQueueWorker(context.Background(), config.Config{QueuePollIntervalMS: 1000}, slog.Default(), databasePools{})
	select {
	case <-done:
	default:
		t.Fatal("disabled queue worker did not stop")
	}
}
