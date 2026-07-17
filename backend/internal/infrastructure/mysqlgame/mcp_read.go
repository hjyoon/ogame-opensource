package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type MCPReadRepository struct {
	queryer Queryer
	prefix  string
	now     func() time.Time
}

func NewMCPReadRepository(db *sql.DB, prefix string) MCPReadRepository {
	return NewMCPReadRepositoryWithQueryer(SQLQueryer{DB: db}, prefix)
}

func NewMCPReadRepositoryWithQueryer(queryer Queryer, prefix string) MCPReadRepository {
	return MCPReadRepository{queryer: queryer, prefix: prefix, now: time.Now}
}

func (r MCPReadRepository) ListMCPPlanets(ctx context.Context, playerID int) ([]domainmcp.Planet, error) {
	if r.queryer == nil {
		return nil, errors.New("mcp read repository queryer unavailable")
	}
	usersTable, planetsTable, _, err := r.mcpReadTables()
	if err != nil {
		return nil, err
	}

	currentPlanetID, sortBy, sortOrder, err := r.loadMCPPlanetListSettings(ctx, usersTable, playerID)
	if err != nil {
		return nil, err
	}
	return r.loadMCPPlanets(ctx, planetsTable, playerID, currentPlanetID, sortBy, sortOrder)
}

func (r MCPReadRepository) GetMCPAccountOverview(ctx context.Context, playerID int) (domainmcp.AccountOverview, error) {
	if r.queryer == nil {
		return domainmcp.AccountOverview{}, errors.New("mcp read repository queryer unavailable")
	}
	usersTable, planetsTable, messagesTable, err := r.mcpReadTables()
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	account, err := r.loadMCPAccount(ctx, usersTable, playerID)
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	currentPlanetID := account.activePlanetID
	if currentPlanetID <= 0 {
		currentPlanetID = account.homePlanetID
	}
	current, err := r.loadMCPPlanet(ctx, planetsTable, playerID, currentPlanetID)
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	planetCount, err := r.countMCPPlanets(ctx, planetsTable, playerID)
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	unread, err := r.countMCPUnreadMessages(ctx, messagesTable, playerID)
	if err != nil {
		return domainmcp.AccountOverview{}, err
	}
	current.Current = true
	return domainmcp.AccountOverview{
		PlayerID:  playerID,
		Commander: account.commander,
		Score: domainmcp.Score{
			Raw:     account.score,
			Display: displayMCPScore(account.score),
			Rank:    account.rank,
		},
		CurrentPlanet:  current,
		PlanetCount:    planetCount,
		UnreadMessages: unread,
	}, nil
}

