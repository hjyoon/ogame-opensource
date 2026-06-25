package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestAdminRepositoryReadsAdminHome(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{42, "legor", int64(1700000000), int64(1700003600), 0, 0, 0, 0, 99, "Homeworld", 1, 2, 3},
		)},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{42, "legor", int64(1700000000), int64(1700003600), 0, 0, 0, 0, 99, "Homeworld", 1, 2, 3},
		)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Users"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if admin.Commander != "legor" || admin.Viewer.Level != domaingame.AdminLevelAdmin || admin.Mode != "Users" || len(admin.Menu) != 25 {
		t.Fatalf("unexpected admin: %+v", admin)
	}
	if len(admin.UserRows) != 1 || admin.UserRows[0].HomePlanet == nil || admin.UserRows[0].HomePlanet.Name != "Homeworld" {
		t.Fatalf("unexpected user rows: %+v", admin.UserRows)
	}
	if len(admin.ActiveUsers) != 1 || admin.ActiveUsers[0].Name != "legor" {
		t.Fatalf("unexpected active users: %+v", admin.ActiveUsers)
	}
	if !strings.Contains(queryer.calls[len(queryer.calls)-2].sql, "ORDER BY u.regdate DESC LIMIT 25") {
		t.Fatalf("expected new users query, got %s", queryer.calls[len(queryer.calls)-2].sql)
	}
	if !strings.Contains(queryer.calls[len(queryer.calls)-1].sql, "WHERE u.lastclick >= ? ORDER BY u.oname ASC") {
		t.Fatalf("expected active users query, got %s", queryer.calls[len(queryer.calls)-1].sql)
	}
	_ = NewAdminRepository(nil, "ogame_")
}

func TestNewAdminRepositoryWithQueryerKeepsMutationRunner(t *testing.T) {
	runner := &fakeGalaxyRunner{}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")

	if repository.queryer != runner {
		t.Fatalf("expected queryer to be preserved")
	}
	if repository.execer != runner {
		t.Fatalf("expected execer to be preserved")
	}
	if repository.overview.execer != runner {
		t.Fatalf("expected overview execer to reuse runner")
	}
	if repository.legacyGameDir != "game" {
		t.Fatalf("unexpected legacy game dir: %q", repository.legacyGameDir)
	}
}

func TestAdminRepositorySkipsRestrictedOperatorModeData(t *testing.T) {
	results := append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "operator", domaingame.AdminLevelOperator})},
		fakeQueryResult{err: errors.New("bot strategies should not load")},
	)
	queryer := &fakeQueryer{results: results}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "BotEdit"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if admin.Mode != "BotEdit" || admin.CanAccessMode() || len(admin.BotStrategies) != 0 {
		t.Fatalf("unexpected restricted operator admin payload: %+v", admin)
	}
	if len(queryer.calls) != len(shipyardOverviewResults())+1 {
		t.Fatalf("restricted operator mode should not issue detail query, calls=%d", len(queryer.calls))
	}
}

func TestAdminRepositoryReadsFleetLogRows(t *testing.T) {
	fleetIDs := domaingame.FleetIDs()
	shipValues := make([]any, len(fleetIDs))
	shipValues[0] = 2
	for index := 1; index < len(shipValues); index++ {
		shipValues[index] = 0
	}
	rowValues := []any{
		1001, int64(1700000000), int64(1700007200), 3, 7200, 5, 6, 501, 502,
		"Fleet Home", 1, 470, 4, 1, 42, "legor",
		"Fleet Target", 1, 470, 5, 1, 77, "target",
		123, 0, 45,
	}
	rowValues = append(rowValues, shipValues...)
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(rowValues)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Fleetlogs"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.FleetLogRows) != 1 {
		t.Fatalf("expected one fleet log row, got %+v", admin.FleetLogRows)
	}
	row := admin.FleetLogRows[0]
	if row.Number != 1 || row.TaskID != 1001 || row.Mission != 3 || row.UnionID != 6 ||
		row.Origin.OwnerID != 42 || row.Target.OwnerName != "target" || row.Target.Coordinates.Position != 5 {
		t.Fatalf("unexpected fleet log row: %+v", row)
	}
	if len(row.Cargo) != 2 || row.Cargo[0].Name != "Metal" || row.Cargo[0].Loaded != 123 || row.Cargo[1].Name != "Deuterium" {
		t.Fatalf("unexpected cargo rows: %+v", row.Cargo)
	}
	if len(row.Ships) != 1 || row.Ships[0].ID != fleetIDs[0] || row.Ships[0].Count != 2 {
		t.Fatalf("unexpected ship rows: %+v", row.Ships)
	}
	lastCall := queryer.calls[len(queryer.calls)-1]
	if !strings.Contains(lastCall.sql, "`ogame_queue` q JOIN `ogame_fleet` f") || lastCall.args[0] != queueTypeFleet {
		t.Fatalf("expected fleetlogs queue/fleet query, got %+v", lastCall)
	}
}

func TestAdminRepositoryFleetLogRowEdges(t *testing.T) {
	t.Run("invalid prefix", func(t *testing.T) {
		repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_")
		if _, err := repository.loadAdminFleetLogRows(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected prefix error, got %v", err)
		}
	})

	t.Run("query error", func(t *testing.T) {
		repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("fleetlogs query failed")}}}, "ogame_")
		if _, err := repository.loadAdminFleetLogRows(context.Background()); err == nil || !strings.Contains(err.Error(), "fleetlogs query failed") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("scan error", func(t *testing.T) {
		repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1001})}}}, "ogame_")
		if _, err := repository.loadAdminFleetLogRows(context.Background()); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected scan error, got %v", err)
		}
	})
}

