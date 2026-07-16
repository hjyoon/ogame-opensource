# Backend API Endpoints

Source of truth: `backend/internal/delivery/http/server.go`. Keep this file under 4KB; split details if request/response schemas grow.

## Conventions

- `GET` routes also allow `HEAD` unless noted.
- Game APIs use `session` query and/or private login cookies.
- Most game APIs accept `cp` to select the active planet or moon.
- Game JSON responses usually include `authenticated`, `issues`, and a page-specific payload.

## Public

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/livez` | Process liveness; independent of DB readiness |
| `GET` | `/api/healthz` | Asset, DB, and PHP-Mod compatibility readiness; `503` when unavailable |
| `GET` | `/api/public/universes` | Universe catalog |
| `POST` | `/api/public/registration/validate` | Registration draft validation |
| `POST` | `/api/public/registration` | Account registration |
| `POST` | `/api/public/password-recovery` | Password recovery |
| `POST` | `/api/public/login/validate` | Login draft validation |
| `POST` | `/api/public/login` | Login and cookie issue |

## Game Session

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/game/session` | Current game session |
| `POST` | `/api/game/logout` | Logout and cookie clear |

## MCP And OAuth

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/mcp` | MCP JSON-RPC; live Player/Operator/Admin bearer authorization |
| `GET` | `/.well-known/oauth-authorization-server`, `/.well-known/oauth-protected-resource`, `/.well-known/jwks.json` | OAuth/OIDC discovery and keys |
| `GET` | `/oauth/authorize` | Role-checked user consent and authorization code |
| `POST` | `/oauth/register`, `/oauth/token`, `/oauth/revoke` | DCR, PKCE token exchange, revocation |
| `GET/POST` | `/api/game/mcp-tokens` | List/create role-scoped user bearer tokens |
| `POST` | `/api/game/mcp-tokens/revoke` | Revoke a user-owned token |

Token creation accepts `name`, `scopes`, and `expiresInSeconds`. The list returns allowed expiry options and role scopes. At most five unexpired, unrevoked tokens may exist per user; `expiresInSeconds: 0` means no expiry.

Tools and scopes: [MCP.md](../MCP.md).

## Game Pages

| Method | Path | Purpose |
| --- | --- | --- |
| `GET/POST` | `/api/game/overview` | Overview, rename, planet delete |
| `GET/POST` | `/api/game/buildings` | Building queue and mutations |
| `GET` | `/api/game/empire` | Empire view; legacy `modus` shortcuts |
| `GET/POST` | `/api/game/resources` | Production settings |
| `GET/POST` | `/api/game/merchant` | Merchant trades |
| `GET/POST` | `/api/game/officers` | Officer recruitment |
| `GET/POST` | `/api/game/alliance` | Alliance views and mutations |
| `GET/POST` | `/api/game/admin` | Admin console by `mode`, audit/localization/bot add/stop/list |
| `GET/POST` | `/api/game/research` | Research queue and mutations |
| `GET/POST` | `/api/game/shipyard` | Ship build orders |
| `GET/POST` | `/api/game/defense` | Defense build orders |
| `GET/POST` | `/api/game/fleet` | Recall and dispatch flow |
| `GET/POST` | `/api/game/fleet-templates` | Fleet templates |
| `GET/POST` | `/api/game/galaxy` | Galaxy, missiles, instant spy/recycle |
| `GET` | `/api/game/technology` | Technology tree and details |
| `GET` | `/api/game/statistics` | Rankings: `who`, `type`, `start` |
| `GET` | `/api/game/search` | Player/alliance search |
| `GET/POST` | `/api/game/buddy` | Buddy list and requests |
| `GET/POST` | `/api/game/notes` | Notes CRUD |
| `GET/POST` | `/api/game/messages` | Messages, compose, delete/report |
| `GET` | `/api/game/report` | Report by `bericht` or `report` |
| `GET` | `/api/game/phalanx` | Phalanx by `spid`/`targetPlanetId` |
| `GET/POST` | `/api/game/jump-gate` | Jump Gate screen and ship transfer |
| `GET/POST` | `/api/game/options` | User/game options |
| `GET/POST` | `/api/game/payment` | Coupon payment |

## Compatibility

Legacy aliases and static routes are split into [API_COMPATIBILITY.md](./API_COMPATIBILITY.md).
