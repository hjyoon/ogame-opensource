package mcpaudit

import (
	"context"
	"log/slog"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type SlogLogger struct {
	logger *slog.Logger
}

func NewSlogLogger(logger *slog.Logger) SlogLogger {
	return SlogLogger{logger: logger}
}

func (l SlogLogger) RecordMCPToolCall(ctx context.Context, event domainmcp.ToolCallAudit) {
	if l.logger == nil {
		return
	}
	attrs := []any{
		"event", "mcp_tool_call",
		"tool", event.ToolName,
		"authorized", event.Authorized,
		"duration_ms", event.DurationMS,
		"at", event.At,
	}
	if event.PlayerID > 0 {
		attrs = append(attrs, "player_id", event.PlayerID)
	}
	if len(event.Scopes) > 0 {
		attrs = append(attrs, "scopes", event.Scopes)
	}
	if event.Error != "" {
		attrs = append(attrs, "error", event.Error)
	}
	l.logger.InfoContext(ctx, "mcp tool call", attrs...)
}
