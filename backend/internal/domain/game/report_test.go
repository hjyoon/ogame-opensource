package game

import "testing"

func TestNewReportMatchesLegacyTitleAndAccess(t *testing.T) {
	spy := NewReport(11, MessageTypeSpyReport, "<table>spy</table>", true)
	if spy.Title != ReportTitleSpy || spy.Text == "" || !spy.Allowed {
		t.Fatalf("unexpected spy report: %+v", spy)
	}

	battle := NewReport(12, MessageTypeBattleReportText, "secret", false)
	if battle.Title != ReportTitleBattle || battle.Text != "" || battle.Allowed {
		t.Fatalf("unexpected denied battle report: %+v", battle)
	}
}
