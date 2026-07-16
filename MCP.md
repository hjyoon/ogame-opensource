# MCP Server

Updated: 2026-07-16. Keep this file under 4KB.

The Go backend exposes MCP to ordinary authenticated players. MCP follows the same Clean Architecture boundaries as game HTTP APIs and does not expose Admin, Bot, debug, database, or arbitrary game mutation access.

## Transport

- `POST /mcp`: Streamable HTTP JSON-RPC for `initialize`, `ping`, `tools/list`, and `tools/call`.
- `GET /mcp`: `405`; SSE is not implemented.
- Protocol: `2025-06-18` with negotiated fallback.
- Browser origins, bearer challenges, request rate limits, and JSON tool-call audit logs are enforced.

## OAuth 2.1

- Discovery: `/.well-known/oauth-authorization-server`, `/.well-known/oauth-protected-resource`, `/.well-known/jwks.json`.
- Authorization code + PKCE S256: `/oauth/authorize`, `/oauth/token`.
- Dynamic client registration and revocation: `/oauth/register`, `/oauth/revoke`.
- Consent binds client, redirect URI, MCP resource, scopes, state, and PKCE. Codes are one-time hashes.
- Optional OIDC `openid profile` uses Ed25519 `id_token` signing and key rotation seeds.

## Player Tokens

Authenticated players manage hashed, expiring bearer tokens through:

- `GET/POST /api/game/mcp-tokens?session=...`
- `POST /api/game/mcp-tokens/revoke?session=...`

The Options UI lists, creates, and revokes tokens. Plaintext secrets are returned once. Default lifetime is 30 days via `OGAME_MCP_TOKEN_TTL_SECONDS`; `0` disables expiry.

Static test/service tokens use `OGAME_MCP_STATIC_TOKENS`:

```text
token:player_id:scope1,scope2;next:7:mcp:read
```

## Scopes And Tools

The fully wired server exposes up to 58 tools: one public health tool, 26 authenticated read tools, three message readers, and 28 confirmed mutation tools. Coupon redemption is present only when both universe and master databases are available.

Self-service scopes are `mcp:read`, `mcp:messages`, `mcp:message_write`, `mcp:notes_write`, `mcp:buddy_write`, `mcp:fleet`, `mcp:fleet_write`, `mcp:queue_write`, `mcp:resources_write`, `mcp:premium_write`, `mcp:merchant_write`, `mcp:planet_write`, `mcp:alliance_write`, `mcp:account_write`, and `mcp:payment_write`.

Read tools cover access, planets, overview, resources, queues, fleets, officers, search, galaxy, statistics, alliance, buddy, pranger, notes, options, maintenance, merchant, Jump Gate, empire, technology, buildings, research, shipyard, and defense.

Mutation tools also cover building construction/demolition, research start, planet rename/abandon, Commander fleet templates and cross-planet queues, all player alliance mutations, account settings/identity/vacation/deletion, interplanetary missiles, Galaxy spy/recycle quick actions, and coupon redemption. They call the same Go repositories and transactions as browser actions; game rules are not reimplemented in MCP.

Mutations default to dry-run and require the returned confirmation token. Commander-only actions report `commander_required`; account results exclude password hashes and validation secrets. `mcp:write` and `mcp:admin` are reserved and unavailable to self-service tokens.

## Verification

The full migration QA runs MCP transport, token, OAuth/OIDC, scope, mutation, expiry, revocation, error, rate-limit, and audit checks. Options visual parity intentionally excludes the Go-only MCP token table while API behavior is tested separately.

Endpoint inventory: [backend/API_ENDPOINTS.md](./backend/API_ENDPOINTS.md).
