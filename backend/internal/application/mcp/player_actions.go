package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type BuildingMutationRepository interface {
	MutateBuildings(context.Context, appgame.BuildingsMutationQuery) (appgame.BuildingsMutationOutcome, error)
}

type ResearchMutationRepository interface {
	MutateResearch(context.Context, appgame.ResearchMutationQuery) (appgame.ResearchMutationOutcome, error)
}

type OverviewMutationRepository interface {
	RenamePlanet(context.Context, appgame.OverviewRenameQuery) (domaingame.Overview, error)
	DeletePlanet(context.Context, appgame.OverviewDeleteQuery) (domaingame.Overview, *domaingame.OverviewActionIssue, error)
}

type FleetTemplateMutationRepository interface {
	GetFleet(context.Context, appgame.FleetQuery) (domaingame.Fleet, error)
	MutateFleetTemplate(context.Context, appgame.FleetTemplateMutationQuery) error
}

type AllianceMutationRepository interface {
	MutateAlliance(context.Context, appgame.AllianceMutationQuery) (domaingame.Alliance, *domaingame.AllianceActionIssue, error)
}

type OptionsMutationRepository interface {
	GetOptions(context.Context, appgame.OptionsQuery) (domaingame.Options, error)
	UpdateOptions(context.Context, appgame.OptionsUpdateQuery) (domaingame.Options, *domaingame.OptionsActionIssue, error)
}

type EmpireMutationRepository interface {
	GetEmpire(context.Context, appgame.EmpireQuery) (domaingame.Empire, *domaingame.EmpireActionIssue, error)
	MutateEmpire(context.Context, appgame.EmpireMutationQuery) (appgame.EmpireMutationOutcome, error)
}

type GalaxyMutationRepository interface {
	LaunchMissiles(context.Context, appgame.GalaxyMissileLaunchQuery) (*domaingame.GalaxyActionIssue, error)
	DispatchInstantFleet(context.Context, appgame.GalaxyInstantDispatchQuery) (*domaingame.GalaxyActionIssue, error)
}

type PaymentMutationRepository interface {
	CheckCoupon(context.Context, appgame.PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error)
	ActivateCoupon(context.Context, appgame.PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error)
}

// PlayerActions reuses the authenticated game repositories so MCP mutations
// remain governed by the same rules and transactions as browser actions.
type PlayerActions struct {
	Buildings      BuildingMutationRepository
	Research       ResearchMutationRepository
	Overview       OverviewMutationRepository
	FleetTemplates FleetTemplateMutationRepository
	Alliance       AllianceMutationRepository
	Options        OptionsMutationRepository
	OptionsMailer  appgame.OptionsMailer
	Empire         EmpireMutationRepository
	Galaxy         GalaxyMutationRepository
	Payment        PaymentMutationRepository
}

type playerActionResult struct {
	PlayerID             int                    `json:"playerId"`
	Tool                 string                 `json:"tool"`
	Action               string                 `json:"action"`
	PlanetID             int                    `json:"planetId,omitempty"`
	TargetID             int                    `json:"targetId,omitempty"`
	DryRun               bool                   `json:"dryRun"`
	RequiresConfirmation bool                   `json:"requiresConfirmation"`
	Confirmation         string                 `json:"confirmation,omitempty"`
	Executed             bool                   `json:"executed"`
	Issue                *domainmcp.ActionIssue `json:"issue,omitempty"`
	Details              map[string]any         `json:"details,omitempty"`
	Timing               *playerActionTiming    `json:"timing,omitempty"`
}

type playerActionTiming struct {
	DurationSeconds  int    `json:"durationSeconds"`
	StartsAt         int64  `json:"startsAt"`
	FinishesAt       int64  `json:"finishesAt"`
	RemainingSeconds int    `json:"remainingSeconds"`
	Status           string `json:"status"`
}

func playerActionIssue(code string, message string) *domainmcp.ActionIssue {
	if strings.TrimSpace(code) == "" {
		return nil
	}
	return &domainmcp.ActionIssue{Code: code, Message: message}
}

func playerActionConfirmation(tool string, playerID int, payload any) string {
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(append([]byte(fmt.Sprintf("%s:%d:", tool, playerID)), encoded...))
	return fmt.Sprintf("%s:%d:%s", tool, playerID, hex.EncodeToString(sum[:])[:12])
}

func playerActionDryRun(arguments map[string]any) (bool, string, error) {
	dryRun := true
	var err error
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return false, "", err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	return dryRun, strings.TrimSpace(confirm), err
}

func playerActionPayload(arguments map[string]any) map[string]any {
	payload := make(map[string]any, len(arguments))
	for key, value := range arguments {
		if key != "dryRun" && key != "confirm" {
			payload[key] = value
		}
	}
	return payload
}

func playerActionToolResult(key string, result playerActionResult) domainmcp.ToolCallResult {
	structured := map[string]any{key: result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content:           []domainmcp.Content{{Type: "text", Text: string(text)}},
		StructuredContent: structured,
	}
}

func preparePlayerAction(tool string, access domainmcp.Access, action string, planetID int, targetID int, arguments map[string]any) (playerActionResult, bool, error) {
	dryRun, confirm, err := playerActionDryRun(arguments)
	if err != nil {
		return playerActionResult{}, false, err
	}
	confirmation := playerActionConfirmation(tool, access.PlayerID, playerActionPayload(arguments))
	result := playerActionResult{
		PlayerID: access.PlayerID, Tool: tool, Action: action, PlanetID: planetID, TargetID: targetID,
		DryRun: dryRun, RequiresConfirmation: dryRun, Confirmation: confirmation,
	}
	if dryRun {
		return result, false, nil
	}
	if confirm != confirmation {
		return playerActionResult{}, false, domainmcp.ErrInvalidParams
	}
	result.RequiresConfirmation = false
	result.Executed = true
	return result, true, nil
}

