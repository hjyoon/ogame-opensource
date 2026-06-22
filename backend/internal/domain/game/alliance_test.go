package game

import (
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
	if !founder.CanReadMembers() || !founder.CanWriteApplications() || founder.CanLeaveAlliance() {
		t.Fatalf("unexpected founder permissions: %+v", founder)
	}
	member := AllianceViewer{AllianceID: 7, RankID: AllianceRankNewcomer, RankRights: AllianceRightMembers | AllianceRightWriteApps}
	if !member.CanReadMembers() || !member.CanWriteApplications() || !member.CanLeaveAlliance() {
		t.Fatalf("unexpected member permissions: %+v", member)
	}
	outsider := AllianceViewer{}
	if outsider.CanReadMembers() || outsider.CanWriteApplications() || outsider.CanLeaveAlliance() {
		t.Fatalf("unexpected outsider permissions: %+v", outsider)
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
		AllianceIssueInvalidTag,
		AllianceIssueInvalidName,
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
