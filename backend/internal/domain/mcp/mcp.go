package mcp

import "errors"

const ProtocolVersion = "2025-06-18"

const (
	ScopeRead           = "mcp:read"
	ScopeWrite          = "mcp:write"
	ScopeFleet          = "mcp:fleet"
	ScopeFleetWrite     = "mcp:fleet_write"
	ScopeQueueWrite     = "mcp:queue_write"
	ScopeResourcesWrite = "mcp:resources_write"
	ScopePremiumWrite   = "mcp:premium_write"
	ScopeMessages       = "mcp:messages"
	ScopeMessageWrite   = "mcp:message_write"
	ScopeAdmin          = "mcp:admin"
)

var (
	ErrToolNotFound  = errors.New("mcp tool not found")
	ErrUnauthorized  = errors.New("mcp unauthorized")
	ErrForbidden     = errors.New("mcp forbidden")
	ErrInvalidParams = errors.New("mcp invalid params")
)

type Token struct {
	ID         int      `json:"id"`
	PlayerID   int      `json:"playerId,omitempty"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	CreatedAt  int64    `json:"createdAt"`
	ExpiresAt  int64    `json:"expiresAt,omitempty"`
	LastUsedAt int64    `json:"lastUsedAt,omitempty"`
	RevokedAt  int64    `json:"revokedAt,omitempty"`
}

type TokenCreation struct {
	Token  Token  `json:"token"`
	Secret string `json:"secret"`
}

type OAuthAuthorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	JWKSURI                           string   `json:"jwks_uri,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethods     []string `json:"revocation_endpoint_auth_methods_supported,omitempty"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported"`
}

type OAuthProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported,omitempty"`
	BearerMethods        []string `json:"bearer_methods_supported,omitempty"`
}

type JSONWebKey struct {
	KeyType string `json:"kty"`
	Use     string `json:"use,omitempty"`
	KeyID   string `json:"kid,omitempty"`
	Alg     string `json:"alg,omitempty"`
	Curve   string `json:"crv,omitempty"`
	X       string `json:"x,omitempty"`
}

type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

type IDTokenCommand struct {
	Issuer   string
	Audience string
	PlayerID int
	Scopes   []string
}

type OAuthAuthorizationCode struct {
	ID                  int
	PlayerID            int
	ClientID            string
	RedirectURI         string
	Resource            string
	Scopes              []string
	CodeHash            string
	CodeChallenge       string
	CodeChallengeMethod string
	CreatedAt           int64
	ExpiresAt           int64
	ConsumedAt          int64
}

type ToolCallAudit struct {
	ToolName   string
	PlayerID   int
	Scopes     []string
	Authorized bool
	Error      string
	At         int64
	DurationMS int64
}

type Coordinates struct {
	Galaxy   int `json:"galaxy"`
	System   int `json:"system"`
	Position int `json:"position"`
}

type Planet struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Type        int         `json:"type"`
	TypeName    string      `json:"typeName"`
	Coordinates Coordinates `json:"coordinates"`
	Current     bool        `json:"current"`
}

type Score struct {
	Raw     int64 `json:"raw"`
	Display int64 `json:"display"`
	Rank    int   `json:"rank"`
}

type AccountOverview struct {
	PlayerID       int    `json:"playerId"`
	Commander      string `json:"commander"`
	Score          Score  `json:"score"`
	CurrentPlanet  Planet `json:"currentPlanet"`
	PlanetCount    int    `json:"planetCount"`
	UnreadMessages int    `json:"unreadMessages"`
}

type ResourceAmounts struct {
	Metal      float64 `json:"metal"`
	Crystal    float64 `json:"crystal"`
	Deuterium  float64 `json:"deuterium"`
	DarkMatter int     `json:"darkMatter"`
}

type ResourceCapacity struct {
	Metal     int `json:"metal"`
	Crystal   int `json:"crystal"`
	Deuterium int `json:"deuterium"`
}

