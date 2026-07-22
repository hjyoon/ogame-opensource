package mysqlgame

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestMCPReadRepositoryListsPlanetsWithLegacyOrdering(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 2, 0})},
		{rows: fakeRowsFromValues(
			[]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3},
			[]any{100, "Moon", domaingame.PlanetTypeMoon, 1, 2, 3},
		)},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	planets, err := repository.ListMCPPlanets(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListMCPPlanets returned error: %v", err)
	}
	if len(planets) != 2 || !planets[0].Current || planets[1].TypeName != "moon" {
		t.Fatalf("unexpected planets: %+v", planets)
	}
	if !strings.Contains(queryer.calls[1].sql, "FROM `uni1_planets`") || !strings.Contains(queryer.calls[1].sql, "ORDER BY name ASC") {
		t.Fatalf("expected legacy planet ordering SQL, got %s", queryer.calls[1].sql)
	}
}

func TestMCPReadRepositoryFallsBackToHomePlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0, 88, 0, 0})},
		{rows: fakeRowsFromValues([]any{88, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	planets, err := repository.ListMCPPlanets(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListMCPPlanets returned error: %v", err)
	}
	if len(planets) != 1 || !planets[0].Current || planets[0].TypeName != "planet" {
		t.Fatalf("unexpected fallback planet: %+v", planets)
	}
}

func TestMCPReadRepositoryGetsAccountOverview(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(123456), 1, 99, 88})},
		{rows: fakeRowsFromValues([]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{2})},
		{rows: fakeRowsFromValues([]any{5})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	overview, err := repository.GetMCPAccountOverview(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetMCPAccountOverview returned error: %v", err)
	}
	if overview.Commander != "legor" || overview.Score.Display != 123 || overview.CurrentPlanet.ID != 99 || overview.PlanetCount != 2 || overview.UnreadMessages != 5 {
		t.Fatalf("unexpected account overview: %+v", overview)
	}
	if !strings.Contains(queryer.calls[0].sql, "FROM `uni1_users`") ||
		!strings.Contains(queryer.calls[1].sql, "FROM `uni1_planets`") ||
		!strings.Contains(queryer.calls[3].sql, "FROM `uni1_messages`") {
		t.Fatalf("unexpected account overview SQL calls: %+v", queryer.calls)
	}
}

func TestMCPReadRepositoryAccountOverviewFallsBackToHomePlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(-1), 2, 0, 88})},
		{rows: fakeRowsFromValues([]any{88, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues([]any{0})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	overview, err := repository.GetMCPAccountOverview(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetMCPAccountOverview returned error: %v", err)
	}
	if overview.CurrentPlanet.ID != 88 || overview.Score.Display != 0 {
		t.Fatalf("unexpected fallback overview: %+v", overview)
	}
}

func TestMCPReadRepositoryGetsPlanetResources(t *testing.T) {
	now := time.Unix(1700000000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 1000, 37, 4, now.Add(time.Hour).Unix(), now.Add(time.Hour).Unix()})},
		{rows: fakeRowsFromValues([]any{
			99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3,
			12345.0, 23456.0, 34567.0,
			40,
			10, 9, 8,
			10, 8, 6, 12, 2, 5,
			1.0, 1.0, 1.0, 1.0, 1.0, 1.0,
		})},
		{rows: fakeRowsFromValues([]any{128.0})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")
	repository.now = func() time.Time { return now }

	resources, err := repository.GetMCPPlanetResources(context.Background(), 42, 0)
	if err != nil {
		t.Fatalf("GetMCPPlanetResources returned error: %v", err)
	}
	if resources.PlayerID != 42 ||
		resources.Planet.ID != 99 ||
		!resources.Planet.Current ||
		resources.Resources.Metal != 12345 ||
		resources.Resources.DarkMatter != 1037 ||
		resources.Capacity.Metal != storageCapacity(10) ||
		resources.Energy.Capacity <= 0 ||
		resources.ProductionPerHour.Metal <= 0 {
		t.Fatalf("unexpected planet resources: %+v", resources)
	}
	if !strings.Contains(queryer.calls[0].sql, "COALESCE(dm, 0)") ||
		!strings.Contains(queryer.calls[1].sql, "COALESCE(`700`, 0)") ||
		!strings.Contains(queryer.calls[2].sql, "FROM `uni1_uni`") {
		t.Fatalf("unexpected resource SQL calls: %+v", queryer.calls)
	}
}

func TestMCPReadRepositoryGetsMoonResourcesWithoutProduction(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 1000, 37, 4, int64(0), int64(0)})},
		{rows: fakeRowsFromValues([]any{
			100, "Moon", domaingame.PlanetTypeMoon, 1, 2, 3,
			100.0, 200.0, 300.0,
			0,
			10, 9, 8,
			10, 8, 6, 12, 2, 5,
			1.0, 1.0, 1.0, 1.0, 1.0, 1.0,
		})},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	resources, err := repository.GetMCPPlanetResources(context.Background(), 42, 100)
	if err != nil {
		t.Fatalf("GetMCPPlanetResources returned error: %v", err)
	}
	if resources.Planet.TypeName != "moon" ||
		resources.Planet.Current ||
		resources.Capacity.Metal != 0 ||
		resources.Energy.Capacity != 0 ||
		resources.ProductionPerHour.Metal != 0 ||
		len(queryer.calls) != 2 {
		t.Fatalf("unexpected moon resources: %+v calls=%+v", resources, queryer.calls)
	}
}

