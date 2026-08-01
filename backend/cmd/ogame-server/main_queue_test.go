package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	infraruntime "github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/runtime"
)

func TestDueQueueWorkerStaysDisabledWithoutUniverseDatabase(t *testing.T) {
	tracker := infraruntime.NewQueueWorkerHealthTracker(true, time.Second)
	done := startDueQueueWorker(context.Background(), config.Config{UniDBEnabled: true, QueuePollIntervalMS: 1000}, slog.Default(), databasePools{}, tracker)
	select {
	case <-done:
	default:
		t.Fatal("disabled queue worker did not stop")
	}
	if status := tracker.Status(); status.Ready || status.ConsecutiveFailures != 1 {
		t.Fatalf("expected missing database to mark worker unavailable: %+v", status)
	}
}
