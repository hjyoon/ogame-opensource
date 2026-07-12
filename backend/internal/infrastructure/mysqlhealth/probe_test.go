package mysqlhealth

import (
	"context"
	"testing"
)

func TestNilDatabaseIsNotReady(t *testing.T) {
	if New(nil).Ready(context.Background()) {
		t.Fatal("nil database must not be ready")
	}
}