func (r MCPReadRepository) GetMCPPlanetResources(ctx context.Context, playerID int, planetID int) (domainmcp.PlanetResources, error) {
	if r.queryer == nil {
		return domainmcp.PlanetResources{}, errors.New("mcp read repository queryer unavailable")
	}
	usersTable, planetsTable, _, err := r.mcpReadTables()
	if err != nil {
		return domainmcp.PlanetResources{}, err
	}
	account, err := r.loadMCPResourceAccount(ctx, usersTable, playerID)
	if err != nil {
		return domainmcp.PlanetResources{}, err
	}
	currentPlanetID := account.activePlanetID
	if currentPlanetID <= 0 {
		currentPlanetID = account.homePlanetID
	}
	if planetID <= 0 {
		planetID = currentPlanetID
	}

	row, err := r.loadMCPPlanetResourceRow(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return domainmcp.PlanetResources{}, err
	}
	row.planet.Current = row.planet.ID == currentPlanetID
	result := domainmcp.PlanetResources{
		PlayerID: playerID,
		Planet:   row.planet,
		Resources: domainmcp.ResourceAmounts{
			Metal:      row.metal,
			Crystal:    row.crystal,
			Deuterium:  row.deuterium,
			DarkMatter: account.darkMatter,
		},
	}
	planet := row.gamePlanet(account.darkMatter)
	if row.planet.Type != domaingame.PlanetTypeMoon {
		result.Capacity = domainmcp.ResourceCapacity{
			Metal:     storageCapacity(row.metalStorageLevel),
			Crystal:   storageCapacity(row.crystalStorageLevel),
			Deuterium: storageCapacity(row.deuteriumStorageLevel),
		}
		planet.Resources.MetalCapacity = result.Capacity.Metal
		planet.Resources.CrystalCapacity = result.Capacity.Crystal
		planet.Resources.DeuteriumCapacity = result.Capacity.Deuterium
	}
	if row.planet.Type == domaingame.PlanetTypePlanet {
		speed, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadUniverseSpeed(ctx)
		if err != nil {
			return domainmcp.PlanetResources{}, err
		}
		production := domaingame.BuildResourceProduction(domaingame.Overview{Commander: account.commander, CurrentPlanet: planet}, domaingame.ResourceProductionInputs{
			Levels: domaingame.BuildingLevels{
				domaingame.BuildingMetalMine:      row.metalMine,
				domaingame.BuildingCrystalMine:    row.crystalMine,
				domaingame.BuildingDeuteriumSynth: row.deuteriumSynth,
				domaingame.BuildingSolarPlant:     row.solarPlant,
				domaingame.BuildingFusionReactor:  row.fusionReactor,
			},
			SolarSatellites: row.solarSatellites,
			ProductionFactors: domaingame.ProductionFactors{
				domaingame.BuildingMetalMine:      row.prodMetal,
				domaingame.BuildingCrystalMine:    row.prodCrystal,
				domaingame.BuildingDeuteriumSynth: row.prodDeuterium,
				domaingame.BuildingSolarPlant:     row.prodSolar,
				domaingame.BuildingFusionReactor:  row.prodFusion,
				domaingame.FleetSolarSatellite:    row.prodSatellite,
			},
			EnergyResearch: account.energyResearch,
			UniverseSpeed:  speed,
			Geologist:      account.geologist,
			Engineer:       account.engineer,
		})
		result.Energy = domainmcp.Energy{
			Available: int(production.Totals.Hour.Energy),
			Capacity:  overviewEnergyCapacity(production),
		}
		result.ProductionPerHour = domainmcp.ResourceRates{
			Metal:     production.Totals.Hour.Metal,
			Crystal:   production.Totals.Hour.Crystal,
			Deuterium: production.Totals.Hour.Deuterium,
		}
	}
	return result, nil
}

func (r MCPReadRepository) GetMCPBuildingQueue(ctx context.Context, playerID int, planetID int) (domainmcp.BuildingQueue, error) {
	if r.queryer == nil {
		return domainmcp.BuildingQueue{}, errors.New("mcp read repository queryer unavailable")
	}
	usersTable, planetsTable, _, err := r.mcpReadTables()
	if err != nil {
		return domainmcp.BuildingQueue{}, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return domainmcp.BuildingQueue{}, err
	}
	currentPlanetID, _, _, err := r.loadMCPPlanetListSettings(ctx, usersTable, playerID)
	if err != nil {
		return domainmcp.BuildingQueue{}, err
	}
	if planetID <= 0 {
		planetID = currentPlanetID
	}
	planet, err := r.loadMCPPlanet(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return domainmcp.BuildingQueue{}, err
	}
	planet.Current = planet.ID == currentPlanetID

	now := time.Now
	if r.now != nil {
		now = r.now
	}
	entries, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingQueueEntries(ctx, buildQueueTable, planet.ID, int(now().Unix()))
	if err != nil {
		return domainmcp.BuildingQueue{}, err
	}
	queueEntries := make([]domainmcp.BuildingQueueEntry, 0, len(entries))
	for _, entry := range entries {
		queueEntries = append(queueEntries, domainmcp.BuildingQueueEntry{
			ListID:           entry.ListID,
			TechID:           entry.TechID,
			Name:             entry.Name,
			Level:            entry.Level,
			Destroy:          entry.Destroy,
			Start:            entry.Start,
			End:              entry.End,
			RemainingSeconds: entry.RemainingSeconds,
		})
	}
	return domainmcp.BuildingQueue{
		PlayerID: playerID,
		Planet:   planet,
		Count:    len(queueEntries),
		Entries:  queueEntries,
	}, nil
}