func TestAdminRepositoryMutatesBans(t *testing.T) {
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "admin"})},
		{rows: fakeRowsFromValues([]any{77, "target"})},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return time.Unix(1_000, 0) }

	issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
		PlayerID:  42,
		Mode:      "Bans",
		Action:    "ban",
		TargetIDs: []int{77, 77, 0},
		BanMode:   1,
		Hours:     2,
		Reason:    `quote "tick'`,
	})

	if err != nil {
		t.Fatalf("MutateAdmin returned error: %v", err)
	}
	if issue == nil || issue.Code != domaingame.AdminIssueActionSaved {
		t.Fatalf("unexpected issue: %+v", issue)
	}
	if len(runner.execCalls) != 4 {
		t.Fatalf("expected pranger/delete/queue/update execs, got %+v", runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[0].sql, "ogame_pranger") || runner.execCalls[0].args[0] != "admin" || runner.execCalls[0].args[1] != "target" {
		t.Fatalf("unexpected pranger insert: %+v", runner.execCalls[0])
	}
	if runner.execCalls[1].args[0] != "UnbanPlayer" || runner.execCalls[1].args[1] != 77 {
		t.Fatalf("unexpected queue delete: %+v", runner.execCalls[1])
	}
	if runner.execCalls[2].args[0] != 77 || runner.execCalls[2].args[1] != "UnbanPlayer" || runner.execCalls[2].args[2] != 1_000 || runner.execCalls[2].args[3] != 9_200 {
		t.Fatalf("unexpected queue insert: %+v", runner.execCalls[2])
	}
	if !strings.Contains(runner.execCalls[3].sql, "vacation = 1") || runner.execCalls[3].args[0] != 8_200 || runner.execCalls[3].args[2] != 77 {
		t.Fatalf("unexpected ban update: %+v", runner.execCalls[3])
	}
}

func TestAdminRepositoryMutatesBanWithoutVacation(t *testing.T) {
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "admin"})},
		{rows: fakeRowsFromValues([]any{77, "target"})},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return time.Unix(1_000, 0) }

	_, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
		PlayerID:  42,
		Mode:      "Bans",
		Action:    "ban",
		TargetIDs: []int{77},
		BanMode:   0,
		Hours:     1,
	})

	if err != nil {
		t.Fatalf("MutateAdmin returned error: %v", err)
	}
	if len(runner.execCalls) != 4 || strings.Contains(runner.execCalls[3].sql, "vacation = 1") || runner.execCalls[3].args[0] != 4_600 {
		t.Fatalf("unexpected ban without vacation execs: %+v", runner.execCalls)
	}
}

func TestAdminRepositoryMutatesAttackBans(t *testing.T) {
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "admin"})},
		{rows: fakeRowsFromValues([]any{77, "target"})},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
	repository.now = func() time.Time { return time.Unix(1_000, 0) }

	_, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
		PlayerID:  42,
		Mode:      "Bans",
		Action:    "ban",
		TargetIDs: []int{77},
		BanMode:   2,
		Hours:     1,
	})

	if err != nil {
		t.Fatalf("MutateAdmin returned error: %v", err)
	}
	if len(runner.execCalls) != 4 || runner.execCalls[1].args[0] != "AllowAttacks" || !strings.Contains(runner.execCalls[3].sql, "noattack = 1") {
		t.Fatalf("unexpected attack ban execs: %+v", runner.execCalls)
	}
}

func TestAdminRepositoryUnbansUsers(t *testing.T) {
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "admin"})},
		{rows: fakeRowsFromValues([]any{77, "target"})},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")

	issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
		PlayerID:  42,
		Mode:      "Bans",
		Action:    "ban",
		TargetIDs: []int{77},
		BanMode:   3,
	})

	if err != nil {
		t.Fatalf("MutateAdmin returned error: %v", err)
	}
	if issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 2 {
		t.Fatalf("unexpected unban issue=%+v execs=%+v", issue, runner.execCalls)
	}
	if runner.execCalls[0].args[0] != "UnbanPlayer" || !strings.Contains(runner.execCalls[1].sql, "banned = 0") {
		t.Fatalf("unexpected unban execs: %+v", runner.execCalls)
	}
}

func TestAdminRepositoryAllowsAttacks(t *testing.T) {
	runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{42, "admin"})},
		{rows: fakeRowsFromValues([]any{77, "target"})},
	}}}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")

	issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
		PlayerID:  42,
		Mode:      "Bans",
		Action:    "ban",
		TargetIDs: []int{77},
		BanMode:   4,
	})

	if err != nil {
		t.Fatalf("MutateAdmin returned error: %v", err)
	}
	if issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 2 {
		t.Fatalf("unexpected allow attacks issue=%+v execs=%+v", issue, runner.execCalls)
	}
	if runner.execCalls[0].args[0] != "AllowAttacks" || !strings.Contains(runner.execCalls[1].sql, "noattack = 0") {
		t.Fatalf("unexpected allow attacks execs: %+v", runner.execCalls)
	}
}

