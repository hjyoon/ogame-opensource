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
	ErrToolNotFound = errors.New("mcp tool not found")
	ErrUnauthorized = errors.New("mcp unauthorized")
	ErrForbidden    = errors.New("mcp forbidden")
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

type ToolCallAudit struct {
	ToolName   string
	PlayerID   int
	Scopes     []string
	Authorized bool
	Error      string
	At         int64
	DurationMS int64
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
