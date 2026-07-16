package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type AdminService interface {
	GetAdminForPlayer(context.Context, appgame.AdminQuery) (appgame.AdminResult, error)
	MutateAdminForPlayer(context.Context, int, appgame.AdminMutationCommand) (appgame.AdminResult, error)
	MutateAdminBotEditForPlayer(context.Context, int, appgame.AdminBotEditMutationCommand) (appgame.AdminBotEditMutationResult, error)
}

type adminReadArguments struct {
	PlanetID       int                            `json:"planetId"`
	Mode           string                         `json:"mode"`
	TargetPlayerID int                            `json:"targetPlayerId"`
	TargetPlanetID int                            `json:"targetPlanetId"`
	Filter         string                         `json:"filter"`
	LoginName      string                         `json:"loginName"`
	LoginUserID    int                            `json:"loginUserId"`
	LoginIP        string                         `json:"loginIp"`
	LocaSource     string                         `json:"localizationSource"`
	LocaTarget     string                         `json:"localizationTarget"`
	CouponFrom     int                            `json:"couponFrom"`
	PlanetSearch   *domaingame.AdminPlanetSearch  `json:"planetSearch"`
	UserLogSearch  *domaingame.AdminUserLogSearch `json:"userLogSearch"`
}

type adminMutationArguments struct {
	adminReadArguments
	Action       string                            `json:"action"`
	TaskID       int                               `json:"taskId"`
	TargetIDs    []int                             `json:"targetIds"`
	BanMode      int                               `json:"banMode"`
	Days         int                               `json:"days"`
	Hours        int                               `json:"hours"`
	Reason       string                            `json:"reason"`
	Values       map[string]int                    `json:"values"`
	User         *adminUserMutationArguments       `json:"userSettings"`
	Planet       *adminPlanetMutationArguments     `json:"planetSettings"`
	Universe     *domaingame.AdminUniverseMutation `json:"universeSettings"`
	Category     int                               `json:"category"`
	Subject      string                            `json:"subject"`
	Text         string                            `json:"text"`
	ReportIDs    []int                             `json:"reportIds"`
	DeleteMode   string                            `json:"deleteMode"`
	FileName     string                            `json:"fileName"`
	Amount       int                               `json:"amount"`
	ItemID       int                               `json:"itemId"`
	StrategyID   int                               `json:"strategyId"`
	DayMonth     string                            `json:"dayMonth"`
	HourMinute   string                            `json:"hourMinute"`
	InactiveDays int                               `json:"inactiveDays"`
	IngameDays   int                               `json:"ingameDays"`
	PeriodicDays int                               `json:"periodicDays"`
	ModName      string                            `json:"modName"`
	Name         string                            `json:"name"`
	Source       string                            `json:"source"`
	DryRun       *bool                             `json:"dryRun"`
	Confirm      string                            `json:"confirm"`
}

type adminUserMutationArguments struct {
	PermanentEmail string         `json:"permanentEmail"`
	Email          string         `json:"email"`
	Skin           string         `json:"skin"`
	Disable        bool           `json:"disable"`
	Vacation       bool           `json:"vacation"`
	Banned         bool           `json:"banned"`
	NoAttack       bool           `json:"noAttack"`
	Validated      bool           `json:"validated"`
	Sniff          bool           `json:"sniff"`
	Debug          bool           `json:"debug"`
	UseSkin        bool           `json:"useSkin"`
	DeactivateIP   bool           `json:"deactivateIp"`
	AdminLevel     int            `json:"adminLevel"`
	DarkMatter     int            `json:"darkMatter"`
	DarkMatterFree int            `json:"darkMatterFree"`
	SortBy         int            `json:"sortBy"`
	SortOrder      int            `json:"sortOrder"`
	MaxSpy         int            `json:"maxSpy"`
	MaxFleetMsg    int            `json:"maxFleetMessages"`
	Research       map[string]int `json:"research"`
	OfficerDays    map[string]int `json:"officerDays"`
}

type adminPlanetMutationArguments struct {
	Coordinates domaingame.Coordinates `json:"coordinates"`
	Diameter    int                    `json:"diameter"`
	Type        int                    `json:"type"`
	Temperature int                    `json:"temperature"`
	Resources   map[string]int         `json:"resources"`
	Buildings   map[string]int         `json:"buildings"`
	Fleet       map[string]int         `json:"fleet"`
	Defense     map[string]int         `json:"defense"`
	Production  map[string]float64     `json:"production"`
	Delete      bool                   `json:"delete"`
}

