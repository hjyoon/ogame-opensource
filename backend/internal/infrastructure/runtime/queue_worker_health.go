package runtime

import (
	"sync"
	"time"

	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

const minimumQueueWorkerStaleAfter = 30 * time.Second

type QueueWorkerHealthTracker struct {
	mu                  sync.RWMutex
	required            bool
	enabled             bool
	interval            time.Duration
	now                 func() time.Time
	lastAttempt         time.Time
	lastSuccess         time.Time
	consecutiveFailures int
}

func NewQueueWorkerHealthTracker(required bool, interval time.Duration) *QueueWorkerHealthTracker {
	return &QueueWorkerHealthTracker{
		required: required,
		enabled:  required && interval > 0,
		interval: interval,
		now:      time.Now,
	}
}

func (t *QueueWorkerHealthTracker) RecordSuccess(at time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.lastAttempt = at
	t.lastSuccess = at
	t.consecutiveFailures = 0
	t.mu.Unlock()
}

func (t *QueueWorkerHealthTracker) RecordFailure(at time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.lastAttempt = at
	t.consecutiveFailures++
	t.mu.Unlock()
}

func (t *QueueWorkerHealthTracker) Status() domainsystem.QueueWorkerHealth {
	if t == nil {
		return domainsystem.QueueWorkerHealth{Ready: true}
	}
	t.mu.RLock()
	required := t.required
	enabled := t.enabled
	interval := t.interval
	now := t.now()
	lastAttempt := t.lastAttempt
	lastSuccess := t.lastSuccess
	failures := t.consecutiveFailures
	t.mu.RUnlock()

	staleAfter := 3 * interval
	if staleAfter < minimumQueueWorkerStaleAfter {
		staleAfter = minimumQueueWorkerStaleAfter
	}
	lag := int64(0)
	if !lastSuccess.IsZero() {
		lag = max(0, int64(now.Sub(lastSuccess).Seconds()))
	}
	ready := !required || enabled && !lastSuccess.IsZero() && now.Sub(lastSuccess) <= staleAfter
	return domainsystem.QueueWorkerHealth{
		Enabled:             enabled,
		Ready:               ready,
		IntervalMS:          int(interval / time.Millisecond),
		LastAttemptAt:       unixTime(lastAttempt),
		LastSuccessAt:       unixTime(lastSuccess),
		LagSeconds:          lag,
		ConsecutiveFailures: failures,
	}
}

func unixTime(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}
