package game

import "testing"

func TestFeedTextStripsLegacyHTML(t *testing.T) {
	got := FeedText(` <a href="x">Hello&nbsp;World</a><script>alert(1)</script><b> Done </b> `)
	if got != "Hello Worldalert(1) Done" {
		t.Fatalf("unexpected feed text: %q", got)
	}
}

func TestFeedCDataEscapesTerminator(t *testing.T) {
	got := FeedCData("safe ]]> tail")
	if got != "safe ]]&gt; tail" {
		t.Fatalf("unexpected cdata text: %q", got)
	}
}
