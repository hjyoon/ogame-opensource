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
