package mysqlgame

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const botQueuePriority = 1000

var (
	botFunctionPattern     = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)$`)
	botPercentBranch       = regexp.MustCompile(`^([0-9]{1,2}|100)%$`)
	botComparisonOperators = []string{"===", "!==", "==", "!=", ">=", "<=", ">", "<"}
)

type BotRuntimeRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	now     func() time.Time
	randInt func(max int) int
}

type botStrategyGraph struct {
	Nodes []botStrategyNode `json:"nodeDataArray"`
	Links []botStrategyLink `json:"linkDataArray"`
}

type botStrategyNode struct {
	Key      int    `json:"key"`
	Category string `json:"category"`
	Text     string `json:"text"`
}

type botStrategyLink struct {
	From     int    `json:"from"`
	To       int    `json:"to"`
	Text     string `json:"text"`
	FromPort string `json:"fromPort"`
}

type botValue struct {
	text string
	num  float64
	bool bool
	kind string
}

type botActivePlanet struct {
	ID          int
	Type        int
	Temperature int
	Resources   domaingame.Resources
}

func (r BotRuntimeRepository) FinishDueBotQueues(ctx context.Context, until int) error {
	if r.execer == nil {
		return fmt.Errorf("bot queue updater unavailable")
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return err
	}
	botstratTable, err := tableName(r.prefix, "botstrat")
	if err != nil {
		return err
	}
	botvarsTable, err := tableName(r.prefix, "botvars")
	if err != nil {
		return err
	}
	config, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingUniverseConfig(ctx)
	if err != nil {
		return err
	}
	if config.Frozen {
		return nil
	}
	tasks, err := r.loadDueBotQueueTasks(ctx, queueTable, until, buildQueueBatch)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := r.finishBotQueueTask(ctx, queueTable, botstratTable, botvarsTable, task); err != nil {
			return err
		}
	}
	return nil
}

func (r BotRuntimeRepository) loadDueBotQueueTasks(ctx context.Context, queueTable string, until int, limit int) ([]buildingQueueTask, error) {
	if limit <= 0 {
		limit = buildQueueBatch
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT task_id, owner_id, type, sub_id, obj_id, level, start, end, prio, freeze, frozen FROM %s WHERE end <= ? AND freeze = 0 AND type = ? ORDER BY end ASC, prio DESC LIMIT ?", queueTable),
		until,
		queueTypeAI,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []buildingQueueTask{}
	for rows.Next() {
		var task buildingQueueTask
		if err := rows.Scan(&task.TaskID, &task.OwnerID, &task.Type, &task.SubID, &task.ObjID, &task.Level, &task.Start, &task.End, &task.Prio, &task.Freeze, &task.Frozen); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r BotRuntimeRepository) finishBotQueueTask(ctx context.Context, queueTable string, botstratTable string, botvarsTable string, task buildingQueueTask) error {
	graph, found, err := r.loadBotStrategyByID(ctx, botstratTable, task.SubID)
	if err != nil || !found {
		return err
	}
	var block *botStrategyNode
	for index := range graph.Nodes {
		if graph.Nodes[index].Key == task.ObjID {
			block = &graph.Nodes[index]
			break
		}
	}
	if block == nil {
		return nil
	}
	children := make([]botStrategyLink, 0, len(graph.Links))
	for _, link := range graph.Links {
		if link.From == block.Key {
			children = append(children, link)
		}
	}
	return r.executeBotBlock(ctx, queueTable, botstratTable, botvarsTable, task, *block, graph, children)
}

func (r BotRuntimeRepository) executeBotBlock(ctx context.Context, queueTable string, botstratTable string, botvarsTable string, task buildingQueueTask, block botStrategyNode, graph botStrategyGraph, children []botStrategyLink) error {
	addChild := func(blockID int, seconds int) error {
		if blockID == 0 {
			return nil
		}
		return r.insertBotQueue(ctx, queueTable, task.OwnerID, task.SubID, blockID, task.End, seconds)
	}
	removeCurrent := func() error {
		return (BuildingsRepository{execer: r.execer}).removeGlobalQueue(ctx, queueTable, task.TaskID)
	}

	switch block.Category {
	case "Start":
		if len(children) > 0 {
			if err := addChild(children[0].To, 0); err != nil {
				return err
			}
		}
		return removeCurrent()
	case "End":
		return removeCurrent()
	case "Label":
		if len(children) > 0 {
			next := children[0].To
			for _, child := range children {
				if child.FromPort == "B" {
					next = child.To
					break
				}
			}
			if err := addChild(next, 0); err != nil {
				return err
			}
		}
		return removeCurrent()
	case "Branch":
		for _, node := range graph.Nodes {
			if node.Category == "Label" && node.Text == block.Text {
				if err := addChild(node.Key, 0); err != nil {
					return err
				}
				break
			}
		}
		return removeCurrent()
	case "Cond":
		result, err := r.evalBotCondition(ctx, botstratTable, botvarsTable, task, block.Text)
		if err != nil {
			result = false
		}
		next := r.chooseBotConditionChild(result, children)
		if next != 0 {
			if err := addChild(next, 0); err != nil {
				return err
			}
		}
		return removeCurrent()
	default:
		sleep, err := r.executeBotStatements(ctx, botstratTable, botvarsTable, task, block.Text)
		if err != nil {
			sleep = 0
		}
		if len(children) > 0 {
			if err := addChild(children[0].To, sleep); err != nil {
				return err
			}
		}
		return removeCurrent()
	}
}

func (r BotRuntimeRepository) chooseBotConditionChild(result bool, children []botStrategyLink) int {
	noBlock := 0
	for _, child := range children {
		text := strings.ToLower(strings.TrimSpace(child.Text))
		switch text {
		case "no":
			if !result {
				return child.To
			}
			noBlock = child.To
		case "yes":
			if result {
				return child.To
			}
		default:
			if !result {
				continue
			}
			match := botPercentBranch.FindStringSubmatch(text)
			if len(match) == 0 {
				continue
			}
			percent, err := strconv.Atoi(match[1])
			if err != nil {
				continue
			}
			if r.rollBotPercent() <= percent {
				return child.To
			}
			if noBlock != 0 {
				return noBlock
			}
			result = false
		}
	}
	return 0
}

func (r BotRuntimeRepository) rollBotPercent() int {
	if r.randInt != nil {
		roll := r.randInt(100)
		if roll < 1 {
			return 1
		}
		if roll > 100 {
			return 100
		}
		return roll
	}
	return rand.Intn(100) + 1
}

func (r BotRuntimeRepository) insertBotQueue(ctx context.Context, queueTable string, playerID int, strategyID int, blockID int, start int, seconds int) error {
	if seconds < 0 {
		seconds = 0
	}
	_, err := r.execer.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s (owner_id, type, sub_id, obj_id, level, start, end, prio) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", queueTable),
		playerID,
		queueTypeAI,
		strategyID,
		blockID,
		0,
		start,
		start+seconds,
		botQueuePriority,
	)
	return err
}

func (r BotRuntimeRepository) loadBotStrategyByID(ctx context.Context, botstratTable string, strategyID int) (botStrategyGraph, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(source, '') FROM %s WHERE id = ? LIMIT 1", botstratTable), strategyID)
	if err != nil {
		return botStrategyGraph{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return botStrategyGraph{}, false, err
		}
		return botStrategyGraph{}, false, nil
	}
	var source string
	if err := rows.Scan(&source); err != nil {
		return botStrategyGraph{}, false, err
	}
	if err := rows.Err(); err != nil {
		return botStrategyGraph{}, false, err
	}
	graph, err := parseBotStrategyGraph(source)
	if err != nil {
		return botStrategyGraph{}, true, nil
	}
	return graph, true, nil
}

func (r BotRuntimeRepository) loadBotStrategyByName(ctx context.Context, botstratTable string, name string) (int, botStrategyGraph, bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT id, COALESCE(source, '') FROM %s WHERE name = ? LIMIT 1", botstratTable), name)
	if err != nil {
		return 0, botStrategyGraph{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, botStrategyGraph{}, false, err
		}
		return 0, botStrategyGraph{}, false, nil
	}
	var id int
	var source string
	if err := rows.Scan(&id, &source); err != nil {
		return 0, botStrategyGraph{}, false, err
	}
	if err := rows.Err(); err != nil {
		return 0, botStrategyGraph{}, false, err
	}
	graph, err := parseBotStrategyGraph(source)
	if err != nil {
		return id, botStrategyGraph{}, true, nil
	}
	return id, graph, true, nil
}

func parseBotStrategyGraph(source string) (botStrategyGraph, error) {
	graph := botStrategyGraph{}
	if strings.TrimSpace(source) == "" {
		return graph, nil
	}
	if err := json.Unmarshal([]byte(source), &graph); err != nil {
		return botStrategyGraph{}, err
	}
	return graph, nil
}

func (r BotRuntimeRepository) executeBotStatements(ctx context.Context, botstratTable string, botvarsTable string, task buildingQueueTask, source string) (int, error) {
	sleep := 0
	for _, statement := range splitBotStatements(source, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if strings.HasPrefix(statement, "return ") {
			value, err := r.evalBotValue(ctx, botstratTable, botvarsTable, task, strings.TrimSpace(strings.TrimPrefix(statement, "return ")))
			if err != nil {
				return sleep, err
			}
			return value.asInt(), nil
		}
		value, err := r.evalBotValue(ctx, botstratTable, botvarsTable, task, statement)
		if err != nil {
			return sleep, err
		}
		if value.kind == "number" && value.asInt() > 0 {
			sleep = value.asInt()
		}
	}
	return sleep, nil
}

func (r BotRuntimeRepository) evalBotCondition(ctx context.Context, botstratTable string, botvarsTable string, task buildingQueueTask, expression string) (bool, error) {
	expression = trimBotExpression(expression)
	if expression == "" {
		return false, nil
	}
	if parts := splitTopLevelBotExpression(expression, "||"); len(parts) > 1 {
		for _, part := range parts {
			ok, err := r.evalBotCondition(ctx, botstratTable, botvarsTable, task, part)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}
	if parts := splitTopLevelBotExpression(expression, "&&"); len(parts) > 1 {
		for _, part := range parts {
			ok, err := r.evalBotCondition(ctx, botstratTable, botvarsTable, task, part)
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	}
	if strings.HasPrefix(expression, "!") {
		ok, err := r.evalBotCondition(ctx, botstratTable, botvarsTable, task, strings.TrimSpace(strings.TrimPrefix(expression, "!")))
		return !ok, err
	}
	for _, op := range botComparisonOperators {
		left, right, ok := splitTopLevelComparison(expression, op)
		if !ok {
			continue
		}
		leftValue, err := r.evalBotValue(ctx, botstratTable, botvarsTable, task, left)
		if err != nil {
			return false, err
		}
		rightValue, err := r.evalBotValue(ctx, botstratTable, botvarsTable, task, right)
		if err != nil {
			return false, err
		}
		return compareBotValues(leftValue, rightValue, op), nil
	}
	value, err := r.evalBotValue(ctx, botstratTable, botvarsTable, task, expression)
	if err != nil {
		return false, err
	}
	return value.truthy(), nil
}

func (r BotRuntimeRepository) evalBotValue(ctx context.Context, botstratTable string, botvarsTable string, task buildingQueueTask, expression string) (botValue, error) {
	expression = trimBotExpression(expression)
	if expression == "" || strings.EqualFold(expression, "null") {
		return botValue{}, nil
	}
	if strings.EqualFold(expression, "true") {
		return botValue{bool: true, num: 1, text: "1", kind: "bool"}, nil
	}
	if strings.EqualFold(expression, "false") {
		return botValue{bool: false, num: 0, text: "", kind: "bool"}, nil
	}
	if unquoted, ok := unquoteBotString(expression); ok {
		return botValue{text: unquoted, kind: "string"}, nil
	}
	if number, err := strconv.ParseFloat(expression, 64); err == nil {
		return botValue{text: trimFloatText(number), num: number, bool: number != 0, kind: "number"}, nil
	}
	match := botFunctionPattern.FindStringSubmatch(expression)
	if len(match) != 3 {
		return botValue{}, nil
	}
	args := parseBotArguments(match[2])
	switch match[1] {
	case "BotIdle":
		return botValue{}, nil
	case "BotStrategyExists":
		name := botArgString(args, 0, "")
		exists, err := r.botStrategyExists(ctx, botstratTable, name)
		return boolBotValue(exists), err
	case "BotExec":
		name := botArgString(args, 0, "")
		ok, err := r.botExec(ctx, botstratTable, task, name)
		return boolBotValue(ok), err
	case "BotGetVar":
		name := botArgString(args, 0, "")
		def := botArgNullableString(args, 1)
		value, err := r.botGetVar(ctx, botvarsTable, task.OwnerID, name, def)
		if value == nil {
			return botValue{}, err
		}
		return stringBotValue(*value), err
	case "BotSetVar":
		name := botArgString(args, 0, "")
		value := botArgString(args, 1, "")
		return botValue{}, r.botSetVar(ctx, botvarsTable, task.OwnerID, name, value)
	case "BotGetBuild":
		level, err := r.botGetBuild(ctx, task.OwnerID, botArgInt(args, 0, 0))
		return numberBotValue(level), err
	case "BotCanBuild":
		ok, err := r.botCanBuild(ctx, task.OwnerID, botArgInt(args, 0, 0), task.End)
		return boolBotValue(ok), err
	case "BotBuild":
		duration, err := r.botBuild(ctx, task.OwnerID, botArgInt(args, 0, 0), task.End)
		return numberBotValue(duration), err
	case "BotResourceSettings":
		values := []int{100, 100, 100, 100, 100, 100}
		for index := range values {
			values[index] = botArgInt(args, index, values[index])
		}
		return botValue{}, r.botResourceSettings(ctx, task.OwnerID, values)
	case "BotEnergyAbove":
		ok, err := r.botEnergyAbove(ctx, task.OwnerID, botArgInt(args, 0, 0))
		return boolBotValue(ok), err
	case "BotGetResearch":
		level, err := r.botGetResearch(ctx, task.OwnerID, botArgInt(args, 0, 0))
		return numberBotValue(level), err
	case "BotCanResearch":
		ok, err := r.botCanResearch(ctx, task.OwnerID, botArgInt(args, 0, 0), task.End)
		return boolBotValue(ok), err
	case "BotResearch":
		duration, err := r.botResearch(ctx, task.OwnerID, botArgInt(args, 0, 0), task.End)
		return numberBotValue(duration), err
	case "BotBuildFleet":
		duration, err := r.botBuildFleet(ctx, task.OwnerID, botArgInt(args, 0, 0), botArgInt(args, 1, 0), task.End)
		return numberBotValue(duration), err
	default:
		return botValue{}, nil
	}
}

func (r BotRuntimeRepository) botStrategyExists(ctx context.Context, botstratTable string, name string) (bool, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT id FROM %s WHERE name = ? LIMIT 1", botstratTable), name)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	exists := rows.Next()
	return exists, rows.Err()
}

func (r BotRuntimeRepository) botExec(ctx context.Context, botstratTable string, task buildingQueueTask, name string) (bool, error) {
	strategyID, graph, found, err := r.loadBotStrategyByName(ctx, botstratTable, name)
	if err != nil || !found {
		return false, err
	}
	for _, node := range graph.Nodes {
		if node.Category == "Start" {
			queueTable, err := tableName(r.prefix, "queue")
			if err != nil {
				return false, err
			}
			return true, r.insertBotQueue(ctx, queueTable, task.OwnerID, strategyID, node.Key, task.End, 0)
		}
	}
	return false, nil
}

func (r BotRuntimeRepository) botGetVar(ctx context.Context, botvarsTable string, ownerID int, name string, def *string) (*string, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(value, '') FROM %s WHERE var = ? AND owner_id = ? LIMIT 1", botvarsTable), name, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		return &value, rows.Err()
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var value any
	if def != nil {
		value = *def
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, var, value) VALUES (?, ?, ?)", botvarsTable), ownerID, name, value); err != nil {
		return nil, err
	}
	return def, nil
}

func (r BotRuntimeRepository) botSetVar(ctx context.Context, botvarsTable string, ownerID int, name string, value string) error {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT id FROM %s WHERE var = ? AND owner_id = ? LIMIT 1", botvarsTable), name, ownerID)
	if err != nil {
		return err
	}
	defer rows.Close()
	exists := rows.Next()
	if err := rows.Err(); err != nil {
		return err
	}
	if exists {
		_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET value = ? WHERE var = ? AND owner_id = ?", botvarsTable), value, name, ownerID)
		return err
	}
	_, err = r.execer.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (owner_id, var, value) VALUES (?, ?, ?)", botvarsTable), ownerID, name, value)
	return err
}

func (r BotRuntimeRepository) botGetBuild(ctx context.Context, playerID int, techID int) (int, error) {
	planetID, err := r.loadBotActivePlanetID(ctx, playerID)
	if err != nil || planetID == 0 {
		return 0, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return 0, err
	}
	planet, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingMutationPlanet(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return 0, err
	}
	return planet.Levels[techID], nil
}

func (r BotRuntimeRepository) botCanBuild(ctx context.Context, playerID int, techID int, now int) (bool, error) {
	_, _, duration, err := r.botValidateBuild(ctx, playerID, techID, now)
	if err != nil {
		return false, err
	}
	return duration > 0, nil
}

func (r BotRuntimeRepository) botBuild(ctx context.Context, playerID int, techID int, now int) (int, error) {
	usersTable, planetsTable, buildQueueTable, queueTable, err := r.buildingTables()
	if err != nil {
		return 0, err
	}
	planetID, err := r.loadBotActivePlanetID(ctx, playerID)
	if err != nil || planetID == 0 {
		return 0, err
	}
	issue, _, duration, err := r.botValidateBuild(ctx, playerID, techID, now)
	if err != nil || issue != nil || duration <= 0 {
		return 0, err
	}
	issue, err = (BuildingsRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix}).enqueueBuilding(ctx, usersTable, planetsTable, buildQueueTable, queueTable, playerID, planetID, techID, false, now)
	if err != nil || issue != nil {
		return 0, err
	}
	return duration, nil
}

func (r BotRuntimeRepository) botValidateBuild(ctx context.Context, playerID int, techID int, now int) (*domaingame.BuildingsActionIssue, domaingame.BuildingCost, int, error) {
	_ = now
	usersTable, planetsTable, buildQueueTable, queueTable, err := r.buildingTables()
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	planetID, err := r.loadBotActivePlanetID(ctx, playerID)
	if err != nil || planetID == 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), domaingame.BuildingCost{}, 0, err
	}
	buildings := BuildingsRepository{queryer: r.queryer, prefix: r.prefix}
	user, err := buildings.loadBuildingMutationUser(ctx, usersTable, playerID)
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	if user.Vacation {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueVacation), domaingame.BuildingCost{}, 0, nil
	}
	config, err := buildings.loadBuildingUniverseConfig(ctx)
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	if config.Frozen {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueUniversePause), domaingame.BuildingCost{}, 0, nil
	}
	planet, err := buildings.loadBuildingMutationPlanet(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	rows, err := buildings.loadBuildQueueRows(ctx, buildQueueTable, planetID)
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	nowLevel := planet.Levels[techID]
	listID := 1
	for _, row := range rows {
		if row.TechID == techID {
			nowLevel = row.Level
		}
		if row.ListID >= listID {
			listID = row.ListID + 1
		}
	}
	return buildings.validateBuildingOrder(ctx, queueTable, user, planet, techID, nowLevel+1, false, listID == 1, config.Speed)
}

func (r BotRuntimeRepository) botResourceSettings(ctx context.Context, playerID int, values []int) error {
	planetID, err := r.loadBotActivePlanetID(ctx, playerID)
	if err != nil || planetID == 0 {
		return err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return err
	}
	normalized := make([]float64, 6)
	for index, value := range values {
		if value < 0 {
			value = 0
		}
		if value > 100 {
			value = 100
		}
		normalized[index] = math.Round(float64(value)/10) * 10 / 100
	}
	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET prod%d = ?, prod%d = ?, prod%d = ?, prod%d = ?, prod%d = ?, prod%d = ? WHERE planet_id = ? AND owner_id = ?", planetsTable, domaingame.BuildingMetalMine, domaingame.BuildingCrystalMine, domaingame.BuildingDeuteriumSynth, domaingame.BuildingSolarPlant, domaingame.BuildingFusionReactor, domaingame.FleetSolarSatellite),
		normalized[0],
		normalized[1],
		normalized[2],
		normalized[3],
		normalized[4],
		normalized[5],
		planetID,
		playerID,
	)
	return err
}

func (r BotRuntimeRepository) botEnergyAbove(ctx context.Context, playerID int, energy int) (bool, error) {
	planet, err := r.loadBotActivePlanet(ctx, playerID)
	if err != nil || planet.ID == 0 {
		return false, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return false, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return false, err
	}
	resources := ResourcesRepository{queryer: r.queryer, prefix: r.prefix, now: r.clock()}
	levels, satellites, factors, err := resources.loadProductionSettings(ctx, planetsTable, playerID, planet.ID)
	if err != nil {
		return false, err
	}
	energyResearch, _, engineer, err := resources.loadResourceUser(ctx, usersTable, playerID)
	if err != nil {
		return false, err
	}
	speed, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadUniverseSpeed(ctx)
	if err != nil {
		return false, err
	}
	production := domaingame.BuildResourceProduction(domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{
		ID:          planet.ID,
		Type:        planet.Type,
		Temperature: planet.Temperature,
	}}, domaingame.ResourceProductionInputs{
		Levels:            levels,
		SolarSatellites:   satellites,
		ProductionFactors: factors,
		EnergyResearch:    energyResearch,
		UniverseSpeed:     speed,
		Engineer:          engineer,
	})
	return int(production.Totals.Hour.Energy) >= energy, nil
}

func (r BotRuntimeRepository) botGetResearch(ctx context.Context, playerID int, techID int) (int, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return 0, err
	}
	user, err := (ResearchRepository{queryer: r.queryer, prefix: r.prefix}).loadResearchMutationUser(ctx, usersTable, playerID)
	if err != nil {
		return 0, err
	}
	return user.Research[techID], nil
}

func (r BotRuntimeRepository) botCanResearch(ctx context.Context, playerID int, techID int, now int) (bool, error) {
	issue, _, duration, err := r.botValidateResearch(ctx, playerID, techID, now)
	if err != nil || issue != nil {
		return false, err
	}
	return duration > 0, nil
}

func (r BotRuntimeRepository) botResearch(ctx context.Context, playerID int, techID int, now int) (int, error) {
	usersTable, planetsTable, queueTable, err := r.researchTables()
	if err != nil {
		return 0, err
	}
	planetID, err := r.loadBotActivePlanetID(ctx, playerID)
	if err != nil || planetID == 0 {
		return 0, err
	}
	issue, _, duration, err := r.botValidateResearch(ctx, playerID, techID, now)
	if err != nil || issue != nil || duration <= 0 {
		return 0, err
	}
	issue, err = (ResearchRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.clock()}).startResearch(ctx, usersTable, planetsTable, queueTable, playerID, planetID, techID, now)
	if err != nil || issue != nil {
		return 0, err
	}
	return duration, nil
}

func (r BotRuntimeRepository) botValidateResearch(ctx context.Context, playerID int, techID int, now int) (*domaingame.BuildingsActionIssue, domaingame.BuildingCost, int, error) {
	usersTable, planetsTable, queueTable, err := r.researchTables()
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	planetID, err := r.loadBotActivePlanetID(ctx, playerID)
	if err != nil || planetID == 0 {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueInvalid), domaingame.BuildingCost{}, 0, err
	}
	research := ResearchRepository{queryer: r.queryer, prefix: r.prefix, now: r.clock()}
	user, err := research.loadResearchMutationUser(ctx, usersTable, playerID)
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	if user.Vacation {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueVacation), domaingame.BuildingCost{}, 0, nil
	}
	config, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingUniverseConfig(ctx)
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	if config.Frozen {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueUniversePause), domaingame.BuildingCost{}, 0, nil
	}
	if active, err := research.loadActiveResearchQueue(ctx, queueTable, playerID, planetID, now); err != nil || active != nil {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueBusy), domaingame.BuildingCost{}, 0, err
	}
	if busy, err := research.researchLabBusy(ctx, queueTable, playerID); err != nil || busy {
		return domaingame.BuildingActionIssue(domaingame.BuildingsIssueBusy), domaingame.BuildingCost{}, 0, err
	}
	planet, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingMutationPlanet(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return nil, domaingame.BuildingCost{}, 0, err
	}
	return research.validateResearchOrder(ctx, planetsTable, user, planet, techID, user.Research[techID]+1, config.Speed)
}

func (r BotRuntimeRepository) botBuildFleet(ctx context.Context, playerID int, unitID int, amount int, now int) (int, error) {
	if amount <= 0 {
		return 0, nil
	}
	planetID, err := r.loadBotActivePlanetID(ctx, playerID)
	if err != nil || planetID == 0 {
		return 0, err
	}
	state, err := r.loadBotShipyardState(ctx, playerID, planetID, unitID)
	if err != nil {
		return 0, err
	}
	for _, item := range state.items {
		if item.ID != unitID {
			continue
		}
		if item.DurationSeconds <= 0 || !item.CanBuild {
			return 0, nil
		}
		issue, ok, err := (ShipyardRepository{queryer: r.queryer, execer: r.execer, prefix: r.prefix, now: r.clock()}).enqueueShipyardItem(ctx, state, item, amount, now)
		if err != nil || issue != nil || !ok {
			return 0, err
		}
		return item.DurationSeconds, nil
	}
	return 0, nil
}

func (r BotRuntimeRepository) loadBotShipyardState(ctx context.Context, playerID int, planetID int, unitID int) (shipyardMutationState, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return shipyardMutationState{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return shipyardMutationState{}, err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return shipyardMutationState{}, err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return shipyardMutationState{}, err
	}
	shipyard := ShipyardRepository{queryer: r.queryer, prefix: r.prefix, now: r.clock()}
	user, err := shipyard.loadShipyardMutationUser(ctx, usersTable, playerID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	config, err := shipyard.loadShipyardMutationConfig(ctx)
	if err != nil {
		return shipyardMutationState{}, err
	}
	planet, err := r.loadBotActivePlanet(ctx, playerID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	levels, err := (BuildingsRepository{queryer: r.queryer, prefix: r.prefix}).loadBuildingLevels(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	busy, err := shipyard.loadShipyardBusy(ctx, buildQueueTable, planetID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	defense, err := DefenseRepository{queryer: r.queryer, prefix: r.prefix}.loadDefenseCounts(ctx, planetsTable, playerID, planetID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	queueRows, err := shipyard.loadShipyardQueueTasks(ctx, queueTable, planetID)
	if err != nil {
		return shipyardMutationState{}, err
	}
	overview := domaingame.Overview{CurrentPlanet: domaingame.PlanetOverview{ID: planet.ID, Type: planet.Type, Resources: planet.Resources}}
	items := []domaingame.ShipyardItem{}
	if isDefenseID(unitID) {
		items = domaingame.BuildDefense(overview, levels, user.Research, defense, config.Speed, busy, config.OrderCap).Items
	} else {
		fleet, err := shipyard.loadFleetCounts(ctx, planetsTable, playerID, planetID)
		if err != nil {
			return shipyardMutationState{}, err
		}
		items = domaingame.BuildShipyard(overview, levels, user.Research, fleet, config.Speed, busy, config.OrderCap).Items
	}
	return shipyardMutationState{user: user, playerID: playerID, planetID: planetID, levels: levels, defense: defense, items: items, config: config, queueRows: queueRows}, nil
}

func (r BotRuntimeRepository) loadBotActivePlanetID(ctx context.Context, playerID int) (int, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return 0, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(aktplanet, 0), COALESCE(hplanetid, 0) FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
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
	var active int
	var home int
	if err := rows.Scan(&active, &home); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if active != 0 {
		return active, nil
	}
	return home, nil
}

func (r BotRuntimeRepository) loadBotActivePlanet(ctx context.Context, playerID int) (botActivePlanet, error) {
	planetID, err := r.loadBotActivePlanetID(ctx, playerID)
	if err != nil || planetID == 0 {
		return botActivePlanet{}, err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return botActivePlanet{}, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT planet_id, type, temp, `%d`, `%d`, `%d` FROM %s WHERE planet_id = ? AND owner_id = ? AND type < ? LIMIT 1", resourceMetal, resourceCrystal, resourceDeuterium, planetsTable),
		planetID,
		playerID,
		planetTypeDebris,
	)
	if err != nil {
		return botActivePlanet{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return botActivePlanet{}, err
		}
		return botActivePlanet{}, nil
	}
	var planet botActivePlanet
	if err := rows.Scan(&planet.ID, &planet.Type, &planet.Temperature, &planet.Resources.Metal, &planet.Resources.Crystal, &planet.Resources.Deuterium); err != nil {
		return botActivePlanet{}, err
	}
	return planet, rows.Err()
}

func (r BotRuntimeRepository) buildingTables() (string, string, string, string, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return "", "", "", "", err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return "", "", "", "", err
	}
	buildQueueTable, err := tableName(r.prefix, "buildqueue")
	if err != nil {
		return "", "", "", "", err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return "", "", "", "", err
	}
	return usersTable, planetsTable, buildQueueTable, queueTable, nil
}

func (r BotRuntimeRepository) researchTables() (string, string, string, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return "", "", "", err
	}
	planetsTable, err := tableName(r.prefix, "planets")
	if err != nil {
		return "", "", "", err
	}
	queueTable, err := tableName(r.prefix, "queue")
	if err != nil {
		return "", "", "", err
	}
	return usersTable, planetsTable, queueTable, nil
}

func (r BotRuntimeRepository) clock() func() time.Time {
	if r.now != nil {
		return r.now
	}
	return time.Now
}

func isDefenseID(id int) bool {
	for _, defenseID := range domaingame.DefenseIDs() {
		if id == defenseID {
			return true
		}
	}
	return false
}

func splitBotStatements(source string, separator string) []string {
	if separator == "" {
		return []string{source}
	}
	result := []string{}
	start := 0
	depth := 0
	quote := rune(0)
	escaped := false
	for index, ch := range source {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(source[index:], separator) {
				result = append(result, source[start:index])
				start = index + len(separator)
			}
		}
	}
	result = append(result, source[start:])
	return result
}

func splitTopLevelBotExpression(expression string, separator string) []string {
	parts := splitBotStatements(expression, separator)
	if len(parts) <= 1 {
		return parts
	}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		filtered = append(filtered, strings.TrimSpace(part))
	}
	return filtered
}

func splitTopLevelComparison(expression string, operator string) (string, string, bool) {
	depth := 0
	quote := rune(0)
	escaped := false
	for index, ch := range expression {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(expression[index:], operator) {
				return strings.TrimSpace(expression[:index]), strings.TrimSpace(expression[index+len(operator):]), true
			}
		}
	}
	return "", "", false
}

func parseBotArguments(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return splitTopLevelBotExpression(raw, ",")
}

func botArgString(args []string, index int, def string) string {
	if index >= len(args) {
		return def
	}
	raw := trimBotExpression(args[index])
	if unquoted, ok := unquoteBotString(raw); ok {
		return unquoted
	}
	return raw
}

func botArgNullableString(args []string, index int) *string {
	if index >= len(args) {
		return nil
	}
	raw := trimBotExpression(args[index])
	if strings.EqualFold(raw, "null") {
		return nil
	}
	value := botArgString(args, index, "")
	return &value
}

func botArgInt(args []string, index int, def int) int {
	if index >= len(args) {
		return def
	}
	raw := botArgString(args, index, "")
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return value
}

func trimBotExpression(expression string) string {
	expression = strings.TrimSpace(expression)
	expression = strings.TrimSuffix(expression, ";")
	expression = strings.TrimSpace(expression)
	if strings.HasPrefix(expression, "return ") {
		expression = strings.TrimSpace(strings.TrimPrefix(expression, "return "))
	}
	for strings.HasPrefix(expression, "(") && strings.HasSuffix(expression, ")") && botOuterParensBalanced(expression) {
		expression = strings.TrimSpace(expression[1 : len(expression)-1])
	}
	return expression
}

func botOuterParensBalanced(expression string) bool {
	depth := 0
	quote := rune(0)
	escaped := false
	for index, ch := range expression {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && index != len(expression)-1 {
				return false
			}
		}
	}
	return depth == 0
}

func unquoteBotString(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return "", false
	}
	quote := raw[0]
	if (quote != '\'' && quote != '"') || raw[len(raw)-1] != quote {
		return "", false
	}
	unquoted, err := strconv.Unquote(`"` + strings.ReplaceAll(raw[1:len(raw)-1], `"`, `\"`) + `"`)
	if err != nil {
		return raw[1 : len(raw)-1], true
	}
	return unquoted, true
}

func compareBotValues(left botValue, right botValue, operator string) bool {
	leftNum, leftNumeric := left.numeric()
	rightNum, rightNumeric := right.numeric()
	switch operator {
	case "==", "===":
		if leftNumeric && rightNumeric {
			return leftNum == rightNum
		}
		return left.asString() == right.asString()
	case "!=", "!==":
		if leftNumeric && rightNumeric {
			return leftNum != rightNum
		}
		return left.asString() != right.asString()
	case ">":
		return leftNumeric && rightNumeric && leftNum > rightNum
	case "<":
		return leftNumeric && rightNumeric && leftNum < rightNum
	case ">=":
		return leftNumeric && rightNumeric && leftNum >= rightNum
	case "<=":
		return leftNumeric && rightNumeric && leftNum <= rightNum
	default:
		return false
	}
}

func boolBotValue(value bool) botValue {
	if value {
		return botValue{text: "1", num: 1, bool: true, kind: "bool"}
	}
	return botValue{text: "", num: 0, bool: false, kind: "bool"}
}

func numberBotValue(value int) botValue {
	return botValue{text: strconv.Itoa(value), num: float64(value), bool: value != 0, kind: "number"}
}

func stringBotValue(value string) botValue {
	return botValue{text: value, kind: "string"}
}

func (v botValue) truthy() bool {
	switch v.kind {
	case "bool":
		return v.bool
	case "number":
		return v.num != 0
	case "string":
		return v.text != "" && v.text != "0"
	default:
		return false
	}
}

func (v botValue) asInt() int {
	if number, ok := v.numeric(); ok {
		return int(number)
	}
	return 0
}

func (v botValue) asString() string {
	if v.kind == "number" {
		return trimFloatText(v.num)
	}
	if v.kind == "bool" && v.bool {
		return "1"
	}
	return v.text
}

func (v botValue) numeric() (float64, bool) {
	if v.kind == "number" || v.kind == "bool" {
		return v.num, true
	}
	number, err := strconv.ParseFloat(v.text, 64)
	return number, err == nil
}

func trimFloatText(value float64) string {
	if value == math.Trunc(value) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
