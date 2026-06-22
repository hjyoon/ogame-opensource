package game

import "testing"

func TestNewAdminNormalizesModeAndCopiesMenu(t *testing.T) {
	overview := Overview{
		Commander:      "legor",
		CurrentPlanet:  PlanetOverview{ID: 99, Name: "Arakis"},
		PlanetSwitcher: []PlanetSummary{{ID: 99, Name: "Arakis"}},
	}
	admin := NewAdmin(overview, AdminViewer{PlayerID: 42, Name: "legor", Level: AdminLevelAdmin}, "Users")

	if admin.Commander != "legor" || admin.Mode != "Users" || len(admin.Menu) != 25 || !admin.CanAccess() {
		t.Fatalf("unexpected admin: %+v", admin)
	}
	admin.Menu[0].Label = "changed"
	if AdminMenu()[0].Label == "changed" {
		t.Fatal("admin menu should be copied")
	}
	if NormalizeAdminMode("") != "Home" || NormalizeAdminMode("missing") != "Home" {
		t.Fatal("admin mode normalization mismatch")
	}
	if NewAdmin(Overview{}, AdminViewer{Level: AdminLevelPlayer}, "Home").CanAccess() {
		t.Fatal("regular players must not access admin")
	}
	if issue := AdminIssue(AdminIssueAccessDenied); issue == nil || issue.Message != "Access denied." {
		t.Fatalf("unexpected admin issue: %+v", issue)
	}
	if issue := AdminIssue("unknown"); issue == nil || issue.Code != "unknown" {
		t.Fatalf("unexpected unknown admin issue: %+v", issue)
	}
}
