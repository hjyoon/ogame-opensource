package mysqlgame

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

func TestOverviewRepositoryReadsLegacyOverview(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 7, 99, 1, 0, 0, 0, 30, 7, 3, int64(2000000000)})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3, 12800, 19, 12, 163, 1234.5, 234.5, 12.0, 0, 1, 2, 1, 1, 0, 3, 0, 2, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0})},
		{rows: fakeRowsFromValues(
			[]any{99, "Arakis", 1, 1, 2, 3},
			[]any{100, "Colony", 1, 1, 2, 4},
		)},
		{rows: fakeRowsFromValues(
			[]any{99, domaingame.BuildingMetalMine, 3, 0, int64(2000)},
			[]any{100, domaingame.BuildingCrystalMine, 4, 1, int64(2001)},
			[]any{100, domaingame.BuildingDeuteriumSynth, 5, 0, int64(2002)},
		)},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues([]any{4})},
		{rows: fakeRowsFromValues(
			overviewEventRow(11, 42, "legor", domaingame.FleetMissionTransport, map[int]int{domaingame.FleetSmallCargo: 2}, 100, 200, 3, 4),
			overviewEventRow(12, 77, "raider", domaingame.FleetMissionAttack, map[int]int{domaingame.FleetLightFighter: 5}, 110, 210, 4, 3),
			overviewMissileEventRow(13, 77, "raider", 4, domaingame.DefenseHeavyLaser, 120, 220, 4, 3),
		)},
		{rows: fakeRowsFromValues()},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	repository.includeUnread = true
	repository.includeBuildQueue = true
	repository.includeEvents = true
	repository.now = func() time.Time { return time.Date(2026, 6, 19, 15, 23, 7, 0, time.UTC) }

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
	if err != nil {
		t.Fatal(err)
	}

	if overview.Commander != "legor" || overview.Score.RawScore != 123456 || overview.Score.Rank != 7 || overview.Score.DisplayPoints() != 123 {
		t.Fatalf("unexpected score overview: %+v", overview)
	}
	if overview.ServerTime != "Fri Jun 19 18:23:07" {
		t.Fatalf("expected legacy server time, got %q", overview.ServerTime)
	}
	if overview.CurrentPlanet.ID != 99 || overview.CurrentPlanet.Name != "Arakis" || overview.CurrentPlanet.Coordinates.Position != 3 {
		t.Fatalf("unexpected current planet: %+v", overview.CurrentPlanet)
	}
	if overview.CurrentPlanet.Resources.Metal != 1234.5 || overview.CurrentPlanet.Resources.Crystal != 234.5 || overview.CurrentPlanet.Resources.Deuterium != 12 {
		t.Fatalf("unexpected resources: %+v", overview.CurrentPlanet.Resources)
	}
	if overview.CurrentPlanet.Resources.DarkMatter != 37 ||
		overview.CurrentPlanet.Resources.Energy != 140 ||
		overview.CurrentPlanet.Resources.EnergyCapacity != 162 {
		t.Fatalf("unexpected premium or energy resources: %+v", overview.CurrentPlanet.Resources)
	}
	if overview.CurrentPlanet.Resources.MetalCapacity != 100000 ||
		overview.CurrentPlanet.Resources.CrystalCapacity != 150000 ||
		overview.CurrentPlanet.Resources.DeuteriumCapacity != 200000 {
		t.Fatalf("unexpected resource capacities: %+v", overview.CurrentPlanet.Resources)
	}
	if overview.Score.UniversePlayers != 2 {
		t.Fatalf("expected universe player count, got %+v", overview.Score)
	}
	if overview.UnreadMessages != 4 {
		t.Fatalf("expected unread messages, got %d", overview.UnreadMessages)
	}
	if len(overview.Events) != 4 ||
		overview.Events[0].MissionName != "Transport" ||
		overview.Events[0].TotalShips != 2 ||
		overview.Events[0].StateShort != "(G)" ||
		overview.Events[0].OriginName != "Origin" ||
		overview.Events[0].TargetName != "Target" ||
		overview.Events[0].Foreign {
		t.Fatalf("unexpected overview events: %+v", overview.Events)
	}
	if overview.Events[1].OwnerID != 77 ||
		overview.Events[1].OwnerName != "raider" ||
		!overview.Events[1].Foreign ||
		overview.Events[1].MissionName != "Attack" ||
		overview.Events[1].TotalShips != 5 ||
		overview.Events[1].CanRecall {
		t.Fatalf("unexpected incoming overview event: %+v", overview.Events[1])
	}
	if overview.Events[2].MissionName != "Missile Attack" ||
		overview.Events[2].MissileAmount != 4 ||
		overview.Events[2].MissileTargetID != domaingame.DefenseHeavyLaser ||
		overview.Events[2].MissileTarget != "Heavy Laser" ||
		!overview.Events[2].Foreign ||
		overview.Events[2].CanRecall {
		t.Fatalf("unexpected missile overview event: %+v", overview.Events[2])
	}
	if overview.Events[3].MissionName != "Transport" ||
		overview.Events[3].StateShort != "(F)" ||
		overview.Events[3].Origin.Position != 4 ||
		overview.Events[3].Target.Position != 3 ||
		overview.Events[3].OriginName != "Target" ||
		overview.Events[3].TargetName != "Origin" ||
		overview.Events[3].CanRecall {
		t.Fatalf("unexpected own return pseudo-event: %+v", overview.Events[3])
	}
	if overview.CurrentPlanet.BuildQueue == nil ||
		overview.CurrentPlanet.BuildQueue.Name != "Metal Mine" ||
		overview.CurrentPlanet.BuildQueue.Level != 3 ||
		overview.CurrentPlanet.BuildQueue.Destroy {
		t.Fatalf("unexpected current build queue: %+v", overview.CurrentPlanet.BuildQueue)
	}
	if len(overview.PlanetSwitcher) != 2 || !overview.PlanetSwitcher[0].Current || overview.PlanetSwitcher[1].Current {
		t.Fatalf("unexpected planet switcher: %+v", overview.PlanetSwitcher)
	}
	if overview.PlanetSwitcher[0].BuildQueue == nil || overview.PlanetSwitcher[0].BuildQueue.Name != "Metal Mine" {
		t.Fatalf("expected current switcher build queue, got %+v", overview.PlanetSwitcher[0].BuildQueue)
	}
	if overview.PlanetSwitcher[1].BuildQueue == nil ||
		overview.PlanetSwitcher[1].BuildQueue.Name != "Crystal Mine" ||
		overview.PlanetSwitcher[1].BuildQueue.Level != 4 ||
		!overview.PlanetSwitcher[1].BuildQueue.Destroy {
		t.Fatalf("unexpected planet switcher build queue: %+v", overview.PlanetSwitcher[1].BuildQueue)
	}
	if !strings.Contains(queryer.calls[2].sql, "ORDER BY planet_id ASC, type DESC") {
		t.Fatalf("expected legacy default planet order, got %q", queryer.calls[2].sql)
	}
	if !strings.Contains(queryer.calls[3].sql, "FROM `ogame_buildqueue`") || !strings.Contains(queryer.calls[3].sql, "ORDER BY planet_id ASC, list_id ASC") {
		t.Fatalf("expected overview buildqueue query, got %+v", queryer.calls[3])
	}
	if !strings.Contains(queryer.calls[6].sql, "FROM `ogame_queue`") || !strings.Contains(queryer.calls[6].sql, "JOIN `ogame_fleet`") {
		t.Fatalf("expected overview event query, got %+v", queryer.calls[6])
	}
	if !strings.Contains(queryer.calls[6].sql, "owner_id = ? AND type < ?") || !strings.Contains(queryer.calls[6].sql, "COALESCE(f.union_id, 0) = 0") {
		t.Fatalf("expected incoming non-ACS event filter, got %+v", queryer.calls[6])
	}
	if !strings.Contains(queryer.calls[0].sql, "`ogame_users`") || !strings.Contains(queryer.calls[1].sql, "`ogame_planets`") {
		t.Fatalf("expected prefixed table names, got %+v", queryer.calls)
	}
}

