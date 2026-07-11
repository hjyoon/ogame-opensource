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

type BuildingOption struct {
	ID              int            `json:"id"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Level           int            `json:"level"`
	NextLevel       int            `json:"nextLevel"`
	Cost            TechnologyCost `json:"cost"`
	DurationSeconds int            `json:"durationSeconds"`
	CanBuild        bool           `json:"canBuild"`
	Action          string         `json:"action"`
}

type BuildingOptions struct {
	PlayerID        int                  `json:"playerId"`
	Planet          Planet               `json:"planet"`
	CommanderActive bool                 `json:"commanderActive"`
	Queue           []BuildingQueueEntry `json:"queue"`
	Items           []BuildingOption     `json:"items"`
}

type ResearchQueueEntry struct {
	TaskID           int  `json:"taskId"`
	PlanetID         int  `json:"planetId"`
	TechID           int  `json:"techId"`
	Level            int  `json:"level"`
	Start            int  `json:"start"`
	End              int  `json:"end"`
	RemainingSeconds int  `json:"remainingSeconds"`
	Cancelable       bool `json:"cancelable"`
}

type ResearchOptions struct {
	PlayerID int                 `json:"playerId"`
	Planet   Planet              `json:"planet"`
	HasLab   bool                `json:"hasLab"`
	Active   *ResearchQueueEntry `json:"active,omitempty"`
	Items    []BuildingOption    `json:"items"`
}

type ShipyardQueueEntry struct {
	TaskID           int    `json:"taskId"`
	UnitID           int    `json:"unitId"`
	Name             string `json:"name"`
	Count            int    `json:"count"`
	Start            int    `json:"start"`
	End              int    `json:"end"`
	RemainingSeconds int    `json:"remainingSeconds"`
}

type ShipyardOption struct {
	ID               int            `json:"id"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Count            int            `json:"count"`
	Cost             TechnologyCost `json:"cost"`
	DurationSeconds  int            `json:"durationSeconds"`
	CanBuild         bool           `json:"canBuild"`
	MeetsRequirement bool           `json:"meetsRequirement"`
	MaxBuild         int            `json:"maxBuild"`
	BlockedReason    string         `json:"blockedReason,omitempty"`
}

type ShipyardOptions struct {
	PlayerID        int                  `json:"playerId"`
	Planet          Planet               `json:"planet"`
	CommanderActive bool                 `json:"commanderActive"`
	HasShipyard     bool                 `json:"hasShipyard"`
	Busy            bool                 `json:"busy"`
	Queue           []ShipyardQueueEntry `json:"queue"`
	Items           []ShipyardOption     `json:"items"`
}