func (s Service) callMutateBuilding(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	action, _ := optionalStringArgument(arguments, "action")
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "add" && action != "destroy" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	techID, err := optionalNonNegativeIntArgument(arguments, "techId")
	if err != nil || techID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	result, execute, err := preparePlayerAction("mutate_building", access, action, planetID, techID, arguments)
	if err != nil || !execute {
		if err == nil {
			result.Timing = s.buildingMutationTiming(ctx, access.PlayerID, planetID, techID, action, false)
		}
		return playerActionToolResult("buildingMutation", result), err
	}
	outcome, err := s.playerActions.Buildings.MutateBuildings(ctx, appgame.BuildingsMutationQuery{PlayerID: access.PlayerID, PlanetID: planetID, Action: action, TechID: techID})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if outcome.ActionIssue != nil {
		result.Issue = playerActionIssue(outcome.ActionIssue.Code, outcome.ActionIssue.Message)
	} else {
		result.Timing = s.buildingMutationTiming(ctx, access.PlayerID, planetID, techID, action, true)
	}
	result.Details = map[string]any{"techId": techID}
	return playerActionToolResult("buildingMutation", result), nil
}

func (s Service) callStartResearch(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	techID, err := optionalNonNegativeIntArgument(arguments, "techId")
	if err != nil || techID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	result, execute, err := preparePlayerAction("start_research", access, "start", planetID, techID, arguments)
	if err != nil || !execute {
		if err == nil {
			result.Timing = s.researchMutationTiming(ctx, access.PlayerID, planetID, techID, false)
		}
		return playerActionToolResult("researchMutation", result), err
	}
	outcome, err := s.playerActions.Research.MutateResearch(ctx, appgame.ResearchMutationQuery{PlayerID: access.PlayerID, PlanetID: planetID, Action: "start", TechID: techID})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if outcome.ActionIssue != nil {
		result.Issue = playerActionIssue(outcome.ActionIssue.Code, outcome.ActionIssue.Message)
	} else {
		result.Timing = s.researchMutationTiming(ctx, access.PlayerID, planetID, techID, true)
	}
	result.Details = map[string]any{"techId": techID}
	return playerActionToolResult("researchMutation", result), nil
}

func (s Service) buildingMutationTiming(ctx context.Context, playerID int, planetID int, techID int, action string, executed bool) *playerActionTiming {
	if s.buildingRead == nil {
		return nil
	}
	options, err := s.buildingRead.GetMCPBuildingOptions(ctx, playerID, planetID)
	if err != nil {
		return nil
	}
	now := s.currentTime().Unix()
	if executed {
		if timing := buildingQueueTiming(options.Queue, techID, action == "destroy", now); timing != nil {
			return timing
		}
	}
	duration := 0
	for _, item := range options.Items {
		if item.ID == techID {
			duration = item.DurationSeconds
			break
		}
	}
	if action == "destroy" && s.technologyRead != nil {
		technology, technologyErr := s.technologyRead.GetMCPTechnology(ctx, playerID, domainmcp.TechnologyCommand{PlanetID: options.Planet.ID, DetailsID: techID})
		if technologyErr == nil && technology.Details != nil && technology.Details.Demolish != nil {
			duration = technology.Details.Demolish.DurationSeconds
		}
	}
	if duration < 1 {
		return nil
	}
	startsAt := projectedBuildingQueueEnd(options.Queue, now)
	status := "preview"
	if executed {
		status = queueTimingStatus(startsAt, now)
	}
	return newPlayerActionTiming(duration, startsAt, startsAt+int64(duration), now, status)
}

func (s Service) researchMutationTiming(ctx context.Context, playerID int, planetID int, techID int, executed bool) *playerActionTiming {
	if s.researchRead == nil {
		return nil
	}
	options, err := s.researchRead.GetMCPResearchOptions(ctx, playerID, planetID)
	if err != nil {
		return nil
	}
	now := s.currentTime().Unix()
	if options.Active != nil {
		if executed && options.Active.TechID == techID {
			status := mcpActionQueueStatus(options.Active.RemainingSeconds, false)
			return newPlayerActionTiming(options.Active.End-options.Active.Start, int64(options.Active.Start), int64(options.Active.End), now, status)
		}
		return nil
	}
	for _, item := range options.Items {
		if item.ID == techID && item.DurationSeconds > 0 {
			status := "preview"
			if executed {
				status = "running"
			}
			return newPlayerActionTiming(item.DurationSeconds, now, now+int64(item.DurationSeconds), now, status)
		}
	}
	return nil
}

func buildingQueueTiming(queue []domainmcp.BuildingQueueEntry, techID int, destroy bool, now int64) *playerActionTiming {
	cursor := now
	var found *playerActionTiming
	for index, entry := range queue {
		duration := entry.End - entry.Start
		if duration < 1 {
			duration = 1
		}
		startsAt := cursor
		finishesAt := cursor + int64(duration)
		if index == 0 {
			startsAt = int64(entry.Start)
			finishesAt = int64(entry.End)
			cursor = finishesAt
			if cursor < now {
				cursor = now
			}
		} else {
			cursor = finishesAt
		}
		if entry.TechID == techID && entry.Destroy == destroy {
			status := entry.Status
			if status == "" {
				status = queueTimingStatus(startsAt, now)
			}
			found = newPlayerActionTiming(duration, startsAt, finishesAt, now, status)
		}
	}
	return found
}

