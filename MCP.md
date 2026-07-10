# MCP Server Plan

Keep this file under 4KB. Split implementation details if needed.

## Current Step

The Go backend exposes a first MCP endpoint at `/mcp`.

Implemented:

- Streamable HTTP-compatible JSON-RPC entrypoint.
- `POST /mcp` for JSON-RPC requests.
- `GET /mcp` returns `405` because no server-sent event stream is implemented yet.
- `initialize`, `ping`, `tools/list`, and `tools/call`.
- Protocol version guard for `2025-06-18` and legacy fallback `2025-03-26`.
- Browser `Origin` validation to reduce DNS rebinding risk.
- Clean Architecture split:
  - `internal/domain/mcp`
  - `internal/application/mcp`
  - `internal/delivery/http`
- First safe read-only tool: `get_server_health`.

## Security Rule

Do not expose account or game mutation tools to general users until token/OAuth
authorization and scoped consent are implemented. Mutating tools must use
dry-run plus explicit confirmation.

## Next Steps

1. Add user-owned MCP tokens with scopes and revoke support.
2. Add audit logging for every MCP tool call.
3. Add read-only authenticated tools:
   - `list_planets`
   - `get_account_overview`
   - `get_planet_resources`
   - `get_building_queue`
   - `get_fleet_movements`
4. Add E2E smoke calls against `/mcp`.
5. Add OAuth 2.1/OIDC consent flow before public user rollout.

## General User Policy

General users may receive read-only tools first. Fleet launch, build, research,
message send, admin, bot, debug, and DB actions stay blocked until scoped
authorization, rate limits, audit logs, and confirmation flow are in place.