type DefenseOptions struct {
	PlayerID        int                  `json:"playerId"`
	Planet          Planet               `json:"planet"`
	CommanderActive bool                 `json:"commanderActive"`
	HasShipyard     bool                 `json:"hasShipyard"`
	Busy            bool                 `json:"busy"`
	Queue           []ShipyardQueueEntry `json:"queue"`
	Items           []ShipyardOption     `json:"items"`
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

type ResourceProductionValues struct {
	Metal        float64 `json:"metal"`
	Crystal      float64 `json:"crystal"`
	Deuterium    float64 `json:"deuterium"`
	Energy       float64 `json:"energy"`
	EnergyRaw    float64 `json:"energyRaw"`
	EnergyStored bool    `json:"energyStored"`
}

type ResourceProductionBonusIcon struct {
	Image string `json:"image"`
	Alt   string `json:"alt"`
}

type ResourceProductionRow struct {
	ID         int                           `json:"id"`
	Name       string                        `json:"name"`
	Level      int                           `json:"level"`
	Percent    int                           `json:"percent"`
	Values     ResourceProductionValues      `json:"values"`
	BonusIcons []ResourceProductionBonusIcon `json:"bonusIcons"`
}

type ResourceProductionTotals struct {
	Hour ResourceProductionValues `json:"hour"`
	Day  ResourceProductionValues `json:"day"`
	Week ResourceProductionValues `json:"week"`
}

type ResourceProductionOptions struct {
	PlayerID int                         `json:"playerId"`
	Planet   Planet                      `json:"planet"`
	Factor   float64                     `json:"factor"`
	Natural  ResourceProductionValues    `json:"natural"`
	Rows     []ResourceProductionRow     `json:"rows"`
	Storage  ResourceProductionValues    `json:"storage"`
	Totals   ResourceProductionTotals    `json:"totals"`
	Settings []ResourceProductionSetting `json:"settings"`
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

type AllianceStatusCommand struct {
	PlanetID      int
	View          string
	SearchText    string
	TextKind      int
	AllianceID    int
	ApplicationID int
}

type AllianceViewer struct {
	PlayerID   int    `json:"playerId"`
	Name       string `json:"name"`
	Validated  bool   `json:"validated"`
	AllianceID int    `json:"allianceId"`
	RankID     int    `json:"rankId"`
	RankName   string `json:"rankName"`
	RankRights int    `json:"rankRights"`
	Founder    bool   `json:"founder"`
}

type AllianceInfo struct {
	ID               int    `json:"id"`
	Tag              string `json:"tag"`
	Name             string `json:"name"`
	OwnerID          int    `json:"ownerId"`
	Homepage         string `json:"homepage"`
	ImageLogo        string `json:"imageLogo"`
	Open             bool   `json:"open"`
	InsertApp        bool   `json:"insertApp"`
	ExternalText     string `json:"externalText"`
	InternalText     string `json:"internalText"`
	ApplicationText  string `json:"applicationText"`
	OldTag           string `json:"oldTag"`
	OldName          string `json:"oldName"`
	TagUntil         int64  `json:"tagUntil"`
	NameUntil        int64  `json:"nameUntil"`
	MemberCount      int    `json:"memberCount"`
	ApplicationCount int    `json:"applicationCount"`
}

type AllianceSearchResult struct {
	ID          int    `json:"id"`
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	MemberCount int    `json:"memberCount"`
}

type AllianceApplication struct {
	ID         int    `json:"id"`
	AllianceID int    `json:"allianceId"`
	PlayerID   int    `json:"playerId"`
	PlayerName string `json:"playerName"`
	Text       string `json:"text"`
	Date       int64  `json:"date"`
}

type AllianceMember struct {
	PlayerID  int    `json:"playerId"`
	Name      string `json:"name"`
	RankID    int    `json:"rankId"`
	RankName  string `json:"rankName"`
	Score     int64  `json:"score"`
	JoinedAt  int64  `json:"joinedAt"`
	LastClick int64  `json:"lastClick"`
	Galaxy    int    `json:"galaxy"`
	System    int    `json:"system"`
	Position  int    `json:"position"`
}

type AllianceRank struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Rights int    `json:"rights"`
}

type AllianceCircularResult struct {
	Recipients []string `json:"recipients"`
}

type AllianceStatus struct {
	PlayerID       int                     `json:"playerId"`
	Planet         Planet                  `json:"planet"`
	View           string                  `json:"view"`
	Viewer         AllianceViewer          `json:"viewer"`
	Own            *AllianceInfo           `json:"own,omitempty"`
	Target         *AllianceInfo           `json:"target,omitempty"`
	Pending        *AllianceApplication    `json:"pending,omitempty"`
	SearchText     string                  `json:"searchText"`
	TextKind       int                     `json:"textKind"`
	SearchResults  []AllianceSearchResult  `json:"searchResults"`
	Applications   []AllianceApplication   `json:"applications"`
	SelectedApp    *AllianceApplication    `json:"selectedApp,omitempty"`
	Members        []AllianceMember        `json:"members"`
	Ranks          []AllianceRank          `json:"ranks"`
	CircularResult *AllianceCircularResult `json:"circularResult,omitempty"`
}

type BuddyStatusCommand struct {
	PlanetID int
	Action   int
	BuddyID  int
}

type BuddyAllianceRef struct {
	ID      int    `json:"id"`
	Tag     string `json:"tag"`
	Founder bool   `json:"founder"`
}

type BuddyPlayer struct {
	PlayerID    int               `json:"playerId"`
	Name        string            `json:"name"`
	Alliance    *BuddyAllianceRef `json:"alliance,omitempty"`
	Coordinates Coordinates       `json:"coordinates"`
}

type BuddyOnlineStatus struct {
	Text  string `json:"text"`
	Color string `json:"color"`
}

type BuddyRow struct {
	BuddyID int               `json:"buddyId"`
	Player  BuddyPlayer       `json:"player"`
	Text    string            `json:"text"`
	Status  BuddyOnlineStatus `json:"status"`
}

type BuddyStatus struct {
	PlayerID  int          `json:"playerId"`
	Planet    Planet       `json:"planet"`
	Commander string       `json:"commander"`
	Action    int          `json:"action"`
	Rows      []BuddyRow   `json:"rows"`
	Target    *BuddyPlayer `json:"target,omitempty"`
}

type NotesStatusCommand struct {
	PlanetID int
	Action   int
	NoteID   int
}

type Note struct {
	ID            int    `json:"id"`
	Subject       string `json:"subject"`
	Text          string `json:"text"`
	TextSize      int    `json:"textSize"`
	Priority      int    `json:"priority"`
	PriorityColor string `json:"priorityColor"`
	Date          int64  `json:"date"`
}

type NotesStatus struct {
	PlayerID  int    `json:"playerId"`
	Planet    Planet `json:"planet"`
	Commander string `json:"commander"`
	Action    string `json:"action"`
	Rows      []Note `json:"rows"`
	EditNote  *Note  `json:"editNote,omitempty"`
}

type OptionsStatusCommand struct {
	PlanetID int
}

type OptionsUser struct {
	Name            string `json:"name"`
	NameLocked      bool   `json:"nameLocked"`
	Validated       bool   `json:"validated"`
	Admin           int    `json:"admin"`
	CommanderActive bool   `json:"commanderActive"`
}

type OptionsUniverse struct {
	Language      string `json:"language"`
	ForceLanguage bool   `json:"forceLanguage"`
	FeedAge       int    `json:"feedAge"`
	Speed         int    `json:"speed"`
}

type OptionsSettings struct {
	Language         string `json:"language"`
	SkinPath         string `json:"skinPath"`
	UseSkin          bool   `json:"useSkin"`
	DeactivateIP     bool   `json:"deactivateIp"`
	SortBy           int    `json:"sortBy"`
	SortOrder        int    `json:"sortOrder"`
	MaxSpy           int    `json:"maxSpy"`
	MaxFleetMessages int    `json:"maxFleetMessages"`
}

type OptionsAccount struct {
	Vacation       bool  `json:"vacation"`
	VacationUntil  int64 `json:"vacationUntil,omitempty"`
	DeletionQueued bool  `json:"deletionQueued"`
	DeletionAt     int64 `json:"deletionAt,omitempty"`
}

type OptionsFlags struct {
	ShowEspionageButton bool `json:"showEspionageButton"`
	ShowWriteMessage    bool `json:"showWriteMessage"`
	ShowBuddy           bool `json:"showBuddy"`
	ShowRocketAttack    bool `json:"showRocketAttack"`
	ShowViewReport      bool `json:"showViewReport"`
	DoNotUseFolders     bool `json:"doNotUseFolders"`
	FeedEnabled         bool `json:"feedEnabled"`
	FeedAtom            bool `json:"feedAtom"`
	HideGOEmail         bool `json:"hideGoEmail"`
}

type OptionsStatus struct {
	PlayerID  int             `json:"playerId"`
	Planet    Planet          `json:"planet"`
	Commander string          `json:"commander"`
	User      OptionsUser     `json:"user"`
	Universe  OptionsUniverse `json:"universe"`
	Settings  OptionsSettings `json:"settings"`
	Account   OptionsAccount  `json:"account"`
	Flags     OptionsFlags    `json:"flags"`
}

type MerchantStatusCommand struct {
	PlanetID int
}

type MerchantUser struct {
	PaidDarkMatter int `json:"paidDarkMatter"`
	FreeDarkMatter int `json:"freeDarkMatter"`
}

type MerchantRates struct {
	Metal     float64 `json:"metal"`
	Crystal   float64 `json:"crystal"`
	Deuterium float64 `json:"deuterium"`
}

type MerchantResourceRow struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Offered     bool    `json:"offered"`
	Value       int     `json:"value"`
	FreeStorage int     `json:"freeStorage"`
	Rate        float64 `json:"rate"`
}