func TestAdminRepositoryMutationNoops(t *testing.T) {
	runner := &fakeGalaxyRunner{}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")

	issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Users", Action: "ban"})
	if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
		t.Fatalf("unexpected non-bans mutation result issue=%+v execs=%+v err=%v", issue, runner.execCalls, err)
	}

	issue, err = repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Bans", Action: "ban"})
	if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
		t.Fatalf("unexpected empty target mutation result issue=%+v execs=%+v err=%v", issue, runner.execCalls, err)
	}
}

func TestAdminRepositoryMutatesQueueControls(t *testing.T) {
	t.Run("end", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		repository.now = func() time.Time { return time.Unix(1_000, 0) }

		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Queue",
			Action: domaingame.AdminActionQueueEnd,
			TaskID: 1001,
		})

		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 1 {
			t.Fatalf("unexpected queue end issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
		if !strings.Contains(runner.execCalls[0].sql, "UPDATE `ogame_queue` SET end = ?") ||
			runner.execCalls[0].args[0] != 1_000 || runner.execCalls[0].args[1] != 1001 {
			t.Fatalf("unexpected queue end exec: %+v", runner.execCalls[0])
		}
	})

	t.Run("freeze", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		repository.now = func() time.Time { return time.Unix(2_000, 0) }

		_, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Queue",
			Action: domaingame.AdminActionQueueFreeze,
			TaskID: 1002,
		})

		if err != nil || len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "freeze = 1, frozen = ?") ||
			runner.execCalls[0].args[0] != 2_000 || runner.execCalls[0].args[1] != 1002 {
			t.Fatalf("unexpected queue freeze err=%v execs=%+v", err, runner.execCalls)
		}
	})

	t.Run("unfreeze extends end", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{1, 2_000, 5_000})},
		}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		repository.now = func() time.Time { return time.Unix(2_300, 0) }

		_, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Queue",
			Action: domaingame.AdminActionQueueUnfreeze,
			TaskID: 1003,
		})

		if err != nil || len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "SET freeze = 0, frozen = 0, end = ?") ||
			runner.execCalls[0].args[0] != 5_300 || runner.execCalls[0].args[1] != 1003 {
			t.Fatalf("unexpected queue unfreeze err=%v execs=%+v", err, runner.execCalls)
		}
	})

	t.Run("remove", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")

		_, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Queue",
			Action: domaingame.AdminActionQueueRemove,
			TaskID: 1004,
		})

		if err != nil || len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "DELETE FROM `ogame_queue` WHERE task_id = ?") ||
			runner.execCalls[0].args[0] != 1004 {
			t.Fatalf("unexpected queue remove err=%v execs=%+v", err, runner.execCalls)
		}
	})

	t.Run("missing task noops", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueFreeze})
		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
			t.Fatalf("expected missing queue task no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
	})
}

func TestAdminRepositoryQueueMutationEdges(t *testing.T) {
	t.Run("invalid prefix", func(t *testing.T) {
		repository := NewAdminRepositoryWithQueryer(&fakeGalaxyRunner{}, "bad-prefix_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueEnd, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected queue prefix error, got %v", err)
		}
	})

	t.Run("exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{execErrs: []error{errors.New("queue exec failed")}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueEnd, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "queue exec failed") {
			t.Fatalf("expected queue exec error, got %v", err)
		}
	})

	t.Run("unknown action noops", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: "unknown", TaskID: 1001})
		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
			t.Fatalf("expected unknown queue action no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
	})

	t.Run("unfreeze query error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("unfreeze query failed")}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueUnfreeze, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "unfreeze query failed") {
			t.Fatalf("expected unfreeze query error, got %v", err)
		}
	})

	t.Run("unfreeze missing row noops", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueUnfreeze, TaskID: 1001})
		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
			t.Fatalf("expected missing unfreeze row no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
	})

	t.Run("unfreeze rows error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("unfreeze rows failed"))}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueUnfreeze, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "unfreeze rows failed") {
			t.Fatalf("expected unfreeze rows error, got %v", err)
		}
	})

	t.Run("unfreeze scan error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueUnfreeze, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected unfreeze scan error, got %v", err)
		}
	})

	t.Run("unfreeze not frozen noops", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0, 0, 5_000})}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueUnfreeze, TaskID: 1001})
		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
			t.Fatalf("expected not frozen unfreeze no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
	})

	t.Run("unfreeze exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1, 2_000, 5_000})}}},
			execErrs:    []error{errors.New("unfreeze update failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		repository.now = func() time.Time { return time.Unix(2_300, 0) }
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Queue", Action: domaingame.AdminActionQueueUnfreeze, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "unfreeze update failed") {
			t.Fatalf("expected unfreeze update error, got %v", err)
		}
	})
}

