package game

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeSkinPathMatchesLegacyLoopbackRules(t *testing.T) {
	tests := []struct {
		name string
		path string
		host string
		port int
		want string
	}{
		{name: "empty defaults", want: "/evolution/"},
		{name: "relative kept", path: "/download/use/lego/", want: "/download/use/lego/"},
		{name: "loopback stripped", path: "http://127.0.0.1:8888/evolution/formate.css", want: "/evolution/formate.css/"},
		{name: "loopback root stripped", path: "http://127.0.0.1", want: "/"},
		{name: "same origin stripped", path: "http://10.8.0.2:8890/evolution", host: "10.8.0.2", port: 8890, want: "/evolution/"},
		{name: "http default port same origin stripped", path: "http://example.test/skin", host: "example.test", port: 80, want: "/skin/"},
		{name: "https default port same origin stripped", path: "https://example.test/skin", host: "example.test", port: 443, want: "/skin/"},
		{name: "ipv6 loopback stripped", path: "http://[::1]/skin", want: "/skin/"},
		{name: "non http scheme kept", path: "ftp://127.0.0.1/skin", want: "ftp://127.0.0.1/skin"},
		{name: "malformed kept", path: "http://[::1", want: "http://[::1"},
		{name: "no request host kept", path: "http://example.test/skin", want: "http://example.test/skin"},
		{name: "external kept", path: "https://cdn.example.test/skin/", host: "10.8.0.2", port: 8890, want: "https://cdn.example.test/skin/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeSkinPath(tt.path, tt.host, tt.port); got != tt.want {
				t.Fatalf("NormalizeSkinPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeOptionsMutationHonorsForcedLanguageAndDeletionState(t *testing.T) {
	current := NewOptions(Overview{}, OptionsUser{}, OptionsUniverse{Language: "de", ForceLanguage: true}, OptionsSettings{}, OptionsAccount{
		DeletionQueued: true,
	}, 0)

	normalized := NormalizeOptionsMutation(OptionsMutation{
		Language:      "en",
		DeleteAccount: true,
	}, current)

	if normalized.Language != "de" || normalized.AccountDeletionChanged {
		t.Fatalf("unexpected forced-language options normalization: %+v", normalized)
	}
}

func TestNormalizeOptionsMutationClampsLegacySettings(t *testing.T) {
	current := NewOptions(Overview{}, OptionsUser{}, OptionsUniverse{Language: "en"}, OptionsSettings{
		Language:         "en",
		SkinPath:         "/evolution/",
		SortBy:           0,
		SortOrder:        0,
		MaxSpy:           1,
		MaxFleetMessages: 3,
	}, OptionsAccount{}, 0)

	normalized := NormalizeOptionsMutation(OptionsMutation{
		Language:         "english",
		SkinPath:         "",
		SortBy:           999,
		SortOrder:        -2,
		MaxSpy:           -42,
		MaxFleetMessages: 999,
		DeleteAccount:    true,
	}, current)

	if normalized.Language != "en" || normalized.SkinPath != "/evolution/" ||
		normalized.SortBy != 2 || normalized.SortOrder != 0 || normalized.MaxSpy != 1 || normalized.MaxFleetMessages != 99 ||
		!normalized.AccountDeletionChanged {
		t.Fatalf("unexpected normalized options: %+v", normalized)
	}
}

func TestNewOptionsForcesUniverseLanguageAndMapsFlags(t *testing.T) {
	options := NewOptions(
		Overview{Commander: "legor"},
		OptionsUser{Name: "Legor"},
		OptionsUniverse{Language: "de", ForceLanguage: true},
		OptionsSettings{Language: "en", SortBy: -1, SortOrder: 9, MaxSpy: 0, MaxFleetMessages: 100},
		OptionsAccount{},
		userFlagShowEspionageButton|userFlagFeedEnable|UserFlagHideGOEmail,
	)

	if options.Settings.Language != "de" || options.Settings.SortBy != 0 || options.Settings.SortOrder != 1 ||
		options.Settings.MaxSpy != 1 || options.Settings.MaxFleetMessages != 99 {
		t.Fatalf("unexpected settings: %+v", options.Settings)
	}
	if !options.User.CommanderOn || !options.Flags.ShowEspionageButton || !options.Flags.FeedEnabled || !options.Flags.HideGOEmail {
		t.Fatalf("unexpected flags/user: user=%+v flags=%+v", options.User, options.Flags)
	}
}

func TestOptionsActionIssues(t *testing.T) {
	if issue := OptionsSavedIssue(); issue.Code != OptionsIssueSaved || issue.Message == "" {
		t.Fatalf("unexpected saved issue: %+v", issue)
	}
	if issue := OptionsAccountDeletionQueuedIssue(testUnix(1_700_000_000)); issue.Code != OptionsIssueAccountDeletionQueued || !contains(issue.Message, "Deletion date:") {
		t.Fatalf("unexpected queued issue: %+v", issue)
	}
	if issue := OptionsAccountDeletionQueuedIssue(testUnix(0)); issue.Code != OptionsIssueAccountDeletionQueued || contains(issue.Message, "1970") {
		t.Fatalf("unexpected zero-time queued issue: %+v", issue)
	}
	if issue := OptionsAccountDeletionClearedIssue(); issue.Code != OptionsIssueAccountDeletionClear || issue.Message == "" {
		t.Fatalf("unexpected cleared issue: %+v", issue)
	}
}

func testUnix(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}

func contains(value string, fragment string) bool {
	return strings.Contains(value, fragment)
}