func TestOverviewRepositoryFinishesDueBuildingQueuesBeforeRead(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{128.0, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")
	repository.updateResources = false
	repository.includeBuildQueue = true
	repository.now = func() time.Time { return time.Unix(2000, 0) }

	if _, err := repository.GetOverview(context.Background(), overviewQuery(42, 0)); err != nil {
		t.Fatal(err)
	}

	if len(runner.calls) < 2 ||
		!strings.Contains(runner.calls[0].sql, "FROM `ogame_users`") ||
		!strings.Contains(runner.calls[1].sql, "SELECT speed, freeze FROM `ogame_uni`") ||
		!strings.Contains(runner.calls[2].sql, "WHERE end <= ?") ||
		runner.calls[2].args[0] != 2000 {
		t.Fatalf("expected due building queue flush before overview read, got %+v", runner.calls)
	}
}

func TestOverviewRepositorySkipsDueBuildingQueuesForAdmins(t *testing.T) {
	runner := &fakeBuildingsRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 2})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{99, domaingame.BuildingMetalMine, 1, 0, int64(1990)})},
		{rows: fakeRowsFromValues([]any{1})},
	}}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")
	repository.updateResources = false
	repository.includeBuildQueue = true
	repository.now = func() time.Time { return time.Unix(2000, 0) }

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
	if err != nil {
		t.Fatal(err)
	}

	if overview.CurrentPlanet.BuildQueue == nil || overview.CurrentPlanet.BuildQueue.End != 1990 {
		t.Fatalf("expected admin overview to keep stale build queue, got %+v", overview.CurrentPlanet.BuildQueue)
	}
	for _, call := range runner.calls {
		if strings.Contains(call.sql, "SELECT speed, freeze FROM `ogame_uni`") || strings.Contains(call.sql, "WHERE end <= ?") {
			t.Fatalf("admin overview should not finish due queues, got %+v", runner.calls)
		}
	}
}

func TestOverviewRepositoryAddsAdminNotice(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 2})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3, 12800, 19, 12, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues([]any{0})},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))

	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Messages) != 1 || overview.Messages[0] != domaingame.OverviewAdminNotice {
		t.Fatalf("expected admin notice, got %+v", overview.Messages)
	}
	if !strings.Contains(queryer.calls[0].sql, "admin") {
		t.Fatalf("expected admin column in overview user query, got %q", queryer.calls[0].sql)
	}
}

func TestNewOverviewRepositoryKeepsSQLQueryer(t *testing.T) {
	repository := NewOverviewRepository(nil, "ogame_")

	if repository.prefix != "ogame_" {
		t.Fatalf("unexpected prefix: %q", repository.prefix)
	}
	if _, ok := repository.queryer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL queryer, got %T", repository.queryer)
	}
	if _, ok := repository.execer.(SQLQueryer); !ok {
		t.Fatalf("expected SQL execer, got %T", repository.execer)
	}
	if !repository.updateResources || !repository.includeUnread || !repository.includeBuildQueue || !repository.includeEvents {
		t.Fatalf("expected production overview repository to update resources and overview side data")
	}
}

func TestSQLQueryerUsesDatabase(t *testing.T) {
	db := openOverviewTestDB(t)
	defer db.Close()

	rows, err := (SQLQueryer{DB: db}).QueryContext(context.Background(), "SELECT value")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected fake row")
	}
	var value int
	if err := rows.Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != 1 {
		t.Fatalf("unexpected fake value: %d", value)
	}

	result, err := (SQLQueryer{DB: db}).ExecContext(context.Background(), "UPDATE value SET value = ?", 2)
	if err != nil {
		t.Fatal(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("unexpected affected rows: %d", affected)
	}
}

func TestOverviewRepositoryFallsBackToHomePlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 999, 1, 1, 1, 0})},
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 2, 3, 4, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 2, 3, 4})},
		{rows: fakeRowsFromValues([]any{1})},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
	if err != nil {
		t.Fatal(err)
	}

	if overview.CurrentPlanet.ID != 1 || overview.CurrentPlanet.Name != "Homeworld" {
		t.Fatalf("expected home planet fallback, got %+v", overview.CurrentPlanet)
	}
	if !strings.Contains(queryer.calls[3].sql, "ORDER BY g DESC, s DESC, p DESC, type DESC") {
		t.Fatalf("expected coordinate sort fallback, got %q", queryer.calls[3].sql)
	}
}

func TestOverviewRepositoryUsesRequestedPlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 2, 0, 0})},
		{rows: fakeRowsFromValues([]any{100, "Colony", 1, 1, 2, 4, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3}, []any{100, "Colony", 1, 1, 2, 4})},
		{rows: fakeRowsFromValues()},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 100))
	if err != nil {
		t.Fatal(err)
	}

	if overview.CurrentPlanet.ID != 100 {
		t.Fatalf("expected requested planet, got %+v", overview.CurrentPlanet)
	}
	if got := queryer.calls[1].args[0]; got != 100 {
		t.Fatalf("expected requested planet id query arg, got %+v", queryer.calls[1].args)
	}
	if !strings.Contains(queryer.calls[2].sql, "ORDER BY name ASC, type DESC") {
		t.Fatalf("expected name sort order, got %q", queryer.calls[2].sql)
	}
	if overview.Score.UniversePlayers != 0 {
		t.Fatalf("expected missing universe player row to default to zero, got %+v", overview.Score)
	}
}

func TestOverviewRepositoryPersistsRequestedPlanet(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{100, "Colony", 1, 1, 2, 4, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Arakis", 1, 1, 2, 3}, []any{100, "Colony", 1, 1, 2, 4})},
		{rows: fakeRowsFromValues([]any{2})},
	}}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")

	overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 100))
	if err != nil {
		t.Fatal(err)
	}

	if overview.CurrentPlanet.ID != 100 {
		t.Fatalf("expected requested planet, got %+v", overview.CurrentPlanet)
	}
	if !strings.Contains(runner.execSQL, "UPDATE `ogame_users` SET aktplanet = ? WHERE player_id = ? LIMIT 1") {
		t.Fatalf("expected active planet update, got %q", runner.execSQL)
	}
	if len(runner.execArgs) != 2 || runner.execArgs[0] != 100 || runner.execArgs[1] != 42 {
		t.Fatalf("unexpected active planet update args: %+v", runner.execArgs)
	}
}

func TestOverviewRepositoryUpdatesActivityOnLoginMarker(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: overviewResultsForPlanet(99, "Arakis")}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")
	repository.now = func() time.Time { return time.Unix(1234, 0) }

	overview, err := repository.GetOverview(context.Background(), appgame.OverviewQuery{PlayerID: 42, PlanetID: 99, Login: true})

	if err != nil {
		t.Fatal(err)
	}
	if overview.CurrentPlanet.ID != 99 {
		t.Fatalf("expected login overview to load current planet, got %+v", overview.CurrentPlanet)
	}
	if len(runner.execCalls) != 1 {
		t.Fatalf("expected only activity update, got %+v", runner.execCalls)
	}
	call := runner.execCalls[0]
	if !strings.Contains(call.sql, "UPDATE `ogame_planets` SET lastakt = ? WHERE planet_id = ?") ||
		call.args[0] != int64(1234) || call.args[1] != 99 {
		t.Fatalf("unexpected activity update: %+v", call)
	}
}

func TestOverviewRepositoryActivityUpdateErrors(t *testing.T) {
	repository := NewOverviewRepositoryWithQueryer(&fakeQueryer{results: overviewResultsForPlanet(99, "Arakis")}, "ogame_")
	_, err := repository.GetOverview(context.Background(), appgame.OverviewQuery{PlayerID: 42, PlanetID: 99, Login: true})
	if err == nil || !strings.Contains(err.Error(), "activity updater unavailable") {
		t.Fatalf("expected missing activity updater error, got %v", err)
	}

	runner := &fakeOverviewRunner{
		fakeQueryer: fakeQueryer{results: overviewResultsForPlanet(99, "Arakis")},
		execErr:     errors.New("activity failed"),
	}
	repository = NewOverviewRepositoryWithRunner(runner, runner, "ogame_")
	_, err = repository.GetOverview(context.Background(), appgame.OverviewQuery{PlayerID: 42, PlanetID: 99, Login: true})
	if err == nil || !strings.Contains(err.Error(), "activity failed") {
		t.Fatalf("expected activity exec error, got %v", err)
	}
}

func TestOverviewRepositoryRenamesCurrentPlanet(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: append(overviewResultsForPlanet(99, "Arakis"),
		overviewResultsForPlanet(99, "New Colony")...,
	)}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")

	overview, err := repository.RenamePlanet(context.Background(), appgame.OverviewRenameQuery{
		PlayerID: 42,
		PlanetID: 99,
		Name:     ` New   Colony*" `,
	})
	if err != nil {
		t.Fatal(err)
	}
	if overview.CurrentPlanet.Name != "New Colony" {
		t.Fatalf("expected refreshed overview with renamed planet, got %+v", overview.CurrentPlanet)
	}
	if !strings.Contains(runner.execSQL, "UPDATE `ogame_planets` SET name = ? WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1") {
		t.Fatalf("expected legacy rename update, got %q", runner.execSQL)
	}
	if len(runner.execArgs) != 4 || runner.execArgs[0] != "New Colony" || runner.execArgs[1] != 99 || runner.execArgs[2] != 42 || runner.execArgs[3] != planetTypeDebris {
		t.Fatalf("unexpected rename args: %+v", runner.execArgs)
	}
}