func TestAdminRepositoryMutatesFleetlogControls(t *testing.T) {
	t.Run("two minutes updates single fleet queue", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{0})},
		}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		repository.now = func() time.Time { return time.Unix(1_000, 0) }

		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Fleetlogs",
			Action: domaingame.AdminActionFleetlogsTwoMinutes,
			TaskID: 1001,
		})

		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 1 {
			t.Fatalf("unexpected fleetlogs 2m issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
		if !strings.Contains(runner.execCalls[0].sql, "UPDATE `ogame_queue` SET end = ? WHERE task_id = ? AND type = ?") ||
			runner.execCalls[0].args[0] != 1_120 || runner.execCalls[0].args[1] != 1001 || runner.execCalls[0].args[2] != queueTypeFleet {
			t.Fatalf("unexpected fleetlogs 2m exec: %+v", runner.execCalls[0])
		}
	})

	t.Run("finish updates union queues", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{77})},
		}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		repository.now = func() time.Time { return time.Unix(2_000, 0) }

		_, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Fleetlogs",
			Action: domaingame.AdminActionFleetlogsEnd,
			TaskID: 1002,
		})

		if err != nil || len(runner.execCalls) != 1 || !strings.Contains(runner.execCalls[0].sql, "JOIN `ogame_fleet`") ||
			runner.execCalls[0].args[0] != 2_000 || runner.execCalls[0].args[1] != queueTypeFleet || runner.execCalls[0].args[2] != 77 {
			t.Fatalf("unexpected fleetlogs union finish err=%v execs=%+v", err, runner.execCalls)
		}
	})

	t.Run("return is explicit no-op until recall parity", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Fleetlogs",
			Action: domaingame.AdminActionFleetlogsReturn,
			TaskID: 1003,
		})
		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
			t.Fatalf("expected fleetlogs return no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
	})

	t.Run("missing task noops", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Fleetlogs", Action: domaingame.AdminActionFleetlogsEnd})
		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
			t.Fatalf("expected missing fleetlog task no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
	})
}

func TestAdminRepositoryFleetlogMutationEdges(t *testing.T) {
	t.Run("invalid fleet prefix", func(t *testing.T) {
		repository := NewAdminRepositoryWithQueryer(&fakeGalaxyRunner{}, "bad-prefix_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Fleetlogs", Action: domaingame.AdminActionFleetlogsEnd, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected fleetlogs prefix error, got %v", err)
		}
	})

	t.Run("load union query error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("fleetlog union failed")}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Fleetlogs", Action: domaingame.AdminActionFleetlogsEnd, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "fleetlog union failed") {
			t.Fatalf("expected fleetlog union error, got %v", err)
		}
	})

	t.Run("load union rows error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("fleetlog rows failed"))}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Fleetlogs", Action: domaingame.AdminActionFleetlogsEnd, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "fleetlog rows failed") {
			t.Fatalf("expected fleetlog rows error, got %v", err)
		}
	})

	t.Run("load union scan error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Fleetlogs", Action: domaingame.AdminActionFleetlogsEnd, TaskID: 1001}); err != nil {
			t.Fatalf("empty fleetlog row set should be no-op, got %v", err)
		}

		runner = &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}}
		repository = NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Fleetlogs", Action: domaingame.AdminActionFleetlogsEnd, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "expected int") {
			t.Fatalf("expected fleetlog scan conversion error, got %v", err)
		}
	})

	t.Run("exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}},
			execErrs:    []error{errors.New("fleetlog update failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Fleetlogs", Action: domaingame.AdminActionFleetlogsEnd, TaskID: 1001}); err == nil || !strings.Contains(err.Error(), "fleetlog update failed") {
			t.Fatalf("expected fleetlog update error, got %v", err)
		}
	})
}

func TestAdminRepositoryMutatesExpeditionSettings(t *testing.T) {
	runner := &fakeGalaxyRunner{}
	repository := NewAdminRepositoryWithQueryer(runner, "ogame_")

	issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
		Mode:   "Expedition",
		Action: "settings",
		Values: map[string]int{
			"chance_success": 77,
			"ignored":        1,
			"limit_max":      12345,
		},
	})

	if err != nil {
		t.Fatalf("MutateAdmin returned error: %v", err)
	}
	if issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 1 {
		t.Fatalf("unexpected expedition settings issue=%+v execs=%+v", issue, runner.execCalls)
	}
	if !strings.Contains(runner.execCalls[0].sql, "UPDATE `ogame_exptab` SET `chance_success` = ?, `limit_max` = ?") ||
		len(runner.execCalls[0].args) != 2 || runner.execCalls[0].args[0] != 77 || runner.execCalls[0].args[1] != 12345 {
		t.Fatalf("unexpected expedition settings update: %+v", runner.execCalls[0])
	}
}

func TestAdminRepositoryExpeditionSettingsEdges(t *testing.T) {
	t.Run("no allowed values", func(t *testing.T) {
		runner := &fakeGalaxyRunner{}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Expedition",
			Action: "settings",
			Values: map[string]int{"ignored": 1},
		})
		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
			t.Fatalf("expected expedition settings no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
	})

	t.Run("exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{execErrs: []error{errors.New("expedition update failed")}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{
			Mode:   "Expedition",
			Action: "settings",
			Values: map[string]int{"dm_factor": 9},
		}); err == nil || !strings.Contains(err.Error(), "expedition update failed") {
			t.Fatalf("expected expedition update error, got %v", err)
		}
	})
}

func TestAdminRepositoryMutationRequiresWriter(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_")
	if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Bans", Action: "ban"}); err == nil || !strings.Contains(err.Error(), "mutation unavailable") {
		t.Fatalf("expected mutation unavailable error, got %v", err)
	}
}