func (r MCPReadRepository) ListMCPMessages(ctx context.Context, playerID int, query domainmcp.MessageQuery) (domainmcp.MessageList, error) {
	if r.queryer == nil {
		return domainmcp.MessageList{}, errors.New("mcp read repository queryer unavailable")
	}
	_, _, messagesTable, err := r.mcpReadTables()
	if err != nil {
		return domainmcp.MessageList{}, err
	}
	rows, err := r.loadMCPMessageRows(ctx, messagesTable, playerID, query)
	if err != nil {
		return domainmcp.MessageList{}, err
	}
	return domainmcp.MessageList{
		PlayerID: playerID,
		Count:    len(rows),
		Limit:    query.Limit,
		Messages: rows,
	}, nil
}

func (r MCPReadRepository) GetMCPMessage(ctx context.Context, playerID int, messageID int) (domainmcp.MessageDetail, error) {
	if r.queryer == nil {
		return domainmcp.MessageDetail{}, errors.New("mcp read repository queryer unavailable")
	}
	_, _, messagesTable, err := r.mcpReadTables()
	if err != nil {
		return domainmcp.MessageDetail{}, err
	}
	row, err := r.loadMCPMessageByID(ctx, messagesTable, playerID, messageID)
	if err != nil {
		return domainmcp.MessageDetail{}, err
	}
	return domainmcp.MessageDetail{PlayerID: playerID, Message: row}, nil
}

func (r MCPReadRepository) GetMCPFleetMovements(ctx context.Context, playerID int) (domainmcp.FleetMovements, error) {
	if r.queryer == nil {
		return domainmcp.FleetMovements{}, errors.New("mcp read repository queryer unavailable")
	}
	queueTable, fleetTable, planetsTable, usersTable, unionTable, err := r.mcpFleetMovementTables()
	if err != nil {
		return domainmcp.FleetMovements{}, err
	}
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	detailLevel, err := r.loadMCPFleetDetailLevel(ctx, usersTable, playerID, now().Unix())
	if err != nil {
		return domainmcp.FleetMovements{}, err
	}
	overviewRepository := NewOverviewRepositoryWithQueryer(r.queryer, r.prefix)
	overviewRepository.now = now
	events, err := overviewRepository.loadOverviewEvents(ctx, queueTable, fleetTable, planetsTable, usersTable, unionTable, playerID, detailLevel)
	if err != nil {
		return domainmcp.FleetMovements{}, err
	}
	movements := make([]domainmcp.FleetMovement, 0, len(events))
	nowUnix := now().Unix()
	for _, event := range events {
		movements = append(movements, mcpFleetMovementFromMission(event, nowUnix))
	}
	return domainmcp.FleetMovements{
		PlayerID: playerID,
		Now:      nowUnix,
		Count:    len(movements),
		Events:   movements,
	}, nil
}

