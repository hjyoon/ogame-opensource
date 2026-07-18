package main

import (
	"testing"
	"time"
)

func TestSetServerTimezone(t *testing.T) {
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	if err := setServerTimezone("Asia/Seoul"); err != nil {
		t.Fatalf("set valid server timezone: %v", err)
	}
	if time.Local.String() != "Asia/Seoul" {
		t.Fatalf("unexpected local timezone: %s", time.Local)
	}

	if err := setServerTimezone("Not/A_Timezone"); err == nil {
		t.Fatal("expected invalid server timezone to fail")
	}
	if time.Local.String() != "Asia/Seoul" {
		t.Fatalf("invalid timezone changed the active timezone: %s", time.Local)
	}
}