func TestAdminRepositoryMutationEdges(t *testing.T) {
	t.Run("invalid prefix", func(t *testing.T) {
		repository := NewAdminRepositoryWithQueryer(&fakeGalaxyRunner{}, "bad-prefix_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Bans", Action: "ban", TargetIDs: []int{77}}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected prefix error, got %v", err)
		}
	})

	t.Run("invalid expedition prefix", func(t *testing.T) {
		repository := NewAdminRepositoryWithQueryer(&fakeGalaxyRunner{}, "bad-prefix_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{Mode: "Expedition", Action: "settings", Values: map[string]int{"dm_factor": 9}}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
			t.Fatalf("expected expedition prefix error, got %v", err)
		}
	})

	t.Run("actor query error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: errors.New("actor failed")}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}}); err == nil || !strings.Contains(err.Error(), "actor failed") {
			t.Fatalf("expected actor query error, got %v", err)
		}
		if len(runner.execCalls) != 0 {
			t.Fatalf("query failure must not mutate, got %+v", runner.execCalls)
		}
	})

	t.Run("actor rows error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("actor rows failed"))}}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}}); err == nil || !strings.Contains(err.Error(), "actor rows failed") {
			t.Fatalf("expected actor rows error, got %v", err)
		}
	})

	t.Run("missing target", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{42, "admin"})},
			{rows: fakeRowsFromValues()},
		}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		issue, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}})
		if err != nil || issue == nil || issue.Code != domaingame.AdminIssueActionSaved || len(runner.execCalls) != 0 {
			t.Fatalf("expected missing target no-op, issue=%+v err=%v execs=%+v", issue, err, runner.execCalls)
		}
	})

	t.Run("target scan error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{42, "admin"})},
			{rows: fakeRowsFromValues([]any{77})},
		}}}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}}); err == nil || !strings.Contains(err.Error(), "unexpected scan destination count") {
			t.Fatalf("expected target scan error, got %v", err)
		}
	})

	t.Run("pranger exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, "admin"})},
				{rows: fakeRowsFromValues([]any{77, "target"})},
			}},
			execErrs: []error{errors.New("pranger failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}, BanMode: 1}); err == nil || !strings.Contains(err.Error(), "pranger failed") {
			t.Fatalf("expected pranger exec error, got %v", err)
		}
	})

	t.Run("queue insert exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, "admin"})},
				{rows: fakeRowsFromValues([]any{77, "target"})},
			}},
			execErrs: []error{nil, nil, errors.New("queue insert failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}, BanMode: 2}); err == nil || !strings.Contains(err.Error(), "queue insert failed") {
			t.Fatalf("expected queue insert error, got %v", err)
		}
	})

	t.Run("update exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, "admin"})},
				{rows: fakeRowsFromValues([]any{77, "target"})},
			}},
			execErrs: []error{nil, nil, nil, errors.New("update failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}, BanMode: 0}); err == nil || !strings.Contains(err.Error(), "update failed") {
			t.Fatalf("expected update error, got %v", err)
		}
	})

	t.Run("ban queue delete exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, "admin"})},
				{rows: fakeRowsFromValues([]any{77, "target"})},
			}},
			execErrs: []error{nil, errors.New("delete failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}, BanMode: 0}); err == nil || !strings.Contains(err.Error(), "delete failed") {
			t.Fatalf("expected queue delete error, got %v", err)
		}
	})

	t.Run("ban queue insert exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, "admin"})},
				{rows: fakeRowsFromValues([]any{77, "target"})},
			}},
			execErrs: []error{nil, nil, errors.New("ban queue insert failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}, BanMode: 0}); err == nil || !strings.Contains(err.Error(), "ban queue insert failed") {
			t.Fatalf("expected ban queue insert error, got %v", err)
		}
	})

	t.Run("attack queue delete exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, "admin"})},
				{rows: fakeRowsFromValues([]any{77, "target"})},
			}},
			execErrs: []error{nil, errors.New("attack delete failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}, BanMode: 2}); err == nil || !strings.Contains(err.Error(), "attack delete failed") {
			t.Fatalf("expected attack queue delete error, got %v", err)
		}
	})

	t.Run("unban queue delete exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, "admin"})},
				{rows: fakeRowsFromValues([]any{77, "target"})},
			}},
			execErrs: []error{errors.New("unban delete failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}, BanMode: 3}); err == nil || !strings.Contains(err.Error(), "unban delete failed") {
			t.Fatalf("expected unban queue delete error, got %v", err)
		}
	})

	t.Run("allow attacks queue delete exec error", func(t *testing.T) {
		runner := &fakeGalaxyRunner{
			fakeQueryer: fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42, "admin"})},
				{rows: fakeRowsFromValues([]any{77, "target"})},
			}},
			execErrs: []error{errors.New("allow attacks delete failed")},
		}
		repository := NewAdminRepositoryWithQueryer(runner, "ogame_")
		if _, err := repository.MutateAdmin(context.Background(), appgame.AdminMutationQuery{PlayerID: 42, Mode: "Bans", Action: "ban", TargetIDs: []int{77}, BanMode: 4}); err == nil || !strings.Contains(err.Error(), "allow attacks delete failed") {
			t.Fatalf("expected allow attacks queue delete error, got %v", err)
		}
	})
}

