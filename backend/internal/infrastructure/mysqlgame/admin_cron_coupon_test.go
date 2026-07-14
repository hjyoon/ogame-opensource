package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAdminCouponCronOneOffCreatesCouponMailAndRemovesTask(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	game := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"Commander", "user@example.local", "fr"})}}}}
	master := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository := NewAdminRepositoryWithQueryer(game, "ogame_").WithMasterRunner(master, master)
	repository.couponCode = func() (string, error) { return "AAAA-BBBB-CCCC-DDDD-EEEE", nil }
	task := buildingQueueTask{TaskID: 7, SubID: 5000, ObjID: (3 << 16) | 7, End: 2_000_000}
	mails, err := repository.finishAdminCouponCronTask(context.Background(), tables, task)
	if err != nil || len(mails) != 1 {
		t.Fatalf("mails=%+v err=%v", mails, err)
	}
	if mails[0].Character != "Commander" || mails[0].Recipient != "user@example.local" || mails[0].Language != "fr" || mails[0].Code != "AAAA-BBBB-CCCC-DDDD-EEEE" {
		t.Fatalf("unexpected mail: %+v", mails[0])
	}
	if len(game.calls) != 1 || game.calls[0].args[0] != task.End-7*adminCronDaySeconds || game.calls[0].args[1] != task.End-3*adminCronDaySeconds {
		t.Fatalf("unexpected eligibility query: %+v", game.calls)
	}
	if len(master.execCalls) != 1 || master.execCalls[0].args[1] != 5000 || len(game.execCalls) != 1 || !strings.Contains(game.execCalls[0].sql, "DELETE FROM") {
		t.Fatalf("master=%+v game=%+v", master.execCalls, game.execCalls)
	}
}

func TestAdminCouponCronPeriodicProcessesEveryRecipientAndProlongs(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	game := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(
		[]any{"One", "one@example.local", "en"}, []any{"Two", "two@example.local", "ru"},
	)}}}}
	master := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()}, {rows: fakeRowsFromValues()},
	}}}
	repository := NewAdminRepositoryWithQueryer(game, "ogame_").WithMasterRunner(master, master)
	code := 0
	repository.couponCode = func() (string, error) {
		code++
		return []string{"CODE-ONE", "CODE-TWO"}[code-1], nil
	}
	mails, err := repository.finishAdminCouponCronTask(context.Background(), tables, buildingQueueTask{TaskID: 8, SubID: 100, Level: 14})
	if err != nil || len(mails) != 2 || mails[1].Code != "CODE-TWO" {
		t.Fatalf("mails=%+v err=%v", mails, err)
	}
	if len(game.execCalls) != 1 || !strings.Contains(game.execCalls[0].sql, "end = end +") || game.execCalls[0].args[0] != 14*adminCronDaySeconds {
		t.Fatalf("unexpected prolong: %+v", game.execCalls)
	}
}

func TestAdminCouponCronRunReturnsOutboundMail(t *testing.T) {
	task := buildingQueueTask{TaskID: 3, OwnerID: adminCronSpaceID, Type: adminCouponQueueType, SubID: 50, End: 100}
	game := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(adminUniverseSettingsRow())},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		{rows: fakeRowsFromValues(buildingQueueTaskValues(task))},
		{rows: fakeRowsFromValues([]any{"Legor", "legor@example.local", "en"})},
	}}}
	master := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
	repository := NewAdminRepositoryWithQueryer(game, "ogame_").WithMasterRunner(master, master)
	repository.couponCode = func() (string, error) { return "RUN-CODE", nil }
	mails, err := repository.runAdminCron(context.Background(), 100)
	if err != nil || len(mails) != 1 || mails[0].Code != "RUN-CODE" {
		t.Fatalf("mails=%+v err=%v", mails, err)
	}
}

func TestAdminCouponCronLoadersAndErrors(t *testing.T) {
	task := buildingQueueTask{TaskID: 1, Type: adminCouponQueueType, SubID: 2, ObjID: 3, Level: 4, Start: 5, End: 6, Prio: 7, Freeze: 8, Frozen: 9}
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(buildingQueueTaskValues(task))}}}, "ogame_")
	tasks, err := repository.loadAdminCouponCronTasks(context.Background(), "queue", 10)
	if err != nil || len(tasks) != 1 || tasks[0] != task {
		t.Fatalf("tasks=%+v err=%v", tasks, err)
	}
	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"Name", "mail@example.local", "de"})}}}, "ogame_")
	recipients, err := repository.loadAdminCouponCronRecipients(context.Background(), "users", 10, 20)
	if err != nil || len(recipients) != 1 || recipients[0].Language != "de" {
		t.Fatalf("recipients=%+v err=%v", recipients, err)
	}
	loaderCases := []struct {
		coupon bool
		result fakeQueryResult
	}{
		{coupon: true, result: fakeQueryResult{err: errors.New("query failed")}},
		{coupon: true, result: fakeQueryResult{rows: fakeRowsFromValues([]any{1})}},
		{coupon: true, result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("rows failed"))}},
		{result: fakeQueryResult{err: errors.New("query failed")}},
		{result: fakeQueryResult{rows: fakeRowsFromValues([]any{"only"})}},
		{result: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("rows failed"))}},
	}
	for index, test := range loaderCases {
		repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{test.result}}, "ogame_")
		if test.coupon {
			_, err = repository.loadAdminCouponCronTasks(context.Background(), "queue", 10)
		} else {
			_, err = repository.loadAdminCouponCronRecipients(context.Background(), "users", 10, 20)
		}
		if err == nil {
			t.Fatalf("loader case %d expected error", index)
		}
	}
}

func TestAdminCouponCronPropagatesProcessingErrors(t *testing.T) {
	tables, _ := loadAdminCronTables("ogame_")
	game := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("recipients failed")}}}}
	if _, err := NewAdminRepositoryWithQueryer(game, "ogame_").finishAdminCouponCronTask(context.Background(), tables, buildingQueueTask{}); err == nil {
		t.Fatal("expected recipient error")
	}
	game = &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"User", "user@example.local", "en"})}}}}
	if _, err := NewAdminRepositoryWithQueryer(game, "ogame_").finishAdminCouponCronTask(context.Background(), tables, buildingQueueTask{}); err == nil || !strings.Contains(err.Error(), "master DB") {
		t.Fatalf("expected master error, got %v", err)
	}
	for _, level := range []int{0, 1} {
		game = &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, execErrs: []error{errors.New("queue write failed")}}
		if _, err := NewAdminRepositoryWithQueryer(game, "ogame_").finishAdminCouponCronTask(context.Background(), tables, buildingQueueTask{Level: level}); err == nil {
			t.Fatalf("level %d expected queue write error", level)
		}
	}
	game = &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("tasks failed")}}}}
	if _, err := NewAdminRepositoryWithQueryer(game, "ogame_").finishDueAdminCouponCronTasks(context.Background(), tables, 10); err == nil {
		t.Fatal("expected tasks error")
	}
	game = &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(buildingQueueTaskValues(buildingQueueTask{Type: adminCouponQueueType}))},
		{err: errors.New("task failed")},
	}}}
	if _, err := NewAdminRepositoryWithQueryer(game, "ogame_").finishDueAdminCouponCronTasks(context.Background(), tables, 10); err == nil {
		t.Fatal("expected task processing error")
	}
}
