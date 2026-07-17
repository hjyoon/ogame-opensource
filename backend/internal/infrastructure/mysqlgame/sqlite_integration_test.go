package mysqlgame_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlgame"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlregistration"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/sqlitedb"
)

func TestSQLiteConcurrentAttackQueueCompletesExactlyOnce(t *testing.T) {
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Unix(1_700_000_000, 0)
	if err := sqlitedb.BootstrapUniverse(context.Background(), db, sqlitedb.BootstrapOptions{
		Prefix:        "uni1_",
		Secret:        "secret",
		Universe:      1,
		AdminEmail:    "admin@example.local",
		AdminPassword: "admin",
		Now:           now,
	}); err != nil {
		t.Fatal(err)
	}
	defender, err := mysqlregistration.NewAccountCreator(db, "uni1_", "secret").CreateRegistrationAccount(
		context.Background(),
		domainpublicsite.RegistrationDraft{
			Character: "QueueDefender",
			Password:  "Queue123!",
			Email:     "queue-defender@example.local",
		},
		"127.0.0.1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE uni1_planets SET `700` = 1000, `701` = 1000, `702` = 1000, lastpeek = ? WHERE planet_id = ?", now.Unix(), defender.HomePlanetID); err != nil {
		t.Fatal(err)
	}
	fleetResult, err := db.Exec("INSERT INTO uni1_fleet (owner_id, union_id, `700`, `701`, `702`, fuel, mission, start_planet, target_planet, flight_time, deploy_time, `202`) VALUES (1, 0, 0, 0, 0, 0, ?, 1, ?, 30, 0, 1)", domaingame.FleetMissionAttack, defender.HomePlanetID)
	if err != nil {
		t.Fatal(err)
	}
	fleetID, err := fleetResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO uni1_queue (owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen) VALUES (1, 'Fleet', ?, ?, 0, ?, ?, 0, 0, 0)", fleetID, domaingame.FleetMissionAttack, now.Unix()-30, now.Unix()); err != nil {
		t.Fatal(err)
	}

	const workers = 12
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			errs <- mysqlgame.NewFleetRepository(db, "uni1_").FinishDueFleetQueues(context.Background(), int(now.Unix()))
		}()
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent fleet completion failed: %v", err)
		}
	}

	for _, check := range []struct {
		name  string
		query string
		args  []any
		want  int
	}{
		{name: "battle", query: "SELECT COUNT(*) FROM uni1_battledata WHERE date = ?", args: []any{now.Unix()}, want: 1},
		{name: "return fleet", query: "SELECT COUNT(*) FROM uni1_fleet WHERE owner_id = 1 AND mission = ?", args: []any{domaingame.FleetMissionAttack + domaingame.FleetMissionReturnOffset}, want: 1},
		{name: "return queue", query: "SELECT COUNT(*) FROM uni1_queue WHERE owner_id = 1 AND type = 'Fleet'", want: 1},
		{name: "return log", query: "SELECT COUNT(*) FROM uni1_fleetlogs WHERE owner_id = 1 AND mission = ?", args: []any{domaingame.FleetMissionAttack + domaingame.FleetMissionReturnOffset}, want: 1},
		{name: "battle messages", query: "SELECT COUNT(*) FROM uni1_messages WHERE date = ? AND pm IN (?, ?)", args: []any{now.Unix(), domaingame.MessageTypeBattleReportText, domaingame.MessageTypeBattleReportLink}, want: 4},
	} {
		var got int
		if err := db.QueryRow(check.query, check.args...).Scan(&got); err != nil {
			t.Fatalf("%s count: %v", check.name, err)
		}
		if got != check.want {
			t.Fatalf("%s count = %d, want %d", check.name, got, check.want)
		}
	}
}

func TestSQLiteGalaxyRepositoryLoadsSeededSystem(t *testing.T) {
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlitedb.BootstrapUniverse(context.Background(), db, sqlitedb.BootstrapOptions{
		Prefix:        "uni1_",
		Secret:        "secret",
		Universe:      1,
		AdminEmail:    "admin@example.local",
		AdminPassword: "admin",
		Now:           time.Unix(1700000000, 0),
	}); err != nil {
		t.Fatal(err)
	}
	galaxy, err := mysqlgame.NewGalaxyRepository(db, "uni1_").GetGalaxy(context.Background(), appgame.GalaxyQuery{
		PlayerID: 1,
		PlanetID: 1,
		Coordinates: domaingame.Coordinates{
			Galaxy: 1,
			System: 1,
		},
	})
	if err != nil {
		t.Fatalf("load SQLite galaxy: %v", err)
	}
	if galaxy.CurrentPlanet.ID != 1 || galaxy.Coordinates.Galaxy != 1 || galaxy.Coordinates.System != 1 {
		t.Fatalf("unexpected SQLite galaxy: %+v", galaxy)
	}
}