func TestOverviewRepositoryRenameNoopsForForbiddenName(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: overviewResultsForPlanet(99, "Arakis")}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")

	overview, err := repository.RenamePlanet(context.Background(), appgame.OverviewRenameQuery{PlayerID: 42, PlanetID: 99, Name: "bad;name"})
	if err != nil {
		t.Fatal(err)
	}
	if overview.CurrentPlanet.Name != "Arakis" || runner.execSQL != "" {
		t.Fatalf("forbidden legacy name should not update, overview=%+v exec=%q", overview.CurrentPlanet, runner.execSQL)
	}
}

func TestOverviewRepositoryRenameErrors(t *testing.T) {
	repository := NewOverviewRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_")
	if _, err := repository.RenamePlanet(context.Background(), appgame.OverviewRenameQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected missing updater error, got %v", err)
	}

	runner := &fakeOverviewRunner{}
	repository = NewOverviewRepositoryWithRunner(runner, runner, "bad-prefix_")
	if _, err := repository.RenamePlanet(context.Background(), appgame.OverviewRenameQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}

	runner = &fakeOverviewRunner{
		fakeQueryer: fakeQueryer{results: overviewResultsForPlanet(99, "Arakis")},
		execErr:     errors.New("rename failed"),
	}
	repository = NewOverviewRepositoryWithRunner(runner, runner, "ogame_")
	if _, err := repository.RenamePlanet(context.Background(), appgame.OverviewRenameQuery{PlayerID: 42, PlanetID: 99, Name: "New Colony"}); err == nil || !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("expected rename exec error, got %v", err)
	}
}

func TestOverviewRepositoryDeletesColonyAndMoon(t *testing.T) {
	results := overviewResultsForPlanet(99, "Colony")
	results = append(results,
		fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues([]any{200, domaingame.PlanetTypeMoon, 1, 2, 3})},
		fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, map[int]int{domaingame.BuildingLunarBase: 1}, nil, nil))},
		fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, map[int]int{domaingame.BuildingMetalMine: 2}, map[int]int{domaingame.FleetSmallCargo: 3}, map[int]int{domaingame.DefenseRocketLauncher: 4}))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
	)
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOverviewRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret")
	repository.now = func() time.Time { return time.Unix(1000, 0) }

	overview, issue, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{
		PlayerID: 42,
		PlanetID: 99,
		DeleteID: 99,
		Password: "admin",
	})

	if err != nil {
		t.Fatal(err)
	}
	if issue != nil {
		t.Fatalf("expected successful delete without issue, got %+v", issue)
	}
	if overview.CurrentPlanet.ID != 1 || overview.CurrentPlanet.Name != "Homeworld" {
		t.Fatalf("expected refreshed home overview, got %+v", overview.CurrentPlanet)
	}
	if len(runner.execCalls) != 27 {
		t.Fatalf("expected moon/planet destroy, queue flush, and active planet update execs, got %+v", runner.execCalls)
	}
	first := runner.execCalls[0]
	if !strings.Contains(first.sql, "UPDATE `ogame_planets` SET type = ?") ||
		first.args[0] != planetTypeDestroyedMoon || first.args[1] != userSpace ||
		first.args[2] != int64(1000) || first.args[3] != int64(87400) || first.args[5] != 200 {
		t.Fatalf("unexpected moon destroy exec: %+v", first)
	}
	moonStats := runner.execCalls[4]
	if !strings.Contains(moonStats.sql, "score1 = score1 - ?") || moonStats.args[0] != int64(80000) || moonStats.args[1] != int64(0) || moonStats.args[3] != 42 {
		t.Fatalf("unexpected moon stats adjustment: %+v", moonStats)
	}
	planetDestroy := runner.execCalls[13]
	if planetDestroy.args[0] != planetTypeDestroyed || planetDestroy.args[5] != 99 {
		t.Fatalf("unexpected planet destroy exec: %+v", planetDestroy)
	}
	planetStats := runner.execCalls[17]
	if !strings.Contains(planetStats.sql, "score1 = score1 - ?") || planetStats.args[0] != int64(20187) || planetStats.args[1] != int64(3) || planetStats.args[3] != 42 {
		t.Fatalf("unexpected planet stats adjustment: %+v", planetStats)
	}
	last := runner.execCalls[len(runner.execCalls)-1]
	if !strings.Contains(last.sql, "UPDATE `ogame_users` SET aktplanet = ?") || last.args[0] != 1 || last.args[1] != 42 {
		t.Fatalf("expected active planet restore, got %+v", last)
	}
}

func TestOverviewRepositoryDeletesColonyWithoutReDeletingDestroyedMoon(t *testing.T) {
	results := overviewResultsForPlanet(99, "Colony")
	results = append(results,
		fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues([]any{200, planetTypeDestroyedMoon, 1, 2, 3})},
		fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
	)
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOverviewRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret")

	overview, issue, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{
		PlayerID: 42,
		PlanetID: 99,
		DeleteID: 99,
		Password: "admin",
	})

	if err != nil {
		t.Fatal(err)
	}
	if issue != nil || overview.CurrentPlanet.ID != 1 {
		t.Fatalf("expected colony delete to restore home, overview=%+v issue=%+v", overview.CurrentPlanet, issue)
	}
	if len(runner.execCalls) != 14 {
		t.Fatalf("destroyed moon marker should not be updated again, execs=%+v", runner.execCalls)
	}
	if runner.execCalls[0].args[0] != planetTypeDestroyed || runner.execCalls[0].args[5] != 99 {
		t.Fatalf("expected colony destroy first, got %+v", runner.execCalls[0])
	}
}

func TestOverviewRepositoryDeleteReturnsPasswordIssue(t *testing.T) {
	results := overviewResultsForPlanet(99, "Colony")
	results = append(results, fakeQueryResult{rows: fakeRowsFromValues()})
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOverviewRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret")

	overview, issue, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{
		PlayerID: 42,
		PlanetID: 99,
		DeleteID: 99,
		Password: "wrong",
	})

	if err != nil {
		t.Fatal(err)
	}
	if overview.CurrentPlanet.Name != "Colony" || issue == nil || issue.Code != domaingame.OverviewIssuePasswordInvalid || runner.execSQL != "" {
		t.Fatalf("expected wrong password no-op issue, overview=%+v issue=%+v exec=%q", overview.CurrentPlanet, issue, runner.execSQL)
	}
}

func TestOverviewRepositoryDeleteBlocksHomeAndFleet(t *testing.T) {
	tests := []struct {
		name    string
		results []fakeQueryResult
		want    string
	}{
		{
			name: "home planet",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42})},
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, domaingame.PlanetTypePlanet, 1, 1, 1})},
			},
			want: domaingame.OverviewIssueHomePlanet,
		},
		{
			name: "incoming fleet",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42})},
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				{rows: fakeRowsFromValues([]any{7})},
			},
			want: domaingame.OverviewIssueFleetIncoming,
		},
		{
			name: "outgoing fleet",
			results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{42})},
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues([]any{8})},
			},
			want: domaingame.OverviewIssueFleetOutgoing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := overviewResultsForPlanet(99, "Colony")
			results = append(results, tt.results...)
			runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: results}}
			repository := NewOverviewRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret")

			_, issue, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{
				PlayerID: 42,
				PlanetID: 99,
				DeleteID: 99,
				Password: "admin",
			})

			if err != nil {
				t.Fatal(err)
			}
			if issue == nil || issue.Code != tt.want || runner.execSQL != "" {
				t.Fatalf("expected %s issue without exec, issue=%+v exec=%q", tt.want, issue, runner.execSQL)
			}
		})
	}
}

func TestOverviewRepositoryDeleteNoopsForForeignPlanet(t *testing.T) {
	results := overviewResultsForPlanet(99, "Colony")
	results = append(results,
		fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues()},
	)
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOverviewRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret")

	overview, issue, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{
		PlayerID: 42,
		PlanetID: 99,
		DeleteID: 404,
		Password: "admin",
	})

	if err != nil {
		t.Fatal(err)
	}
	if issue != nil || overview.CurrentPlanet.ID != 99 || runner.execSQL != "" {
		t.Fatalf("expected foreign delete no-op, overview=%+v issue=%+v exec=%q", overview.CurrentPlanet, issue, runner.execSQL)
	}
}

