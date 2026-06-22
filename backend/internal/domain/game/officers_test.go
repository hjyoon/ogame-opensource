package game

import (
	"testing"
	"time"
)

func TestResolveOfficerRecruitmentSpendsPaidBeforeFree(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	result, issue := ResolveOfficerRecruitment(
		OfficersUser{PaidDarkMatter: 4000, FreeDarkMatter: 7000},
		OfficerTimers{},
		OfficerMutation{OfficerID: OfficerEngineer, Days: OfficerWeekDays},
		now,
	)
	if issue == nil || issue.Code != OfficerIssueRecruited {
		t.Fatalf("expected recruited issue, got %+v", issue)
	}
	if !result.Changed || result.User.PaidDarkMatter != 0 || result.User.FreeDarkMatter != 1000 {
		t.Fatalf("unexpected DM spend result: %+v", result)
	}
	if result.Timers.Engineer != now.Add(7*24*time.Hour).Unix() {
		t.Fatalf("unexpected engineer timer: %+v", result.Timers)
	}
}

func TestResolveOfficerRecruitmentExtendsActiveTimer(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	oldUntil := now.Add(3 * 24 * time.Hour).Unix()
	result, issue := ResolveOfficerRecruitment(
		OfficersUser{PaidDarkMatter: 20000},
		OfficerTimers{Geologist: oldUntil},
		OfficerMutation{OfficerID: OfficerGeologist, Days: OfficerWeekDays},
		now,
	)
	if issue == nil || issue.Code != OfficerIssueRecruited {
		t.Fatalf("expected recruited issue, got %+v", issue)
	}
	if result.Timers.Geologist != oldUntil+7*24*60*60 {
		t.Fatalf("expected active timer extension, got %+v", result.Timers)
	}

	result, issue = ResolveOfficerRecruitment(
		OfficersUser{PaidDarkMatter: OfficerThreeMonthCost},
		OfficerTimers{},
		OfficerMutation{OfficerID: OfficerAdmiral, Days: OfficerThreeMonthDays},
		now,
	)
	if issue == nil || issue.Code != OfficerIssueRecruited || result.User.PaidDarkMatter != 0 {
		t.Fatalf("expected three-month recruitment spend, result=%+v issue=%+v", result, issue)
	}
	if result.Timers.Admiral != now.Add(90*24*time.Hour).Unix() {
		t.Fatalf("unexpected admiral timer: %+v", result.Timers)
	}
}

func TestResolveOfficerRecruitmentRejectsInsufficientAndInvalid(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	result, issue := ResolveOfficerRecruitment(
		OfficersUser{PaidDarkMatter: 9999},
		OfficerTimers{},
		OfficerMutation{OfficerID: OfficerAdmiral, Days: OfficerWeekDays},
		now,
	)
	if issue == nil || issue.Code != OfficerIssueNotEnough || result.Changed {
		t.Fatalf("expected insufficient DM issue without change, result=%+v issue=%+v", result, issue)
	}

	result, issue = ResolveOfficerRecruitment(
		OfficersUser{PaidDarkMatter: 50000},
		OfficerTimers{},
		OfficerMutation{OfficerID: 99, Days: OfficerWeekDays},
		now,
	)
	if issue != nil || result.Changed {
		t.Fatalf("invalid legacy parameters should be ignored, result=%+v issue=%+v", result, issue)
	}

	result, issue = ResolveOfficerRecruitment(
		OfficersUser{PaidDarkMatter: 50000},
		OfficerTimers{},
		OfficerMutation{OfficerID: OfficerCommander, Days: 1},
		now,
	)
	if issue != nil || result.Changed {
		t.Fatalf("invalid legacy duration should be ignored, result=%+v issue=%+v", result, issue)
	}
}

func TestOfficerRowsCalculateActiveDays(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	rows := OfficerRows(OfficerTimers{Commander: now.Add(25 * time.Hour).Unix()}, now)
	if !rows[0].Active || rows[0].DaysLeft != 2 {
		t.Fatalf("expected commander active for rounded-up days, got %+v", rows[0])
	}
	if rows[1].Active || rows[1].DaysLeft != 0 {
		t.Fatalf("expected inactive admiral, got %+v", rows[1])
	}
}

func TestOfficerMatterSpendingAndTimerHelpers(t *testing.T) {
	user := OfficersUser{PaidDarkMatter: 20000, FreeDarkMatter: 30000}
	spent, ok := SpendOfficerDarkMatter(user, 0)
	if !ok || spent != user {
		t.Fatalf("zero cost should preserve user, got %+v ok=%v", spent, ok)
	}
	spent, ok = SpendOfficerDarkMatter(user, 10000)
	if !ok || spent.PaidDarkMatter != 10000 || spent.FreeDarkMatter != 30000 {
		t.Fatalf("paid dark matter should be spent first, got %+v ok=%v", spent, ok)
	}

	timers := OfficerTimers{}
	timers = SetOfficerUntil(timers, OfficerCommander, 1)
	timers = SetOfficerUntil(timers, OfficerAdmiral, 2)
	timers = SetOfficerUntil(timers, OfficerEngineer, 3)
	timers = SetOfficerUntil(timers, OfficerGeologist, 4)
	timers = SetOfficerUntil(timers, OfficerTechnocrat, 5)
	timers = SetOfficerUntil(timers, 99, 99)
	if OfficerUntil(timers, OfficerCommander) != 1 ||
		OfficerUntil(timers, OfficerAdmiral) != 2 ||
		OfficerUntil(timers, OfficerEngineer) != 3 ||
		OfficerUntil(timers, OfficerGeologist) != 4 ||
		OfficerUntil(timers, OfficerTechnocrat) != 5 ||
		OfficerUntil(timers, 99) != 0 {
		t.Fatalf("unexpected timer helper result: %+v", timers)
	}
}