func (r MCPReadRepository) loadMCPMessageRows(ctx context.Context, messagesTable string, playerID int, query domainmcp.MessageQuery) ([]domainmcp.PlayerMessage, error) {
	textColumn := "''"
	if query.IncludeText {
		textColumn = "text"
	}
	statement := fmt.Sprintf("SELECT msg_id, pm, msgfrom, subj, %s, shown, date FROM %s WHERE owner_id = ? AND pm <> ?", textColumn, messagesTable)
	args := []any{playerID, domaingame.MessageTypeBattleReportText}
	if query.HasMessageType {
		statement += " AND pm = ?"
		args = append(args, query.MessageType)
	}
	statement += " ORDER BY date DESC, msg_id DESC LIMIT ?"
	args = append(args, query.Limit)
	rows, err := r.queryer.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []domainmcp.PlayerMessage{}
	for rows.Next() {
		message, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, mcpPlayerMessageFromGame(message))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r MCPReadRepository) loadMCPMessageByID(ctx context.Context, messagesTable string, playerID int, messageID int) (domainmcp.PlayerMessage, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT msg_id, pm, msgfrom, subj, text, shown, date FROM %s WHERE owner_id = ? AND msg_id = ? LIMIT 1", messagesTable), playerID, messageID)
	if err != nil {
		return domainmcp.PlayerMessage{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domainmcp.PlayerMessage{}, err
		}
		return domainmcp.PlayerMessage{}, errors.New("mcp message not found")
	}
	message, err := scanMessageRow(rows)
	if err != nil {
		return domainmcp.PlayerMessage{}, err
	}
	if err := rows.Err(); err != nil {
		return domainmcp.PlayerMessage{}, err
	}
	return mcpPlayerMessageFromGame(message), nil
}

func mcpPlayerMessageFromGame(message domaingame.Message) domainmcp.PlayerMessage {
	return domainmcp.PlayerMessage{
		ID:         message.ID,
		Type:       message.Type,
		TypeName:   mcpMessageTypeName(message.Type),
		From:       message.From,
		Subject:    message.Subject,
		Text:       message.Text,
		Date:       message.Date,
		Unread:     message.Unread,
		Reportable: message.Reportable,
	}
}

func mcpMessageTypeName(messageType int) string {
	switch messageType {
	case domaingame.MessageTypePM:
		return "personal"
	case domaingame.MessageTypeSpyReport:
		return "spy_report"
	case domaingame.MessageTypeBattleReportLink:
		return "battle_report"
	case domaingame.MessageTypeExpedition:
		return "expedition"
	case domaingame.MessageTypeAlliance:
		return "alliance"
	case domaingame.MessageTypeBattleReportText:
		return "battle_report_text"
	default:
		return "other"
	}
}

func (r MCPReadRepository) mcpReadTables() (string, string, string, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return "", "", "", err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return "", "", "", err
	}
	messagesTable, err := tableName(r.prefix, "messages")
	if err != nil {
		return "", "", "", err
	}
	return usersTable, planetsTable, messagesTable, nil
}

func (r MCPReadRepository) mcpFleetMovementTables() (string, string, string, string, string, error) {
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return "", "", "", "", "", err
	}
	fleetTable, err := tableName(r.prefix, "fleet")
	if err != nil {
		return "", "", "", "", "", err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return "", "", "", "", "", err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return "", "", "", "", "", err
	}
	unionTable, err := tableName(r.prefix, "union")
	if err != nil {
		return "", "", "", "", "", err
	}
	return queueTable, fleetTable, planetsTable, usersTable, unionTable, nil
}

type mcpAccountRow struct {
	commander      string
	score          int64
	rank           int
	activePlanetID int
	homePlanetID   int
}

func (r MCPReadRepository) loadMCPAccount(ctx context.Context, usersTable string, playerID int) (mcpAccountRow, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT oname, score1, place1, aktplanet, hplanetid FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return mcpAccountRow{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return mcpAccountRow{}, err
		}
		return mcpAccountRow{}, errors.New("mcp player not found")
	}
	var account mcpAccountRow
	if err := rows.Scan(&account.commander, &account.score, &account.rank, &account.activePlanetID, &account.homePlanetID); err != nil {
		return mcpAccountRow{}, err
	}
	if err := rows.Err(); err != nil {
		return mcpAccountRow{}, err
	}
	return account, nil
}

type mcpResourceAccountRow struct {
	commander      string
	activePlanetID int
	homePlanetID   int
	darkMatter     int
	energyResearch int
	geologist      bool
	engineer       bool
}

