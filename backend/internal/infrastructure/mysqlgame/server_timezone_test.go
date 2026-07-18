package mysqlgame

import (
	"testing"
	"time"
)

func useServerTimezone(t *testing.T, location *time.Location) {
	t.Helper()
	original := time.Local
	time.Local = location
	t.Cleanup(func() { time.Local = original })
}

func useDefaultServerTimezone(t *testing.T) {
	t.Helper()
	useServerTimezone(t, time.FixedZone("Europe/Moscow", 3*60*60))
}