func (s Service) WithAdminService(service AdminService) Service {
	s.adminService = service
	return s
}

func staffAccessLevel(access domainmcp.Access) int {
	if access.UserType >= domaingame.AdminLevelAdmin && access.HasScope(domainmcp.ScopeAdmin) {
		return domaingame.AdminLevelAdmin
	}
	if access.UserType >= domaingame.AdminLevelOperator && access.HasScope(domainmcp.ScopeOperator) {
		return domaingame.AdminLevelOperator
	}
	return domaingame.AdminLevelPlayer
}

func (s Service) authorizeStaff(ctx context.Context, token string) (domainmcp.Access, error) {
	access, err := s.verify(ctx, token)
	if err != nil {
		return domainmcp.Access{}, err
	}
	if staffAccessLevel(access) == domaingame.AdminLevelPlayer {
		return domainmcp.Access{}, domainmcp.ErrForbidden
	}
	return access, nil
}

func callAdminAccess(access domainmcp.Access) (domainmcp.ToolCallResult, error) {
	level := staffAccessLevel(access)
	modes := make([]map[string]any, 0, len(domaingame.AdminMenu())+1)
	modes = append(modes, map[string]any{"mode": "Home", "label": "Home", "requiresAdmin": false, "available": true})
	for _, item := range domaingame.AdminMenu() {
		requiresAdmin := domaingame.AdminModeRequiresAdmin(item.Mode)
		modes = append(modes, map[string]any{
			"mode": item.Mode, "label": item.Label, "requiresAdmin": requiresAdmin,
			"available": !requiresAdmin || level >= domaingame.AdminLevelAdmin,
		})
	}
	return adminToolResult("adminAccess", map[string]any{
		"playerId":  access.PlayerID,
		"userType":  access.UserType,
		"role":      domaingame.AdminRoleName(access.UserType),
		"scopeRole": domaingame.AdminRoleName(level),
		"modes":     modes,
	})
}

func (s Service) callGetAdminPanel(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.adminService == nil {
		return domainmcp.ToolCallResult{}, fmt.Errorf("mcp admin service unavailable")
	}
	var args adminReadArguments
	if err := decodeAdminArguments(arguments, &args); err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	mode, err := validateAdminReadArguments(args)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if staffAccessLevel(access) < domaingame.AdminLevelAdmin && domaingame.AdminModeRequiresAdmin(mode) {
		return domainmcp.ToolCallResult{}, domainmcp.ErrForbidden
	}
	result, err := s.adminService.GetAdminForPlayer(ctx, adminQuery(access.PlayerID, args, mode))
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if result.ActionIssue != nil && result.ActionIssue.Code == domaingame.AdminIssueAccessDenied {
		return domainmcp.ToolCallResult{}, domainmcp.ErrForbidden
	}
	return adminToolResult("adminPanel", adminPanelPayload(result.Admin, access))
}