func TestMCPReadRepositoryPlanetResourcesErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected nil queryer error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected invalid prefix error")
	}

	wantErr := errors.New("query failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected account query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected missing account error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor", "bad", 88, 0, 0, 0, int64(0), int64(0)})}}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected account scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 0, 0, 0, int64(0), int64(0)})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet resource query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 0, 0, 0, int64(0), int64(0)})},
		{rows: fakeRowsFromValues()},
	}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected missing planet resources error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 0, 0, 0, int64(0), int64(0)})},
		{rows: fakeRowsFromValues([]any{"bad"})},
	}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected planet resource scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", 99, 88, 0, 0, 0, int64(0), int64(0)})},
		{rows: fakeRowsFromValues([]any{
			99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3,
			1.0, 2.0, 3.0,
			40,
			0, 0, 0,
			1, 0, 0, 0, 0, 0,
			1.0, 0.0, 0.0, 0.0, 0.0, 0.0,
		})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPPlanetResources(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected universe speed error, got %v", err)
	}
}

func TestMCPReadRepositoryGetsBuildingQueue(t *testing.T) {
	now := time.Unix(1700000000, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues(
			buildQueueRowValues(buildQueueRow{ID: 1, OwnerID: 42, PlanetID: 99, ListID: 1, TechID: domaingame.BuildingMetalMine, Level: 3, Start: int(now.Unix()) - 10, End: int(now.Unix()) + 50}),
			buildQueueRowValues(buildQueueRow{ID: 2, OwnerID: 42, PlanetID: 99, ListID: 2, TechID: domaingame.BuildingCrystalMine, Level: 2, Destroy: 1, Start: int(now.Unix()) - 100, End: int(now.Unix()) - 1}),
			buildQueueRowValues(buildQueueRow{ID: 3, OwnerID: 42, PlanetID: 99, ListID: 3, TechID: 999999, Level: 1, Start: int(now.Unix()), End: int(now.Unix()) + 100}),
		)},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")
	repository.now = func() time.Time { return now }

	queue, err := repository.GetMCPBuildingQueue(context.Background(), 42, 0)
	if err != nil {
		t.Fatalf("GetMCPBuildingQueue returned error: %v", err)
	}
	if queue.PlayerID != 42 ||
		queue.Planet.ID != 99 ||
		!queue.Planet.Current ||
		queue.Count != 2 ||
		queue.Entries[0].Name != "Metal Mine" ||
		queue.Entries[0].RemainingSeconds != 50 ||
		queue.Entries[0].Status != "running" ||
		!queue.Entries[1].Destroy ||
		queue.Entries[1].RemainingSeconds != 0 ||
		queue.Entries[1].Status != "due" {
		t.Fatalf("unexpected building queue: %+v", queue)
	}
	if !strings.Contains(queryer.calls[2].sql, "FROM `uni1_buildqueue`") ||
		!strings.Contains(queryer.calls[2].sql, "ORDER BY list_id ASC") {
		t.Fatalf("unexpected building queue SQL: %+v", queryer.calls)
	}
}

