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

func assertLoginIssue(t *testing.T, result LoginValidation, code string, legacyCode int) {
	t.Helper()
	for _, issue := range result.Issues {
		if issue.Code == code && issue.LegacyErrorCode == legacyCode {
			return
		}
	}
	t.Fatalf("expected issue %s/%d in %+v", code, legacyCode, result)
}
