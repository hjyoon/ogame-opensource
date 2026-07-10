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
- Static bearer token verifier for early scoped testing.
- Protected read-only tool: `get_mcp_access`.
- User-owned DB-backed bearer tokens:
  - `GET /api/game/mcp-tokens?session=...`
  - `POST /api/game/mcp-tokens?session=...`
  - `POST /api/game/mcp-tokens/revoke?session=...`
  - Uses the same game public session plus private session cookie as other
    authenticated `/api/game/*` endpoints.
  - Stores only SHA-256 token hashes in `uni*_mcp_tokens`.
- React options page UI for listing, creating, showing one-time secrets, and
  revoking DB-backed MCP tokens.
- Dedicated Go MCP smoke E2E:
  - `testing/e2e/golang-mcp-smoke.mjs`
  - Runs from `testing/e2e/run-golang-migration-qa.sh`.
  - Covers transport guards, public tools, DB token create/list/revoke, bearer
    protected read tools, invalid params, and revoked-token rejection.
- JSON audit logging for every `tools/call` request path. Audit logs include
  tool name, authorization outcome, player id/scopes when authenticated,
  duration, and error text. Bearer token secrets are never logged.
- Authenticated read tools requiring `mcp:read`:
  - `list_planets`: selectable planets/moons using legacy switcher ordering.
  - `get_account_overview`: commander, score/rank, current planet, planet
    count, and unread messages without overview mutations.
  - `get_planet_resources`: optional `planetId`; resources, dark matter,
    storage capacity, energy, and hourly production.
  - `get_building_queue`: optional `planetId`; queued building/demolition rows
    without finishing due queues or mutating resources.
  - `get_fleet_movements`: overview-style outgoing, incoming, return, hold,
    missile, and ACS grouped events without queue mutation.

## Static Token Format

`OGAME_MCP_STATIC_TOKENS` accepts semicolon-separated records:

```text
token:player_id:scope1,scope2;another-token:7:mcp:read
```

Current scopes:

- `mcp:read`
- `mcp:write`
- `mcp:fleet`
- `mcp:messages`
- `mcp:admin`

Static tokens are only a bootstrap path. Revocation currently means removing
the token from config and restarting. Prefer DB-backed user tokens for normal
users because they can be revoked without restart.

## User Token API

Create body:

```json
{"name":"Claude Desktop","scopes":["mcp:read"]}
```

The plaintext `secret` is returned only in the create response. Persist it in
the MCP client and treat it like a password.

User tokens currently allow:

- `mcp:read`
- `mcp:messages`
- `mcp:fleet`

`mcp:write` and `mcp:admin` stay unavailable for self-service user tokens.

## Security Rule

Do not expose account or game mutation tools to general users until token/OAuth
authorization and scoped consent are implemented. Mutating tools must use
dry-run plus explicit confirmation.

## Next Steps

1. Add OAuth 2.1/OIDC consent flow before public user rollout.

## General User Policy

General users may receive read-only tools first. Fleet launch, build, research,
message send, admin, bot, debug, and DB actions stay blocked until scoped
authorization, rate limits, audit logs, and confirmation flow are in place.