func TestOverviewRepositoryDeleteErrors(t *testing.T) {
	repository := NewOverviewRepositoryWithRunner(&fakeQueryer{}, nil, "ogame_")
	if _, _, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{}); err == nil || !strings.Contains(err.Error(), "updater unavailable") {
		t.Fatalf("expected missing updater error, got %v", err)
	}

	runner := &fakeOverviewRunner{}
	repository = NewOverviewRepositoryWithRunner(runner, runner, "bad-prefix_")
	if _, _, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{}); err == nil || !strings.Contains(err.Error(), "invalid database table prefix") {
		t.Fatalf("expected unsafe prefix error, got %v", err)
	}

	for _, tt := range []struct {
		name      string
		results   []fakeQueryResult
		execErr   error
		execErrAt int
		want      string
	}{
		{
			name:    "overview",
			results: []fakeQueryResult{{err: errors.New("overview failed")}},
			want:    "overview failed",
		},
		{
			name:    "password query",
			results: append(overviewResultsForPlanet(99, "Colony"), fakeQueryResult{err: errors.New("password failed")}),
			want:    "password failed",
		},
		{
			name:    "password scan",
			results: append(overviewResultsForPlanet(99, "Colony"), fakeQueryResult{rows: fakeRowsFromValues([]any{"bad"})}),
			want:    "expected int",
		},
		{
			name: "user reload",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{err: errors.New("user failed")},
			),
			want: "user failed",
		},
		{
			name: "target query",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{err: errors.New("target failed")},
			),
			want: "target failed",
		},
		{
			name: "incoming query",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{err: errors.New("incoming failed")},
			),
			want: "incoming failed",
		},
		{
			name: "outgoing query",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{err: errors.New("outgoing failed")},
			),
			want: "outgoing failed",
		},
		{
			name: "moon query",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{err: errors.New("moon failed")},
			),
			want: "moon failed",
		},
		{
			name: "moon score query",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues([]any{200, domaingame.PlanetTypeMoon, 1, 2, 3})},
				fakeQueryResult{err: errors.New("moon score failed")},
			),
			want: "moon score failed",
		},
		{
			name: "target score query",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{err: errors.New("target score failed")},
			),
			want: "target score failed",
		},
		{
			name: "moon stats adjustment exec",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues([]any{200, domaingame.PlanetTypeMoon, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
			),
			execErr:   errors.New("moon stats failed"),
			execErrAt: 5,
			want:      "moon stats failed",
		},
		{
			name: "destroy exec",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
			),
			execErr: errors.New("destroy failed"),
			want:    "destroy failed",
		},
		{
			name: "queue flush exec",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
			),
			execErr:   errors.New("flush failed"),
			execErrAt: 2,
			want:      "flush failed",
		},
		{
			name: "queue build flush exec",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
			),
			execErr:   errors.New("build flush failed"),
			execErrAt: 3,
			want:      "build flush failed",
		},
		{
			name: "buildqueue flush exec",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
			),
			execErr:   errors.New("buildqueue flush failed"),
			execErrAt: 4,
			want:      "buildqueue flush failed",
		},
		{
			name: "stats adjustment exec",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
			),
			execErr:   errors.New("stats adjustment failed"),
			execErrAt: 5,
			want:      "stats adjustment failed",
		},
		{
			name: "rank recalculation exec",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
			),
			execErr:   errors.New("rank failed"),
			execErrAt: 6,
			want:      "rank failed",
		},
		{
			name: "active restore exec",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
			),
			execErr:   errors.New("active restore failed"),
			execErrAt: 14,
			want:      "active restore failed",
		},
		{
			name: "final overview",
			results: append(overviewResultsForPlanet(99, "Colony"),
				fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
				fakeQueryResult{rows: fakeRowsFromValues([]any{99, domaingame.PlanetTypePlanet, 1, 2, 3})},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues()},
				fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, nil, nil, nil))},
				fakeQueryResult{err: errors.New("final overview failed")},
			),
			want: "final overview failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeOverviewRunner{
				fakeQueryer: fakeQueryer{results: tt.results},
				execErr:     tt.execErr,
				execErrAt:   tt.execErrAt,
			}
			repository := NewOverviewRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret")
			_, _, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{
				PlayerID: 42,
				PlanetID: 99,
				DeleteID: 99,
				Password: "admin",
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestOverviewRepositoryScoreRemovalAndQueueFlushHelpersPropagateErrors(t *testing.T) {
	score := overviewPlanetScore{OwnerID: 42, Score: domaingame.PlanetScore{Points: 10, FleetPoints: 2}}
	adjustErr := errors.New("adjust failed")
	repository := OverviewRepository{execer: &fakeBuildingsRunner{execErr: adjustErr}}
	if err := repository.applyPlanetScoreRemoval(context.Background(), "`ogame_users`", score); !errors.Is(err, adjustErr) {
		t.Fatalf("expected adjust error, got %v", err)
	}

	recalcErr := errors.New("recalc failed")
	repository = OverviewRepository{execer: &fakeBuildingsRunner{execErrs: []error{nil, recalcErr}}}
	if err := repository.applyPlanetScoreRemoval(context.Background(), "`ogame_users`", score); !errors.Is(err, recalcErr) {
		t.Fatalf("expected recalc error, got %v", err)
	}

	firstFlushErr := errors.New("shipyard queue flush failed")
	repository = OverviewRepository{execer: &fakeBuildingsRunner{execErr: firstFlushErr}}
	if err := repository.flushPlanetQueue(context.Background(), "`ogame_queue`", "`ogame_buildqueue`", 99); !errors.Is(err, firstFlushErr) {
		t.Fatalf("expected first flush error, got %v", err)
	}

	secondFlushErr := errors.New("building queue flush failed")
	repository = OverviewRepository{execer: &fakeBuildingsRunner{execErrs: []error{nil, secondFlushErr}}}
	if err := repository.flushPlanetQueue(context.Background(), "`ogame_queue`", "`ogame_buildqueue`", 99); !errors.Is(err, secondFlushErr) {
		t.Fatalf("expected second flush error, got %v", err)
	}

	thirdFlushErr := errors.New("buildqueue flush failed")
	repository = OverviewRepository{execer: &fakeBuildingsRunner{execErrs: []error{nil, nil, thirdFlushErr}}}
	if err := repository.flushPlanetQueue(context.Background(), "`ogame_queue`", "`ogame_buildqueue`", 99); !errors.Is(err, thirdFlushErr) {
		t.Fatalf("expected third flush error, got %v", err)
	}
}

func TestOverviewRepositoryDeletesMoon(t *testing.T) {
	results := overviewResultsForPlanet(200, "Moon")
	results = append(results,
		fakeQueryResult{rows: fakeRowsFromValues([]any{42})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 200, 1, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{200, domaingame.PlanetTypeMoon, 1, 2, 3})},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues()},
		fakeQueryResult{rows: fakeRowsFromValues(overviewScoreRow(42, map[int]int{domaingame.BuildingLunarBase: 1}, nil, nil))},
		fakeQueryResult{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
		fakeQueryResult{rows: fakeRowsFromValues([]any{1})},
	)
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: results}}
	repository := NewOverviewRepositoryWithRunnerAndSecret(runner, runner, "ogame_", "secret")

	overview, issue, err := repository.DeletePlanet(context.Background(), appgame.OverviewDeleteQuery{
		PlayerID: 42,
		PlanetID: 200,
		DeleteID: 200,
		Password: "admin",
	})

	if err != nil {
		t.Fatal(err)
	}
	if issue != nil || overview.CurrentPlanet.ID != 1 {
		t.Fatalf("expected moon delete to restore home, overview=%+v issue=%+v", overview.CurrentPlanet, issue)
	}
	if runner.execCalls[0].args[0] != planetTypeDestroyedMoon || runner.execCalls[0].args[5] != 200 {
		t.Fatalf("expected moon destroy marker, got %+v", runner.execCalls[0])
	}
}