type MerchantStatus struct {
	PlayerID      int                   `json:"playerId"`
	Planet        Planet                `json:"planet"`
	Commander     string                `json:"commander"`
	User          MerchantUser          `json:"user"`
	ActiveOfferID int                   `json:"activeOfferId"`
	Rates         MerchantRates         `json:"rates"`
	Rows          []MerchantResourceRow `json:"rows"`
}

type JumpGateStatusCommand struct {
	PlanetID int
}

type JumpGateMoon struct {
	ID          int         `json:"id"`
	OwnerID     int         `json:"ownerId"`
	Name        string      `json:"name"`
	Type        int         `json:"type"`
	TypeName    string      `json:"typeName"`
	Coordinates Coordinates `json:"coordinates"`
	GateLevel   int         `json:"gateLevel"`
	GateUntil   int64       `json:"gateUntil,omitempty"`
}

type JumpGateStatus struct {
	PlayerID  int            `json:"playerId"`
	Planet    Planet         `json:"planet"`
	Commander string         `json:"commander"`
	Source    JumpGateMoon   `json:"source"`
	Targets   []JumpGateMoon `json:"targets"`
	Ships     []FleetShip    `json:"ships"`
	Issue     *ActionIssue   `json:"issue,omitempty"`
}

type EmpireCommand struct {
	PlanetID   int
	PlanetType int
}