type ResourceRates struct {
	Metal     float64 `json:"metal"`
	Crystal   float64 `json:"crystal"`
	Deuterium float64 `json:"deuterium"`
}

type Energy struct {
	Available int `json:"available"`
	Capacity  int `json:"capacity"`
}

type PlanetResources struct {
	PlayerID          int              `json:"playerId"`
	Planet            Planet           `json:"planet"`
	Resources         ResourceAmounts  `json:"resources"`
	Capacity          ResourceCapacity `json:"capacity"`
	Energy            Energy           `json:"energy"`
	ProductionPerHour ResourceRates    `json:"productionPerHour"`
}

type BuildingQueueEntry struct {
	ListID           int    `json:"listId"`
	TechID           int    `json:"techId"`
	Name             string `json:"name"`
	Level            int    `json:"level"`
	Destroy          bool   `json:"destroy"`
	Start            int    `json:"start"`
	End              int    `json:"end"`
	RemainingSeconds int    `json:"remainingSeconds"`
}

type BuildingQueue struct {
	PlayerID int                  `json:"playerId"`
	Planet   Planet               `json:"planet"`
	Count    int                  `json:"count"`
	Entries  []BuildingQueueEntry `json:"entries"`
}

type MessageQuery struct {
	Limit          int
	MessageType    int
	HasMessageType bool
	IncludeText    bool
}

type PlayerMessage struct {
	ID         int    `json:"id"`
	Type       int    `json:"type"`
	TypeName   string `json:"typeName"`
	From       string `json:"from"`
	Subject    string `json:"subject"`
	Text       string `json:"text,omitempty"`
	Date       int64  `json:"date"`
	Unread     bool   `json:"unread"`
	Reportable bool   `json:"reportable"`
}

type MessageList struct {
	PlayerID int             `json:"playerId"`
	Count    int             `json:"count"`
	Limit    int             `json:"limit"`
	Messages []PlayerMessage `json:"messages"`
}

type MessageDetail struct {
	PlayerID int           `json:"playerId"`
	Message  PlayerMessage `json:"message"`
}

type ActionIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SendMessageCommand struct {
	TargetPlayerID int
	Subject        string
	Text           string
	DryRun         bool
	Confirm        string
}

type SendMessageResult struct {
	PlayerID             int          `json:"playerId"`
	TargetPlayerID       int          `json:"targetPlayerId"`
	Subject              string       `json:"subject"`
	TextChars            int          `json:"textChars"`
	DryRun               bool         `json:"dryRun"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Confirmation         string       `json:"confirmation,omitempty"`
	Executed             bool         `json:"executed"`
	Issue                *ActionIssue `json:"issue,omitempty"`
}

type DeleteMessagesCommand struct {
	MessageIDs []int
	DryRun     bool
	Confirm    string
}

type DeleteMessagesResult struct {
	PlayerID             int    `json:"playerId"`
	MessageIDs           []int  `json:"messageIds"`
	DeleteCount          int    `json:"deleteCount"`
	DryRun               bool   `json:"dryRun"`
	RequiresConfirmation bool   `json:"requiresConfirmation"`
	Confirmation         string `json:"confirmation,omitempty"`
	Executed             bool   `json:"executed"`
}

type ReportMessageCommand struct {
	MessageID int
	DryRun    bool
	Confirm   string
}

type ReportMessageResult struct {
	PlayerID             int          `json:"playerId"`
	MessageID            int          `json:"messageId"`
	Reportable           bool         `json:"reportable"`
	DryRun               bool         `json:"dryRun"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Confirmation         string       `json:"confirmation,omitempty"`
	Executed             bool         `json:"executed"`
	Issue                *ActionIssue `json:"issue,omitempty"`
}

type RecallFleetCommand struct {
	FleetID int
	DryRun  bool
	Confirm string
}

