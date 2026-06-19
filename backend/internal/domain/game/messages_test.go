package game

import (
	"strings"
	"testing"
)

func TestMessagesNormalization(t *testing.T) {
	if NormalizeMessagesLimit(false) != MessagesLimitRegular || NormalizeMessagesLimit(true) != MessagesLimitCommander {
		t.Fatal("unexpected messages limit")
	}
	if NormalizeMessagesAction(7) != MessagesActionCompose || NormalizeMessagesAction(0) != MessagesActionInbox {
		t.Fatal("unexpected messages action")
	}
	if NormalizeMessagesMutationAction(MessagesMutationActionSend) != MessagesMutationActionSend ||
		NormalizeMessagesMutationAction("unknown") != MessagesMutationActionDelete {
		t.Fatal("unexpected messages mutation action")
	}
	if NormalizeMessageDeleteMode(MessageDeleteModeMarked) != MessageDeleteModeMarked ||
		NormalizeMessageDeleteMode("unknown") != MessageDeleteModeNone {
		t.Fatal("unexpected message delete mode")
	}
	ids := NormalizeMessageIDs([]int{3, 0, 3, -1, 5})
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 5 {
		t.Fatalf("unexpected normalized ids: %+v", ids)
	}
	draft := NormalizeMessageDraft(9, strings.Repeat("s", MessageSubjectMaxChars+2), strings.Repeat("t", MessageComposeMaxChars+2))
	if draft.TargetPlayerID != 9 || len(draft.Subject) != MessageSubjectMaxChars || len(draft.Text) != MessageComposeMaxChars {
		t.Fatalf("unexpected normalized draft: %+v", draft)
	}
}

func TestMessageActionIssues(t *testing.T) {
	issues := []*MessageActionIssue{
		MessageMissingSubjectIssue(),
		MessageMissingTextIssue(),
		MessageNotActivatedIssue(),
		MessageSentIssue(),
		MessageReportedIssue(),
		MessageReportExistsIssue(),
	}
	wantCodes := []string{
		MessageIssueMissingSubject,
		MessageIssueMissingText,
		MessageIssueNotActivated,
		MessageIssueSent,
		MessageIssueReported,
		MessageIssueReportExists,
	}
	for index, issue := range issues {
		if issue == nil || issue.Code != wantCodes[index] || issue.Message == "" {
			t.Fatalf("unexpected issue at %d: %+v", index, issue)
		}
	}
}
