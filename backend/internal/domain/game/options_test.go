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
		Vacation:       true,
	}, 0)

	normalized := NormalizeOptionsMutation(OptionsMutation{
		Language:      "en",
		DeleteAccount: true,
	}, current)

	if normalized.Language != "de" || normalized.AccountDeletionChanged || normalized.VacationChanged || !normalized.VacationMode {
		t.Fatalf("unexpected forced-language options normalization: %+v", normalized)
	}
}

func TestNormalizeOptionsMutationTracksExplicitVacationChange(t *testing.T) {
	current := NewOptions(Overview{}, OptionsUser{}, OptionsUniverse{Language: "en"}, OptionsSettings{}, OptionsAccount{}, 0)
	normalized := NormalizeOptionsMutation(OptionsMutation{VacationMode: true, VacationModeSet: true}, current)
	if !normalized.VacationChanged || !normalized.VacationMode {
		t.Fatalf("expected explicit vacation enable to be tracked, got %+v", normalized)
	}

	current = NewOptions(Overview{}, OptionsUser{}, OptionsUniverse{Language: "en"}, OptionsSettings{}, OptionsAccount{Vacation: true}, 0)
	normalized = NormalizeOptionsMutation(OptionsMutation{VacationMode: false, VacationModeSet: true}, current)
	if !normalized.VacationChanged || normalized.VacationMode {
		t.Fatalf("expected explicit vacation disable to be tracked, got %+v", normalized)
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

func TestOptionsCredentialMutationValidation(t *testing.T) {
	current := NewOptions(Overview{}, OptionsUser{
		Email:      "pending@example.test",
		PlainEmail: "permanent@example.test",
		Validated:  true,
	}, OptionsUniverse{Language: "en"}, OptionsSettings{}, OptionsAccount{}, 0)

	if (OptionsMutation{Email: "permanent@example.test"}).EmailChangeRequested(current) {
		t.Fatal("permanent email should not request a change for validated accounts")
	}
	if !(OptionsMutation{Email: "new@example.test"}).EmailChangeRequested(current) {
		t.Fatal("new email should request a change")
	}
	if issue := (OptionsMutation{Email: "bad address"}).EmailValidationIssue(current); issue == nil || issue.Code != OptionsIssueEmailInvalid {
		t.Fatalf("expected invalid email issue, got %+v", issue)
	}
	if issue := (OptionsMutation{Email: "new@example.test"}).EmailValidationIssue(current); issue != nil {
		t.Fatalf("expected valid email, got %+v", issue)
	}
	if issue := (OptionsMutation{NewPassword: "abcdef12", NewPasswordRepeat: "abcdef13"}).PasswordValidationIssue(); issue == nil || issue.Code != OptionsIssuePasswordMismatch {
		t.Fatalf("expected mismatch issue, got %+v", issue)
	}
	if issue := (OptionsMutation{NewPassword: "abcdef!!", NewPasswordRepeat: "abcdef!!"}).PasswordValidationIssue(); issue == nil || issue.Code != OptionsIssuePasswordSpecial {
		t.Fatalf("expected special-character issue, got %+v", issue)
	}
	if issue := (OptionsMutation{NewPassword: "abc123", NewPasswordRepeat: "abc123"}).PasswordValidationIssue(); issue == nil || issue.Code != OptionsIssuePasswordTooShort {
		t.Fatalf("expected short-password issue, got %+v", issue)
	}
	if issue := (OptionsMutation{NewPassword: "abc_1234", NewPasswordRepeat: "abc_1234"}).PasswordValidationIssue(); issue != nil {
		t.Fatalf("expected valid legacy password, got %+v", issue)
	}
}

func TestOptionsEmailChangeForUnvalidatedAccountsUsesPendingEmail(t *testing.T) {
	current := NewOptions(Overview{}, OptionsUser{
		Email:      "pending@example.test",
		PlainEmail: "permanent@example.test",
		Validated:  false,
	}, OptionsUniverse{Language: "en"}, OptionsSettings{}, OptionsAccount{}, 0)

	if (OptionsMutation{Email: "pending@example.test"}).EmailChangeRequested(current) {
		t.Fatal("pending email should not request a change for unvalidated accounts")
	}
	if !(OptionsMutation{Email: "permanent@example.test"}).EmailChangeRequested(current) {
		t.Fatal("different email should request a change for unvalidated accounts")
	}
}

func TestNewOptionsForcesUniverseLanguageAndMapsFlags(t *testing.T) {
	options := NewOptions(
		Overview{Commander: "legor"},
		OptionsUser{Name: "Legor", CommanderOn: true},
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

func TestNewOptionsPreservesPremiumStatusFromUserData(t *testing.T) {
	options := NewOptions(
		Overview{Commander: "legor"},
		OptionsUser{Name: "Legor", CommanderOn: false},
		OptionsUniverse{Language: "en"},
		OptionsSettings{},
		OptionsAccount{},
		0,
	)

	if options.User.CommanderOn {
		t.Fatalf("commander status must come from premium user data, got %+v", options.User)
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
	if issue := OptionsVacationEnabledIssue(testUnix(1_700_000_000)); issue.Code != OptionsIssueVacationEnabled || !contains(issue.Message, "Minimum until:") {
		t.Fatalf("unexpected vacation enabled issue: %+v", issue)
	}
	if issue := OptionsVacationDisabledIssue("Legor"); issue.Code != OptionsIssueVacationDisabled || !contains(issue.Message, "Legor") {
		t.Fatalf("unexpected vacation disabled issue: %+v", issue)
	}
	if issue := OptionsVacationDisabledIssue(""); issue.Code != OptionsIssueVacationDisabled || !contains(issue.Message, "Commander") {
		t.Fatalf("unexpected default vacation disabled issue: %+v", issue)
	}
	if issue := OptionsVacationBlockedIssue(); issue.Code != OptionsIssueVacationBlocked || issue.Message == "" {
		t.Fatalf("unexpected vacation blocked issue: %+v", issue)
	}
	if issue := OptionsVacationLockedIssue(testUnix(1_700_000_000)); issue.Code != OptionsIssueVacationLocked || !contains(issue.Message, "Minimum until:") {
		t.Fatalf("unexpected vacation locked issue: %+v", issue)
	}
	if issue := OptionsPasswordChangedIssue(); issue.Code != OptionsIssuePasswordChanged || issue.Message == "" {
		t.Fatalf("unexpected password changed issue: %+v", issue)
	}
	if issue := OptionsPasswordWrongOldIssue(); issue.Code != OptionsIssuePasswordWrongOld || issue.Message == "" {
		t.Fatalf("unexpected wrong old password issue: %+v", issue)
	}
	if issue := OptionsEmailChangedIssue(); issue.Code != OptionsIssueEmailChanged || issue.Message == "" {
		t.Fatalf("unexpected email changed issue: %+v", issue)
	}
	if issue := OptionsEmailNeedPasswordIssue(); issue.Code != OptionsIssueEmailNeedPassword || issue.Message == "" {
		t.Fatalf("unexpected email password issue: %+v", issue)
	}
	if issue := OptionsEmailUsedIssue(); issue.Code != OptionsIssueEmailUsed || issue.Message == "" {
		t.Fatalf("unexpected email used issue: %+v", issue)
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