type EmpireBuildQueueEntry struct {
	ListID   int  `json:"listId"`
	Level    int  `json:"level"`
	Active   bool `json:"active"`
	Demolish bool `json:"demolish"`
}

type EmpirePlanet struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	Type        int              `json:"type"`
	TypeName    string           `json:"typeName"`
	Coordinates Coordinates      `json:"coordinates"`
	Fields      int              `json:"fields"`
	MaxFields   int              `json:"maxFields"`
	Resources   EmpireResources  `json:"resources"`
	Production  EmpireProduction `json:"production"`
}

type EmpireResources struct {
	Metal     int `json:"metal"`
	Crystal   int `json:"crystal"`
	Deuterium int `json:"deuterium"`
}

type EmpireProduction struct {
	MetalHourly     int `json:"metalHourly"`
	CrystalHourly   int `json:"crystalHourly"`
	DeuteriumHourly int `json:"deuteriumHourly"`
	EnergyBalance   int `json:"energyBalance"`
	EnergyCapacity  int `json:"energyCapacity"`
}

type EmpireResourceValue struct {
	PlanetID   int `json:"planetId"`
	Amount     int `json:"amount"`
	Production int `json:"production"`
}

type EmpireResourceRow struct {
	ID         int                   `json:"id"`
	Name       string                `json:"name"`
	Values     []EmpireResourceValue `json:"values"`
	Total      int                   `json:"total"`
	Production int                   `json:"production"`
}

type EmpireLevelValue struct {
	PlanetID int                     `json:"planetId"`
	Level    int                     `json:"level"`
	CanBuild bool                    `json:"canBuild"`
	Queue    []EmpireBuildQueueEntry `json:"queue,omitempty"`
}

type EmpireLevelRow struct {
	ID      int                `json:"id"`
	Name    string             `json:"name"`
	Values  []EmpireLevelValue `json:"values"`
	Total   int                `json:"total"`
	Average float64            `json:"average"`
}

type EmpireCountValue struct {
	PlanetID int `json:"planetId"`
	Count    int `json:"count"`
}

type EmpireCountRow struct {
	ID     int                `json:"id"`
	Name   string             `json:"name"`
	Values []EmpireCountValue `json:"values"`
	Total  int                `json:"total"`
}

type EmpireOverview struct {
	PlayerID        int                 `json:"playerId"`
	PlanetID        int                 `json:"planetId"`
	CommanderActive bool                `json:"commanderActive"`
	PlanetType      int                 `json:"planetType"`
	MoonEnabled     bool                `json:"moonEnabled"`
	HasMoons        bool                `json:"hasMoons"`
	Issue           *ActionIssue        `json:"issue,omitempty"`
	Planets         []EmpirePlanet      `json:"planets"`
	Resources       []EmpireResourceRow `json:"resources"`
	Buildings       []EmpireLevelRow    `json:"buildings"`
	Research        []EmpireLevelRow    `json:"research"`
	Fleet           []EmpireCountRow    `json:"fleet"`
	Defense         []EmpireCountRow    `json:"defense"`
}

type TechnologyCommand struct {
	PlanetID  int
	DetailsID int
	InfoID    int
}

type TechnologyRequirement struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Level        int    `json:"level"`
	CurrentLevel int    `json:"currentLevel"`
	Met          bool   `json:"met"`
}

type TechnologyItem struct {
	ID               int                     `json:"id"`
	Name             string                  `json:"name"`
	Requirements     []TechnologyRequirement `json:"requirements"`
	DetailsAvailable bool                    `json:"detailsAvailable"`
}

type TechnologyGroup struct {
	Key   string           `json:"key"`
	Name  string           `json:"name"`
	Items []TechnologyItem `json:"items"`
}

type TechnologyDetailsLevel struct {
	Step         int                     `json:"step"`
	Requirements []TechnologyRequirement `json:"requirements"`
}

type TechnologyCost struct {
	Metal     float64 `json:"metal"`
	Crystal   float64 `json:"crystal"`
	Deuterium float64 `json:"deuterium"`
	Energy    float64 `json:"energy"`
}

