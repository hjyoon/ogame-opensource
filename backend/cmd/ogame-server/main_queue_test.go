package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlgame"
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

func TestRecordDueQueueSettlementDistinguishesBusyAndFailedTicks(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tracker := infraruntime.NewQueueWorkerHealthTracker(true, time.Second)
	now := time.Now()

	recordDueQueueSettlement(logger, tracker, 1, 1000, now, nil)
	recordDueQueueSettlement(logger, tracker, 1, 1000, now.Add(time.Second), mysqlgame.ErrQueueSettlementBusy)
	status := tracker.Status()
	if !status.Ready || status.ConsecutiveFailures != 0 || status.LastSuccessAt != now.Unix() || status.LastAttemptAt != now.Add(time.Second).Unix() {
		t.Fatalf("busy tick should preserve successful worker health: %+v", status)
	}
	if logged := output.String(); !strings.Contains(logged, `"event":"queue_settlement_deferred"`) || !strings.Contains(logged, `"reason":"database_lock_busy"`) {
		t.Fatalf("missing structured busy log: %s", logged)
	}

	recordDueQueueSettlement(logger, tracker, 1, 1000, now.Add(2*time.Second), errors.New("settlement failed"))
	if status = tracker.Status(); status.ConsecutiveFailures != 1 {
		t.Fatalf("real settlement failure was not recorded: %+v", status)
	}
	if logged := output.String(); !strings.Contains(logged, `"event":"queue_settlement_failed"`) || !strings.Contains(logged, `"error":"settlement failed"`) {
		t.Fatalf("missing structured failure log: %s", logged)
	}
}
