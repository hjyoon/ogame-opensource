package main

import (
	"testing"
	"time"
)

func TestSetServerTimezone(t *testing.T) {
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	time.Local = time.FixedZone("test", 9*60*60)
	setServerTimezone()
	if time.Local != time.UTC {
		t.Fatalf("server timezone must be UTC, got %s", time.Local)
	}
}