func projectedBuildingQueueEnd(queue []domainmcp.BuildingQueueEntry, now int64) int64 {
	cursor := now
	for index, entry := range queue {
		duration := entry.End - entry.Start
		if duration < 1 {
			duration = 1
		}
		if index == 0 && int64(entry.End) > cursor {
			cursor = int64(entry.End)
		} else {
			cursor += int64(duration)
		}
	}
	return cursor
}

func newPlayerActionTiming(duration int, startsAt int64, finishesAt int64, now int64, status string) *playerActionTiming {
	if duration < 1 || finishesAt < startsAt {
		return nil
	}
	remaining := finishesAt - now
	if remaining < 0 {
		remaining = 0
	}
	return &playerActionTiming{DurationSeconds: duration, StartsAt: startsAt, FinishesAt: finishesAt, RemainingSeconds: int(remaining), Status: status}
}

func (s Service) currentTime() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

func queueTimingStatus(startsAt int64, now int64) string {
	if startsAt > now {
		return "queued"
	}
	return "running"
}

func mcpActionQueueStatus(remaining int, queued bool) string {
	if remaining <= 0 {
		return "due"
	}
	if queued {
		return "queued"
	}
	return "running"
}

func (s Service) callMutatePlanet(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if _, supplied := arguments["password"]; supplied {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	action, _ := optionalStringArgument(arguments, "action")
	action = strings.ToLower(strings.TrimSpace(action))
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil || planetID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	targetID := planetID
	if action == "delete" {
		if value, valueErr := optionalNonNegativeIntArgument(arguments, "targetPlanetId"); valueErr != nil {
			return domainmcp.ToolCallResult{}, valueErr
		} else if value > 0 {
			targetID = value
		}
	}
	if action != "rename" && action != "delete" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	name, _ := optionalStringArgument(arguments, "name")
	if action == "rename" && strings.TrimSpace(name) == "" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	result, execute, err := preparePlayerAction("mutate_planet", access, action, planetID, targetID, arguments)
	if err != nil || !execute {
		return playerActionToolResult("planetMutation", result), err
	}
	if action == "rename" {
		overview, err := s.playerActions.Overview.RenamePlanet(ctx, appgame.OverviewRenameQuery{PlayerID: access.PlayerID, PlanetID: planetID, Name: name})
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.Details = map[string]any{"planetId": overview.CurrentPlanet.ID, "name": overview.CurrentPlanet.Name}
	} else {
		overview, issue, err := s.playerActions.Overview.DeletePlanet(ctx, appgame.OverviewDeleteQuery{
			PlayerID: access.PlayerID, PlanetID: planetID, DeleteID: targetID, SkipPasswordVerification: true,
		})
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		if issue != nil {
			result.Issue = playerActionIssue(issue.Code, issue.Message)
		}
		result.Details = map[string]any{"currentPlanetId": overview.CurrentPlanet.ID}
	}
	return playerActionToolResult("planetMutation", result), nil
}

func (s Service) callMutateFleetTemplate(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	action, _ := optionalStringArgument(arguments, "action")
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "save" && action != "delete" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	templateID, err := optionalNonNegativeIntArgument(arguments, "templateId")
	if err != nil || action == "delete" && templateID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	name, _ := optionalStringArgument(arguments, "name")
	ships, err := intObjectArgument(arguments, "ships")
	if err != nil || action == "save" && (strings.TrimSpace(name) == "" || len(ships) == 0) {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	result, execute, err := preparePlayerAction("mutate_fleet_template", access, action, planetID, templateID, arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	fleet, err := s.playerActions.FleetTemplates.GetFleet(ctx, appgame.FleetQuery{PlayerID: access.PlayerID, PlanetID: planetID})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if !fleet.CommanderActive {
		result.RequiresConfirmation = false
		result.Confirmation = ""
		result.Issue = playerActionIssue(domaingame.EmpireIssueCommanderRequired, "Commander is required to manage fleet templates.")
		return playerActionToolResult("fleetTemplateMutation", result), nil
	}
	if !execute {
		result.Details = map[string]any{"templateId": templateID, "templateLimit": fleet.TemplateLimit}
		return playerActionToolResult("fleetTemplateMutation", result), nil
	}
	if err := s.playerActions.FleetTemplates.MutateFleetTemplate(ctx, appgame.FleetTemplateMutationQuery{PlayerID: access.PlayerID, TemplateID: templateID, Action: action, Name: name, Ships: ships}); err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	updated, err := s.playerActions.FleetTemplates.GetFleet(ctx, appgame.FleetQuery{PlayerID: access.PlayerID, PlanetID: planetID})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result.Details = map[string]any{"templateId": templateID, "templateCount": len(updated.Templates), "templateLimit": updated.TemplateLimit}
	return playerActionToolResult("fleetTemplateMutation", result), nil
}

func (s Service) callMutateCommanderQueue(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	action, _ := optionalStringArgument(arguments, "action")
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "add" && action != "destroy" && action != "remove" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	targetID, err := optionalNonNegativeIntArgument(arguments, "targetPlanetId")
	if err != nil || targetID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	techID, err := optionalNonNegativeIntArgument(arguments, "techId")
	if err != nil || action != "remove" && techID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	listID, err := optionalNonNegativeIntArgument(arguments, "listId")
	if err != nil || action == "remove" && listID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	result, execute, err := preparePlayerAction("mutate_commander_queue", access, action, planetID, targetID, arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	empire, issue, err := s.playerActions.Empire.GetEmpire(ctx, appgame.EmpireQuery{PlayerID: access.PlayerID, PlanetID: planetID, PlanetType: domaingame.EmpirePlanetTypePlanets})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if !empire.CommanderActive {
		result.RequiresConfirmation = false
		result.Confirmation = ""
		if issue != nil {
			result.Issue = playerActionIssue(issue.Code, issue.Message)
		} else {
			result.Issue = playerActionIssue(domaingame.EmpireIssueCommanderRequired, "Commander is required to manage queues across planets.")
		}
		return playerActionToolResult("commanderQueueMutation", result), nil
	}
	if !execute {
		result.Details = map[string]any{"techId": techID, "listId": listID}
		return playerActionToolResult("commanderQueueMutation", result), nil
	}
	outcome, err := s.playerActions.Empire.MutateEmpire(ctx, appgame.EmpireMutationQuery{PlayerID: access.PlayerID, PlanetID: targetID, Action: action, TechID: techID, ListID: listID})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if outcome.ActionIssue != nil {
		result.Issue = playerActionIssue(outcome.ActionIssue.Code, outcome.ActionIssue.Message)
	}
	result.Details = map[string]any{"techId": techID, "listId": listID}
	return playerActionToolResult("commanderQueueMutation", result), nil
}

func (s Service) callLaunchInterplanetaryMissiles(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	targetID, err := optionalNonNegativeIntArgument(arguments, "targetPlanetId")
	if err != nil || targetID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	amount, err := optionalNonNegativeIntArgument(arguments, "amount")
	if err != nil || amount <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	defenseID, err := optionalNonNegativeIntArgument(arguments, "targetDefenseId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, execute, err := preparePlayerAction("launch_interplanetary_missiles", access, "launch", planetID, targetID, arguments)
	if err != nil || !execute {
		result.Details = map[string]any{"amount": amount, "targetDefenseId": defenseID}
		return playerActionToolResult("missileLaunch", result), err
	}
	issue, err := s.playerActions.Galaxy.LaunchMissiles(ctx, appgame.GalaxyMissileLaunchQuery{PlayerID: access.PlayerID, PlanetID: planetID, TargetPlanetID: targetID, Amount: amount, TargetDefenseID: defenseID})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if issue != nil {
		result.Issue = playerActionIssue(issue.Code, issue.Message)
	}
	result.Details = map[string]any{"amount": amount, "targetDefenseId": defenseID}
	return playerActionToolResult("missileLaunch", result), nil
}

func (s Service) callDispatchGalaxyAction(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	action, _ := optionalStringArgument(arguments, "action")
	action = strings.ToLower(strings.TrimSpace(action))
	mission := domaingame.FleetMissionSpy
	targetType := domaingame.GamePlanetTypePlanet
	if action == "recycle" {
		mission = domaingame.FleetMissionRecycle
		targetType = domaingame.GamePlanetTypeDebris
	} else if action != "spy" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	galaxy, err := optionalNonNegativeIntArgument(arguments, "targetGalaxy")
	if err != nil || galaxy <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	system, err := optionalNonNegativeIntArgument(arguments, "targetSystem")
	if err != nil || system <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	position, err := optionalNonNegativeIntArgument(arguments, "targetPosition")
	if err != nil || position <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	amount, err := optionalNonNegativeIntArgument(arguments, "amount")
	if err != nil || amount <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	if value, valueErr := optionalNonNegativeIntArgument(arguments, "targetType"); valueErr != nil {
		return domainmcp.ToolCallResult{}, valueErr
	} else if value > 0 {
		targetType = value
	}
	result, execute, err := preparePlayerAction("dispatch_galaxy_action", access, action, planetID, 0, arguments)
	result.Details = map[string]any{"target": map[string]int{"galaxy": galaxy, "system": system, "position": position}, "targetType": targetType, "amount": amount}
	if err != nil || !execute {
		return playerActionToolResult("galaxyDispatch", result), err
	}
	issue, err := s.playerActions.Galaxy.DispatchInstantFleet(ctx, appgame.GalaxyInstantDispatchQuery{
		PlayerID: access.PlayerID, PlanetID: planetID,
		Target:     domaingame.Coordinates{Galaxy: galaxy, System: system, Position: position},
		TargetType: targetType, Mission: mission, Amount: amount,
	})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if issue != nil {
		result.Issue = playerActionIssue(issue.Code, issue.Message)
	}
	return playerActionToolResult("galaxyDispatch", result), nil
}

func allianceRankRightsArgument(arguments map[string]any) ([]domaingame.AllianceRank, error) {
	if arguments == nil || arguments["rankRights"] == nil {
		return nil, nil
	}
	rows, ok := arguments["rankRights"].([]any)
	if !ok {
		return nil, domainmcp.ErrInvalidParams
	}
	ranks := make([]domaingame.AllianceRank, 0, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, domainmcp.ErrInvalidParams
		}
		id, err := optionalNonNegativeIntArgument(row, "id")
		if err != nil || id <= 0 {
			return nil, domainmcp.ErrInvalidParams
		}
		rights, err := optionalNonNegativeIntArgument(row, "rights")
		if err != nil {
			return nil, err
		}
		ranks = append(ranks, domaingame.AllianceRank{ID: id, Rights: rights})
	}
	return ranks, nil
}

func optionalStringArguments(arguments map[string]any, names ...string) (map[string]string, error) {
	values := make(map[string]string, len(names))
	for _, name := range names {
		value, err := optionalStringArgument(arguments, name)
		if err != nil {
			return nil, err
		}
		values[name] = value
	}
	return values, nil
}

func optionalIntegerArguments(arguments map[string]any, names ...string) (map[string]int, error) {
	values := make(map[string]int, len(names))
	for _, name := range names {
		value, err := optionalNonNegativeIntArgument(arguments, name)
		if err != nil {
			return nil, err
		}
		values[name] = value
	}
	return values, nil
}

func allianceMutationArgument(arguments map[string]any) (domaingame.AllianceMutation, int, error) {
	action, err := optionalStringArgument(arguments, "action")
	if err != nil {
		return domaingame.AllianceMutation{}, 0, err
	}
	action = strings.ToLower(strings.TrimSpace(action))
	allowed := map[string]bool{"create": true, "apply": true, "withdraw": true, "accept": true, "reject": true, "leave": true, "change_tag": true, "change_name": true, "dismiss": true, "transfer_founder": true, "save_text": true, "save_settings": true, "add_rank": true, "save_ranks": true, "delete_rank": true, "assign_rank": true, "kick_member": true, "send_circular": true}
	if !allowed[action] {
		return domaingame.AllianceMutation{}, 0, domainmcp.ErrInvalidParams
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domaingame.AllianceMutation{}, 0, err
	}
	stringsByName, err := optionalStringArguments(arguments, "text", "tag", "name", "homepage", "imageLogo", "founderRankName", "rankName")
	if err != nil {
		return domaingame.AllianceMutation{}, 0, err
	}
	integersByName, err := optionalIntegerArguments(arguments, "textKind", "allianceId", "applicationId", "rankId", "targetPlayerId", "targetRankId", "circularRankId")
	if err != nil {
		return domaingame.AllianceMutation{}, 0, err
	}
	open, err := optionalBoolArgument(arguments, "open")
	if err != nil {
		return domaingame.AllianceMutation{}, 0, err
	}
	insertApp, err := optionalBoolArgument(arguments, "insertApp")
	if err != nil {
		return domaingame.AllianceMutation{}, 0, err
	}
	ranks, err := allianceRankRightsArgument(arguments)
	if err != nil {
		return domaingame.AllianceMutation{}, 0, err
	}
	return domaingame.AllianceMutation{
		Action: action, Tag: stringsByName["tag"], Name: stringsByName["name"], Text: stringsByName["text"], TextKind: domaingame.NormalizeAllianceTextKind(integersByName["textKind"]), Homepage: stringsByName["homepage"], ImageLogo: stringsByName["imageLogo"],
		Open: open, InsertApp: insertApp, FounderRankName: stringsByName["founderRankName"], AllianceID: integersByName["allianceId"], ApplicationID: integersByName["applicationId"], RankID: integersByName["rankId"],
		RankName: stringsByName["rankName"], RankRights: ranks, TargetPlayerID: integersByName["targetPlayerId"], TargetRankID: integersByName["targetRankId"], CircularRankID: integersByName["circularRankId"],
	}, planetID, nil
}

func (s Service) callMutateAlliance(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	mutation, planetID, err := allianceMutationArgument(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, execute, err := preparePlayerAction("mutate_alliance", access, mutation.Action, planetID, mutation.AllianceID, arguments)
	if err != nil || !execute {
		return playerActionToolResult("allianceMutation", result), err
	}
	query := appgame.AllianceQuery{PlayerID: access.PlayerID, PlanetID: planetID, View: domaingame.AllianceViewHome, AllianceID: mutation.AllianceID, ApplicationID: mutation.ApplicationID, TextKind: mutation.TextKind}
	alliance, issue, err := s.playerActions.Alliance.MutateAlliance(ctx, appgame.AllianceMutationQuery{PlayerID: access.PlayerID, PlanetID: planetID, Query: query, Mutation: mutation})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if issue != nil {
		result.Issue = playerActionIssue(issue.Code, issue.Message)
	}
	result.Details = map[string]any{"view": alliance.View, "allianceId": mutation.AllianceID, "applicationId": mutation.ApplicationID}
	return playerActionToolResult("allianceMutation", result), nil
}

func baseOptionsMutation(current domaingame.Options) domaingame.OptionsMutation {
	feedType := "rss"
	if current.Flags.FeedAtom {
		feedType = "atom"
	}
	return domaingame.OptionsMutation{
		Name: current.User.Name, Language: current.Settings.Language, SkinPath: current.Settings.SkinPath,
		UseSkin: current.Settings.UseSkin, DeactivateIP: current.Settings.DeactivateIP, SortBy: current.Settings.SortBy,
		SortOrder: current.Settings.SortOrder, MaxSpy: current.Settings.MaxSpy, MaxFleetMessages: current.Settings.MaxFleetMessages,
		Email: current.User.PlainEmail, DeleteAccount: current.Account.DeletionQueued,
		ShowEspionageButton: current.Flags.ShowEspionageButton, ShowWriteMessage: current.Flags.ShowWriteMessage,
		ShowBuddy: current.Flags.ShowBuddy, ShowRocketAttack: current.Flags.ShowRocketAttack, ShowViewReport: current.Flags.ShowViewReport,
		DoNotUseFolders: current.Flags.DoNotUseFolders, FeedEnabled: current.Flags.FeedEnabled, FeedType: feedType, HideGOEmail: current.Flags.HideGOEmail,
	}
}

func argumentPresent(arguments map[string]any, key string) bool {
	return arguments != nil && arguments[key] != nil
}

func accountOptionsMutation(arguments map[string]any, current domaingame.Options) (string, domaingame.OptionsMutation, error) {
	if _, supplied := arguments["oldPassword"]; supplied {
		return "", domaingame.OptionsMutation{}, domainmcp.ErrInvalidParams
	}
	action, err := optionalStringArgument(arguments, "action")
	if err != nil {
		return "", domaingame.OptionsMutation{}, err
	}
	action = strings.ToLower(strings.TrimSpace(action))
	mutation := baseOptionsMutation(current)
	switch action {
	case "settings":
		if argumentPresent(arguments, "language") {
			mutation.Language, err = optionalStringArgument(arguments, "language")
		}
		if err == nil && argumentPresent(arguments, "skinPath") {
			mutation.SkinPath, err = optionalStringArgument(arguments, "skinPath")
		}
		if err == nil && argumentPresent(arguments, "useSkin") {
			mutation.UseSkin, err = optionalBoolArgument(arguments, "useSkin")
		}
		if err == nil && argumentPresent(arguments, "deactivateIp") {
			mutation.DeactivateIP, err = optionalBoolArgument(arguments, "deactivateIp")
		}
		if err == nil && argumentPresent(arguments, "sortBy") {
			mutation.SortBy, err = optionalNonNegativeIntArgument(arguments, "sortBy")
		}
		if err == nil && argumentPresent(arguments, "sortOrder") {
			mutation.SortOrder, err = optionalNonNegativeIntArgument(arguments, "sortOrder")
		}
		if err == nil && argumentPresent(arguments, "maxSpy") {
			mutation.MaxSpy, err = optionalNonNegativeIntArgument(arguments, "maxSpy")
		}
		if err == nil && argumentPresent(arguments, "maxFleetMessages") {
			mutation.MaxFleetMessages, err = optionalNonNegativeIntArgument(arguments, "maxFleetMessages")
		}
		for key, target := range map[string]*bool{"showEspionageButton": &mutation.ShowEspionageButton, "showWriteMessage": &mutation.ShowWriteMessage, "showBuddy": &mutation.ShowBuddy, "showRocketAttack": &mutation.ShowRocketAttack, "showViewReport": &mutation.ShowViewReport, "doNotUseFolders": &mutation.DoNotUseFolders, "feedEnabled": &mutation.FeedEnabled, "hideGoEmail": &mutation.HideGOEmail} {
			if err == nil && argumentPresent(arguments, key) {
				*target, err = optionalBoolArgument(arguments, key)
			}
		}
		if err == nil && argumentPresent(arguments, "feedType") {
			mutation.FeedType, err = optionalStringArgument(arguments, "feedType")
		}
	case "rename":
		mutation.Name, err = optionalStringArgument(arguments, "name")
		if strings.TrimSpace(mutation.Name) == "" {
			err = domainmcp.ErrInvalidParams
		}
	case "password":
		mutation.NewPassword, err = optionalStringArgument(arguments, "newPassword")
		if err == nil {
			mutation.NewPasswordRepeat, err = optionalStringArgument(arguments, "newPasswordRepeat")
		}
		if mutation.NewPassword == "" || mutation.NewPasswordRepeat == "" {
			err = domainmcp.ErrInvalidParams
		}
	case "email":
		mutation.Email, err = optionalStringArgument(arguments, "email")
		if strings.TrimSpace(mutation.Email) == "" {
			err = domainmcp.ErrInvalidParams
		}
	case "vacation":
		if !argumentPresent(arguments, "enabled") {
			return "", domaingame.OptionsMutation{}, domainmcp.ErrInvalidParams
		}
		mutation.VacationMode, err = optionalBoolArgument(arguments, "enabled")
		mutation.VacationModeSet = true
		mutation.DisableVacation = !mutation.VacationMode
	case "deletion":
		if !argumentPresent(arguments, "enabled") {
			return "", domaingame.OptionsMutation{}, domainmcp.ErrInvalidParams
		}
		mutation.DeleteAccount, err = optionalBoolArgument(arguments, "enabled")
	case "resend_activation":
		mutation.ResendActivation = true
	default:
		return "", domaingame.OptionsMutation{}, domainmcp.ErrInvalidParams
	}
	if err != nil {
		return "", domaingame.OptionsMutation{}, err
	}
	return action, mutation, nil
}

func safeOptionsDetails(options domaingame.Options) map[string]any {
	return map[string]any{
		"user":     map[string]any{"name": options.User.Name, "email": options.User.Email, "validated": options.User.Validated},
		"settings": map[string]any{"language": options.Settings.Language, "skinPath": options.Settings.SkinPath, "useSkin": options.Settings.UseSkin, "deactivateIp": options.Settings.DeactivateIP, "sortBy": options.Settings.SortBy, "sortOrder": options.Settings.SortOrder, "maxSpy": options.Settings.MaxSpy, "maxFleetMessages": options.Settings.MaxFleetMessages},
		"account":  map[string]any{"vacation": options.Account.Vacation, "vacationUntil": options.Account.VacationUntil, "deletionQueued": options.Account.DeletionQueued, "deletionAt": options.Account.DeletionAt},
		"flags":    map[string]any{"showEspionageButton": options.Flags.ShowEspionageButton, "showWriteMessage": options.Flags.ShowWriteMessage, "showBuddy": options.Flags.ShowBuddy, "showRocketAttack": options.Flags.ShowRocketAttack, "showViewReport": options.Flags.ShowViewReport, "doNotUseFolders": options.Flags.DoNotUseFolders, "feedEnabled": options.Flags.FeedEnabled, "feedAtom": options.Flags.FeedAtom, "hideGoEmail": options.Flags.HideGOEmail},
	}
}

func (s Service) callUpdateAccountOptions(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	current, err := s.playerActions.Options.GetOptions(ctx, appgame.OptionsQuery{PlayerID: access.PlayerID, PlanetID: planetID})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	action, mutation, err := accountOptionsMutation(arguments, current)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, execute, err := preparePlayerAction("update_account_options", access, action, planetID, 0, arguments)
	if err != nil || !execute {
		result.Details = safeOptionsDetails(current)
		return playerActionToolResult("accountOptionsMutation", result), err
	}
	updated, issue, err := s.playerActions.Options.UpdateOptions(ctx, appgame.OptionsUpdateQuery{
		PlayerID: access.PlayerID, PlanetID: planetID, Mutation: mutation, SkipPasswordVerification: true,
	})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if updated.OutboundMail != nil && s.playerActions.OptionsMailer != nil {
		if err := s.playerActions.OptionsMailer.SendOptionsChange(ctx, *updated.OutboundMail); err != nil {
			return domainmcp.ToolCallResult{}, err
		}
	}
	updated.OutboundMail = nil
	if issue != nil {
		result.Issue = playerActionIssue(issue.Code, issue.Message)
	}
	result.Details = safeOptionsDetails(updated)
	return playerActionToolResult("accountOptionsMutation", result), nil
}

func (s Service) callRedeemCoupon(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	code, err := optionalStringArgument(arguments, "couponCode")
	if err != nil || strings.TrimSpace(code) == "" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	dryRun, confirm, err := playerActionDryRun(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := playerActionConfirmation("redeem_coupon", access.PlayerID, playerActionPayload(arguments))
	result := playerActionResult{PlayerID: access.PlayerID, Tool: "redeem_coupon", Action: "activate", DryRun: dryRun, Confirmation: confirmation}
	query := appgame.PaymentMutationQuery{PlayerID: access.PlayerID, CouponCode: code}
	if dryRun {
		coupon, found, err := s.playerActions.Payment.CheckCoupon(ctx, query)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.RequiresConfirmation = found
		if found {
			result.Details = map[string]any{"amount": coupon.Amount}
			result.Issue = playerActionIssue(domaingame.PaymentIssueCouponValid, domaingame.PaymentIssue(domaingame.PaymentIssueCouponValid).Message)
		} else {
			result.Confirmation = ""
			result.Issue = playerActionIssue(domaingame.PaymentIssueInvalidCoupon, domaingame.PaymentIssue(domaingame.PaymentIssueInvalidCoupon).Message)
		}
		return playerActionToolResult("couponRedemption", result), nil
	}
	if strings.TrimSpace(confirm) != confirmation {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	coupon, activated, err := s.playerActions.Payment.ActivateCoupon(ctx, query)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result.Executed = activated
	if activated {
		result.Details = map[string]any{"amount": coupon.Amount}
		result.Issue = playerActionIssue(domaingame.PaymentIssueCouponActivated, domaingame.PaymentIssue(domaingame.PaymentIssueCouponActivated).Message)
	} else {
		result.Issue = playerActionIssue(domaingame.PaymentIssueInvalidCoupon, domaingame.PaymentIssue(domaingame.PaymentIssueInvalidCoupon).Message)
	}
	return playerActionToolResult("couponRedemption", result), nil
}

func playerMutationOutputSchema(key string) map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{key: map[string]any{"type": "object"}}, "required": []string{key}}
}

func playerMutationTool(name string, title string, description string, properties map[string]any, required []string, outputKey string) domainmcp.Tool {
	properties["dryRun"] = map[string]any{"type": "boolean", "default": true, "description": "Preview by default; set false only with the confirmation returned by preview."}
	properties["confirm"] = map[string]any{"type": "string", "description": "Confirmation returned by dry-run."}
	return domainmcp.Tool{
		Name: name, Title: title, Description: description,
		InputSchema:  map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false},
		OutputSchema: playerMutationOutputSchema(outputKey),
		Annotations:  map[string]any{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": false, "openWorldHint": false},
	}
}

func mutateBuildingTool() domainmcp.Tool {
	return playerMutationTool("mutate_building", "Start building construction", "Queue construction or demolition using the same validation as the game Buildings page and return explicit duration and completion timing when available.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 0}, "action": map[string]any{"type": "string", "enum": []string{"add", "destroy"}}, "techId": map[string]any{"type": "integer", "minimum": 1},
	}, []string{"action", "techId"}, "buildingMutation")
}

func startResearchTool() domainmcp.Tool {
	return playerMutationTool("start_research", "Start research", "Start a research task using the same requirements, resources, and queue rules as the game Research page and return explicit duration and completion timing when available.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 0}, "techId": map[string]any{"type": "integer", "minimum": 1},
	}, []string{"techId"}, "researchMutation")
}

func mutatePlanetTool() domainmcp.Tool {
	return playerMutationTool("mutate_planet", "Rename or abandon planet", "Rename an owned planet or abandon it after fleet validation. The scoped bearer token authorizes the action; no account password is accepted.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 1}, "action": map[string]any{"type": "string", "enum": []string{"rename", "delete"}}, "targetPlanetId": map[string]any{"type": "integer", "minimum": 1}, "name": map[string]any{"type": "string"},
	}, []string{"planetId", "action"}, "planetMutation")
}

func mutateFleetTemplateTool() domainmcp.Tool {
	return playerMutationTool("mutate_fleet_template", "Manage Commander fleet template", "Create, update, or delete a Commander fleet template.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 0}, "action": map[string]any{"type": "string", "enum": []string{"save", "delete"}}, "templateId": map[string]any{"type": "integer", "minimum": 0}, "name": map[string]any{"type": "string"}, "ships": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "minimum": 0}},
	}, []string{"action"}, "fleetTemplateMutation")
}

func mutateCommanderQueueTool() domainmcp.Tool {
	return playerMutationTool("mutate_commander_queue", "Manage Commander planet queue", "Queue or remove a building task on another owned planet through Commander Empire controls.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 0}, "targetPlanetId": map[string]any{"type": "integer", "minimum": 1}, "action": map[string]any{"type": "string", "enum": []string{"add", "destroy", "remove"}}, "techId": map[string]any{"type": "integer", "minimum": 0}, "listId": map[string]any{"type": "integer", "minimum": 0},
	}, []string{"targetPlanetId", "action"}, "commanderQueueMutation")
}