func TestSQLiteRegisteredAccountCanQueueBuilding(t *testing.T) {
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlitedb.BootstrapUniverse(context.Background(), db, sqlitedb.BootstrapOptions{
		Prefix:        "uni1_",
		Secret:        "secret",
		Universe:      1,
		AdminEmail:    "admin@example.local",
		AdminPassword: "admin",
		Now:           time.Unix(1700000000, 0),
	}); err != nil {
		t.Fatal(err)
	}
	account, err := mysqlregistration.NewAccountCreator(db, "uni1_", "secret").CreateRegistrationAccount(
		context.Background(),
		domainpublicsite.RegistrationDraft{
			Character: "SQLiteBuilder",
			Password:  "Sqlite123!",
			Email:     "sqlite-builder@example.local",
		},
		"127.0.0.1",
	)
	if err != nil {
		t.Fatalf("register SQLite account: %v", err)
	}
	result, err := mysqlgame.NewBuildingsRepository(db, "uni1_").MutateBuildings(
		context.Background(),
		appgame.BuildingsMutationQuery{
			PlayerID: account.PlayerID,
			PlanetID: account.HomePlanetID,
			Action:   "add",
			TechID:   domaingame.BuildingMetalMine,
		},
	)
	if err != nil {
		t.Fatalf("queue SQLite building: %v", err)
	}
	if result.ActionIssue != nil {
		t.Fatalf("unexpected SQLite building issue: %s", *result.ActionIssue)
	}
	if _, err := db.Exec("UPDATE `uni1_queue` SET end = 0 WHERE owner_id = ?", account.PlayerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE `uni1_buildqueue` SET end = 0 WHERE owner_id = ?", account.PlayerID); err != nil {
		t.Fatal(err)
	}
	if err := mysqlgame.NewBuildingsRepository(db, "uni1_").FinishDueBuildingQueues(context.Background(), int(time.Now().Unix())); err != nil {
		t.Fatalf("finish SQLite building: %v", err)
	}
	var metalMine int
	if err := db.QueryRow("SELECT `1` FROM `uni1_planets` WHERE planet_id = ?", account.HomePlanetID).Scan(&metalMine); err != nil {
		t.Fatal(err)
	}
	if metalMine != 1 {
		t.Fatalf("expected completed SQLite metal mine, got level %d", metalMine)
	}

	if _, err := db.Exec("UPDATE `uni1_planets` SET `31` = 1, `700` = 1000000, `701` = 1000000, `702` = 1000000 WHERE planet_id = ?", account.HomePlanetID); err != nil {
		t.Fatal(err)
	}
	researchResult, err := mysqlgame.NewResearchRepository(db, "uni1_").MutateResearch(context.Background(), appgame.ResearchMutationQuery{
		PlayerID: account.PlayerID,
		PlanetID: account.HomePlanetID,
		Action:   "start",
		TechID:   domaingame.ResearchEnergy,
	})
	if err != nil {
		t.Fatalf("start SQLite research: %v", err)
	}
	if researchResult.ActionIssue != nil {
		t.Fatalf("unexpected SQLite research issue: %s", *researchResult.ActionIssue)
	}
	if _, err := db.Exec("UPDATE `uni1_queue` SET end = 0 WHERE owner_id = ?", account.PlayerID); err != nil {
		t.Fatal(err)
	}
	if err := mysqlgame.NewResearchRepository(db, "uni1_").FinishDueResearchQueues(context.Background(), int(time.Now().Unix())); err != nil {
		t.Fatalf("finish SQLite research: %v", err)
	}
	var energyResearch int
	if err := db.QueryRow("SELECT `113` FROM `uni1_users` WHERE player_id = ?", account.PlayerID).Scan(&energyResearch); err != nil {
		t.Fatal(err)
	}
	if energyResearch != 1 {
		t.Fatalf("expected completed SQLite energy research, got level %d", energyResearch)
	}

	if _, err := db.Exec("UPDATE `uni1_planets` SET `21` = 2, `700` = 1000000, `701` = 1000000, `702` = 1000000 WHERE planet_id = ?", account.HomePlanetID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE `uni1_users` SET `115` = 2 WHERE player_id = ?", account.PlayerID); err != nil {
		t.Fatal(err)
	}
	shipyardResult, err := mysqlgame.NewShipyardRepository(db, "uni1_").MutateShipyard(context.Background(), appgame.ShipyardMutationQuery{
		PlayerID: account.PlayerID,
		PlanetID: account.HomePlanetID,
		Orders:   map[int]int{domaingame.FleetLightFighter: 1},
	})
	if err != nil {
		t.Fatalf("queue SQLite shipyard order: %v", err)
	}
	if shipyardResult.ActionIssue != nil {
		t.Fatalf("unexpected SQLite shipyard issue: %s", *shipyardResult.ActionIssue)
	}
	if _, err := db.Exec("UPDATE `uni1_queue` SET end = 0 WHERE owner_id = ?", account.PlayerID); err != nil {
		t.Fatal(err)
	}
	if err := mysqlgame.NewShipyardRepository(db, "uni1_").FinishDueShipyardQueues(context.Background(), int(time.Now().Unix())); err != nil {
		t.Fatalf("finish SQLite shipyard order: %v", err)
	}
	var lightFighters int
	if err := db.QueryRow("SELECT `204` FROM `uni1_planets` WHERE planet_id = ?", account.HomePlanetID).Scan(&lightFighters); err != nil {
		t.Fatal(err)
	}
	if lightFighters != 1 {
		t.Fatalf("expected completed SQLite light fighter, got %d", lightFighters)
	}
}

