package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"

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
			[]any{21997, 99999, "space", "UpdateStats", 0, 0, 0, int64(1700000000), int64(1700003600), 510, 0, int64(0)},
			[]any{21994, 1, "Legor", "RecalcPoints", 0, 0, 0, int64(1700000000), int64(1700007200), 500, 1, int64(1700000100)},
		)},
	)}
	repository := NewAdminRepositoryWithQueryer(queryer, "ogame_")

	admin, err := repository.GetAdmin(context.Background(), appgame.AdminQuery{PlayerID: 42, PlanetID: 99, Mode: "Queue"})

	if err != nil {
		t.Fatalf("GetAdmin returned error: %v", err)
	}
	if len(admin.QueueRows) != 2 || admin.QueueRows[0].Description != "Save old statistics" || admin.QueueRows[1].Description != "Recalculate statistics" {
		t.Fatalf("unexpected queue rows: %+v", admin.QueueRows)
	}
	if !admin.QueueRows[1].Freeze {
		t.Fatalf("expected frozen queue row: %+v", admin.QueueRows[1])
	}
	lastSQL := queryer.calls[len(queryer.calls)-1].sql
	if !strings.Contains(lastSQL, "`ogame_queue`") || !strings.Contains(lastSQL, "LEFT JOIN `ogame_users`") || !strings.Contains(lastSQL, "q.type <> ?") {
		t.Fatalf("expected queue rows query, got %s", lastSQL)
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
		if got := legacyAdminQueueDescription(queueType, 0, 0, 0); got != want {
			t.Fatalf("description for %s = %q, want %q", queueType, got, want)
		}
	}
	if got := legacyAdminQueueDescription("Build", 12, 14, 3); got != "Unknown task type (type=Build, sub_id=12, obj_id=14, level=3)" {
		t.Fatalf("unexpected unknown description: %q", got)
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
