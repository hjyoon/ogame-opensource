package publicsite

import "testing"

func TestNormalizeRecoveryEmail(t *testing.T) {
	email, ok := NormalizeRecoveryEmail(" USER@example.LOCAL ")
	if !ok || email != "user@example.local" {
		t.Fatalf("unexpected normalized email: %q ok=%t", email, ok)
	}
	if email, ok := NormalizeRecoveryEmail("not an email"); ok || email != "" {
		t.Fatalf("invalid email should be rejected: %q ok=%t", email, ok)
	}
	if email, ok := NormalizeRecoveryEmail(" "); ok || email != "" {
		t.Fatalf("empty email should be rejected: %q ok=%t", email, ok)
	}
}