func (r MCPReadRepository) loadMCPResourceAccount(ctx context.Context, usersTable string, playerID int) (mcpResourceAccountRow, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(oname, ''), COALESCE(aktplanet, 0), COALESCE(hplanetid, 0), COALESCE(dm, 0), COALESCE(dmfree, 0), COALESCE(`%d`, 0), COALESCE(geo_until, 0), COALESCE(eng_until, 0) FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchEnergy, usersTable),
		playerID,
	)
	if err != nil {
		return mcpResourceAccountRow{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return mcpResourceAccountRow{}, err
		}
		return mcpResourceAccountRow{}, errors.New("mcp player not found")
	}
	var account mcpResourceAccountRow
	var paidDarkMatter int
	var freeDarkMatter int
	var geologistUntil int64
	var engineerUntil int64
	if err := rows.Scan(&account.commander, &account.activePlanetID, &account.homePlanetID, &paidDarkMatter, &freeDarkMatter, &account.energyResearch, &geologistUntil, &engineerUntil); err != nil {
		return mcpResourceAccountRow{}, err
	}
	if err := rows.Err(); err != nil {
		return mcpResourceAccountRow{}, err
	}
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	account.darkMatter = paidDarkMatter + freeDarkMatter
	account.geologist = geologistUntil > now().Unix()
	account.engineer = engineerUntil > now().Unix()
	return account, nil
}

func (r MCPReadRepository) loadMCPFleetDetailLevel(ctx context.Context, usersTable string, playerID int, now int64) (int, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT COALESCE(`%d`, 0), COALESCE(tec_until, 0) FROM %s WHERE player_id = ? LIMIT 1", domaingame.ResearchEspionage, usersTable),
		playerID,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, errors.New("mcp player fleet detail not found")
	}
	var espionage int
	var technocratUntil int64
	if err := rows.Scan(&espionage, &technocratUntil); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return overviewFleetDetailLevel(overviewUser{
		EspionageResearch: espionage,
		Officers:          domaingame.OverviewOfficers{Technocrat: technocratUntil > now},
	}), nil
}

type mcpPlanetResourceRow struct {
	planet                domainmcp.Planet
	metal                 float64
	crystal               float64
	deuterium             float64
	temperature           int
	metalStorageLevel     int
	crystalStorageLevel   int
	deuteriumStorageLevel int
	metalMine             int
	crystalMine           int
	deuteriumSynth        int
	solarPlant            int
	fusionReactor         int
	solarSatellites       int
	prodMetal             float64
	prodCrystal           float64
	prodDeuterium         float64
	prodSolar             float64
	prodFusion            float64
	prodSatellite         float64
}

func (r MCPReadRepository) loadMCPPlanetResourceRow(ctx context.Context, planetsTable string, playerID int, planetID int) (mcpPlanetResourceRow, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf(
			"SELECT planet_id, name, type, g, s, p, COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(temp, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(`%d`, 0), COALESCE(prod%d, 0), COALESCE(prod%d, 0), COALESCE(prod%d, 0), COALESCE(prod%d, 0), COALESCE(prod%d, 0), COALESCE(prod%d, 0) FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1",
			domaingame.ResourceMetal,
			domaingame.ResourceCrystal,
			domaingame.ResourceDeuterium,
			domaingame.BuildingMetalStorage,
			domaingame.BuildingCrystalStorage,
			domaingame.BuildingDeuteriumTank,
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			domaingame.BuildingMetalMine,
			domaingame.BuildingCrystalMine,
			domaingame.BuildingDeuteriumSynth,
			domaingame.BuildingSolarPlant,
			domaingame.BuildingFusionReactor,
			domaingame.FleetSolarSatellite,
			planetsTable,
		),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return mcpPlanetResourceRow{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return mcpPlanetResourceRow{}, err
		}
		return mcpPlanetResourceRow{}, errors.New("mcp planet resources not found")
	}
	var row mcpPlanetResourceRow
	if err := rows.Scan(
		&row.planet.ID,
		&row.planet.Name,
		&row.planet.Type,
		&row.planet.Coordinates.Galaxy,
		&row.planet.Coordinates.System,
		&row.planet.Coordinates.Position,
		&row.metal,
		&row.crystal,
		&row.deuterium,
		&row.temperature,
		&row.metalStorageLevel,
		&row.crystalStorageLevel,
		&row.deuteriumStorageLevel,
		&row.metalMine,
		&row.crystalMine,
		&row.deuteriumSynth,
		&row.solarPlant,
		&row.fusionReactor,
		&row.solarSatellites,
		&row.prodMetal,
		&row.prodCrystal,
		&row.prodDeuterium,
		&row.prodSolar,
		&row.prodFusion,
		&row.prodSatellite,
	); err != nil {
		return mcpPlanetResourceRow{}, err
	}
	if err := rows.Err(); err != nil {
		return mcpPlanetResourceRow{}, err
	}
	row.planet.TypeName = mcpPlanetTypeName(row.planet.Type)
	return row, nil
}