func launchInterplanetaryMissilesTool() domainmcp.Tool {
	return playerMutationTool("launch_interplanetary_missiles", "Launch interplanetary missiles", "Launch interplanetary missiles with the same range, protection, inventory, and defense-target rules as Galaxy.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 0}, "targetPlanetId": map[string]any{"type": "integer", "minimum": 1}, "amount": map[string]any{"type": "integer", "minimum": 1}, "targetDefenseId": map[string]any{"type": "integer", "minimum": 0},
	}, []string{"targetPlanetId", "amount"}, "missileLaunch")
}

func dispatchGalaxyActionTool() domainmcp.Tool {
	return playerMutationTool("dispatch_galaxy_action", "Dispatch Galaxy quick action", "Send espionage probes or recyclers from the Galaxy screen using normal fleet-slot and resource validation.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 0}, "action": map[string]any{"type": "string", "enum": []string{"spy", "recycle"}}, "targetGalaxy": map[string]any{"type": "integer", "minimum": 1}, "targetSystem": map[string]any{"type": "integer", "minimum": 1}, "targetPosition": map[string]any{"type": "integer", "minimum": 1}, "targetType": map[string]any{"type": "integer", "minimum": 1}, "amount": map[string]any{"type": "integer", "minimum": 1},
	}, []string{"action", "targetGalaxy", "targetSystem", "targetPosition", "amount"}, "galaxyDispatch")
}

