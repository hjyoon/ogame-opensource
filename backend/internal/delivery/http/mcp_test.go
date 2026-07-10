package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestMCPInitializeListsAndCallsTools(t *testing.T) {
	server := New(Dependencies{MCP: fakeMCPUseCase{}})

	initBody := mcpPost(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	initResult := initBody["result"].(map[string]any)
	if initResult["protocolVersion"] != domainmcp.ProtocolVersion {
		t.Fatalf("unexpected initialize response: %+v", initBody)
	}
	if _, ok := initResult["capabilities"].(map[string]any)["tools"]; !ok {
		t.Fatalf("expected tools capability: %+v", initBody)
	}

	listBody := mcpPost(t, server, `{"jsonrpc":"2.0","id":"tools","method":"tools/list"}`)
	tools := listBody["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "get_server_health" {
		t.Fatalf("unexpected tools list: %+v", listBody)
	}

	callBody := mcpPost(t, server, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_server_health","arguments":{}}}`)
	callResult := callBody["result"].(map[string]any)
	if callResult["isError"] != false || callResult["structuredContent"].(map[string]any)["status"] != "ok" {
		t.Fatalf("unexpected tool call response: %+v", callBody)
	}
}

func TestMCPPassesBearerTokenToUseCase(t *testing.T) {
	mcp := &recordingMCPUseCase{fakeMCPUseCase: fakeMCPUseCase{}}
	server := New(Dependencies{MCP: mcp})

	req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_server_health"}}`))
	req.Header.Set("Authorization", "Bearer scoped-token")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected tool call success, got status=%d body=%q", rec.Code, rec.Body.String())
	}
	if mcp.callCommand.AccessToken != "scoped-token" {
		t.Fatalf("expected bearer token to be passed to MCP usecase, got %+v", mcp.callCommand)
	}

	req = httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer scoped-token")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || mcp.listCommand.AccessToken != "scoped-token" {
		t.Fatalf("expected bearer token to be passed to tools/list, status=%d body=%q command=%+v", rec.Code, rec.Body.String(), mcp.listCommand)
	}
}

func TestMCPAcceptsNotificationsWithoutJSONRPCResponse(t *testing.T) {
	server := New(Dependencies{MCP: fakeMCPUseCase{}})
	req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Fatalf("expected 202 empty notification response, got status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMCPTransportGuards(t *testing.T) {
	server := New(Dependencies{MCP: fakeMCPUseCase{}})

	t.Run("same origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("Origin", "http://game.local")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected same-origin ping to pass, got status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("forwarded https origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("Origin", "https://game.local")
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected forwarded HTTPS origin to pass, got status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("get stream unavailable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://game.local/mcp", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
			t.Fatalf("unexpected GET response status=%d allow=%q body=%q", rec.Code, rec.Header().Get("Allow"), rec.Body.String())
		}
	})

	t.Run("unsupported method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "http://game.local/mcp", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
			t.Fatalf("unexpected DELETE response status=%d allow=%q body=%q", rec.Code, rec.Header().Get("Allow"), rec.Body.String())
		}
	})

	t.Run("origin mismatch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("Origin", "http://evil.local")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected forbidden origin, got status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("Origin", ":")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected invalid origin to be forbidden, got status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("unsupported protocol version", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("MCP-Protocol-Version", "2024-01-01")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected bad protocol version, got status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("unavailable usecase", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		rec := httptest.NewRecorder()
		New(Dependencies{}).ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected unavailable MCP response, got status=%d body=%q", rec.Code, rec.Body.String())
		}
	})
}

func TestMCPJSONRPCErrors(t *testing.T) {
	server := New(Dependencies{MCP: fakeMCPUseCase{}})

	for _, tt := range []struct {
		name     string
		body     string
		wantCode int
		wantErr  float64
	}{
		{name: "parse error", body: `{`, wantCode: http.StatusBadRequest, wantErr: -32700},
		{name: "invalid request", body: `{"jsonrpc":"1.0","id":1,"method":"ping"}`, wantCode: http.StatusBadRequest, wantErr: -32600},
		{name: "unknown method", body: `{"jsonrpc":"2.0","id":1,"method":"missing/method"}`, wantCode: http.StatusOK, wantErr: -32601},
		{name: "unknown tool", body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"missing"}}`, wantCode: http.StatusOK, wantErr: -32602},
		{name: "invalid params", body: `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":"bad"}`, wantCode: http.StatusOK, wantErr: -32602},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Fatalf("unexpected status=%d body=%q", rec.Code, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON-RPC error: %v", err)
			}
			if body["error"].(map[string]any)["code"] != tt.wantErr {
				t.Fatalf("unexpected error body: %+v", body)
			}
		})
	}
}

