# MCP Server

Updated: 2026-07-19. Keep this file under 4KB.

The Go backend exposes MCP to authenticated Players, Operators, and Admins. MCP follows the same Clean Architecture boundaries and reuses game/Admin permission rules.

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

## User Tokens

Authenticated users manage hashed, expiring bearer tokens through:

- `GET/POST /api/game/mcp-tokens?session=...`
- `POST /api/game/mcp-tokens/revoke?session=...`

The Options UI lists, creates, and revokes up to five active tokens per user. It supports select-all scopes and fixed expiry choices from one hour through one year, plus `Never`. Plaintext secrets are returned once. Omitted expiry uses `OGAME_MCP_TOKEN_TTL_SECONDS` (30 days by default).

Static test/service tokens use `OGAME_MCP_STATIC_TOKENS`; staff role is inferred from a privileged scope:

```text
token:player_id:scope1,scope2;next:7:mcp:read
```

## Scopes And Tools

The server exposes up to 61 tools. Coupons need both databases; staff tools need Admin.

Player scopes are `mcp:read`, `mcp:messages`, `mcp:message_write`, `mcp:notes_write`, `mcp:buddy_write`, `mcp:fleet`, `mcp:fleet_write`, `mcp:queue_write`, `mcp:resources_write`, `mcp:premium_write`, `mcp:merchant_write`, `mcp:planet_write`, `mcp:alliance_write`, `mcp:account_write`, and `mcp:payment_write`.

`mcp:operator` is available only at user type 1+. `mcp:admin` is available only at type 2. Current DB role is checked on issuance, OAuth exchange, tool listing, and every call, so demotion takes effect immediately. An Admin may issue an Operator-only token; its privilege ceiling remains Operator.

Read tools cover access, planets, overview, resources, queues, fleets, officers, search, statistics, alliance, buddy, pranger, notes, options, maintenance, merchant, Jump Gate, empire, technology, buildings, research, shipyard, and defense.

`get_galaxy_system` also needs `mcp:resources_write`; remote views spend 10 deuterium.

Mutation tools also cover building construction/demolition, research start, planet rename/abandon, Commander fleet templates and cross-planet queues, all player alliance mutations, account settings/identity/vacation/deletion, interplanetary missiles, Galaxy spy/recycle quick actions, and coupon redemption. They call the same Go repositories and transactions as browser actions; game rules are not reimplemented in MCP.

Staff tools are `get_admin_access`, `get_admin_panel`, and `mutate_admin_panel`. They cover the legacy Admin mode inventory, including dedicated Bot strategy editing. Operator mode/action limits reuse `AdminModeRequiresAdmin` and `AdminMutationRequiresAdmin`; Admin-only data and actions return `Forbidden` to Operator tokens.

Mutations use scoped bearer authorization instead of current account passwords, default to dry-run, and require the returned confirmation token. New passwords remain write values. Account results exclude password hashes and validation secrets. `mcp:write` remains reserved.

## Verification

The full migration QA runs MCP transport, token, OAuth/OIDC, scope, mutation, expiry, revocation, error, rate-limit, and audit checks. Options visual parity intentionally excludes the Go-only MCP token table while API behavior is tested separately.

Endpoint inventory: [backend/API_ENDPOINTS.md](./backend/API_ENDPOINTS.md).