func (r mcpPlanetResourceRow) gamePlanet(darkMatter int) domaingame.PlanetOverview {
	return domaingame.PlanetOverview{
		ID:          r.planet.ID,
		Name:        r.planet.Name,
		Type:        r.planet.Type,
		Coordinates: domaingame.Coordinates{Galaxy: r.planet.Coordinates.Galaxy, System: r.planet.Coordinates.System, Position: r.planet.Coordinates.Position},
		Temperature: r.temperature,
		Resources: domaingame.Resources{
			Metal:      r.metal,
			Crystal:    r.crystal,
			Deuterium:  r.deuterium,
			DarkMatter: darkMatter,
		},
	}
}

func mcpFleetMovementFromMission(mission domaingame.FleetMission, now int64) domainmcp.FleetMovement {
	ships := make([]domainmcp.FleetShip, 0, len(mission.Ships))
	for _, ship := range mission.Ships {
		ships = append(ships, domainmcp.FleetShip{
			ID:    ship.ID,
			Name:  ship.Name,
			Count: ship.Count,
		})
	}
	unionPlayers := make([]domainmcp.FleetUnionPlayer, 0, len(mission.UnionPlayers))
	for _, player := range mission.UnionPlayers {
		unionPlayers = append(unionPlayers, domainmcp.FleetUnionPlayer{
			ID:   player.ID,
			Name: player.Name,
		})
	}
	group := make([]domainmcp.FleetMovement, 0, len(mission.GroupMissions))
	for _, grouped := range mission.GroupMissions {
		group = append(group, mcpFleetMovementFromMission(grouped, now))
	}
	remaining := mission.ArrivalAt - now
	if remaining < 0 {
		remaining = 0
	}
	return domainmcp.FleetMovement{
		ID:               mission.ID,
		OwnerID:          mission.OwnerID,
		OwnerName:        mission.OwnerName,
		Foreign:          mission.Foreign,
		Mission:          mission.Mission,
		MissionName:      mission.MissionName,
		StateTitle:       mission.StateTitle,
		StateShort:       mission.StateShort,
		FleetDetailLevel: mission.FleetDetailLevel,
		Ships:            ships,
		TotalShips:       mission.TotalShips,
		LoadedResources: domainmcp.FleetResources{
			Metal:     mission.LoadedResources[domaingame.ResourceMetal],
			Crystal:   mission.LoadedResources[domaingame.ResourceCrystal],
			Deuterium: mission.LoadedResources[domaingame.ResourceDeuterium],
		},
		MissileAmount:    mission.MissileAmount,
		MissileTargetID:  mission.MissileTargetID,
		MissileTarget:    mission.MissileTarget,
		UnionID:          mission.UnionID,
		UnionName:        mission.UnionName,
		UnionPlayers:     unionPlayers,
		GroupMissions:    group,
		Origin:           domainmcp.Coordinates{Galaxy: mission.Origin.Galaxy, System: mission.Origin.System, Position: mission.Origin.Position},
		OriginName:       mission.OriginName,
		Target:           domainmcp.Coordinates{Galaxy: mission.Target.Galaxy, System: mission.Target.System, Position: mission.Target.Position},
		TargetName:       mission.TargetName,
		TargetType:       mission.TargetType,
		TargetOwnerName:  mission.TargetOwnerName,
		DepartureAt:      mission.DepartureAt,
		ArrivalAt:        mission.ArrivalAt,
		RemainingSeconds: int(remaining),
		CanRecall:        mission.CanRecall,
		CanCreateUnion:   mission.CanCreateUnion,
	}
}

