package runtime

import (
	"testing"
	"time"
)

func TestQueueWorkerHealthTrackerAllowsTransientFailuresAndDetectsStaleness(t *testing.T) {
	now := time.Unix(1_700, 0)
	clock := now
	tracker := NewQueueWorkerHealthTracker(true, time.Second)
	tracker.now = func() time.Time { return clock }
	if status := tracker.Status(); status.Ready || !status.Enabled {
		t.Fatalf("expected required worker to wait for its first success: %+v", status)
	}

	tracker.RecordSuccess(now)
	clock = now.Add(time.Second)
	tracker.RecordDeferred(clock)
	if status := tracker.Status(); !status.Ready || status.LastAttemptAt != clock.Unix() || status.LastSuccessAt != now.Unix() || status.ConsecutiveFailures != 0 {
		t.Fatalf("expected a busy deferral to preserve the last successful settlement: %+v", status)
	}
	clock = now.Add(2 * time.Second)
	tracker.RecordFailure(clock)
	if status := tracker.Status(); !status.Ready || status.ConsecutiveFailures != 1 || status.LastSuccessAt != now.Unix() {
		t.Fatalf("expected a recent success to tolerate a transient failure: %+v", status)
	}

	clock = now.Add(31 * time.Second)
	if status := tracker.Status(); status.Ready || status.LagSeconds != 31 {
		t.Fatalf("expected stale worker health: %+v", status)
	}
}

func TestQueueWorkerHealthTrackerTreatsOptionalWorkerAsReady(t *testing.T) {
	tracker := NewQueueWorkerHealthTracker(false, 0)
	if status := tracker.Status(); !status.Ready || status.Enabled {
		t.Fatalf("unexpected optional worker status: %+v", status)
	}
}