type TechnologyDemolish struct {
	Level           int            `json:"level"`
	Cost            TechnologyCost `json:"cost"`
	DurationSeconds int            `json:"durationSeconds"`
}

type TechnologyDetails struct {
	Target   TechnologyItem           `json:"target"`
	Levels   []TechnologyDetailsLevel `json:"levels"`
	Demolish *TechnologyDemolish      `json:"demolish,omitempty"`
}

type TechnologyInfoRow struct {
	Level                int  `json:"level"`
	Current              bool `json:"current"`
	Production           int  `json:"production"`
	ProductionDifference int  `json:"productionDifference"`
	Energy               int  `json:"energy"`
	EnergyDifference     int  `json:"energyDifference"`
	Storage              int  `json:"storage"`
	StorageDifference    int  `json:"storageDifference"`
	DeuteriumConsumption int  `json:"deuteriumConsumption"`
	DeuteriumDifference  int  `json:"deuteriumDifference"`
}

type TechnologyInfo struct {
	ID          int                 `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Level       int                 `json:"level"`
	Kind        string              `json:"kind"`
	Rows        []TechnologyInfoRow `json:"rows"`
	Demolish    *TechnologyDemolish `json:"demolish,omitempty"`
}

type TechnologyTree struct {
	PlayerID int                `json:"playerId"`
	PlanetID int                `json:"planetId"`
	Groups   []TechnologyGroup  `json:"groups"`
	Details  *TechnologyDetails `json:"details,omitempty"`
	Info     *TechnologyInfo    `json:"info,omitempty"`
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

type FleetSlots struct {
	Used    int  `json:"used"`
	Max     int  `json:"max"`
	BaseMax int  `json:"baseMax"`
	Admiral bool `json:"admiral"`
}

type ExpeditionSlots struct {
	Used int `json:"used"`
	Max  int `json:"max"`
}

type FleetShipOption struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Speed       int    `json:"speed"`
	Cargo       int    `json:"cargo"`
	Consumption int    `json:"consumption"`
	Selectable  bool   `json:"selectable"`
}

type FleetTemplate struct {
	ID        int         `json:"id"`
	Name      string      `json:"name"`
	UpdatedAt int64       `json:"updatedAt"`
	Ships     []FleetShip `json:"ships"`
}

type FleetMissionOption struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
	Warning  string `json:"warning,omitempty"`
}

type FleetResourceLoad struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Available int    `json:"available"`
	Requested int    `json:"requested"`
	Loaded    int    `json:"loaded"`
}

type FleetDispatchDraft struct {
	Ships           []FleetShip          `json:"ships"`
	TotalShips      int                  `json:"totalShips"`
	Target          Coordinates          `json:"target"`
	TargetType      int                  `json:"targetType"`
	Mission         int                  `json:"mission"`
	Speed           int                  `json:"speed"`
	UnionID         int                  `json:"unionId"`
	Cargo           int                  `json:"cargo"`
	Distance        int                  `json:"distance"`
	DurationSeconds int                  `json:"durationSeconds"`
	MaxSpeed        int                  `json:"maxSpeed"`
	FuelConsumption int                  `json:"fuelConsumption"`
	SpeedFactor     int                  `json:"speedFactor"`
	RemainingCargo  int                  `json:"remainingCargo"`
	Ready           bool                 `json:"ready"`
	HasSelection    bool                 `json:"hasSelection"`
	MissionOptions  []FleetMissionOption `json:"missionOptions"`
	Resources       []FleetResourceLoad  `json:"resources"`
	HoldHours       []int                `json:"holdHours"`
	ExpeditionHours []int                `json:"expeditionHours"`
}

type FleetOptions struct {
	PlayerID        int                 `json:"playerId"`
	Planet          Planet              `json:"planet"`
	CommanderActive bool                `json:"commanderActive"`
	Slots           FleetSlots          `json:"slots"`
	Expeditions     ExpeditionSlots     `json:"expeditions"`
	ExpeditionLevel int                 `json:"expeditionLevel"`
	SpeedFactor     int                 `json:"speedFactor"`
	Missions        []FleetMovement     `json:"missions"`
	Ships           []FleetShipOption   `json:"ships"`
	TemplateLimit   int                 `json:"templateLimit"`
	Templates       []FleetTemplate     `json:"templates"`
	DispatchDraft   *FleetDispatchDraft `json:"dispatchDraft,omitempty"`
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