func TestSQLiteMCPTokenAndOAuthCodeLifecycle(t *testing.T) {
	db, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlitedb.BootstrapUniverse(context.Background(), db, sqlitedb.BootstrapOptions{Prefix: "uni1_", Secret: "secret"}); err != nil {
		t.Fatal(err)
	}
	repository := mysqlgame.NewMCPTokenRepository(db, "uni1_")
	if err := repository.EnsureMCPTokenSchema(context.Background()); err != nil {
		t.Fatalf("create SQLite MCP token schema: %v", err)
	}
	if err := repository.EnsureMCPOAuthCodeSchema(context.Background()); err != nil {
		t.Fatalf("create SQLite MCP OAuth schema: %v", err)
	}
	now := time.Now().Unix()
	secret := "sqlite-mcp-token"
	digest := sha256.Sum256([]byte(secret))
	token, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{
		PlayerID:  1,
		Name:      "SQLite integration",
		Scopes:    []string{"game:read"},
		CreatedAt: now,
		ExpiresAt: now + 3600,
	}, hex.EncodeToString(digest[:]), 5, now)
	if err != nil {
		t.Fatalf("create SQLite MCP token: %v", err)
	}
	access, err := repository.VerifyMCPToken(context.Background(), secret)
	if err != nil {
		t.Fatalf("verify SQLite MCP token: %v", err)
	}
	if !access.Authenticated || access.PlayerID != 1 {
		t.Fatalf("unexpected SQLite MCP access: %+v", access)
	}
	revoked, err := repository.RevokeMCPToken(context.Background(), 1, token.ID, now+1)
	if err != nil || !revoked {
		t.Fatalf("revoke SQLite MCP token: revoked=%t err=%v", revoked, err)
	}

	code, err := repository.CreateMCPOAuthCode(context.Background(), domainmcp.OAuthAuthorizationCode{
		PlayerID:            1,
		ClientID:            "sqlite-client",
		RedirectURI:         "https://client.example/callback",
		Resource:            "http://localhost:8080/mcp",
		Scopes:              []string{"game:read"},
		CodeHash:            "sqlite-code-hash",
		CodeChallenge:       "sqlite-challenge",
		CodeChallengeMethod: "S256",
		CreatedAt:           now,
		ExpiresAt:           now + 300,
	})
	if err != nil {
		t.Fatalf("create SQLite OAuth code: %v", err)
	}
	consumed, err := repository.ConsumeMCPOAuthCode(context.Background(), code.CodeHash, now+1)
	if err != nil {
		t.Fatalf("consume SQLite OAuth code: %v", err)
	}
	if consumed.ID != code.ID || consumed.ConsumedAt != now+1 {
		t.Fatalf("unexpected consumed SQLite OAuth code: %+v", consumed)
	}
}

func TestSQLiteCouponActivationAcrossMasterAndUniverse(t *testing.T) {
	master, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	universe, err := sqlitedb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer universe.Close()
	options := sqlitedb.BootstrapOptions{Prefix: "uni1_", Secret: "secret", Universe: 1}
	if err := sqlitedb.BootstrapMaster(context.Background(), master, options); err != nil {
		t.Fatal(err)
	}
	if err := sqlitedb.BootstrapUniverse(context.Background(), universe, options); err != nil {
		t.Fatal(err)
	}
	if _, err := master.Exec("INSERT INTO coupons (code, amount, used, user_uni, user_id, user_name) VALUES ('SQLITE-COUPON', 5000, 0, 0, 0, '')"); err != nil {
		t.Fatal(err)
	}
	repository := mysqlgame.NewPaymentRepository(universe, master, "uni1_", 1)
	coupon, activated, err := repository.ActivateCoupon(context.Background(), appgame.PaymentMutationQuery{
		PlayerID:   1,
		CouponCode: "sqlite-coupon",
	})
	if err != nil {
		t.Fatalf("activate SQLite coupon: %v", err)
	}
	if !activated || !coupon.Used || coupon.Amount != 5000 || coupon.UserID != 1 {
		t.Fatalf("unexpected SQLite coupon activation: activated=%t coupon=%+v", activated, coupon)
	}
	var darkMatter int
	if err := universe.QueryRow("SELECT dm FROM `uni1_users` WHERE player_id = 1").Scan(&darkMatter); err != nil {
		t.Fatal(err)
	}
	if darkMatter != 5000 {
		t.Fatalf("expected SQLite coupon dark matter, got %d", darkMatter)
	}
}
