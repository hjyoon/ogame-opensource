package runtime

import (
	"testing"
	"time"
)

func TestQueueWorkerHealthTrackerAllowsTransientFailuresAndDetectsStaleness(t *testing.T) {
	now := time.Unix(1_700, 0)
	tracker := NewQueueWorkerHealthTracker(true, time.Second)
	tracker.now = func() time.Time { return now }
	if status := tracker.Status(); status.Ready || !status.Enabled {
		t.Fatalf("expected required worker to wait for its first success: %+v", status)
	}

	tracker.RecordSuccess(now)
	tracker.RecordFailure(now.Add(time.Second))
	if status := tracker.Status(); !status.Ready || status.ConsecutiveFailures != 1 || status.LastSuccessAt != now.Unix() {
		t.Fatalf("expected a recent success to tolerate a transient failure: %+v", status)
	}

	now = now.Add(31 * time.Second)
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