type RecallFleetResult struct {
	PlayerID             int          `json:"playerId"`
	FleetID              int          `json:"fleetId"`
	OwnerID              int          `json:"ownerId,omitempty"`
	Mission              int          `json:"mission,omitempty"`
	TotalShips           int          `json:"totalShips,omitempty"`
	Recallable           bool         `json:"recallable"`
	DryRun               bool         `json:"dryRun"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Confirmation         string       `json:"confirmation,omitempty"`
	Executed             bool         `json:"executed"`
	Issue                *ActionIssue `json:"issue,omitempty"`
}

type CancelBuildingQueueCommand struct {
	PlanetID int
	ListID   int
	DryRun   bool
	Confirm  string
}

type CancelBuildingQueueResult struct {
	PlayerID             int          `json:"playerId"`
	PlanetID             int          `json:"planetId"`
	ListID               int          `json:"listId"`
	TechID               int          `json:"techId,omitempty"`
	Name                 string       `json:"name,omitempty"`
	Level                int          `json:"level,omitempty"`
	Destroy              bool         `json:"destroy,omitempty"`
	Cancelable           bool         `json:"cancelable"`
	DryRun               bool         `json:"dryRun"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Confirmation         string       `json:"confirmation,omitempty"`
	Executed             bool         `json:"executed"`
	Issue                *ActionIssue `json:"issue,omitempty"`
}

type CancelResearchQueueCommand struct {
	DryRun  bool
	Confirm string
}

type CancelResearchQueueResult struct {
	PlayerID             int          `json:"playerId"`
	PlanetID             int          `json:"planetId,omitempty"`
	TaskID               int          `json:"taskId,omitempty"`
	TechID               int          `json:"techId,omitempty"`
	Name                 string       `json:"name,omitempty"`
	Level                int          `json:"level,omitempty"`
	Cancelable           bool         `json:"cancelable"`
	DryRun               bool         `json:"dryRun"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Confirmation         string       `json:"confirmation,omitempty"`
	Executed             bool         `json:"executed"`
	Issue                *ActionIssue `json:"issue,omitempty"`
}

type EnqueueShipyardOrderCommand struct {
	PlanetID int
	Kind     string
	ItemID   int
	Amount   int
	DryRun   bool
	Confirm  string
}

type EnqueueShipyardOrderResult struct {
	PlayerID             int          `json:"playerId"`
	PlanetID             int          `json:"planetId"`
	Kind                 string       `json:"kind"`
	ItemID               int          `json:"itemId"`
	Name                 string       `json:"name,omitempty"`
	Requested            int          `json:"requested"`
	Amount               int          `json:"amount"`
	MaxBuild             int          `json:"maxBuild,omitempty"`
	DurationSeconds      int          `json:"durationSeconds,omitempty"`
	DryRun               bool         `json:"dryRun"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Confirmation         string       `json:"confirmation,omitempty"`
	Executed             bool         `json:"executed"`
	Issue                *ActionIssue `json:"issue,omitempty"`
}

type ResourceProductionSetting struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Percent int    `json:"percent"`
}

type UpdateResourceProductionCommand struct {
	PlanetID   int
	Production map[int]int
	DryRun     bool
	Confirm    string
}

type UpdateResourceProductionResult struct {
	PlayerID             int                         `json:"playerId"`
	PlanetID             int                         `json:"planetId"`
	Settings             []ResourceProductionSetting `json:"settings"`
	DryRun               bool                        `json:"dryRun"`
	RequiresConfirmation bool                        `json:"requiresConfirmation"`
	Confirmation         string                      `json:"confirmation,omitempty"`
	Executed             bool                        `json:"executed"`
	Issue                *ActionIssue                `json:"issue,omitempty"`
}

type RecruitOfficerCommand struct {
	PlanetID  int
	OfficerID int
	Days      int
	DryRun    bool
	Confirm   string
}

