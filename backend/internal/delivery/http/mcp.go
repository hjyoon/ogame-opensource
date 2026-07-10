package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

const legacyMCPProtocolVersion = "2025-03-26"

type mcpUseCase interface {
	Initialize(context.Context) domainmcp.InitializeResult
	ListTools(context.Context, domainmcp.ListToolsCommand) (domainmcp.ListToolsResult, error)
	CallTool(context.Context, domainmcp.CallToolCommand) (domainmcp.ToolCallResult, error)
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string              `json:"jsonrpc"`
	ID      json.RawMessage     `json:"id"`
	Result  any                 `json:"result,omitempty"`
	Error   *jsonRPCErrorObject `json:"error,omitempty"`
}

type jsonRPCErrorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpListToolsParams struct {
	Cursor string `json:"cursor,omitempty"`
}

type mcpCallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func (a app) handleMCP(w http.ResponseWriter, r *http.Request) {
	if !validMCPOrigin(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPost:
		a.handleMCPPost(w, r)
	case http.MethodGet:
		w.Header().Set("Allow", "POST")
		http.Error(w, "mcp server-sent events stream unavailable", http.StatusMethodNotAllowed)
	default:
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) handleMCPPost(w http.ResponseWriter, r *http.Request) {
	if a.deps.MCP == nil {
		http.Error(w, "mcp unavailable", http.StatusServiceUnavailable)
		return
	}
	if !validMCPProtocolVersion(r.Header.Get("MCP-Protocol-Version")) {
		http.Error(w, "unsupported mcp protocol version", http.StatusBadRequest)
		return
	}

	var request jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONRPCError(w, nil, http.StatusBadRequest, -32700, "Parse error")
		return
	}
	if request.JSONRPC != "2.0" || request.Method == "" {
		writeJSONRPCError(w, requestID(request.ID), http.StatusBadRequest, -32600, "Invalid Request")
		return
	}
	if len(request.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch request.Method {
	case "initialize":
		writeJSONRPCResult(w, request.ID, a.deps.MCP.Initialize(r.Context()))
	case "ping":
		writeJSONRPCResult(w, request.ID, map[string]any{})
	case "tools/list":
		params, ok := decodeMCPParams[mcpListToolsParams](w, request)
		if !ok {
			return
		}
		result, err := a.deps.MCP.ListTools(r.Context(), domainmcp.ListToolsCommand{Cursor: params.Cursor})
		if err != nil {
			writeJSONRPCError(w, request.ID, http.StatusInternalServerError, -32603, "Internal error")
			return
		}
		writeJSONRPCResult(w, request.ID, result)
	case "tools/call":
		params, ok := decodeMCPParams[mcpCallToolParams](w, request)
		if !ok {
			return
		}
		if params.Arguments == nil {
			params.Arguments = map[string]any{}
		}
		result, err := a.deps.MCP.CallTool(r.Context(), domainmcp.CallToolCommand{
			Name:      params.Name,
			Arguments: params.Arguments,
		})
		if errors.Is(err, domainmcp.ErrToolNotFound) {
			writeJSONRPCError(w, request.ID, http.StatusOK, -32602, "Unknown tool")
			return
		}
		if err != nil {
			writeJSONRPCError(w, request.ID, http.StatusInternalServerError, -32603, "Internal error")
			return
		}
		writeJSONRPCResult(w, request.ID, result)
	default:
		writeJSONRPCError(w, request.ID, http.StatusOK, -32601, "Method not found")
	}
}

func decodeMCPParams[T any](w http.ResponseWriter, request jsonRPCRequest) (T, bool) {
	var params T
	if len(request.Params) == 0 {
		return params, true
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		writeJSONRPCError(w, request.ID, http.StatusOK, -32602, "Invalid params")
		return params, false
	}
	return params, true
}

func writeJSONRPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	writeJSONRPC(w, http.StatusOK, jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      requestID(id),
		Result:  result,
	})
}

func writeJSONRPCError(w http.ResponseWriter, id json.RawMessage, status int, code int, message string) {
	writeJSONRPC(w, status, jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      requestID(id),
		Error:   &jsonRPCErrorObject{Code: code, Message: message},
	})
}

func writeJSONRPC(w http.ResponseWriter, status int, response jsonRPCResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

func requestID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}

func validMCPProtocolVersion(version string) bool {
	return version == "" || version == domainmcp.ProtocolVersion || version == legacyMCPProtocolVersion
}

func validMCPOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return strings.EqualFold(parsed.Scheme, scheme) && strings.EqualFold(parsed.Host, r.Host)
}