func TestMCPQueueStatusDistinguishesRunningQueuedAndDue(t *testing.T) {
	if got := mcpQueueStatus(10, false); got != "running" {
		t.Fatalf("running status = %q", got)
	}
	if got := mcpQueueStatus(10, true); got != "queued" {
		t.Fatalf("queued status = %q", got)
	}
	if got := mcpQueueStatus(0, true); got != "due" {
		t.Fatalf("due status = %q", got)
	}
}

func TestMCPReadRepositoryGetsEmptyBuildingQueueForRequestedPlanet(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{rows: fakeRowsFromValues([]any{100, "Colony", domaingame.PlanetTypePlanet, 1, 2, 4})},
		{rows: fakeRowsFromValues()},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	queue, err := repository.GetMCPBuildingQueue(context.Background(), 42, 100)
	if err != nil {
		t.Fatalf("GetMCPBuildingQueue returned error: %v", err)
	}
	if queue.Count != 0 || len(queue.Entries) != 0 || queue.Planet.Current {
		t.Fatalf("unexpected empty requested queue: %+v", queue)
	}
	if len(queryer.calls[1].args) == 0 || queryer.calls[1].args[0] != 100 {
		t.Fatalf("expected requested planet id query, got %+v", queryer.calls[1])
	}
}

func TestMCPReadRepositoryListsMessagesWithoutMutatingInbox(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues(
			[]any{11, domaingame.MessageTypePM, `Admin`, `Hello`, "", 0, int64(1000)},
			[]any{12, domaingame.MessageTypeAlliance, `Alliance`, `Notice`, "", 1, int64(900)},
		)},
		{rows: fakeRowsFromValues(
			[]any{13, domaingame.MessageTypeExpedition, `Fleet`, `Find`, "Expedition body", 1, int64(800)},
		)},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	messages, err := repository.ListMCPMessages(context.Background(), 42, domainmcp.MessageQuery{
		Limit:          2,
		MessageType:    domaingame.MessageTypePM,
		HasMessageType: true,
	})
	if err != nil {
		t.Fatalf("ListMCPMessages returned error: %v", err)
	}
	if messages.PlayerID != 42 || messages.Count != 2 || messages.Limit != 2 || messages.Messages[0].TypeName != "personal" || !messages.Messages[0].Unread || messages.Messages[0].Text != "" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	if !strings.Contains(queryer.calls[0].sql, "FROM `uni1_messages`") ||
		!strings.Contains(queryer.calls[0].sql, "''") ||
		!strings.Contains(queryer.calls[0].sql, "pm = ?") ||
		strings.Contains(queryer.calls[0].sql, "UPDATE") {
		t.Fatalf("unexpected MCP message list SQL: %+v", queryer.calls[0])
	}

	messages, err = repository.ListMCPMessages(context.Background(), 42, domainmcp.MessageQuery{Limit: 1, IncludeText: true})
	if err != nil {
		t.Fatalf("ListMCPMessages include text returned error: %v", err)
	}
	if messages.Messages[0].Text != "Expedition body" || messages.Messages[0].TypeName != "expedition" || !strings.Contains(queryer.calls[1].sql, "msgfrom, subj, text") {
		t.Fatalf("unexpected include-text messages=%+v sql=%s", messages, queryer.calls[1].sql)
	}
}