func TestOverviewRepositoryDeleteHelpers(t *testing.T) {
	repository := NewOverviewRepositoryWithRunner(&fakeOverviewRunner{}, nil, "ogame_")
	if !repository.currentTime().After(time.Time{}) {
		t.Fatal("expected default current time")
	}
	emptyRepository := OverviewRepository{}
	if !emptyRepository.currentTime().After(time.Time{}) {
		t.Fatal("expected nil clock to fall back to current time")
	}
	if scanDestinationCountError(nil) {
		t.Fatal("nil scan error should not be a destination-count error")
	}
	if !scanDestinationCountError(errors.New("sql: expected 4 destination arguments in Scan, not 3")) {
		t.Fatal("expected legacy destination arguments scan error to be detected")
	}
	if scanDestinationCountError(errors.New("expected int")) {
		t.Fatal("plain scan type errors should not be destination-count errors")
	}

	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("password rows failed"))}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.passwordMatches(context.Background(), "`ogame_users`", 42, "admin"); err == nil || !strings.Contains(err.Error(), "password rows failed") {
		t.Fatalf("expected password rows error, got %v", err)
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("password post scan failed"), []any{42})}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.passwordMatches(context.Background(), "`ogame_users`", 42, "admin"); err == nil || !strings.Contains(err.Error(), "password post scan failed") {
		t.Fatalf("expected password post-scan rows error, got %v", err)
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{41})}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if matched, err := repository.passwordMatches(context.Background(), "`ogame_users`", 42, "admin"); err != nil || matched {
		t.Fatalf("expected mismatched password row to fail, matched=%v err=%v", matched, err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 1, 2, 3, 4})}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadDeletePlanet(context.Background(), "`ogame_planets`", 42, 99); err == nil {
		t.Fatal("expected delete planet scan error")
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("delete empty rows failed"))}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadDeletePlanet(context.Background(), "`ogame_planets`", 42, 99); err == nil || !strings.Contains(err.Error(), "delete empty rows failed") {
		t.Fatalf("expected delete empty rows error, got %v", err)
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("delete rows failed"), []any{99, domaingame.PlanetTypePlanet, 1, 2, 3})}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadDeletePlanet(context.Background(), "`ogame_planets`", 42, 99); err == nil || !strings.Contains(err.Error(), "delete rows failed") {
		t.Fatalf("expected delete rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("moon rows failed"), []any{200, domaingame.PlanetTypeMoon, 1, 2, 3})}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadCoordinateMoon(context.Background(), "`ogame_planets`", domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}); err == nil || !strings.Contains(err.Error(), "moon rows failed") {
		t.Fatalf("expected moon rows error, got %v", err)
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("moon empty rows failed"))}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadCoordinateMoon(context.Background(), "`ogame_planets`", domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}); err == nil || !strings.Contains(err.Error(), "moon empty rows failed") {
		t.Fatalf("expected moon empty rows error, got %v", err)
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", domaingame.PlanetTypeMoon, 1, 2, 3})}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, _, err := repository.loadCoordinateMoon(context.Background(), "`ogame_planets`", domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3}); err == nil {
		t.Fatal("expected moon scan error")
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{err: errors.New("score query failed")}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadPlanetScore(context.Background(), "`ogame_planets`", 99); err == nil || !strings.Contains(err.Error(), "score query failed") {
		t.Fatalf("expected score query error, got %v", err)
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("score empty rows failed"))}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadPlanetScore(context.Background(), "`ogame_planets`", 99); err == nil || !strings.Contains(err.Error(), "score empty rows failed") {
		t.Fatalf("expected score empty rows error, got %v", err)
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadPlanetScore(context.Background(), "`ogame_planets`", 99); err == nil || !strings.Contains(err.Error(), "planet score not found") {
		t.Fatalf("expected missing score error, got %v", err)
	}
	scoreRow := overviewScoreRow(42, nil, nil, nil)
	scoreRow[0] = "bad"
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(scoreRow)}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadPlanetScore(context.Background(), "`ogame_planets`", 99); err == nil {
		t.Fatal("expected score scan error")
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("score rows failed"), overviewScoreRow(42, nil, nil, nil))}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.loadPlanetScore(context.Background(), "`ogame_planets`", 99); err == nil || !strings.Contains(err.Error(), "score rows failed") {
		t.Fatalf("expected score rows error, got %v", err)
	}

	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.fleetExists(context.Background(), "`ogame_fleet`", "start_planet = ?", 99); err == nil {
		t.Fatal("expected fleet scan error")
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("fleet empty rows failed"))}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.fleetExists(context.Background(), "`ogame_fleet`", "start_planet = ?", 99); err == nil || !strings.Contains(err.Error(), "fleet empty rows failed") {
		t.Fatalf("expected fleet empty rows error, got %v", err)
	}
	queryer = &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("fleet rows failed"), []any{7})}}}
	repository = NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	if _, err := repository.fleetExists(context.Background(), "`ogame_fleet`", "start_planet = ?", 99); err == nil || !strings.Contains(err.Error(), "fleet rows failed") {
		t.Fatalf("expected fleet rows error, got %v", err)
	}
}

func TestOverviewRepositoryMatchesLegacyPlanetSelectionEdges(t *testing.T) {
	t.Run("empty cp uses home when there is no active planet", func(t *testing.T) {
		repository := NewOverviewRepositoryWithQueryer(&fakeQueryer{}, "ogame_")
		planetID, current, persist, err := repository.resolveCurrentPlanet(context.Background(), "`ogame_planets`", overviewUser{HomePlanetID: 99}, appgame.OverviewQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if planetID != 99 || current.ID != 0 || persist {
			t.Fatalf("expected home fallback without persistence, planet=%d current=%+v persist=%v", planetID, current, persist)
		}
	})

	t.Run("foreign cp keeps previous selected planet", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 100, 99, 0, 0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues([]any{555})},
			{rows: fakeRowsFromValues([]any{100, "Colony", 1, 1, 2, 4, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
			{rows: fakeRowsFromValues([]any{99, "Home", 1, 1, 2, 3}, []any{100, "Colony", 1, 1, 2, 4})},
			{rows: fakeRowsFromValues([]any{2})},
		}}}
		repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")

		overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 555))
		if err != nil {
			t.Fatal(err)
		}
		if overview.CurrentPlanet.ID != 100 || runner.execSQL != "" {
			t.Fatalf("foreign cp should keep active planet without update, overview=%+v exec=%q", overview.CurrentPlanet, runner.execSQL)
		}
	})

	t.Run("missing cp falls back and persists home planet", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 100, 99, 0, 0, 0})},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues([]any{99, "Home", 1, 1, 2, 3, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
			{rows: fakeRowsFromValues([]any{99, "Home", 1, 1, 2, 3}, []any{100, "Colony", 1, 1, 2, 4})},
			{rows: fakeRowsFromValues([]any{2})},
		}}}
		repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")

		overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 987654321))
		if err != nil {
			t.Fatal(err)
		}
		if overview.CurrentPlanet.ID != 99 || runner.execArgs[0] != 99 || runner.execArgs[1] != 42 {
			t.Fatalf("missing cp should persist home planet, overview=%+v exec=%+v", overview.CurrentPlanet, runner.execArgs)
		}
	})

	t.Run("owned moon can become selected planet", func(t *testing.T) {
		runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 99, 0, 0, 0})},
			{rows: fakeRowsFromValues([]any{200, "Moon", 0, 1, 2, 3, 12800, 19, 1, 1, 0.0, 0.0, 0.0, 0, 0, 0})},
			{rows: fakeRowsFromValues([]any{99, "Home", 1, 1, 2, 3}, []any{200, "Moon", 0, 1, 2, 3})},
			{rows: fakeRowsFromValues([]any{2})},
		}}}
		repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")

		overview, err := repository.GetOverview(context.Background(), overviewQuery(42, 200))
		if err != nil {
			t.Fatal(err)
		}
		if overview.CurrentPlanet.ID != 200 || overview.CurrentPlanet.Type != 0 || runner.execArgs[0] != 200 {
			t.Fatalf("owned moon should become active, overview=%+v exec=%+v", overview.CurrentPlanet, runner.execArgs)
		}
		if overview.CurrentPlanet.Resources.MetalCapacity != 0 ||
			overview.CurrentPlanet.Resources.CrystalCapacity != 0 ||
			overview.CurrentPlanet.Resources.DeuteriumCapacity != 0 {
			t.Fatalf("moon resource capacities should match legacy zero-capacity header colors, got %+v", overview.CurrentPlanet.Resources)
		}
	})
}

func TestOverviewRepositoryReturnsActivePlanetUpdateError(t *testing.T) {
	runner := &fakeOverviewRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
			{rows: fakeRowsFromValues([]any{100, "Colony", 1, 1, 2, 4, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		}},
		execErr: errors.New("active planet update failed"),
	}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")

	_, err := repository.GetOverview(context.Background(), overviewQuery(42, 100))
	if err == nil || !strings.Contains(err.Error(), "active planet update failed") {
		t.Fatalf("expected active planet update error, got %v", err)
	}
}

func TestOverviewRepositoryReturnsResourceUpdateError(t *testing.T) {
	runner := &fakeOverviewRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 99, 1, 0, 0, 0})},
		{err: errors.New("resource update failed")},
	}}}
	repository := NewOverviewRepositoryWithRunner(runner, runner, "ogame_")
	repository.updateResources = true

	_, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))

	if err == nil || !strings.Contains(err.Error(), "resource update failed") {
		t.Fatalf("expected resource update error, got %v", err)
	}
}

func TestOverviewRepositoryReturnsEventLoadError(t *testing.T) {
	results := append(overviewResultsForPlanet(99, "Arakis"), fakeQueryResult{err: errors.New("overview event load failed")})
	repository := NewOverviewRepositoryWithQueryer(&fakeQueryer{results: results}, "ogame_")
	repository.includeEvents = true

	_, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
	if err == nil || !strings.Contains(err.Error(), "overview event load failed") {
		t.Fatalf("expected overview event load error, got %v", err)
	}
}