func TestAdminRepositoryReadsAdminDebugRows(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{10001, 77, "", "127.0.0.1", "Chrome", "Debug text", int64(1700000000)})},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Debug"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.MessageRows) != 1 || admin.MessageRows[0].ID != 10001 || admin.MessageRows[0].OwnerID != 77 || admin.MessageRows[0].Text != "Debug text" {
		t.Fatalf("unexpected debug rows: %+v", admin.MessageRows)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_debug`") || !strings.Contains(lastSQL, "LEFT JOIN `ogame_users`") || !strings.Contains(lastSQL, "m.error_id DESC") {
		t.Fatalf("expected debug rows query, got %s", lastSQL)
	}
}

func TestAdminRepositoryReadsAdminErrorRows(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{20001, 0, "", "127.0.0.1", "Firefox", "Error text", int64(1700000001)})},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Errors"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.MessageRows) != 1 || admin.MessageRows[0].ID != 20001 || admin.MessageRows[0].Text != "Error text" {
		t.Fatalf("unexpected error rows: %+v", admin.MessageRows)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_errors`") || strings.Contains(lastSQL, "m.error_id DESC") {
		t.Fatalf("expected errors query without error id ordering, got %s", lastSQL)
	}
}

func TestAdminRepositoryReadsPlanetRows(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{501, "Colony", int64(1700000000), 1, 55, 7, 43, "owner", int64(1690000000), int64(1700003600), 1, 0, 0, 0},
		)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Planets"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.PlanetRows) != 1 || admin.PlanetRows[0].Name != "Colony" || admin.PlanetRows[0].Owner == nil || !admin.PlanetRows[0].Owner.Vacation {
		t.Fatalf("unexpected planet rows: %+v", admin.PlanetRows)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_planets`") || !strings.Contains(lastSQL, "LEFT JOIN `ogame_users`") || !strings.Contains(lastSQL, "ORDER BY p.date DESC LIMIT 25") {
		t.Fatalf("expected planet rows query, got %s", lastSQL)
	}
}

func TestAdminRepositoryReadsPlanetRowsWithoutOwner(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{502, "Abandoned", int64(1700000001), 1, 56, 8, 0, "", int64(0), int64(0), 0, 0, 0, 0},
		)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Planets"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.PlanetRows) != 1 || admin.PlanetRows[0].Owner != nil || admin.PlanetRows[0].Coordinates.System != 56 {
		t.Fatalf("unexpected ownerless planet rows: %+v", admin.PlanetRows)
	}
}

func TestAdminRepositoryReadsUniverseSettings(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{
			1, float64(128), float64(2), 9, 499, 1000, 5, 30, 0, 1, 1, 70, 10, 42, 0,
			"news one", "news two", int64(1700003600), int64(1700000000), "../cgi-bin/battle", "en", 3,
			"https://board.example", "https://discord.example", "", "", "", 1, 1000000, 0, 1000, 999, 60,
		})},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Uni"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if admin.Universe == nil || admin.Universe.Speed != 128 || !admin.Universe.RapidFire || !admin.Universe.PHPBattle || admin.Universe.ExtBoard != "https://board.example" {
		t.Fatalf("unexpected universe settings: %+v", admin.Universe)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_uni`") || !strings.Contains(lastSQL, "COALESCE(start_dm, 0)") || !strings.Contains(lastSQL, "LIMIT 1") {
		t.Fatalf("expected universe settings query, got %s", lastSQL)
	}
}

func TestAdminRepositoryReadsExpeditionSettings(t *testing.T) {
	values := make([]any, len(adminExpeditionColumns))
	for index := range values {
		values[index] = index + 1
	}
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(values)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Expedition"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if admin.Expedition["dm_factor"] != 1 || admin.Expedition["limit_max"] != len(adminExpeditionColumns) {
		t.Fatalf("unexpected expedition settings: %+v", admin.Expedition)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_exptab`") || !strings.Contains(lastSQL, "`dm_factor`") || !strings.Contains(lastSQL, "`limit_max`") {
		t.Fatalf("expected expedition settings query, got %s", lastSQL)
	}
}

func TestAdminRepositoryReadsBotStrategies(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{1, "backup"},
			[]any{2, "raider"},
		)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "BotEdit"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.BotStrategies) != 2 || admin.BotStrategies[0].Name != "backup" || admin.BotStrategies[1].ID != 2 {
		t.Fatalf("unexpected bot strategies: %+v", admin.BotStrategies)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_botstrat`") || !strings.Contains(lastSQL, "ORDER BY id ASC") {
		t.Fatalf("expected bot strategy query, got %s", lastSQL)
	}
}

