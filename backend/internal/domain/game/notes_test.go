package game

import "testing"

func TestNotesNormalizesLegacyActions(t *testing.T) {
	if NormalizeNotesAction(0) != NotesActionList || NormalizeNotesAction(1) != NotesActionCreate || NormalizeNotesAction(2) != NotesActionEdit {
		t.Fatal("unexpected notes action normalization")
	}
	if NormalizeNotesAction(99) != NotesActionList {
		t.Fatal("unknown notes actions should render the list")
	}
}

func TestNotePriorityColorMatchesLegacy(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{0, "lime"},
		{1, "yellow"},
		{2, "red"},
		{9, "white"},
	}
	for _, tt := range tests {
		if got := (Note{Priority: tt.priority}).PriorityColor(); got != tt.want {
			t.Fatalf("priority %d: got %q want %q", tt.priority, got, tt.want)
		}
	}
}

func TestNormalizeNoteDraftMatchesLegacyBounds(t *testing.T) {
	draft := NormalizeNoteDraft("", "", 9)
	if draft.Subject != "no subject" || draft.Text != "no text" || draft.TextSize != 7 || draft.Priority != 2 {
		t.Fatalf("unexpected empty note draft: %+v", draft)
	}

	draft = NormalizeNoteDraft("abcdefghijklmnopqrstuvwxyz0123456789", "가나다라마", -1)
	if draft.Subject != "abcdefghijklmnopqrstuvwxyz0123" || draft.Text != "가나다라마" || draft.TextSize != 5 || draft.Priority != 0 {
		t.Fatalf("unexpected bounded note draft: %+v", draft)
	}

	longText := "가나다라마바"
	draft = NormalizeNoteDraft("subject", longText, 1)
	if draft.Text != longText || draft.TextSize != 6 {
		t.Fatalf("unexpected unicode text size: %+v", draft)
	}
	if got := truncateRunes("abc", 0); got != "" {
		t.Fatalf("zero rune limit should truncate to empty string, got %q", got)
	}
}

func TestNormalizeNoteIDsKeepsPositiveUniqueIDs(t *testing.T) {
	ids := NormalizeNoteIDs([]int{3, 0, -1, 3, 2})
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 2 {
		t.Fatalf("unexpected normalized note ids: %+v", ids)
	}
}