func TestOverviewRepositoryReturnsErrors(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:   "unsafe prefix",
			prefix: "bad-prefix_",
			queryer: &fakeQueryer{
				results: []fakeQueryResult{},
			},
			want: "invalid database table prefix",
		},
		{
			name:   "user query",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{err: errors.New("user query failed")},
			}},
			want: "user query failed",
		},
		{
			name:   "missing user",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues()},
			}},
			want: "overview user not found",
		},
		{
			name:   "missing current planet after fallback",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 9, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues()},
				{rows: fakeRowsFromValues()},
			}},
			want: "current planet not found",
		},
		{
			name:   "planet list",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{err: errors.New("planet list failed")},
			}},
			want: "planet list failed",
		},
		{
			name:   "universe",
			prefix: "ogame_",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
				{err: errors.New("universe failed")},
			}},
			want: "universe failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, tt.prefix)

			_, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}

	t.Run("foreign selectable cp uses home when there is no active planet", func(t *testing.T) {
		queryer := &fakeQueryer{results: []fakeQueryResult{
			{rows: fakeRowsFromValues()},
			{rows: fakeRowsFromValues([]any{555})},
		}}
		repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")
		planetID, current, persist, err := repository.resolveCurrentPlanet(context.Background(), "`ogame_planets`", overviewUser{HomePlanetID: 99}, appgame.OverviewQuery{PlayerID: 42, PlanetID: 555})
		if err != nil {
			t.Fatal(err)
		}
		if planetID != 99 || current.ID != 0 || persist {
			t.Fatalf("expected home fallback for selectable foreign cp, planet=%d current=%+v persist=%v", planetID, current, persist)
		}
	})
}

func TestOverviewRepositoryPropagatesRowErrors(t *testing.T) {
	tests := []struct {
		name    string
		queryer *fakeQueryer
		want    string
	}{
		{
			name: "user rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsError(errors.New("user rows failed"))},
			}},
			want: "user rows failed",
		},
		{
			name: "user scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{99, int64(0), 0, 1, 1, 0, 0, 0})},
			}},
			want: "expected string",
		},
		{
			name: "current planet query error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{err: errors.New("current planet failed")},
			}},
			want: "current planet failed",
		},
		{
			name: "current planet rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsError(errors.New("current planet rows failed"))},
			}},
			want: "current planet rows failed",
		},
		{
			name: "current planet scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{"bad", "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
			}},
			want: "expected int",
		},
		{
			name: "current planet post-scan rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValuesWithErr(errors.New("current planet post scan failed"), []any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
			}},
			want: "current planet post scan failed",
		},
		{
			name: "planet list scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{"bad", "Homeworld", 1, 1, 1, 1})},
			}},
			want: "expected int",
		},
		{
			name: "planet list rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValuesWithErr(errors.New("planet list rows failed"), []any{1, "Homeworld", 1, 1, 1, 1})},
			}},
			want: "planet list rows failed",
		},
		{
			name: "universe scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "expected int",
		},
		{
			name: "universe rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, 1, 1, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
				{rows: fakeRowsFromValues([]any{1, "Homeworld", 1, 1, 1, 1})},
				{rows: fakeRowsFromValuesWithErr(errors.New("universe rows failed"), []any{1})},
			}},
			want: "universe rows failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, "ogame_")

			_, err := repository.GetOverview(context.Background(), overviewQuery(42, 0))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestOverviewRepositorySelectablePlanetExistsEdges(t *testing.T) {
	for _, tt := range []struct {
		name    string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:    "query error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("selectable query failed")}}},
			want:    "selectable query failed",
		},
		{
			name:    "empty rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("selectable empty rows failed"))}}},
			want:    "selectable empty rows failed",
		},
		{
			name:    "scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}},
			want:    "expected int",
		},
		{
			name:    "rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("selectable rows failed"), []any{1})}}},
			want:    "selectable rows failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, "ogame_")
			_, err := repository.selectablePlanetExists(context.Background(), "ogame_planets", 1)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestOverviewRepositoryLoadUnreadMessagesEdges(t *testing.T) {
	for _, tt := range []struct {
		name    string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:    "query error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("unread query failed")}}},
			want:    "unread query failed",
		},
		{
			name:    "empty rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(errors.New("unread empty rows failed"))}}},
			want:    "unread empty rows failed",
		},
		{
			name:    "scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}},
			want:    "expected int",
		},
		{
			name:    "rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("unread rows failed"), []any{1})}}},
			want:    "unread rows failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, "ogame_")
			_, err := repository.loadUnreadMessages(context.Background(), "ogame_messages", 42)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}

	repository := NewOverviewRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "ogame_")
	count, err := repository.loadUnreadMessages(context.Background(), "ogame_messages", 42)
	if err != nil || count != 0 {
		t.Fatalf("expected empty unread rows to default to zero, count=%d err=%v", count, err)
	}
}

func TestOverviewRepositoryAttachBuildQueues(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues(
		[]any{99, domaingame.BuildingMetalMine, 3, 0, int64(2000)},
		[]any{100, domaingame.BuildingCrystalMine, 4, 1, int64(2001)},
		[]any{100, domaingame.BuildingDeuteriumSynth, 5, 0, int64(2002)},
		[]any{101, domaingame.ResearchEnergy, 6, 0, int64(2003)},
	)}}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")
	current := domaingame.PlanetOverview{ID: 99}
	planets := []domaingame.PlanetSummary{{ID: 99}, {ID: 100}, {ID: 101}}

	if err := repository.attachOverviewBuildQueues(context.Background(), "`ogame_buildqueue`", 42, &current, planets); err != nil {
		t.Fatal(err)
	}

	if current.BuildQueue == nil || current.BuildQueue.Name != "Metal Mine" || current.BuildQueue.End != 2000 {
		t.Fatalf("unexpected current build queue: %+v", current.BuildQueue)
	}
	if planets[0].BuildQueue == nil || planets[0].BuildQueue.Name != "Metal Mine" {
		t.Fatalf("expected current planet switcher build queue, got %+v", planets[0].BuildQueue)
	}
	if planets[1].BuildQueue == nil ||
		planets[1].BuildQueue.Name != "Crystal Mine" ||
		planets[1].BuildQueue.Level != 4 ||
		!planets[1].BuildQueue.Destroy {
		t.Fatalf("unexpected planet build queue: %+v", planets[1].BuildQueue)
	}
	if planets[2].BuildQueue != nil {
		t.Fatalf("expected non-building queue row to be ignored, got %+v", planets[2].BuildQueue)
	}
	if !strings.Contains(queryer.calls[0].sql, "ORDER BY planet_id ASC, list_id ASC") {
		t.Fatalf("expected legacy build queue ordering, got %q", queryer.calls[0].sql)
	}
}

func TestOverviewRepositoryAttachBuildQueuesEdges(t *testing.T) {
	for _, tt := range []struct {
		name    string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:    "query error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("buildqueue query failed")}}},
			want:    "buildqueue query failed",
		},
		{
			name:    "scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", domaingame.BuildingMetalMine, 3, 0, int64(2000)})}}},
			want:    "expected int",
		},
		{
			name:    "rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("buildqueue rows failed"), []any{99, domaingame.BuildingMetalMine, 3, 0, int64(2000)})}}},
			want:    "buildqueue rows failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, "ogame_")
			current := domaingame.PlanetOverview{ID: 99}
			err := repository.attachOverviewBuildQueues(context.Background(), "`ogame_buildqueue`", 42, &current, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}

	repository := NewOverviewRepositoryWithQueryer(&fakeQueryer{}, "ogame_")
	if err := repository.attachOverviewBuildQueues(context.Background(), "`ogame_buildqueue`", 42, nil, nil); err != nil {
		t.Fatalf("expected empty planet list to skip query, got %v", err)
	}
}

