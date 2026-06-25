package game

import (
	"html"
	"regexp"
	"strings"
)

const (
	UserFlagFeedEnable = 0x8000
	UserFlagFeedAtom   = 0x10000
	FeedMaxMessages    = 50
)

type Feed struct {
	FeedID   string
	Owner    string
	LastFeed int64
	Atom     bool
	Messages []FeedMessage
}

type FeedMessage struct {
	ID      int
	Subject string
	Text    string
	Date    int64
}

type FeedItem struct {
	Subject string
	Text    string
}

var (
	feedAnchorPattern     = regexp.MustCompile(`(?is)<a[^>]*>(.*?)</a>`)
	feedTagPattern        = regexp.MustCompile(`(?s)<[^>]*>`)
	feedWhitespacePattern = regexp.MustCompile(`\s+`)
)

func FeedText(value string) string {
	withoutLinks := feedAnchorPattern.ReplaceAllString(value, "$1")
	withoutTags := feedTagPattern.ReplaceAllString(withoutLinks, "")
	decoded := html.UnescapeString(withoutTags)
	decoded = strings.NewReplacer("\u00a0", " ", "\u202f", " ").Replace(decoded)
	return feedWhitespacePattern.ReplaceAllString(strings.TrimSpace(decoded), " ")
}

func FeedCData(value string) string {
	return strings.ReplaceAll(value, "]]>", "]]&gt;")
}
