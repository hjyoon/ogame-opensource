package mysqlgame

import "testing"

func TestClientLocalTimeHTMLCarriesUTCInstant(t *testing.T) {
	got := clientLocalTimeHTML(0, clientTimeFormatMessage)
	want := `<time datetime="1970-01-01T00:00:00Z" data-ogame-unix="0" data-ogame-time-format="message">01-01 00:00:00</time>`
	if got != want {
		t.Fatalf("unexpected client-local time markup:\nwant %s\ngot  %s", want, got)
	}
}
