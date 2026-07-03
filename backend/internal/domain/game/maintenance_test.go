package game

import "testing"

func TestNormalizeMaintenanceLanguage(t *testing.T) {
	for _, language := range []string{"de", "en", "es", "fr", "it", "ru"} {
		if got := NormalizeMaintenanceLanguage(language); got != language {
			t.Fatalf("expected %q to remain supported, got %q", language, got)
		}
	}
	for _, language := range []string{"", "jp", "bad"} {
		if got := NormalizeMaintenanceLanguage(language); got != "en" {
			t.Fatalf("expected %q to fall back to en, got %q", language, got)
		}
	}
}
