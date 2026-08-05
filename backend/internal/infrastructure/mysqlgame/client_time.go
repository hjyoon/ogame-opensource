package mysqlgame

import (
	"fmt"
	"time"
)

const clientTimeFormatMessage = "message"

func clientLocalTimeHTML(timestamp int64, format string) string {
	utc := time.Unix(timestamp, 0).UTC()
	return fmt.Sprintf(
		`<time datetime="%s" data-ogame-unix="%d" data-ogame-time-format="%s">%s</time>`,
		utc.Format(time.RFC3339),
		timestamp,
		format,
		utc.Format("01-02 15:04:05"),
	)
}
