package game

import "testing"

func TestScoreSummaryDisplayPoints(t *testing.T) {
	cases := map[int64]int64{
		123456: 123,
		999:    0,
		0:      0,
		-1:     0,
	}
	for raw, expected := range cases {
		if got := (ScoreSummary{RawScore: raw}).DisplayPoints(); got != expected {
			t.Fatalf("raw score %d: expected %d, got %d", raw, expected, got)
		}
	}
}

func TestCoordinatesValid(t *testing.T) {
	if !(Coordinates{Galaxy: 1, System: 2, Position: 3}).Valid() {
		t.Fatal("expected positive coordinates to be valid")
	}
	if (Coordinates{Galaxy: 1, System: 0, Position: 3}).Valid() {
		t.Fatal("expected missing system to be invalid")
	}
}
