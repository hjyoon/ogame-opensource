package mcp

import "errors"

const ProtocolVersion = "2025-06-18"

const (
	ScopeRead     = "mcp:read"
	ScopeWrite    = "mcp:write"
	ScopeFleet    = "mcp:fleet"
	ScopeMessages = "mcp:messages"
	ScopeAdmin    = "mcp:admin"
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
