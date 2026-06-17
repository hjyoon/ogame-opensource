package session

import "testing"

func TestTokenGeneratorCreatesLegacyLengthTokens(t *testing.T) {
	generator := TokenGenerator{}

	publicID, err := generator.NewPublicSession()
	if err != nil {
		t.Fatal(err)
	}
	privateID, err := generator.NewPrivateSession()
	if err != nil {
		t.Fatal(err)
	}

	if len(publicID) != 12 {
		t.Fatalf("expected 12-char public session, got %q", publicID)
	}
	if len(privateID) != 32 {
		t.Fatalf("expected 32-char private session, got %q", privateID)
	}
}

func TestRandomHexAllowsZeroByteLength(t *testing.T) {
	value, err := randomHex(0)
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("expected empty token for zero bytes, got %q", value)
	}
}
