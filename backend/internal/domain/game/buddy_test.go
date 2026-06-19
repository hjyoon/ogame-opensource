package game

import "testing"

func TestNormalizeBuddyAction(t *testing.T) {
	if NormalizeBuddyAction(BuddyActionIncoming) != BuddyActionIncoming ||
		NormalizeBuddyAction(BuddyActionOutgoing) != BuddyActionOutgoing ||
		NormalizeBuddyAction(BuddyActionRequest) != BuddyActionRequest {
		t.Fatalf("expected legacy buddy actions to be kept")
	}
	if NormalizeBuddyAction(8) != BuddyActionHome {
		t.Fatalf("state-changing buddy actions should fall back to home until mutations are ported")
	}
}

func TestBuddyOnlineStatus(t *testing.T) {
	tests := []struct {
		name      string
		lastClick int64
		now       int64
		wantText  string
		wantColor string
	}{
		{name: "online", lastClick: 1_000, now: 1_600, wantText: "On", wantColor: "lime"},
		{name: "minutes", lastClick: 1_000, now: 2_500, wantText: "25 min", wantColor: "yellow"},
		{name: "offline", lastClick: 1_000, now: 5_000, wantText: "Off", wantColor: "red"},
		{name: "future clock", lastClick: 2_000, now: 1_000, wantText: "On", wantColor: "lime"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := BuddyOnlineStatus(tt.lastClick, tt.now)
			if status.Text != tt.wantText || status.Color != tt.wantColor {
				t.Fatalf("unexpected status: %+v", status)
			}
		})
	}
}