type RecruitOfficerResult struct {
	PlayerID             int          `json:"playerId"`
	PlanetID             int          `json:"planetId"`
	OfficerID            int          `json:"officerId"`
	Name                 string       `json:"name,omitempty"`
	Days                 int          `json:"days"`
	Cost                 int          `json:"cost"`
	PaidDarkMatter       int          `json:"paidDarkMatter"`
	FreeDarkMatter       int          `json:"freeDarkMatter"`
	Until                int64        `json:"until,omitempty"`
	DaysLeft             int          `json:"daysLeft,omitempty"`
	Active               bool         `json:"active"`
	DryRun               bool         `json:"dryRun"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Confirmation         string       `json:"confirmation,omitempty"`
	Executed             bool         `json:"executed"`
	Issue                *ActionIssue `json:"issue,omitempty"`
}

type OfficerStatusRow struct {
	ID             int    `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	Until          int64  `json:"until,omitempty"`
	DaysLeft       int    `json:"daysLeft,omitempty"`
	WeekCost       int    `json:"weekCost"`
	ThreeMonthCost int    `json:"threeMonthCost"`
	Note           string `json:"note,omitempty"`
}

type OfficerStatus struct {
	PlayerID       int                `json:"playerId"`
	PlanetID       int                `json:"planetId"`
	PaidDarkMatter int                `json:"paidDarkMatter"`
	FreeDarkMatter int                `json:"freeDarkMatter"`
	Officers       []OfficerStatusRow `json:"officers"`
}

type SearchCommand struct {
	PlanetID int
	Type     string
	Text     string
}

type SearchAllianceRef struct {
	ID  int    `json:"id"`
	Tag string `json:"tag"`
}

type SearchPlayerRow struct {
	PlayerID     int                `json:"playerId"`
	PlayerName   string             `json:"playerName"`
	Alliance     *SearchAllianceRef `json:"alliance,omitempty"`
	PlanetID     int                `json:"planetId"`
	PlanetName   string             `json:"planetName"`
	Coordinates  Coordinates        `json:"coordinates"`
	Rank         int                `json:"rank"`
	Own          bool               `json:"own"`
	SameAlliance bool               `json:"sameAlliance"`
}

type SearchAllianceRow struct {
	AllianceID int    `json:"allianceId"`
	Tag        string `json:"tag"`
	Name       string `json:"name"`
	Members    int    `json:"members"`
	Score      int64  `json:"score"`
	Display    int64  `json:"display"`
	Own        bool   `json:"own"`
}

type SearchResult struct {
	PlayerID  int                 `json:"playerId"`
	PlanetID  int                 `json:"planetId"`
	Type      string              `json:"type"`
	Text      string              `json:"text"`
	Message   string              `json:"message,omitempty"`
	Players   []SearchPlayerRow   `json:"players"`
	Alliances []SearchAllianceRow `json:"alliances"`
}

type GalaxySystemCommand struct {
	PlanetID int
	Galaxy   int
	System   int
}

type GalaxyBounds struct {
	Galaxies int `json:"galaxies"`
	Systems  int `json:"systems"`
}

type GalaxyFleetSlots struct {
	Used    int  `json:"used"`
	Max     int  `json:"max"`
	BaseMax int  `json:"baseMax"`
	Admiral bool `json:"admiral"`
}

type GalaxySystemExtra struct {
	Commander bool             `json:"commander"`
	SpyProbes int              `json:"spyProbes"`
	Recyclers int              `json:"recyclers"`
	Missiles  int              `json:"missiles"`
	MaxSpy    int              `json:"maxSpy"`
	Slots     GalaxyFleetSlots `json:"slots"`
}

type GalaxySystemActions struct {
	Deploy     bool `json:"deploy"`
	Transport  bool `json:"transport"`
	Spy        bool `json:"spy"`
	Message    bool `json:"message"`
	Buddy      bool `json:"buddy"`
	ViewReport bool `json:"viewReport"`
	Phalanx    bool `json:"phalanx"`
	Missile    bool `json:"missile"`
	Attack     bool `json:"attack"`
	Defend     bool `json:"defend"`
	Destroy    bool `json:"destroy"`
	Recycle    bool `json:"recycle"`
}

