# Migration Status

Last updated: 2026-06-17 KST, branch `hjyoon/golang`.

Living tracker for the React 19 + Bun 1.3 frontend and Go 1.25 native `net/http` backend migration. Update it whenever a step is completed, deferred, or re-scoped.

## Current State

- Clean Architecture baseline exists under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React production build from `frontend/dist`.
- Runtime logs use JSON through `log/slog`.
- Go app runs through `compose.golang.yaml`; default local port is `8890`.
- `/api/healthz` reports tool targets and asset readiness.
- Public routes exist for natural paths and legacy `.php` aliases; visual parity with PHP public screens is still pending.
- Legacy image/assets are served through Go without loopback absolute URLs.
- Public universe catalog is exposed at `/api/public/universes`, backed by the master DB with config fallback.
- Registration draft validation exists at `/api/public/registration/validate`.
- Registration availability checks legacy universe DB state for duplicate names, duplicate email, and max-user capacity.
- Native account creation/activation is not migrated yet.
- Login draft validation exists at `/api/public/login/validate`.
- Login credential validation checks legacy `users.name/password/banned` in the universe DB.
- `/api/public/login` creates public/private sessions, updates legacy `users` session fields, sets the private cookie, and returns a natural `/game/overview` redirect target.
- `/api/game/session` validates public session plus private cookie, including banned and IP checks.
- `/api/game/overview` returns a session-guarded read-only overview summary from legacy `users`, `planets`, and `uni`.
- `/game/overview` now targets the legacy PHP visual composition: skin, header/menu, overview table, planet image, and resource summary.
- Go migration QA smoke covers health, routes, assets, registration, login, session lookup, overview lookup, and method guards.

## Latest Verified Commit

- `cd2eef8c` Add native overview read API.
- Current milestone: legacy-visual overview pass on top of the native overview API.

## Verified QA

- `backend/scripts/test-coverage.sh`: passing, Go internal coverage `97.3%`.
- `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`: passing.
- Direct smoke after restart: `GET /api/healthz`, `POST /api/public/login`, `GET /api/game/session`, `GET /api/game/overview`, and JSON DB enablement logs passed.

Full legacy E2E was not run for the latest Go migration steps unless explicitly noted in a later update.

## Remaining Work

- Implement native registration creation, activation, and login-after-register behavior.
- Add logout, expiry, and session security behavior.
- Expand authenticated React game routes beyond the current overview summary using legacy PHP screen composition.
- Port current planet switching and full overview state/actions from legacy DB.
- Restyle all migrated public/game/admin pages to match their PHP screen composition.
- Port resource production/read model, queues, buildings, research, shipyard, defense, fleet, reports, messages, galaxy, alliance, admin, maintenance, options, password recovery, deletion, vacation, bans, and permissions.
- Convert legacy E2E cases into Go compatibility checks as each flow is migrated.
- Keep legacy PHP behavior as the oracle until each migrated flow has focused unit tests and E2E coverage.
- Run full legacy E2E before declaring any game-flow migration equivalent.

## Update Rules

- Update this file in the same change set as each migration milestone.
- Keep this file under 4KB. If detail grows, split by topic and link the child docs here.
- Record the latest commit, QA commands, coverage percentage, and any skipped validation.
- Do not mark a flow migrated until the React/Go implementation is tested against the legacy behavior.
