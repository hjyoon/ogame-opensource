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

func TestOverviewUnreadMessageText(t *testing.T) {
	if got := OverviewUnreadMessageText(0); got != "" {
		t.Fatalf("expected no text for zero unread messages, got %q", got)
	}
	if got := OverviewUnreadMessageText(1); got != "You have 1 new message" {
		t.Fatalf("unexpected singular unread text: %q", got)
	}
	if got := OverviewUnreadMessageText(2); got != "You have 2 new messages" {
		t.Fatalf("unexpected plural unread text: %q", got)
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

func TestNormalizePlanetNameMatchesLegacy(t *testing.T) {
	name, ok := NormalizePlanetName(`  New   /Colony*(Alpha)'  `, PlanetTypePlanet)
	if !ok || name != "New /ColonyAlp" {
		t.Fatalf("unexpected sanitized planet name: name=%q ok=%t", name, ok)
	}

	name, ok = NormalizePlanetName("abcdefghijklmnopqrstuvwxyz", PlanetTypePlanet)
	if !ok || name != "abcdefghijklmnopqrst" {
		t.Fatalf("unexpected truncated planet name: name=%q ok=%t", name, ok)
	}

	name, ok = NormalizePlanetName("abcdefghijklmnop", PlanetTypeMoon)
	if !ok || name != "abcdefghijklm (Moon)" {
		t.Fatalf("unexpected moon name: name=%q ok=%t", name, ok)
	}

	if name, ok = NormalizePlanetName("bad;name", PlanetTypePlanet); ok || name != "" {
		t.Fatalf("forbidden characters should keep the legacy name unchanged: name=%q ok=%t", name, ok)
	}

	name, ok = NormalizePlanetName(`   ()*"'\   `, PlanetTypePlanet)
	if !ok || name != "\u043f\u043b\u0430\u043d\u0435\u0442\u0430" {
		t.Fatalf("empty planet name should use legacy default: name=%q ok=%t", name, ok)
	}
}
