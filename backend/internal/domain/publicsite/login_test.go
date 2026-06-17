package publicsite

import "testing"

func TestLoginDraftValidatesAcceptedDraft(t *testing.T) {
	result := LoginDraft{
		Login:    "Commander01",
		Password: "E2E_http123",
		Universe: "http://localhost:8888",
	}.Validate()

	if !result.Valid || len(result.Issues) != 0 {
		t.Fatalf("expected valid login draft, got %+v", result)
	}
}

func TestLoginDraftReportsRequiredFields(t *testing.T) {
	result := LoginDraft{}.Validate()

	assertLoginIssue(t, result, LoginIssueLoginRequired, LegacyLoginErrorCredentials)
	assertLoginIssue(t, result, LoginIssuePasswordRequired, LegacyLoginErrorCredentials)
	assertLoginIssue(t, result, LoginIssueUniverseRequired, LegacyLoginErrorNoEquivalent)
	if result.Valid {
		t.Fatalf("expected invalid login draft, got %+v", result)
	}
}

func TestLoginCredentialsReportLegacyCompatibleIssues(t *testing.T) {
	result := LoginValidation{Issues: LoginCredentials{}.Validate()}
	assertLoginIssue(t, result, LoginIssueCredentialsInvalid, LegacyLoginErrorCredentials)

	result = LoginValidation{Issues: LoginCredentials{Authenticated: true, Banned: true, BannedUntil: 123}.Validate()}
	assertLoginIssue(t, result, LoginIssueUserBanned, LegacyLoginErrorBanned)
}

func TestLoginCredentialsAllowAuthenticatedUnbannedUser(t *testing.T) {
	issues := LoginCredentials{Authenticated: true, PlayerID: 1}.Validate()

	if len(issues) != 0 {
		t.Fatalf("expected authenticated unbanned user to pass, got %+v", issues)
	}
}

func TestLoginSessionUsesLegacyCookieName(t *testing.T) {
	session := LoginSession{PlayerID: 42, UniverseNumber: 7}

	if got := session.PrivateCookieName(); got != "prsess_42_7" {
		t.Fatalf("unexpected private session cookie name: %q", got)
	}
}

func TestLoginSessionBuildsNaturalOverviewRedirect(t *testing.T) {
	session := LoginSession{
		PublicID:     "abc123",
		RedirectPath: "/game/overview",
	}

	if got := session.RedirectTarget(); got != "/game/overview?lgn=1&session=abc123" {
		t.Fatalf("unexpected redirect target: %q", got)
	}
}

func TestLoginSessionRedirectDefaultsToOverview(t *testing.T) {
	session := LoginSession{PublicID: "public session"}

	if got := session.RedirectTarget(); got != "/game/overview?lgn=1&session=public+session" {
		t.Fatalf("unexpected default redirect target: %q", got)
	}
}

func assertLoginIssue(t *testing.T, result LoginValidation, code string, legacyCode int) {
	t.Helper()
	for _, issue := range result.Issues {
		if issue.Code == code && issue.LegacyErrorCode == legacyCode {
			return
		}
	}
	t.Fatalf("expected issue %s/%d in %+v", code, legacyCode, result)
}