type GalaxySystemPlayer struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Rank        int      `json:"rank"`
	Status      string   `json:"status"`
	StatusClass string   `json:"statusClass"`
	Suffixes    []string `json:"suffixes"`
	Own         bool     `json:"own"`
}

type GalaxySystemAlliance struct {
	ID      int    `json:"id"`
	Tag     string `json:"tag"`
	Rank    int    `json:"rank"`
	Members int    `json:"members"`
}

type GalaxySystemObject struct {
	ID           int                   `json:"id"`
	Name         string                `json:"name"`
	DisplayName  string                `json:"displayName"`
	Type         int                   `json:"type"`
	Coordinates  Coordinates           `json:"coordinates"`
	ActivityText string                `json:"activityText,omitempty"`
	Destroyed    bool                  `json:"destroyed"`
	Abandoned    bool                  `json:"abandoned"`
	Own          bool                  `json:"own"`
	ReportID     int                   `json:"reportId,omitempty"`
	Player       *GalaxySystemPlayer   `json:"player,omitempty"`
	Alliance     *GalaxySystemAlliance `json:"alliance,omitempty"`
	Actions      GalaxySystemActions   `json:"actions"`
}

type GalaxySystemDebris struct {
	ID         int     `json:"id"`
	Metal      float64 `json:"metal"`
	Crystal    float64 `json:"crystal"`
	Harvesters int     `json:"harvesters"`
	Visible    bool    `json:"visible"`
}

type GalaxySystemRow struct {
	Position int                 `json:"position"`
	Planet   *GalaxySystemObject `json:"planet,omitempty"`
	Moon     *GalaxySystemObject `json:"moon,omitempty"`
	Debris   *GalaxySystemDebris `json:"debris,omitempty"`
}

type GalaxySystem struct {
	PlayerID            int               `json:"playerId"`
	PlanetID            int               `json:"planetId"`
	Coordinates         Coordinates       `json:"coordinates"`
	Bounds              GalaxyBounds      `json:"bounds"`
	Populated           int               `json:"populated"`
	Slots               GalaxyFleetSlots  `json:"slots"`
	Extra               GalaxySystemExtra `json:"extra"`
	NotEnoughDeuterium  bool              `json:"notEnoughDeuterium"`
	RemoteSystemCostDue bool              `json:"remoteSystemCostDue"`
	Rows                []GalaxySystemRow `json:"rows"`
}

type StatisticsCommand struct {
	PlanetID int
	Who      string
	Type     string
	Start    int
}

type StatisticsPlayerRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type StatisticsAllianceRef struct {
	ID  int    `json:"id"`
	Tag string `json:"tag"`
}

type StatisticsRow struct {
	Place          int                    `json:"place"`
	PreviousPlace  int                    `json:"previousPlace"`
	Delta          int                    `json:"delta"`
	Score          int64                  `json:"score"`
	DisplayScore   int64                  `json:"displayScore"`
	ScorePerMember int64                  `json:"scorePerMember,omitempty"`
	ScoreDate      int64                  `json:"scoreDate"`
	Player         *StatisticsPlayerRef   `json:"player,omitempty"`
	Alliance       *StatisticsAllianceRef `json:"alliance,omitempty"`
	Coordinates    Coordinates            `json:"coordinates"`
	Members        int                    `json:"members,omitempty"`
	Own            bool                   `json:"own"`
	SameAlliance   bool                   `json:"sameAlliance"`
}

type Statistics struct {
	PlayerID         int             `json:"playerId"`
	PlanetID         int             `json:"planetId"`
	ViewerAllianceID int             `json:"viewerAllianceId"`
	Who              string          `json:"who"`
	Type             string          `json:"type"`
	Start            int             `json:"start"`
	Total            int             `json:"total"`
	GeneratedAt      int64           `json:"generatedAt"`
	Rows             []StatisticsRow `json:"rows"`
}

