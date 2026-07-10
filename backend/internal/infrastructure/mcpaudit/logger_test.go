package mcpaudit

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestSlogLoggerRecordsMCPToolCall(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&out, nil))

	NewSlogLogger(logger).RecordMCPToolCall(context.Background(), domainmcp.ToolCallAudit{
		ToolName:   "get_mcp_access",
		PlayerID:   42,
		Scopes:     []string{domainmcp.ScopeRead},
		Authorized: true,
		At:         1700000000,
		DurationMS: 12,
	})

	logged := out.String()
	for _, want := range []string{`"event":"mcp_tool_call"`, `"tool":"get_mcp_access"`, `"player_id":42`, `"authorized":true`, `"duration_ms":12`} {
		if !strings.Contains(logged, want) {
			t.Fatalf("expected log to contain %s, got %s", want, logged)
		}
	}
}

func TestSlogLoggerAllowsNilLogger(t *testing.T) {
	NewSlogLogger(nil).RecordMCPToolCall(context.Background(), domainmcp.ToolCallAudit{ToolName: "get_server_health"})
}