func TestMCPReadRepositoryGetsMessageDetail(t *testing.T) {
	queryer := &fakeQueryer{results: []fakeQueryResult{{
		rows: fakeRowsFromValues([]any{11, domaingame.MessageTypeSpyReport, `Scout`, `Spy`, `Report body`, 0, int64(1000)}),
	}}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")

	message, err := repository.GetMCPMessage(context.Background(), 42, 11)
	if err != nil {
		t.Fatalf("GetMCPMessage returned error: %v", err)
	}
	if message.PlayerID != 42 || message.Message.ID != 11 || message.Message.TypeName != "spy_report" || message.Message.Text != "Report body" {
		t.Fatalf("unexpected message detail: %+v", message)
	}
	if queryer.calls[0].args[0] != 42 || queryer.calls[0].args[1] != 11 {
		t.Fatalf("expected owned message lookup, got %+v", queryer.calls[0])
	}
}

func TestMCPMessageTypeNames(t *testing.T) {
	for _, tt := range []struct {
		messageType int
		want        string
	}{
		{domaingame.MessageTypePM, "personal"},
		{domaingame.MessageTypeSpyReport, "spy_report"},
		{domaingame.MessageTypeBattleReportLink, "battle_report"},
		{domaingame.MessageTypeExpedition, "expedition"},
		{domaingame.MessageTypeAlliance, "alliance"},
		{domaingame.MessageTypeBattleReportText, "battle_report_text"},
		{domaingame.MessageTypeMisc, "other"},
		{999, "other"},
	} {
		if got := mcpMessageTypeName(tt.messageType); got != tt.want {
			t.Fatalf("message type %d got %q want %q", tt.messageType, got, tt.want)
		}
	}
}

func TestMCPReadRepositoryMessageErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.ListMCPMessages(context.Background(), 42, domainmcp.MessageQuery{Limit: 1}); err == nil {
		t.Fatalf("expected nil queryer list error")
	}
	if _, err := repository.GetMCPMessage(context.Background(), 42, 1); err == nil {
		t.Fatalf("expected nil queryer detail error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.ListMCPMessages(context.Background(), 42, domainmcp.MessageQuery{Limit: 1}); err == nil {
		t.Fatalf("expected invalid prefix list error")
	}
	if _, err := repository.GetMCPMessage(context.Background(), 42, 1); err == nil {
		t.Fatalf("expected invalid prefix detail error")
	}

	wantErr := errors.New("messages failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.ListMCPMessages(context.Background(), 42, domainmcp.MessageQuery{Limit: 1}); !errors.Is(err, wantErr) {
		t.Fatalf("expected list query error, got %v", err)
	}
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "uni1_")
	if _, err := repository.ListMCPMessages(context.Background(), 42, domainmcp.MessageQuery{Limit: 1}); err == nil {
		t.Fatalf("expected list scan error")
	}
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{11, domaingame.MessageTypePM, "From", "Subj", "", 0, int64(1)})}}}, "uni1_")
	if _, err := repository.ListMCPMessages(context.Background(), 42, domainmcp.MessageQuery{Limit: 1}); !errors.Is(err, wantErr) {
		t.Fatalf("expected list rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.GetMCPMessage(context.Background(), 42, 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected detail query error, got %v", err)
	}
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.GetMCPMessage(context.Background(), 42, 1); err == nil {
		t.Fatalf("expected missing detail error")
	}
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}, "uni1_")
	if _, err := repository.GetMCPMessage(context.Background(), 42, 1); err == nil {
		t.Fatalf("expected detail scan error")
	}
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{11, domaingame.MessageTypePM, "From", "Subj", "Body", 0, int64(1)})}}}, "uni1_")
	if _, err := repository.GetMCPMessage(context.Background(), 42, 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected detail rows error, got %v", err)
	}
}

func TestMCPReadRepositoryBuildingQueueErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.GetMCPBuildingQueue(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected nil queryer error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.GetMCPBuildingQueue(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected invalid prefix error")
	}

	wantErr := errors.New("query failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.GetMCPBuildingQueue(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected settings query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.GetMCPBuildingQueue(context.Background(), 42, 0); err == nil {
		t.Fatalf("expected missing player error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPBuildingQueue(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{rows: fakeRowsFromValues([]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPBuildingQueue(context.Background(), 42, 0); !errors.Is(err, wantErr) {
		t.Fatalf("expected queue query error, got %v", err)
	}
}

func TestMCPReadRepositoryGetsFleetMovements(t *testing.T) {
	now := time.Unix(99, 0)
	queryer := &fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{6, now.Add(time.Hour).Unix()})},
		{rows: fakeRowsFromValues(
			overviewEventRow(31, 42, "legor", domaingame.FleetMissionTransport, map[int]int{domaingame.FleetSmallCargo: 1}, 100, 200, 3, 4),
		)},
		{rows: fakeRowsFromValues([]any{7, 42})},
		{rows: fakeRowsFromValues(
			overviewEventRow(21, 42, "legor", domaingame.FleetMissionACSAttackHead, map[int]int{domaingame.FleetCruiser: 2}, 100, 300, 3, 4),
			overviewEventRow(22, 77, "support", domaingame.FleetMissionACSAttack, map[int]int{domaingame.FleetLightFighter: 5}, 110, 300, 5, 4),
		)},
	}}
	repository := NewMCPReadRepositoryWithQueryer(queryer, "uni1_")
	repository.now = func() time.Time { return now }

	movements, err := repository.GetMCPFleetMovements(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetMCPFleetMovements returned error: %v", err)
	}
	if movements.PlayerID != 42 || movements.Now != now.Unix() || movements.Count != 4 {
		t.Fatalf("unexpected movement summary: %+v", movements)
	}
	if movements.Events[0].MissionName != "Transport" ||
		movements.Events[0].RemainingSeconds != 101 ||
		movements.Events[0].FleetDetailLevel != 8 ||
		!movements.Events[0].CanRecall ||
		movements.Events[1].Mission != domaingame.FleetMissionTransport+domaingame.FleetMissionReturnOffset ||
		movements.Events[1].CanRecall {
		t.Fatalf("unexpected non-ACS movements: %+v", movements.Events)
	}
	group := movements.Events[2]
	if group.ID != -7 ||
		group.UnionID != 7 ||
		len(group.GroupMissions) != 2 ||
		group.GroupMissions[1].OwnerName != "support" ||
		!group.GroupMissions[1].Foreign {
		t.Fatalf("unexpected ACS group movement: %+v", group)
	}
	if movements.Events[3].UnionID != 7 ||
		movements.Events[3].Mission != domaingame.FleetMissionACSAttackHead+domaingame.FleetMissionReturnOffset ||
		movements.Events[3].RemainingSeconds != 401 {
		t.Fatalf("unexpected ACS return movement: %+v", movements.Events[3])
	}
	if !strings.Contains(queryer.calls[0].sql, "COALESCE(`106`, 0)") ||
		!strings.Contains(queryer.calls[1].sql, "FROM `uni1_queue`") ||
		!strings.Contains(queryer.calls[2].sql, "FROM `uni1_union`") {
		t.Fatalf("unexpected fleet movement SQL calls: %+v", queryer.calls)
	}
}

func TestMCPFleetMovementFromMissionPreservesDetails(t *testing.T) {
	mission := domaingame.BuildFleetMission(
		7,
		domaingame.FleetMissionRecycle,
		domaingame.FleetCounts{domaingame.FleetRecycler: 2},
		domaingame.Coordinates{Galaxy: 1, System: 2, Position: 3},
		domaingame.Coordinates{Galaxy: 1, System: 2, Position: 16},
		domaingame.PlanetTypeDebris,
		"debris",
		100,
		200,
	)
	mission.OwnerID = 42
	mission.OwnerName = "legor"
	mission.LoadedResources = map[int]int{domaingame.ResourceMetal: 10, domaingame.ResourceCrystal: 20, domaingame.ResourceDeuterium: 30}
	mission.MissileAmount = 5
	mission.MissileTargetID = domaingame.DefenseLightLaser
	mission.MissileTarget = "Light Laser"
	mission.UnionID = 9
	mission.UnionName = "Group"
	mission.UnionPlayers = []domaingame.FleetUnionPlayer{{ID: 42, Name: "legor"}}
	mission.GroupMissions = []domaingame.FleetMission{
		domaingame.BuildFleetMission(8, domaingame.FleetMissionAttack, domaingame.FleetCounts{domaingame.FleetLightFighter: 1}, domaingame.Coordinates{}, domaingame.Coordinates{}, domaingame.PlanetTypePlanet, "target", 110, 220),
	}
	mission = domaingame.BuildOverviewEvents([]domaingame.FleetMission{mission})[0]

	got := mcpFleetMovementFromMission(mission, 150)
	if got.ID != 7 ||
		got.Ships[0].Name != "Recycler" ||
		got.LoadedResources.Metal != 10 ||
		got.LoadedResources.Crystal != 20 ||
		got.LoadedResources.Deuterium != 30 ||
		got.MissileTarget != "Light Laser" ||
		got.UnionPlayers[0].Name != "legor" ||
		len(got.GroupMissions) != 1 ||
		got.RemainingSeconds != 50 {
		t.Fatalf("unexpected converted movement: %+v", got)
	}
	if expired := mcpFleetMovementFromMission(mission, 250); expired.RemainingSeconds != 0 {
		t.Fatalf("expected expired movement to clamp remaining seconds, got %+v", expired)
	}
}

func TestMCPReadRepositoryFleetMovementsErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.GetMCPFleetMovements(context.Background(), 42); err == nil {
		t.Fatalf("expected nil queryer error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.GetMCPFleetMovements(context.Background(), 42); err == nil {
		t.Fatalf("expected invalid prefix error")
	}

	wantErr := errors.New("query failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.GetMCPFleetMovements(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected detail query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.GetMCPFleetMovements(context.Background(), 42); err == nil {
		t.Fatalf("expected missing fleet detail error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsError(wantErr)}}}, "uni1_")
	if _, err := repository.GetMCPFleetMovements(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected empty detail rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", int64(0)})}}}, "uni1_")
	if _, err := repository.GetMCPFleetMovements(context.Background(), 42); err == nil {
		t.Fatalf("expected fleet detail scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{0, int64(0)})}}}, "uni1_")
	if _, err := repository.GetMCPFleetMovements(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected detail rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{0, int64(0)})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPFleetMovements(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected event query error, got %v", err)
	}
}

func TestMCPReadRepositoryErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected nil queryer error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected invalid prefix error")
	}

	wantErr := errors.New("query failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected settings query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected missing player error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 88, 0, 0})}}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected settings scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{99, 88, 0, 0})}}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected settings rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{rows: fakeRowsFromValues([]any{"bad", "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
	}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); err == nil {
		t.Fatalf("expected planet scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{99, 88, 0, 0})},
		{rows: fakeRowsFromValuesWithErr(wantErr, []any{99, "Unknown", 999, 1, 2, 3})},
	}}, "uni1_")
	if _, err := repository.ListMCPPlanets(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet rows error, got %v", err)
	}
}

func TestMCPReadRepositoryConstructorsAndUnknownType(t *testing.T) {
	if repository := NewMCPReadRepository(nil, "uni1_"); repository.prefix != "uni1_" {
		t.Fatalf("unexpected constructor result: %+v", repository)
	}
	if got := mcpPlanetTypeName(999); got != "unknown" {
		t.Fatalf("expected unknown type name, got %q", got)
	}
}

func TestMCPReadRepositoryAccountOverviewErrorBranches(t *testing.T) {
	repository := NewMCPReadRepositoryWithQueryer(nil, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected nil queryer error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{}, "uni1_;DROP")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected invalid prefix error")
	}

	wantErr := errors.New("query failed")
	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected account query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected missing account error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"legor", "bad", 1, 99, 88})}}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected account scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{"legor", int64(1), 1, 99, 88})}}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected account rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValues()},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected missing current planet error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected current planet query error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValues([]any{"bad", "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected current planet scan error")
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValuesWithErr(wantErr, []any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected current planet rows error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValues([]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{err: wantErr},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected planet count error, got %v", err)
	}

	repository = NewMCPReadRepositoryWithQueryer(&fakeQueryer{results: []fakeQueryResult{
		{rows: fakeRowsFromValues([]any{"legor", int64(1), 1, 99, 88})},
		{rows: fakeRowsFromValues([]any{99, "Homeworld", domaingame.PlanetTypePlanet, 1, 2, 3})},
		{rows: fakeRowsFromValues([]any{1})},
		{rows: fakeRowsFromValues([]any{"bad"})},
	}}, "uni1_")
	if _, err := repository.GetMCPAccountOverview(context.Background(), 42); err == nil {
		t.Fatalf("expected unread count scan error")
	}

	emptyCount, err := (MCPReadRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}).singleMCPCount(context.Background(), "SELECT COUNT(*)")
	if err != nil || emptyCount != 0 {
		t.Fatalf("expected empty count fallback, got count=%d err=%v", emptyCount, err)
	}

	if _, err := (MCPReadRepository{queryer: &fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr)}}}}).singleMCPCount(context.Background(), "SELECT COUNT(*)"); !errors.Is(err, wantErr) {
		t.Fatalf("expected empty count rows error, got %v", err)
	}
}