func mutateAllianceTool() domainmcp.Tool {
	return playerMutationTool("mutate_alliance", "Manage alliance", "Perform player alliance membership and permission-gated management actions.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 0}, "action": map[string]any{"type": "string"}, "tag": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}, "textKind": map[string]any{"type": "integer", "minimum": 0}, "homepage": map[string]any{"type": "string"}, "imageLogo": map[string]any{"type": "string"}, "open": map[string]any{"type": "boolean"}, "insertApp": map[string]any{"type": "boolean"}, "founderRankName": map[string]any{"type": "string"}, "allianceId": map[string]any{"type": "integer", "minimum": 0}, "applicationId": map[string]any{"type": "integer", "minimum": 0}, "rankId": map[string]any{"type": "integer", "minimum": 0}, "rankName": map[string]any{"type": "string"}, "rankRights": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "integer"}, "rights": map[string]any{"type": "integer"}}, "required": []string{"id", "rights"}}}, "targetPlayerId": map[string]any{"type": "integer", "minimum": 0}, "targetRankId": map[string]any{"type": "integer", "minimum": 0}, "circularRankId": map[string]any{"type": "integer", "minimum": 0},
	}, []string{"action"}, "allianceMutation")
}

func updateAccountOptionsTool() domainmcp.Tool {
	return playerMutationTool("update_account_options", "Update account options", "Update settings, name, password, email, vacation mode, deletion state, or activation mail while preserving omitted values. The scoped bearer token replaces current-password reauthentication.", map[string]any{
		"planetId": map[string]any{"type": "integer", "minimum": 0}, "action": map[string]any{"type": "string", "enum": []string{"settings", "rename", "password", "email", "vacation", "deletion", "resend_activation"}}, "name": map[string]any{"type": "string"}, "newPassword": map[string]any{"type": "string"}, "newPasswordRepeat": map[string]any{"type": "string"}, "email": map[string]any{"type": "string"}, "enabled": map[string]any{"type": "boolean"}, "language": map[string]any{"type": "string"}, "skinPath": map[string]any{"type": "string"}, "useSkin": map[string]any{"type": "boolean"}, "deactivateIp": map[string]any{"type": "boolean"}, "sortBy": map[string]any{"type": "integer", "minimum": 0}, "sortOrder": map[string]any{"type": "integer", "minimum": 0}, "maxSpy": map[string]any{"type": "integer", "minimum": 0}, "maxFleetMessages": map[string]any{"type": "integer", "minimum": 0}, "showEspionageButton": map[string]any{"type": "boolean"}, "showWriteMessage": map[string]any{"type": "boolean"}, "showBuddy": map[string]any{"type": "boolean"}, "showRocketAttack": map[string]any{"type": "boolean"}, "showViewReport": map[string]any{"type": "boolean"}, "doNotUseFolders": map[string]any{"type": "boolean"}, "feedEnabled": map[string]any{"type": "boolean"}, "feedType": map[string]any{"type": "string", "enum": []string{"rss", "atom"}}, "hideGoEmail": map[string]any{"type": "boolean"},
	}, []string{"action"}, "accountOptionsMutation")
}

func redeemCouponTool() domainmcp.Tool {
	return playerMutationTool("redeem_coupon", "Redeem Dark Matter coupon", "Validate a coupon during dry-run and redeem it only with the returned confirmation.", map[string]any{
		"couponCode": map[string]any{"type": "string"},
	}, []string{"couponCode"}, "couponRedemption")
}
