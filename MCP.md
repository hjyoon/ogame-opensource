# MCP Server Plan

Keep <4KB.

## Current State

Implemented:

- Streamable HTTP JSON-RPC entrypoint: `POST /mcp`.
- `GET /mcp` returns `405`; SSE stream is not implemented yet.
- `initialize`, `ping`, `tools/list`, and `tools/call`.
- Protocol guard for `2025-06-18` plus fallback.
- Browser `Origin` guard against DNS rebinding.
- Clean Architecture MCP packages: domain, application, HTTP delivery.
- First safe read-only tool: `get_server_health`.
- Static bearer verifier for scoped testing.
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
- Go MCP smoke E2E covers transport, tokens, OAuth, tools, mutations, invalid
  params, expiry, revoke.
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
- Consent CSRF+UX shows resource, redirect, scopes, deny.
- MCP/OAuth rate limits on RPC, OAuth, and token API paths.
- JSON audit logging for every `tools/call`; no secrets are logged.
- Authenticated `mcp:read`: planets, overview, resources, queue,
  fleet movements, officer status, search, galaxy, statistics, empire,
  technology, buildings. No legacy queue/resource mutations.
- Authenticated `mcp:messages` tools: `list_messages` and `get_message`.
  Read owned inbox rows without marking read or cleanup mutation.
- Authenticated `mcp:message_write` tools: `send_message`, `delete_messages`,
  `report_message`. All default to dry-run; execution requires `dryRun:false`
  plus returned confirmation string.
- Authenticated `mcp:fleet_write`: `validate_fleet_dispatch`,
  `dispatch_fleet`, `recall_fleet`; mutations require confirmation.
- Authenticated `mcp:queue_write`: `cancel_building_queue`,
  `cancel_research_queue`, `enqueue_shipyard_order`; confirmed mutations.
- Authenticated `mcp:resources_write`: `update_resource_production`.
- Authenticated `mcp:premium_write`: `recruit_officer`.

## Static Token Format

`OGAME_MCP_STATIC_TOKENS`: `token:player_id:scope1,scope2;next:7:mcp:read`.

Scopes: `mcp:read`, `mcp:messages`, `mcp:message_write`, `mcp:fleet`,
`mcp:fleet_write`, `mcp:queue_write`, `mcp:resources_write`,
`mcp:premium_write`, `mcp:write`, `mcp:admin`.

Static tokens are bootstrap-only; prefer DB-backed user tokens.

## User Token API

Create body:

```json
{"name":"Claude Desktop","scopes":["mcp:read"]}
```

Plaintext `secret` is returned once; store it as a password.

User tokens allow: `mcp:read`, `mcp:messages`, `mcp:message_write`,
`mcp:fleet`, `mcp:fleet_write`, `mcp:queue_write`, `mcp:resources_write`,
`mcp:premium_write`.

`mcp:write` and `mcp:admin` stay unavailable for self-service user tokens.

## Security Rule

No broad game mutation tools for users. Each mutation needs a narrow scope,
consent, audit log, rate limit, dry-run, and confirmation.

## Next Steps

1. Add more scoped actions after parity review.

## General User Policy

General users may receive read tools and individually scoped confirmed actions.
Build/research/admin/bot/debug/DB actions stay blocked until each has scoped
authorization, rate limits, audit logs, and confirmation flow.