func TestAdminRepositoryReadsUserLogRowsInLegacyDisplayOrder(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{2, 43, "later", int64(1700000100), "BUILD", "Later action"},
			[]any{1, 42, "earlier", int64(1700000000), "FLEET", "Earlier action"},
		)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "UserLogs"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.UserLogRows) != 2 || admin.UserLogRows[0].ID != 1 || admin.UserLogRows[1].ID != 2 {
		t.Fatalf("expected reversed user log display order, got %+v", admin.UserLogRows)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_userlogs`") || !strings.Contains(lastSQL, "ORDER BY l.date DESC") {
		t.Fatalf("expected userlogs query, got %s", lastSQL)
	}
}

func TestAdminRepositoryReadsQueueRows(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{21997, 99999, "space", "UpdateStats", 0, 0, 0, int64(1700000000), int64(1700003600), 510, 0, int64(0), ""},
			[]any{21994, 1, "Legor", "RecalcPoints", 0, 0, 0, int64(1700000000), int64(1700007200), 500, 1, int64(1700000100), ""},
			[]any{22001, 1, "Legor", "Build", 635, domaingame.BuildingMetalMine, 13, int64(1700000000), int64(1700007300), 20, 0, int64(0), "Overview Home"},
		)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Queue"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.QueueRows) != 3 || admin.QueueRows[0].Description != "Save old statistics" || admin.QueueRows[1].Description != "Recalculate statistics" {
		t.Fatalf("unexpected queue rows: %+v", admin.QueueRows)
	}
	if !admin.QueueRows[1].Freeze {
		t.Fatalf("expected frozen queue row: %+v", admin.QueueRows[1])
	}
	if admin.QueueRows[2].Description != "Building 'Metal Mine' (13) on planet <a>Overview Home</a>" {
		t.Fatalf("expected build queue description, got %+v", admin.QueueRows)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_queue`") || !strings.Contains(lastSQL, "LEFT JOIN `ogame_users`") || !strings.Contains(lastSQL, "LEFT JOIN `ogame_buildqueue`") || !strings.Contains(lastSQL, "LEFT JOIN `ogame_planets`") || !strings.Contains(lastSQL, "q.type <> ?") {
		t.Fatalf("expected queue rows query, got %s", lastSQL)
	}
}

