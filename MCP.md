# MCP Server Plan

Keep under 4KB; split details if needed.

## Current State

Implemented:

- Streamable HTTP JSON-RPC entrypoint.
- `POST /mcp` for JSON-RPC requests.
- `GET /mcp` returns `405`; SSE stream is not implemented yet.
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
  - Stores only SHA-256 hashes in `uni*_mcp_tokens`.
  - Tokens expire after `OGAME_MCP_TOKEN_TTL_SECONDS` seconds; default 30 days,
    `0` disables expiry.
- React options UI for DB token list/create/one-time secret/revoke.
- Go MCP smoke E2E: `testing/e2e/golang-mcp-smoke.mjs`, run by
  `testing/e2e/run-golang-migration-qa.sh`; covers transport guards, DB tokens,
  OAuth, read/message tools, dry-run mutations, invalid params, expiry, revoke.
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
- Consent CSRF+UX shows resource, redirect, scopes, deny redirect.
- MCP/OAuth rate limits on RPC, OAuth, and token API paths.
- JSON audit logging for every `tools/call`; no secrets are logged.
- Authenticated `mcp:read` tools: `list_planets`, `get_account_overview`,
  `get_planet_resources`, `get_building_queue`, and `get_fleet_movements`.
  These are read-only and avoid legacy queue/resource mutations.
- Authenticated `mcp:messages` tools: `list_messages` and `get_message`.
  They read owned inbox rows without marking messages read or cleanup mutation.
- Authenticated `mcp:message_write` tools: `send_message`, `delete_messages`,
  `report_message`. All default to dry-run; execution requires `dryRun:false`
  plus the returned
  confirmation string.

## Static Token Format

`OGAME_MCP_STATIC_TOKENS`: `token:player_id:scope1,scope2;next:7:mcp:read`.

Scopes: `mcp:read`, `mcp:messages`, `mcp:message_write`, `mcp:fleet`,
`mcp:write`, `mcp:admin`.

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
- `mcp:message_write`
- `mcp:fleet`

`mcp:write` and `mcp:admin` stay unavailable for self-service user tokens.

## Security Rule

Do not expose broad account/game mutation tools to general users. Each mutation
needs its own narrow scope, token/OAuth consent, audit logging, rate limit,
dry-run, and explicit confirmation.

## Next Steps

1. Add more scoped dry-run mutation tools: fleet dispatch and queue actions.

## General User Policy

General users may receive read tools and individually scoped confirmed actions.
Fleet/build/research/admin/bot/debug/DB actions stay blocked until each has
scoped authorization, rate limits, audit logs, and confirmation flow.