func (r MCPReadRepository) loadMCPPlanetListSettings(ctx context.Context, usersTable string, playerID int) (int, int, int, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT aktplanet, hplanetid, sortby, sortorder FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, 0, err
		}
		return 0, 0, 0, errors.New("mcp player not found")
	}
	var activePlanetID int
	var homePlanetID int
	var sortBy int
	var sortOrder int
	if err := rows.Scan(&activePlanetID, &homePlanetID, &sortBy, &sortOrder); err != nil {
		return 0, 0, 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}
	if activePlanetID > 0 {
		return activePlanetID, sortBy, sortOrder, nil
	}
	return homePlanetID, sortBy, sortOrder, nil
}

func (r MCPReadRepository) loadMCPPlanet(ctx context.Context, planetsTable string, playerID int, planetID int) (domainmcp.Planet, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, name, type, g, s, p FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1", planetsTable),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return domainmcp.Planet{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domainmcp.Planet{}, err
		}
		return domainmcp.Planet{}, errors.New("mcp current planet not found")
	}
	planet, err := scanMCPPlanet(rows)
	if err != nil {
		return domainmcp.Planet{}, err
	}
	if err := rows.Err(); err != nil {
		return domainmcp.Planet{}, err
	}
	return planet, nil
}

func (r MCPReadRepository) loadMCPPlanets(ctx context.Context, planetsTable string, playerID int, currentPlanetID int, sortBy int, sortOrder int) ([]domainmcp.Planet, error) {
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, name, type, g, s, p FROM %s WHERE owner_id = ? AND type < ?%s", planetsTable, planetOrder(sortBy, sortOrder)),
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	planets := make([]domainmcp.Planet, 0)
	for rows.Next() {
		planet, err := scanMCPPlanet(rows)
		if err != nil {
			return nil, err
		}
		planet.Current = planet.ID == currentPlanetID
		planets = append(planets, planet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return planets, nil
}

func (r MCPReadRepository) countMCPPlanets(ctx context.Context, planetsTable string, playerID int) (int, error) {
	return r.singleMCPCount(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ? AND type < ?", planetsTable), playerID, planetTypeDebris)
}

func (r MCPReadRepository) countMCPUnreadMessages(ctx context.Context, messagesTable string, playerID int) (int, error) {
	return r.singleMCPCount(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE owner_id = ? AND shown = 0", messagesTable), playerID)
}

func (r MCPReadRepository) singleMCPCount(ctx context.Context, statement string, args ...any) (int, error) {
	rows, err := r.queryer.QueryContext(ctx, statement, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func scanMCPPlanet(rows Rows) (domainmcp.Planet, error) {
	var planet domainmcp.Planet
	if err := rows.Scan(&planet.ID, &planet.Name, &planet.Type, &planet.Coordinates.Galaxy, &planet.Coordinates.System, &planet.Coordinates.Position); err != nil {
		return domainmcp.Planet{}, err
	}
	planet.TypeName = mcpPlanetTypeName(planet.Type)
	return planet, nil
}

func displayMCPScore(score int64) int64 {
	if score < 0 {
		return 0
	}
	return score / domaingame.ScoreDisplayScale
}

func mcpPlanetTypeName(planetType int) string {
	switch planetType {
	case domaingame.PlanetTypePlanet:
		return "planet"
	case domaingame.PlanetTypeMoon:
		return "moon"
	default:
		return "unknown"
	}
}