func TestAdminRepositoryQueueRowsErrors(t *testing.T) {
	validRow := []any{21997, 99999, "space", "UpdateStats", 0, 0, 0, int64(1700000000), int64(1700003600), 510, 0, int64(0), ""}
	cases := []struct {
		name string
		row  fakeQueryResult
		want string
	}{
		{name: "query", row: fakeQueryResult{err: errors.New("queue query failed")}, want: "queue query failed"},
		{name: "scan", row: fakeQueryResult{rows: fakeRowsFromValues([]any{21997})}, want: "unexpected scan destination count"},
		{name: "rows", row: fakeQueryResult{rows: fakeRowsFromValuesWithErr(errors.New("queue rows failed"), validRow)}, want: "queue rows failed"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
				tt.row,
			)}
			repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

			_, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Queue"})

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func TestLegacyAdminQueueDescriptionCoversKnownTypes(t *testing.T) {
	cases := map[string]string{
		"UpdateStats":      "Save old statistics",
		"RecalcPoints":     "Recalculate statistics",
		"RecalcAllyPoints": "Recalculate alliance statistics",
		"AllowName":        "Allow name change",
		"ChangeEmail":      "Update permanent mail address",
		"UnloadAll":        "Unload all the players",
		"CleanDebris":      "Cleaning virtual debris",
		"CleanPlanets":     "Cleanup of destroyed planets",
		"CleanPlayers":     "Deleting inactive players and players put up for deletion",
		"UnbanPlayer":      "Unban a player",
		"AllowAttacks":     "Allow attacks",
	}
	for queueType, want := range cases {
		if got := legacyAdminQueueDescription(queueType, 0, 0, 0, ""); got != want {
			t.Fatalf("description for %s = %q, want %q", queueType, got, want)
		}
	}
	typeCases := map[string]string{
		queueTypeBuild:    "Building 'Metal Mine' (3) on planet <a>Home</a>",
		queueTypeDemolish: "Demolition of 'Metal Mine' (3) on planet <a>Home</a>",
		queueTypeShipyard: "Shipyard assignment: 'Small Cargo' (3) on planet <a>Home</a>",
		queueTypeResearch: "Research is underway 'Energy Technology' (3) from planet <a>Home</a>",
	}
	for queueType, want := range typeCases {
		objID := domaingame.BuildingMetalMine
		if queueType == queueTypeShipyard {
			objID = domaingame.FleetSmallCargo
		}
		if queueType == queueTypeResearch {
			objID = domaingame.ResearchEnergy
		}
		if got := legacyAdminQueueDescription(queueType, 12, objID, 3, "Home"); got != want {
			t.Fatalf("description for %s = %q, want %q", queueType, got, want)
		}
	}
	if got := legacyAdminQueueDescription("Custom", 12, 14, 3, ""); got != "Unknown task type (type=Custom, sub_id=12, obj_id=14, level=3)" {
		t.Fatalf("unexpected unknown description: %q", got)
	}
	if got := legacyAdminQueuePlanetLinkHTML(`A&B "Home"`); got != "<a>A&amp;B &#34;Home&#34;</a>" {
		t.Fatalf("unexpected escaped planet link: %q", got)
	}
}

func TestAdminRepositoryReadsBattleReportRows(t *testing.T) {
	queryer := &fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues([]any{42, "legor", domaingame.AdminLevelAdmin})},
		fakeQueryResult{rows: fakeRowsFromValues(
			[]any{1756, "source", "<a>Battle report</a>", "report", int64(1700003600)},
			[]any{1755, "source", "<a>Older report</a>", "report", int64(1700000000)},
		)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "BattleReport"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.BattleReports) != 2 || admin.BattleReports[0].ID != 1756 || admin.BattleReports[0].Title != "<a>Battle report</a>" {
		t.Fatalf("unexpected battle report rows: %+v", admin.BattleReports)
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_battledata`") || !strings.Contains(lastSQL, "ORDER BY date DESC") {
		t.Fatalf("expected battle reports query, got %s", lastSQL)
	}
}

func TestAdminRepositoryErrors(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{}, "bad-prefix_")
	if _, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected prefix error, got %v", err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{}, "ogame_")
	if _, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42}); err == nil {
		t.Fatal("expected overview query error")
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{err: errors.New("viewer failed")},
	)}, "ogame_")
	if _, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "viewer failed") {
		t.Fatalf("expected viewer query error, got %v", err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: append(shipyardOverviewResults(),
		fakeQueryResult{rows: fakeRowsFromValues()},
	)}, "ogame_")
	if _, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42}); err == nil || !strings.Contains(err.Error(), "admin viewer not found") {
		t.Fatalf("expected missing viewer error, got %v", err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("viewer rows failed"))}}}, "ogame_")
	if _, err := repository.loadAdminViewer(context.Background(), 42); err == nil || !strings.Contains(err.Error(), "viewer rows failed") {
		t.Fatalf("expected viewer rows error, got %v", err)
	}
}

func TestAdminRepositoryAdminLoaderEdgeCases(t *testing.T) {
	repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{100, "nohome", int64(1), int64(2), 1, 1, 1, 1, 0, "", 0, 0, 0})},
	}}, "ogame_")
	users, err := repository.queryAdminUsers(context.Background(), "SELECT users")
	if err != nil {
		t.Fatalf("queryAdminUsers returned error: %v", err)
	}
	if len(users) != 1 || users[0].HomePlanet != nil || !users[0].Vacation || !users[0].Banned || !users[0].NoAttack || !users[0].Disable {
		t.Fatalf("expected optional home planet and flags to map: %+v", users)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: errors.New("new users failed")}}}, "ogame_")
	if _, _, err := repository.loadAdminUsers(context.Background()); err == nil || !strings.Contains(err.Error(), "new users failed") {
		t.Fatalf("expected new users query error, got %v", err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
		{err: errors.New("active users failed")},
	}}, "ogame_")
	if _, _, err := repository.loadAdminUsers(context.Background()); err == nil || !strings.Contains(err.Error(), "active users failed") {
		t.Fatalf("expected active users query error, got %v", err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if _, err := repository.loadAdminUniverse(context.Background()); err == nil || !strings.Contains(err.Error(), "admin universe settings not found") {
		t.Fatalf("expected missing universe settings error, got %v", err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	if _, err := repository.loadAdminExpeditionSettings(context.Background()); err == nil || !strings.Contains(err.Error(), "admin expedition settings not found") {
		t.Fatalf("expected missing expedition settings error, got %v", err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("universe rows failed"))}}}, "ogame_")
	if _, err := repository.loadAdminUniverse(context.Background()); err == nil || !strings.Contains(err.Error(), "universe rows failed") {
		t.Fatalf("expected universe rows error, got %v", err)
	}

	repository = NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("expedition rows failed"))}}}, "ogame_")
	if _, err := repository.loadAdminExpeditionSettings(context.Background()); err == nil || !strings.Contains(err.Error(), "expedition rows failed") {
		t.Fatalf("expected expedition rows error, got %v", err)
	}
}

func TestAdminRepositoryAdminRowsErrEdges(t *testing.T) {
	tests := []struct {
		name string
		run  func(AdminRepository) error
	}{
		{
			name: "bot strategies",
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminBotStrategies(context.Background())
				return err
			},
		},
		{
			name: "messages",
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminMessageRows(context.Background(), "debug", true)
				return err
			},
		},
		{
			name: "user logs",
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminUserLogRows(context.Background())
				return err
			},
		},
		{
			name: "planet rows",
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminPlanetRows(context.Background())
				return err
			},
		},
		{
			name: "queue rows",
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminQueueRows(context.Background())
				return err
			},
		},
		{
			name: "battle reports",
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminBattleReports(context.Background())
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New(tt.name + " rows failed"))}}}, "ogame_")
			if err := tt.run(repository); err == nil || !strings.Contains(err.Error(), tt.name+" rows failed") {
				t.Fatalf("expected rows error for %s, got %v", tt.name, err)
			}
		})
	}
}

func TestAdminRepositoryAdminScanEdges(t *testing.T) {
	tests := []struct {
		name string
		rows *fakeRows
		run  func(AdminRepository) error
	}{
		{
			name: "bot strategies",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminBotStrategies(context.Background())
				return err
			},
		},
		{
			name: "messages",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminMessageRows(context.Background(), "debug", true)
				return err
			},
		},
		{
			name: "user logs",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminUserLogRows(context.Background())
				return err
			},
		},
		{
			name: "users",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, _, err := repository.loadAdminUsers(context.Background())
				return err
			},
		},
		{
			name: "planet rows",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminPlanetRows(context.Background())
				return err
			},
		},
		{
			name: "universe",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminUniverse(context.Background())
				return err
			},
		},
		{
			name: "expedition",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminExpeditionSettings(context.Background())
				return err
			},
		},
		{
			name: "queue rows",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminQueueRows(context.Background())
				return err
			},
		},
		{
			name: "battle reports",
			rows: fakeRowsFromValues([]any{"bad"}),
			run: func(repository AdminRepository) error {
				_, err := repository.loadAdminBattleReports(context.Background())
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewAdminRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: tt.rows}}}, "ogame_")
			if err := tt.run(repository); err == nil {
				t.Fatalf("expected scan error for %s", tt.name)
			}
		})
	}
}
