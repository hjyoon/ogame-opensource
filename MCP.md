# MCP Server Plan

Keep under 4KB; split details if needed.

## Current Step

The Go backend exposes a first MCP endpoint at `/mcp`.

Implemented:

- Streamable HTTP JSON-RPC entrypoint.
- `POST /mcp` for JSON-RPC requests.
- `GET /mcp` returns `405` because no server-sent event stream is implemented yet.
- `initialize`, `ping`, `tools/list`, and `tools/call`.
- Protocol guard for `2025-06-18` and legacy fallback `2025-03-26`.
- Browser `Origin` guard against DNS rebinding.
- Clean Architecture packages:
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
  - Uses game public session plus private session cookie.
  - Stores only SHA-256 token hashes in `uni*_mcp_tokens`.
- React options UI for DB token list/create/one-time secret/revoke.
- Dedicated Go MCP smoke E2E:
  - `testing/e2e/golang-mcp-smoke.mjs`
  - Runs from `testing/e2e/run-golang-migration-qa.sh`.
  - Covers transport guards, DB tokens, OAuth, read tools, invalid params, and
    revoke.
- OAuth 2.1 public-client base:
  - `/.well-known/oauth-authorization-server`
  - PRM endpoint and 401 discovery challenge
  - `/oauth/authorize` consent page and code redirect
  - `/oauth/register` DCR
  - `/oauth/token` code + PKCE S256; `id_token` for `openid`
  - `/oauth/revoke` access-token revocation
  - Strict OAuth `resource`; codes bind client, redirect, resource, PKCE, scopes
  - `/.well-known/jwks.json` Ed25519 JWKS
  - Codes are one-time hashes in `uni*_mcp_oauth_codes`.
  - External redirects require `OGAME_MCP_OAUTH_REDIRECT_URIS`.
  - OIDC seed envs: active `OGAME_MCP_OIDC_ED25519_SEED_B64`; previous
    `OGAME_MCP_OIDC_ED25519_PREVIOUS_SEEDS_B64`.
- MCP/OAuth rate limits on RPC, OAuth, and token API paths.
- JSON audit logging for every `tools/call` request path. Audit logs include
  tool, auth result, player/scopes, duration, and errors. Secrets are not
  logged.
- Authenticated read tools requiring `mcp:read`:
  - `list_planets`: selectable planets/moons using legacy switcher ordering.
  - `get_account_overview`: commander, score/rank, current planet, planet
    count, and unread messages without overview mutations.
  - `get_planet_resources`: optional `planetId`; resources, dark matter,
    storage capacity, energy, and hourly production.
  - `get_building_queue`: optional `planetId`; queued building/demolition rows
    without finishing queues or mutating resources.
  - `get_fleet_movements`: overview-style outgoing, incoming, return, hold,
    missile, and ACS grouped events without queue mutation.

## Static Token Format

`OGAME_MCP_STATIC_TOKENS`: `token:player_id:scope1,scope2;next:7:mcp:read`.

Scopes: `mcp:read`, `mcp:write`, `mcp:fleet`, `mcp:messages`, `mcp:admin`.

Static tokens are bootstrap-only. Prefer DB-backed user tokens because they can
be revoked without restart.

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

1. Tighten OAuth consent scope UX.

## General User Policy

General users may receive read-only tools first. Fleet/build/research/message,
admin, bot, debug, and DB actions stay blocked until scoped authorization,
rate limits, audit logs, and confirmation flow are in place.