func TestOverviewRepositoryLoadOverviewEventsEdges(t *testing.T) {
	for _, tt := range []struct {
		name    string
		queryer *fakeQueryer
		want    string
	}{
		{
			name:    "query error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("overview event query failed")}}},
			want:    "overview event query failed",
		},
		{
			name:    "scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}},
			want:    "unexpected scan destination count",
		},
		{
			name:    "rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("overview event rows failed"), overviewEventRow(11, 42, "legor", domaingame.FleetMissionTransport, nil, 100, 200, 3, 4))}}},
			want:    "overview event rows failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, "ogame_")
			if _, err := repository.loadOverviewEvents(context.Background(), "`ogame_queue`", "`ogame_fleet`", "`ogame_planets`", "`ogame_users`", "`ogame_union`", 42); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestOverviewRepositoryLoadsACSOverviewEvents(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues()},
		{rows: fakeRowsFromValues([]any{7, 42})},
		{rows: fakeRowsFromValues(
			overviewEventRow(21, 42, "legor", domaingame.FleetMissionACSAttackHead, map[int]int{domaingame.FleetCruiser: 2}, 100, 300, 3, 4),
			overviewEventRow(22, 77, "support", domaingame.FleetMissionACSAttack, map[int]int{domaingame.FleetLightFighter: 5}, 110, 300, 5, 4),
		)},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")

	events, err := repository.loadOverviewEvents(context.Background(), "`ogame_queue`", "`ogame_fleet`", "`ogame_planets`", "`ogame_users`", "`ogame_union`", 42)
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 2 ||
		events[0].ID != -7 ||
		events[0].UnionID != 7 ||
		events[0].ArrivalAt != 300 ||
		len(events[0].GroupMissions) != 2 {
		t.Fatalf("unexpected ACS overview event: %+v", events)
	}
	if events[0].GroupMissions[0].MissionName != "Attack" ||
		events[0].GroupMissions[0].TotalShips != 2 ||
		events[0].GroupMissions[1].MissionName != "Joint attack" ||
		events[0].GroupMissions[1].OwnerName != "support" ||
		!events[0].GroupMissions[1].Foreign {
		t.Fatalf("unexpected ACS grouped missions: %+v", events[0].GroupMissions)
	}
	if !strings.Contains(queryer.calls[1].sql, "FROM `ogame_union`") || !strings.Contains(queryer.calls[2].sql, "f.union_id = ?") {
		t.Fatalf("expected ACS union queries, got %+v", queryer.calls)
	}
	if events[1].UnionID != 7 ||
		events[1].Mission != domaingame.FleetMissionACSAttackHead+domaingame.FleetMissionReturnOffset ||
		events[1].ArrivalAt != 500 ||
		events[1].Origin.Position != 4 ||
		events[1].Target.Position != 3 ||
		events[1].OriginName != "Target" ||
		events[1].TargetName != "Origin" ||
		events[1].StateShort != "(F)" ||
		events[1].CanRecall {
		t.Fatalf("expected ACS return pseudo-event, got %+v", events[1])
	}
}

func TestOverviewRepositoryLoadsNonACSOverviewPseudoEvents(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(
			overviewEventRowWithDeploy(31, 42, "legor", domaingame.FleetMissionExpedition, map[int]int{domaingame.FleetSmallCargo: 1}, 100, 200, 60, 3, 16),
			overviewEventRowWithDeploy(32, 77, "holder", domaingame.FleetMissionACSHold, map[int]int{domaingame.FleetCruiser: 2}, 110, 210, 50, 4, 3),
			overviewEventRow(33, 42, "legor", domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset, map[int]int{domaingame.FleetSmallCargo: 3}, 120, 220, 5, 6),
		)},
		{rows: fakeRowsFromValues()},
	}}
	repository := NewOverviewRepositoryWithQueryer(queryer, "ogame_")

	events, err := repository.loadOverviewEvents(context.Background(), "`ogame_queue`", "`ogame_fleet`", "`ogame_planets`", "`ogame_users`", "`ogame_union`", 42)
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 6 {
		t.Fatalf("expected actual plus pseudo events, got %+v", events)
	}
	if events[2].Mission != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset ||
		events[2].Origin.Position != 6 ||
		events[2].Target.Position != 5 ||
		events[2].OriginName != "Target" ||
		events[2].TargetName != "Origin" {
		t.Fatalf("expected actual return coordinates to be reversed, got %+v", events[2])
	}
	if events[3].Mission != domaingame.FleetMissionExpedition+domaingame.FleetMissionOrbitingOffset ||
		events[3].ArrivalAt != 260 ||
		events[3].StateShort != "(H)" {
		t.Fatalf("expected expedition hold pseudo-event, got %+v", events[3])
	}
	if events[4].Mission != domaingame.FleetMissionACSHold+domaingame.FleetMissionOrbitingOffset ||
		events[4].ArrivalAt != 260 ||
		!events[4].Foreign {
		t.Fatalf("expected foreign ACS hold pseudo-event, got %+v", events[4])
	}
	if events[5].Mission != domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset ||
		events[5].ArrivalAt != 360 ||
		events[5].Origin.Position != 16 ||
		events[5].Target.Position != 3 ||
		events[5].OriginName != "Target" ||
		events[5].TargetName != "Origin" ||
		events[5].CanRecall {
		t.Fatalf("expected expedition return pseudo-event, got %+v", events[5])
	}
}

func TestOverviewPseudoEventHelpersEdges(t *testing.T) {
	base := domaingame.BuildFleetMission(
		41,
		domaingame.FleetMissionTransport,
		domaingame.FleetCounts{domaingame.FleetSmallCargo: 2},
		domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
		domaingame.Coordinates{Galaxy: 1, System: 2, Position: 4},
		domaingame.PlanetTypePlanet,
		"target",
		300,
		200,
	)
	base.OwnerID = 42
	base.OriginName = "Origin"
	base.TargetName = "Target"

	hold := overviewHoldPseudoMission(base, -1)
	if hold.Mission != domaingame.FleetMissionTransport+domaingame.FleetMissionOrbitingOffset ||
		hold.ArrivalAt != base.ArrivalAt {
		t.Fatalf("expected negative deploy to clamp in hold pseudo-event, got %+v", hold)
	}
	returning := overviewReturnPseudoMission(base, -1, -1)
	if returning.Mission != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset ||
		returning.ArrivalAt != base.ArrivalAt ||
		returning.Origin.Position != 4 ||
		returning.Target.Position != 3 ||
		returning.OriginName != "Target" ||
		returning.TargetName != "Origin" {
		t.Fatalf("expected negative return timing to clamp and reverse coordinates, got %+v", returning)
	}
	unionReturn := overviewUnionReturnMission(base, -1, 9)
	if unionReturn.ArrivalAt != base.ArrivalAt || unionReturn.UnionID != 9 ||
		unionReturn.OriginName != "Target" ||
		unionReturn.TargetName != "Origin" {
		t.Fatalf("expected union return negative flight clamp, got %+v", unionReturn)
	}

	foreign := base
	foreign.OwnerID = 77
	deploy := base
	deploy.Mission = domaingame.FleetMissionDeploy
	missile := base
	missile.Mission = domaingame.FleetMissionMissile
	if overviewShouldAddReturnPseudoMission(foreign, 42) ||
		overviewShouldAddReturnPseudoMission(deploy, 42) ||
		overviewShouldAddReturnPseudoMission(missile, 42) {
		t.Fatalf("expected foreign/deploy/missile fleets to skip return pseudo-events")
	}

	orbitingExpedition := base
	orbitingExpedition.Mission = domaingame.FleetMissionExpedition + domaingame.FleetMissionOrbitingOffset
	orbitingExpedition.DepartureAt = 100
	orbitingExpedition.ArrivalAt = 500
	orbitingReturn := overviewReturnPseudoMission(orbitingExpedition, 100, 30)
	if orbitingReturn.Mission != domaingame.FleetMissionExpedition+domaingame.FleetMissionReturnOffset ||
		orbitingReturn.ArrivalAt != 530 ||
		overviewBaseMission(domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset) != domaingame.FleetMissionTransport {
		t.Fatalf("expected orbiting expedition return pseudo-event, got %+v", orbitingReturn)
	}
}

func TestOverviewRepositoryLoadOverviewUnionEventsEdges(t *testing.T) {
	fleetIDs := domaingame.FleetIDs()
	for _, tt := range []struct {
		name    string
		queryer *fakeQueryer
		want    string
		wantLen int
	}{
		{
			name:    "union query error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{err: errors.New("union query failed")}}},
			want:    "union query failed",
		},
		{
			name:    "union scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 42})}}},
			want:    "expected int",
		},
		{
			name:    "union rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(errors.New("union rows failed"), []any{7, 42})}}},
			want:    "union rows failed",
		},
		{
			name: "event query error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{7, 42})},
				{err: errors.New("union event query failed")},
			}},
			want: "union event query failed",
		},
		{
			name: "event scan error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{7, 42})},
				{rows: fakeRowsFromValues([]any{"bad"})},
			}},
			want: "unexpected scan destination count",
		},
		{
			name: "event rows error",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{7, 42})},
				{rows: fakeRowsFromValuesWithErr(errors.New("union event rows failed"), overviewEventRow(21, 42, "legor", domaingame.FleetMissionACSAttackHead, nil, 100, 300, 3, 4))},
			}},
			want: "union event rows failed",
		},
		{
			name: "empty event group",
			queryer: &fakeQueryer{results: []fakeQueryResult{
				{rows: fakeRowsFromValues([]any{7, 42})},
				{rows: fakeRowsFromValues()},
			}},
			wantLen: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewOverviewRepositoryWithQueryer(tt.queryer, "ogame_")
			events, err := repository.loadOverviewUnionEvents(context.Background(), "`ogame_queue`", "`ogame_fleet`", "`ogame_planets`", "`ogame_users`", "`ogame_union`", fleetIDs, 42)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("expected %q error, got %v", tt.want, err)
				}
				return
			}
			if err != nil || len(events) != tt.wantLen {
				t.Fatalf("expected %d events and no error, events=%+v err=%v", tt.wantLen, events, err)
			}
		})
	}
}

