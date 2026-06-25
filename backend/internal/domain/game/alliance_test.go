package game

import (
	"strings"
	"testing"
	"time"
)

func TestAllianceDomainNormalizesAndValidatesCreation(t *testing.T) {
	if tag := NormalizeAllianceTag(` "ABCDEFghi" `); tag != "ABCDEFgh" {
		t.Fatalf("unexpected normalized tag: %q", tag)
	}
	if name := NormalizeAllianceName("'Long alliance name that is longer than thirty'"); name != "Long alliance name that is lon" {
		t.Fatalf("unexpected normalized name: %q", name)
	}
	if issue := ValidateAllianceCreate("AB", "Valid"); issue == nil || issue.Code != AllianceIssueInvalidTag {
		t.Fatalf("expected tag issue, got %+v", issue)
	}
	if issue := ValidateAllianceCreate("TAG", "No"); issue == nil || issue.Code != AllianceIssueInvalidName {
		t.Fatalf("expected name issue, got %+v", issue)
	}
	if issue := ValidateAllianceCreate("TAG", "Valid Name"); issue != nil {
		t.Fatalf("expected valid alliance creation, got %+v", issue)
	}
}

func TestAllianceViewerRightsMatchLegacyMasks(t *testing.T) {
	founder := AllianceViewer{AllianceID: 7, RankID: AllianceRankFounder, Founder: true}
	if !founder.CanReadMembers() || !founder.CanWriteApplications() || !founder.CanManageAlliance() || !founder.CanKickMembers() || !founder.CanSendCircular() || founder.CanLeaveAlliance() {
		t.Fatalf("unexpected founder permissions: %+v", founder)
	}
	member := AllianceViewer{AllianceID: 7, RankID: AllianceRankNewcomer, RankRights: AllianceRightMembers | AllianceRightWriteApps | AllianceRightManage | AllianceRightKick | AllianceRightCircular}
	if !member.CanReadMembers() || !member.CanWriteApplications() || !member.CanManageAlliance() || !member.CanKickMembers() || !member.CanSendCircular() || !member.CanLeaveAlliance() {
		t.Fatalf("unexpected member permissions: %+v", member)
	}
	outsider := AllianceViewer{}
	if outsider.CanReadMembers() || outsider.CanWriteApplications() || outsider.CanManageAlliance() || outsider.CanKickMembers() || outsider.CanSendCircular() || outsider.CanLeaveAlliance() {
		t.Fatalf("unexpected outsider permissions: %+v", outsider)
	}
}

func TestAllianceDomainNormalizesManagementInputs(t *testing.T) {
	if NormalizeAllianceTextKind(0) != 1 || NormalizeAllianceTextKind(4) != 1 || NormalizeAllianceTextKind(3) != 3 {
		t.Fatal("unexpected text kind normalization")
	}
	if got := NormalizeAllianceURL(" https://example.com/a "); got != "https://example.com/a" {
		t.Fatalf("unexpected url normalization: %q", got)
	}
	if got := NormalizeAllianceURL("   "); got != "" {
		t.Fatalf("expected blank url to stay blank, got %q", got)
	}
	for _, raw := range []string{"javascript:alert(1)", "https://", "http://user:pass@example.com", "https://example.com/a\nb"} {
		if got := NormalizeAllianceURL(raw); got != "" {
			t.Fatalf("expected unsafe url %q to be stripped, got %q", raw, got)
		}
	}
	if issue := ValidateAllianceRankName("Right Hand_1"); issue != nil {
		t.Fatalf("expected valid rank name, got %+v", issue)
	}
	if NormalizeAllianceRankName(" Long rank name that should be trimmed after thirty chars ") != "Long rank name that should be " {
		t.Fatal("unexpected rank name normalization")
	}
	if issue := ValidateAllianceNewRankName(""); issue == nil || issue.Code != AllianceIssueInvalidRankName {
		t.Fatalf("expected empty new rank issue, got %+v", issue)
	}
	if issue := ValidateAllianceRankName("bad/rank"); issue == nil || issue.Code != AllianceIssueInvalidRankName {
		t.Fatalf("expected invalid rank issue, got %+v", issue)
	}
	if text := NormalizeAllianceText(strings.Repeat("a", 5001)); len(text) != 5000 {
		t.Fatalf("expected text truncation, got %d", len(text))
	}
	if text := NormalizeAllianceCircularText(strings.Repeat("a", 2001)); len(text) != 2000 {
		t.Fatalf("expected circular text truncation, got %d", len(text))
	}
}

func TestNewAllianceDefaultsViewFromMembership(t *testing.T) {
	overview := Overview{Commander: "legor", CurrentPlanet: PlanetOverview{ID: 99}}
	noAlliance := NewAlliance(overview, AllianceViewer{PlayerID: 42}, time.Unix(1, 0))
	if noAlliance.View != AllianceViewNoAlliance || noAlliance.CurrentPlanet.ID != 99 {
		t.Fatalf("unexpected no-alliance view: %+v", noAlliance)
	}
	owned := NewAlliance(overview, AllianceViewer{PlayerID: 42, AllianceID: 7}, time.Unix(1, 0)).WithView(AllianceViewMembers)
	if owned.View != AllianceViewMembers || owned.Commander != "legor" {
		t.Fatalf("unexpected owned alliance view: %+v", owned)
	}
}

func TestAllianceIssueMessages(t *testing.T) {
	codes := []string{
		AllianceIssueCreated,
		AllianceIssueApplied,
		AllianceIssueWithdrawn,
		AllianceIssueAccepted,
		AllianceIssueRejected,
		AllianceIssueLeft,
		AllianceIssueSaved,
		AllianceIssueSent,
		AllianceIssueInvalidTag,
		AllianceIssueInvalidName,
		AllianceIssueInvalidRankName,
		AllianceIssueTagExists,
		AllianceIssueAllianceNotFound,
		AllianceIssueApplicationsClosed,
		AllianceIssueNotActivated,
		AllianceIssueAlreadyApplied,
		AllianceIssueNoPermission,
		AllianceIssueApplicationNotFound,
		AllianceIssueFounderCannotLeave,
		"custom",
	}
	for _, code := range codes {
		issue := AllianceIssue(code)
		if issue == nil || issue.Code != code || issue.Message == "" {
			t.Fatalf("unexpected issue for %q: %+v", code, issue)
		}
	}
}