type DispatchFleetCommand struct {
	PlanetID        int
	Ships           map[int]int
	Resources       FleetResources
	Target          Coordinates
	TargetType      int
	Mission         int
	Speed           int
	HoldHours       int
	ExpeditionHours int
	UnionID         int
}

type DispatchFleetValidationResult struct {
	PlayerID             int          `json:"playerId"`
	PlanetID             int          `json:"planetId"`
	Ready                bool         `json:"ready"`
	DryRun               bool         `json:"dryRun"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Confirmation         string       `json:"confirmation,omitempty"`
	Executed             bool         `json:"executed"`
	TotalShips           int          `json:"totalShips"`
	Mission              int          `json:"mission"`
	Target               Coordinates  `json:"target"`
	TargetType           int          `json:"targetType"`
	Speed                int          `json:"speed"`
	FuelConsumption      int          `json:"fuelConsumption"`
	Cargo                int          `json:"cargo"`
	RemainingCargo       int          `json:"remainingCargo"`
	DurationSeconds      int          `json:"durationSeconds"`
	Distance             int          `json:"distance"`
	Issue                *ActionIssue `json:"issue,omitempty"`
}

type FleetShip struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type FleetResources struct {
	Metal     int `json:"metal"`
	Crystal   int `json:"crystal"`
	Deuterium int `json:"deuterium"`
}

type FleetUnionPlayer struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type FleetMovement struct {
	ID               int                `json:"id"`
	OwnerID          int                `json:"ownerId"`
	OwnerName        string             `json:"ownerName"`
	Foreign          bool               `json:"foreign"`
	Mission          int                `json:"mission"`
	MissionName      string             `json:"missionName"`
	StateTitle       string             `json:"stateTitle"`
	StateShort       string             `json:"stateShort"`
	FleetDetailLevel int                `json:"fleetDetailLevel"`
	Ships            []FleetShip        `json:"ships"`
	TotalShips       int                `json:"totalShips"`
	LoadedResources  FleetResources     `json:"loadedResources"`
	MissileAmount    int                `json:"missileAmount"`
	MissileTargetID  int                `json:"missileTargetId"`
	MissileTarget    string             `json:"missileTarget"`
	UnionID          int                `json:"unionId"`
	UnionName        string             `json:"unionName"`
	UnionPlayers     []FleetUnionPlayer `json:"unionPlayers"`
	GroupMissions    []FleetMovement    `json:"groupMissions"`
	Origin           Coordinates        `json:"origin"`
	OriginName       string             `json:"originName"`
	Target           Coordinates        `json:"target"`
	TargetName       string             `json:"targetName"`
	TargetType       int                `json:"targetType"`
	TargetOwnerName  string             `json:"targetOwnerName"`
	DepartureAt      int64              `json:"departureAt"`
	ArrivalAt        int64              `json:"arrivalAt"`
	RemainingSeconds int                `json:"remainingSeconds"`
	CanRecall        bool               `json:"canRecall"`
	CanCreateUnion   bool               `json:"canCreateUnion"`
}

type FleetMovements struct {
	PlayerID int             `json:"playerId"`
	Now      int64           `json:"now"`
	Count    int             `json:"count"`
	Events   []FleetMovement `json:"events"`
}

type Access struct {
	Authenticated bool     `json:"authenticated"`
	PlayerID      int      `json:"playerId,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
}

func (a Access) HasScope(scope string) bool {
	for _, candidate := range a.Scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type Tool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}

type ListToolsCommand struct {
	Cursor      string
	AccessToken string
}

type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type CallToolCommand struct {
	Name        string
	Arguments   map[string]any
	AccessToken string
}

type ToolCallResult struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