func (s Service) callMutateAdminPanel(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.adminService == nil {
		return domainmcp.ToolCallResult{}, fmt.Errorf("mcp admin service unavailable")
	}
	var args adminMutationArguments
	if err := decodeAdminArguments(arguments, &args); err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	mode, err := validateAdminReadArguments(args.adminReadArguments)
	if err != nil || strings.TrimSpace(args.Action) == "" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	level := staffAccessLevel(access)
	if level < domaingame.AdminLevelAdmin && domaingame.AdminMutationRequiresAdmin(mode, args.Action) {
		return domainmcp.ToolCallResult{}, domainmcp.ErrForbidden
	}
	preview, err := s.adminService.GetAdminForPlayer(ctx, adminQuery(access.PlayerID, args.adminReadArguments, mode))
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	permissionView := preview.Admin
	if permissionView.Viewer.Level > level {
		permissionView.Viewer.Level = level
	}
	if !permissionView.CanMutate(args.Action) {
		return domainmcp.ToolCallResult{}, domainmcp.ErrForbidden
	}
	confirmation := adminMutationConfirmation(arguments)
	dryRun := args.DryRun == nil || *args.DryRun
	if dryRun {
		return adminToolResult("adminMutation", map[string]any{
			"mode": mode, "action": args.Action, "dryRun": true, "executed": false,
			"requiresConfirmation": true, "confirmation": confirmation,
		})
	}
	if strings.TrimSpace(args.Confirm) != confirmation {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	if mode == "BotEdit" {
		botResult, err := s.adminService.MutateAdminBotEditForPlayer(ctx, access.PlayerID, appgame.AdminBotEditMutationCommand{
			PlanetID: args.PlanetID, Action: args.Action, StrategyID: args.StrategyID, Name: args.Name, Source: args.Source,
		})
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		return adminToolResult("adminMutation", map[string]any{
			"mode": mode, "action": args.Action, "dryRun": false,
			"executed":             botResult.ActionIssue == nil || botResult.ActionIssue.Code != domaingame.AdminIssueAccessDenied,
			"requiresConfirmation": false, "confirmation": confirmation, "issue": botResult.ActionIssue,
			"strategyId": botResult.SelectedStrategyID, "name": botResult.Name,
			"source": botResult.Source, "strategies": botResult.Strategies,
		})
	}
	result, err := s.adminService.MutateAdminForPlayer(ctx, access.PlayerID, adminMutationCommand(args, mode))
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	payload := map[string]any{
		"mode": mode, "action": args.Action, "dryRun": false, "executed": result.ActionIssue == nil || result.ActionIssue.Code != domaingame.AdminIssueAccessDenied,
		"requiresConfirmation": false, "confirmation": confirmation, "issue": result.ActionIssue,
		"panel": adminPanelPayload(result.Admin, access),
	}
	return adminToolResult("adminMutation", payload)
}

func decodeAdminArguments(arguments map[string]any, target any) error {
	data, err := json.Marshal(arguments)
	if err != nil {
		return domainmcp.ErrInvalidParams
	}
	if err := json.Unmarshal(data, target); err != nil {
		return domainmcp.ErrInvalidParams
	}
	return nil
}

func validateAdminReadArguments(args adminReadArguments) (string, error) {
	if args.PlanetID < 0 || args.TargetPlayerID < 0 || args.TargetPlanetID < 0 || args.LoginUserID < 0 || args.CouponFrom < 0 {
		return "", domainmcp.ErrInvalidParams
	}
	mode := domaingame.NormalizeAdminMode(args.Mode)
	if strings.TrimSpace(args.Mode) != "" && !strings.EqualFold(strings.TrimSpace(args.Mode), "Home") && mode == "Home" {
		return "", domainmcp.ErrInvalidParams
	}
	return mode, nil
}

func adminQuery(playerID int, args adminReadArguments, mode string) appgame.AdminQuery {
	return appgame.AdminQuery{
		PlayerID: playerID, PlanetID: args.PlanetID, Mode: mode,
		TargetPlayerID: args.TargetPlayerID, TargetPlanetID: args.TargetPlanetID,
		Filter: args.Filter, LoginName: args.LoginName, LoginUserID: args.LoginUserID,
		LoginIP: args.LoginIP, LoginUserIDSet: args.LoginUserID > 0,
		UserLogSearch: args.UserLogSearch, LocaSource: args.LocaSource, LocaTarget: args.LocaTarget,
		CouponFrom: args.CouponFrom, PlanetSearch: args.PlanetSearch,
	}
}

func adminMutationCommand(args adminMutationArguments, mode string) appgame.AdminMutationCommand {
	return appgame.AdminMutationCommand{
		PlanetID: args.PlanetID, Mode: mode, TargetPlayerID: args.TargetPlayerID,
		TargetPlanetID: args.TargetPlanetID, Filter: args.Filter, CouponFrom: args.CouponFrom,
		LoginName: args.LoginName, LoginUserID: args.LoginUserID, LoginIP: args.LoginIP,
		LoginUserIDSet: args.LoginUserID > 0, LocaSource: args.LocaSource, LocaTarget: args.LocaTarget,
		Action: args.Action, TaskID: args.TaskID, TargetIDs: args.TargetIDs, BanMode: args.BanMode,
		Days: args.Days, Hours: args.Hours, Reason: args.Reason, Values: args.Values,
		User: adminUserMutation(args.User), Planet: adminPlanetMutation(args.Planet), PlanetSearch: args.PlanetSearch,
		Universe: args.Universe, Category: args.Category, Subject: args.Subject, Text: args.Text,
		ReportIDs: args.ReportIDs, DeleteMode: args.DeleteMode, FileName: args.FileName,
		Amount: args.Amount, ItemID: args.ItemID, DayMonth: args.DayMonth, HourMinute: args.HourMinute,
		InactiveDays: args.InactiveDays, IngameDays: args.IngameDays, PeriodicDays: args.PeriodicDays,
		ModName: args.ModName, Name: args.Name, UserLogSearch: args.UserLogSearch,
	}
}

func adminUserMutation(args *adminUserMutationArguments) *domaingame.AdminUserMutation {
	if args == nil {
		return nil
	}
	return &domaingame.AdminUserMutation{
		PermanentEmail: args.PermanentEmail, Email: args.Email, Skin: args.Skin,
		Disable: args.Disable, Vacation: args.Vacation, Banned: args.Banned, NoAttack: args.NoAttack,
		Validated: args.Validated, Sniff: args.Sniff, Debug: args.Debug, UseSkin: args.UseSkin,
		DeactivateIP: args.DeactivateIP, AdminLevel: args.AdminLevel, DarkMatter: args.DarkMatter,
		DarkMatterFree: args.DarkMatterFree, SortBy: args.SortBy, SortOrder: args.SortOrder,
		MaxSpy: args.MaxSpy, MaxFleetMsg: args.MaxFleetMsg, Research: stringIntMap(args.Research),
		OfficerDays: stringIntMap(args.OfficerDays),
	}
}

func adminPlanetMutation(args *adminPlanetMutationArguments) *domaingame.AdminPlanetMutation {
	if args == nil {
		return nil
	}
	return &domaingame.AdminPlanetMutation{
		Coordinates: args.Coordinates, Diameter: args.Diameter, Type: args.Type, Temperature: args.Temperature,
		Resources: stringIntMap(args.Resources), Buildings: stringIntMap(args.Buildings),
		Fleet: stringIntMap(args.Fleet), Defense: stringIntMap(args.Defense),
		Production: stringFloatMap(args.Production), Delete: args.Delete,
	}
}

func stringIntMap(values map[string]int) map[int]int {
	result := make(map[int]int, len(values))
	for key, value := range values {
		id, err := strconv.Atoi(key)
		if err == nil {
			result[id] = value
		}
	}
	return result
}

func stringFloatMap(values map[string]float64) map[int]float64 {
	result := make(map[int]float64, len(values))
	for key, value := range values {
		id, err := strconv.Atoi(key)
		if err == nil {
			result[id] = value
		}
	}
	return result
}

func adminMutationConfirmation(arguments map[string]any) string {
	clean := make(map[string]any, len(arguments))
	for key, value := range arguments {
		if key != "dryRun" && key != "confirm" {
			clean[key] = value
		}
	}
	payload, _ := json.Marshal(clean)
	digest := HashToken("admin_mutation:" + string(payload))
	return "admin_mutation:" + digest[:24]
}

func adminPanelPayload(admin domaingame.Admin, access domainmcp.Access) map[string]any {
	return map[string]any{
		"viewer":         map[string]any{"playerId": admin.Viewer.PlayerID, "name": admin.Viewer.Name, "userType": admin.Viewer.Level, "role": domaingame.AdminRoleName(admin.Viewer.Level)},
		"scopeRole":      domaingame.AdminRoleName(staffAccessLevel(access)),
		"mode":           admin.Mode,
		"commander":      admin.Commander,
		"currentPlanet":  admin.CurrentPlanet,
		"planetSwitcher": admin.PlanetSwitcher,
		"data":           adminModeData(admin),
	}
}

func adminModeData(admin domaingame.Admin) any {
	switch admin.Mode {
	case "Fleetlogs":
		return map[string]any{"fleetLogs": admin.FleetLogRows}
	case "Browse":
		return map[string]any{"browseHistory": admin.BrowseRows}
	case "Reports":
		return map[string]any{"reports": admin.ReportRows}
	case "Bans", "Users":
		return map[string]any{"users": admin.UserRows, "activeUsers": admin.ActiveUsers, "selectedUser": admin.SelectedUser}
	case "Planets":
		return map[string]any{"planets": admin.PlanetRows, "selectedPlanet": admin.SelectedPlanet, "searchResults": admin.PlanetSearchRows}
	case "Queue":
		return map[string]any{"queue": admin.QueueRows}
	case "Uni", "BattleSim":
		return map[string]any{"universe": admin.Universe}
	case "Errors", "Debug":
		return map[string]any{"messages": admin.MessageRows}
	case "Expedition":
		return map[string]any{"settings": admin.Expedition}
	case "Logins":
		return map[string]any{"logins": admin.LoginRows}
	case "Checksum":
		return map[string]any{"checksums": admin.ChecksumGroups}
	case "Bots":
		return map[string]any{"bots": admin.BotRows}
	case "BattleReport":
		return map[string]any{"battleReports": admin.BattleReports}
	case "UserLogs":
		return map[string]any{"logs": admin.UserLogRows, "groups": admin.UserLogGroups}
	case "BotEdit":
		return map[string]any{"strategies": admin.BotStrategies}
	case "Coupons":
		return map[string]any{"coupons": admin.CouponRows, "queue": admin.CouponQueueRows, "from": admin.CouponFrom, "total": admin.CouponTotal}
	case "DB":
		return map[string]any{"backups": admin.DatabaseBackups}
	case "ColonySettings":
		return map[string]any{"settings": admin.ColonySettings}
	case "Loca":
		return map[string]any{"localization": admin.Localization}
	case "Mods":
		return map[string]any{"mods": admin.ModRows}
	default:
		return map[string]any{}
	}
}

func adminToolResult(key string, value any) (domainmcp.ToolCallResult, error) {
	structured := map[string]any{key: value}
	text, err := json.Marshal(structured)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	return domainmcp.ToolCallResult{
		Content:           []domainmcp.Content{{Type: "text", Text: string(text)}},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func adminAccessTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name: "get_admin_access", Title: "Get Staff MCP Access",
		Description: "Return the authenticated Operator or Admin role, scope ceiling, and role-filtered admin mode inventory.",
		InputSchema: closedObjectSchema(nil, nil),
		Annotations: readOnlyAnnotations(),
	}
}

func adminPanelTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name: "get_admin_panel", Title: "Get Admin Panel",
		Description: "Read one legacy-equivalent administration mode. Operator tokens cannot read Admin-only modes.",
		InputSchema: adminToolInputSchema(false),
		Annotations: readOnlyAnnotations(),
	}
}

func mutateAdminPanelTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name: "mutate_admin_panel", Title: "Mutate Admin Panel",
		Description: "Preview or execute an administration action. Defaults to dry-run and requires the returned confirmation token. Role rules are enforced again at execution.",
		InputSchema: adminToolInputSchema(true),
		Annotations: map[string]any{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": false, "openWorldHint": false},
	}
}

func closedObjectSchema(properties map[string]any, required []string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func readOnlyAnnotations() map[string]any {
	return map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}
}

func adminToolInputSchema(mutation bool) map[string]any {
	properties := map[string]any{
		"planetId":           map[string]any{"type": "integer", "minimum": 0},
		"mode":               map[string]any{"type": "string", "description": "Admin mode from get_admin_access."},
		"targetPlayerId":     map[string]any{"type": "integer", "minimum": 0},
		"targetPlanetId":     map[string]any{"type": "integer", "minimum": 0},
		"filter":             map[string]any{"type": "string"},
		"loginName":          map[string]any{"type": "string"},
		"loginUserId":        map[string]any{"type": "integer", "minimum": 0},
		"loginIp":            map[string]any{"type": "string"},
		"localizationSource": map[string]any{"type": "string"},
		"localizationTarget": map[string]any{"type": "string"},
		"couponFrom":         map[string]any{"type": "integer", "minimum": 0},
		"planetSearch":       map[string]any{"type": "object"},
		"userLogSearch":      map[string]any{"type": "object"},
	}
	if mutation {
		properties["action"] = map[string]any{"type": "string"}
		for _, name := range []string{"taskId", "banMode", "days", "hours", "category", "amount", "itemId", "strategyId", "inactiveDays", "ingameDays", "periodicDays"} {
			properties[name] = map[string]any{"type": "integer"}
		}
		for _, name := range []string{"reason", "subject", "text", "deleteMode", "fileName", "dayMonth", "hourMinute", "modName", "name", "source", "confirm"} {
			properties[name] = map[string]any{"type": "string"}
		}
		properties["targetIds"] = map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}
		for _, name := range []string{"values", "userSettings", "planetSettings", "universeSettings"} {
			properties[name] = map[string]any{"type": "object"}
		}
		properties["reportIds"] = map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}
		properties["dryRun"] = map[string]any{"type": "boolean", "default": true}
		return closedObjectSchema(properties, []string{"mode", "action"})
	}
	return closedObjectSchema(properties, []string{"mode"})
}