func TestMCPUseCaseErrorsReturnJSONRPCInternalError(t *testing.T) {
	for _, tt := range []struct {
		name string
		mcp  fakeMCPUseCase
		body string
	}{
		{name: "list tools", mcp: fakeMCPUseCase{listErr: errors.New("list down")}, body: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`},
		{name: "call tool", mcp: fakeMCPUseCase{callErr: errors.New("call down")}, body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_server_health"}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := New(Dependencies{MCP: tt.mcp})
			req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("expected internal error status, got status=%d body=%q", rec.Code, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON-RPC error: %v", err)
			}
			if body["error"].(map[string]any)["code"] != float64(-32603) {
				t.Fatalf("unexpected error body: %+v", body)
			}
		})
	}
}

func TestMCPAccessErrorsUseHTTPAuthStatus(t *testing.T) {
	for _, tt := range []struct {
		name       string
		mcp        fakeMCPUseCase
		wantStatus int
		wantCode   float64
		wantHeader bool
	}{
		{name: "unauthorized", mcp: fakeMCPUseCase{callErr: domainmcp.ErrUnauthorized}, wantStatus: http.StatusUnauthorized, wantCode: -32001, wantHeader: true},
		{name: "forbidden", mcp: fakeMCPUseCase{callErr: domainmcp.ErrForbidden}, wantStatus: http.StatusForbidden, wantCode: -32003},
		{name: "invalid params", mcp: fakeMCPUseCase{callErr: domainmcp.ErrInvalidParams}, wantStatus: http.StatusOK, wantCode: -32602},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := New(Dependencies{MCP: tt.mcp})
			req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_server_health"}}`))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("unexpected status=%d body=%q", rec.Code, rec.Body.String())
			}
			if tt.wantHeader && rec.Header().Get("WWW-Authenticate") != `Bearer realm="ogame-mcp"` {
				t.Fatalf("missing WWW-Authenticate header: %v", rec.Header())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON-RPC error: %v", err)
			}
			if body["error"].(map[string]any)["code"] != tt.wantCode {
				t.Fatalf("unexpected error body: %+v", body)
			}
		})
	}
}

func mcpPost(t *testing.T, server http.Handler, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://game.local/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", domainmcp.ProtocolVersion)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got status=%d body=%q", rec.Code, rec.Body.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	return decoded
}

type fakeMCPUseCase struct {
	listErr error
	callErr error
}

func (fakeMCPUseCase) Initialize(context.Context) domainmcp.InitializeResult {
	return domainmcp.InitializeResult{
		ProtocolVersion: domainmcp.ProtocolVersion,
		Capabilities:    domainmcp.Capabilities{Tools: &domainmcp.ToolsCapability{}},
		ServerInfo:      domainmcp.ServerInfo{Name: "ogame-opensource", Version: "test"},
	}
}

type recordingMCPUseCase struct {
	fakeMCPUseCase
	listCommand domainmcp.ListToolsCommand
	callCommand domainmcp.CallToolCommand
}

func (r *recordingMCPUseCase) ListTools(ctx context.Context, command domainmcp.ListToolsCommand) (domainmcp.ListToolsResult, error) {
	r.listCommand = command
	return r.fakeMCPUseCase.ListTools(ctx, command)
}

func (r *recordingMCPUseCase) CallTool(ctx context.Context, command domainmcp.CallToolCommand) (domainmcp.ToolCallResult, error) {
	r.callCommand = command
	return r.fakeMCPUseCase.CallTool(ctx, command)
}

func (f fakeMCPUseCase) ListTools(context.Context, domainmcp.ListToolsCommand) (domainmcp.ListToolsResult, error) {
	if f.listErr != nil {
		return domainmcp.ListToolsResult{}, f.listErr
	}
	return domainmcp.ListToolsResult{Tools: []domainmcp.Tool{{Name: "get_server_health", Description: "health", InputSchema: map[string]any{"type": "object"}}}}, nil
}

func (f fakeMCPUseCase) CallTool(_ context.Context, command domainmcp.CallToolCommand) (domainmcp.ToolCallResult, error) {
	if f.callErr != nil {
		return domainmcp.ToolCallResult{}, f.callErr
	}
	if command.Name != "get_server_health" {
		return domainmcp.ToolCallResult{}, domainmcp.ErrToolNotFound
	}
	if command.Arguments == nil {
		return domainmcp.ToolCallResult{}, errors.New("arguments should be normalized")
	}
	return domainmcp.ToolCallResult{
		Content:           []domainmcp.Content{{Type: "text", Text: `{"status":"ok"}`}},
		StructuredContent: map[string]any{"status": "ok"},
		IsError:           false,
	}, nil
}
