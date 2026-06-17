package publicsite

import "testing"

func TestRegistrationDraftValidatesAcceptedDraft(t *testing.T) {
	result := RegistrationDraft{
		Character:     "Commander01",
		Password:      "E2E_http123",
		Email:         "commander@example.local",
		Universe:      "http://localhost:8888",
		TermsAccepted: true,
	}.Validate()

	if !result.Valid || len(result.Issues) != 0 {
		t.Fatalf("expected valid registration draft, got %+v", result)
	}
}

func TestRegistrationDraftReportsLegacyCompatibleIssues(t *testing.T) {
	result := RegistrationDraft{
		Character:     "ad",
		Password:      "short",
		Email:         "invalid",
		Universe:      "",
		TermsAccepted: false,
	}.Validate()

	assertIssue(t, result, RegistrationIssueTermsRequired, LegacyRegistrationErrorTerms)
	assertIssue(t, result, RegistrationIssuePasswordTooShort, LegacyRegistrationErrorPassword)
	assertIssue(t, result, RegistrationIssueCharacterInvalid, LegacyRegistrationErrorCharacter)
	assertIssue(t, result, RegistrationIssueEmailInvalid, LegacyRegistrationErrorEmail)
	assertIssue(t, result, RegistrationIssueUniverseRequired, LegacyRegistrationErrorNoEquivalent)
	if result.Valid {
		t.Fatalf("expected invalid registration draft, got %+v", result)
	}
}

func TestRegistrationDraftRejectsLegacyNameForbiddenCharacters(t *testing.T) {
	cases := []string{
		"ab",
		"this-name-is-more-than-twenty",
		"bad,name",
		"adminFleet",
		"SpaceMiner",
	}

	for _, name := range cases {
		result := RegistrationDraft{
			Character:     name,
			Password:      "E2E_http123",
			Email:         "commander@example.local",
			Universe:      "http://localhost:8888",
			TermsAccepted: true,
		}.Validate()
		assertIssue(t, result, RegistrationIssueCharacterInvalid, LegacyRegistrationErrorCharacter)
	}
}

func assertIssue(t *testing.T, result RegistrationValidation, code string, legacyCode int) {
	t.Helper()
	for _, issue := range result.Issues {
		if issue.Code == code && issue.LegacyErrorCode == legacyCode {
			return
		}
	}
	t.Fatalf("expected issue %s/%d in %+v", code, legacyCode, result)
}