func TestFormatLegacyOverviewTimeUsesServerTimezone(t *testing.T) {
	got := formatLegacyOverviewTime(time.Date(2026, 6, 19, 15, 23, 7, 0, time.UTC))
	if got != "Fri Jun 19 18:23:07" {
		t.Fatalf("unexpected legacy overview time: %q", got)
	}
}

func TestOverviewTechnologyNameFallsBackToLegacyKey(t *testing.T) {
	if got := overviewTechnologyName(domaingame.BuildingMetalMine); got != "Metal Mine" {
		t.Fatalf("unexpected known technology name: %q", got)
	}
	if got := overviewTechnologyName(999999); got != "NAME_999999" {
		t.Fatalf("unexpected fallback technology name: %q", got)
	}
}

func TestPlanetOrder(t *testing.T) {
	tests := []struct {
		sortBy    int
		sortOrder int
		want      string
	}{
		{sortBy: 0, sortOrder: 0, want: " ORDER BY planet_id ASC, type DESC"},
		{sortBy: 0, sortOrder: 1, want: " ORDER BY planet_id DESC, type DESC"},
		{sortBy: 1, sortOrder: 0, want: " ORDER BY g ASC, s ASC, p ASC, type DESC"},
		{sortBy: 1, sortOrder: 1, want: " ORDER BY g DESC, s DESC, p DESC, type DESC"},
		{sortBy: 2, sortOrder: 0, want: " ORDER BY name ASC, type DESC"},
		{sortBy: 2, sortOrder: 1, want: " ORDER BY name DESC, type DESC"},
	}

	for _, tt := range tests {
		got := planetOrder(tt.sortBy, tt.sortOrder)
		if got != tt.want {
			t.Fatalf("planetOrder(%d, %d) = %q, want %q", tt.sortBy, tt.sortOrder, got, tt.want)
		}
	}
}

func TestStorageCapacityClampsNegativeLevel(t *testing.T) {
	if storageCapacity(-5) != storageCapacity(0) {
		t.Fatalf("expected negative storage level to clamp to zero")
	}
}

func overviewQuery(playerID int, planetID int) appgame.OverviewQuery {
	return appgame.OverviewQuery{PlayerID: playerID, PlanetID: planetID}
}

func overviewResultsForPlanet(planetID int, name string) []fakeQueryResult {
	return []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(0), 0, planetID, 1, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{planetID, name, 1, 1, 2, 3, 12800, 19, 1, 163, 0.0, 0.0, 0.0, 0, 0, 0})},
		{rows: fakeRowsFromValues([]any{planetID, name, 1, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
	}
}

func overviewScoreRow(ownerID int, buildings map[int]int, fleet map[int]int, defense map[int]int) []any {
	row := []any{ownerID}
	for _, id := range domaingame.BuildingIDs() {
		row = append(row, buildings[id])
	}
	for _, id := range domaingame.FleetIDs() {
		row = append(row, fleet[id])
	}
	for _, id := range domaingame.DefenseIDs() {
		row = append(row, defense[id])
	}
	return row
}

func overviewEventRow(id int, ownerID int, ownerName string, mission int, ships map[int]int, start int64, end int64, originPosition int, targetPosition int) []any {
	return overviewEventRowWithDeploy(id, ownerID, ownerName, mission, ships, start, end, 0, originPosition, targetPosition)
}

func overviewMissileEventRow(id int, ownerID int, ownerName string, amount int, targetID int, start int64, end int64, originPosition int, targetPosition int) []any {
	return overviewEventRowWithMissile(id, ownerID, ownerName, domaingame.FleetMissionMissile, amount, targetID, nil, start, end, originPosition, targetPosition)
}

func overviewEventRowWithDeploy(id int, ownerID int, ownerName string, mission int, ships map[int]int, start int64, end int64, deploy int64, originPosition int, targetPosition int) []any {
	return overviewEventRowWithMissileAndDeploy(id, ownerID, ownerName, mission, 0, 0, ships, start, end, deploy, originPosition, targetPosition)
}

func overviewEventRowWithMissile(id int, ownerID int, ownerName string, mission int, missileAmount int, missileTargetID int, ships map[int]int, start int64, end int64, originPosition int, targetPosition int) []any {
	return overviewEventRowWithMissileAndDeploy(id, ownerID, ownerName, mission, missileAmount, missileTargetID, ships, start, end, 0, originPosition, targetPosition)
}

func overviewEventRowWithMissileAndDeploy(id int, ownerID int, ownerName string, mission int, missileAmount int, missileTargetID int, ships map[int]int, start int64, end int64, deploy int64, originPosition int, targetPosition int) []any {
	row := []any{id, start, end, end - start, deploy, mission, missileAmount, missileTargetID, ownerID, ownerName, 99, 100}
	for _, fleetID := range domaingame.FleetIDs() {
		row = append(row, ships[fleetID])
	}
	row = append(row, "Origin", 1, 2, originPosition, "Target", 1, 2, targetPosition, domaingame.PlanetTypePlanet, "target")
	return row
}

type fakeOverviewRunner struct {
	fakeQueryer
	execSQL   string
	execArgs  []any
	execErr   error
	execErrAt int
	execCalls []fakeExecCall
}

type fakeExecCall struct {
	sql  string
	args []any
}

func (f *fakeOverviewRunner) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.execSQL = query
	f.execArgs = args
	f.execCalls = append(f.execCalls, fakeExecCall{sql: query, args: args})
	if f.execErrAt > 0 && len(f.execCalls) == f.execErrAt {
		return fakeSQLResult(0), f.execErr
	}
	return fakeSQLResult(1), f.execErr
}

var registerOverviewTestDriver sync.Once

func openOverviewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	registerOverviewTestDriver.Do(func() {
		sql.Register("overview_queryer_test", overviewTestDriver{})
	})
	db, err := sql.Open("overview_queryer_test", "")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

type overviewTestDriver struct{}

func (overviewTestDriver) Open(string) (driver.Conn, error) {
	return overviewTestConn{}, nil
}

type overviewTestConn struct{}

func (overviewTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}

func (overviewTestConn) Close() error {
	return nil
}

func (overviewTestConn) Begin() (driver.Tx, error) {
	return overviewTestTx{}, nil
}

func (overviewTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &overviewTestRows{}, nil
}

func (overviewTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return overviewTestResult(1), nil
}

type overviewTestTx struct{}

func (overviewTestTx) Commit() error {
	return nil
}

func (overviewTestTx) Rollback() error {
	return nil
}

type overviewTestRows struct {
	done bool
}

func (r *overviewTestRows) Columns() []string {
	return []string{"value"}
}

func (r *overviewTestRows) Close() error {
	return nil
}

func (r *overviewTestRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	dest[0] = int64(1)
	r.done = true
	return nil
}

type overviewTestResult int64

func (r overviewTestResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r overviewTestResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

type fakeQueryer struct {
	results []fakeQueryResult
	calls   []fakeQueryCall
}

type fakeQueryCall struct {
	sql  string
	args []any
}

type fakeQueryResult struct {
	rows *fakeRows
	err  error
}

func (f *fakeQueryer) QueryContext(_ context.Context, sql string, args ...any) (Rows, error) {
	f.calls = append(f.calls, fakeQueryCall{sql: sql, args: args})
	if len(f.results) == 0 {
		return nil, errors.New("unexpected query")
	}
	result := f.results[0]
	f.results = f.results[1:]
	if result.err != nil {
		return nil, result.err
	}
	return result.rows, nil
}

type fakeRows struct {
	values [][]any
	index  int
	err    error
}

func fakeRowsFromValues(values ...[]any) *fakeRows {
	return &fakeRows{values: values, index: -1}
}

func fakeRowsFromValuesWithErr(err error, values ...[]any) *fakeRows {
	return &fakeRows{values: values, index: -1, err: err}
}

func fakeRowsError(err error) *fakeRows {
	return &fakeRows{index: -1, err: err}
}

func (r *fakeRows) Close() error {
	return nil
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) Next() bool {
	r.index++
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("scan without current row")
	}
	if len(dest) != len(r.values[r.index]) {
		return errors.New("unexpected scan destination count")
	}
	for i := range dest {
		if err := assign(dest[i], r.values[r.index][i]); err != nil {
			return err
		}
	}
	return nil
}

func assign(dest any, value any) error {
	switch target := dest.(type) {
	case *string:
		v, ok := value.(string)
		if !ok {
			return errors.New("expected string")
		}
		*target = v
	case *int:
		switch v := value.(type) {
		case int:
			*target = v
		case int64:
			*target = int(v)
		default:
			return errors.New("expected int")
		}
	case *int64:
		switch v := value.(type) {
		case int:
			*target = int64(v)
		case int64:
			*target = v
		default:
			return errors.New("expected int64")
		}
	case *float64:
		switch v := value.(type) {
		case float64:
			*target = v
		case int:
			*target = float64(v)
		default:
			return errors.New("expected float64")
		}
	default:
		return errors.New("unsupported scan destination")
	}
	return nil
}
