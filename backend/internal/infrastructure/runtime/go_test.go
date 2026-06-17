package runtime

import "testing"

func TestGoRuntimeVersion(t *testing.T) {
	if (GoRuntime{}).Version() == "" {
		t.Fatal("expected Go runtime version")
	}
}
