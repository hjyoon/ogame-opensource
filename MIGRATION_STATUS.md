# Migration Status

Last updated: 2026-06-17 KST, branch `hjyoon/golang`.

Living tracker for the React 19 + Bun 1.3 frontend and Go 1.25 native `net/http` backend migration. Keep this file under 4KB; split details into linked docs before exceeding that limit.

## Current State

- Clean Architecture baseline exists under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React production build from `frontend/dist` and logs JSON through `log/slog`.
- `compose.golang.yaml` runs the Go app; default local port is `8890`.
- `/api/healthz` reports tool targets and static/legacy asset readiness.
- Natural public routes and legacy `.php` aliases resolve through the React shell.
- `/home`, `/register`, `/about`, `/story`, `/screenshots`, `/rules`, `/legal`, `/universes`, and legacy aliases target their PHP public compositions.
- Legacy public images from `wwwroot/img` are copied into `frontend/dist/public-assets/img` and served without loopback absolute URLs.
- `/api/public/universes` exposes the universe catalog from the master DB with config fallback.
- `/api/public/registration/validate` performs draft validation and duplicate/capacity checks against the legacy universe DB.
- `/api/public/registration` creates a legacy-compatible unvalidated user, activation code, home planet, private/public session, and `/game/overview` redirect.
- `/api/public/login/validate` and `/api/public/login` validate credentials, create public/private sessions, update legacy session fields, set the private cookie, and return a natural `/game/overview` redirect.
- `/api/game/session` validates public session plus private cookie, including banned and IP checks.
- `/api/game/overview` returns a session-guarded read-only overview summary from legacy `users`, `planets`, and `uni`.
- `/game/overview` targets the legacy PHP game visual composition: skin, header/menu, overview table, planet image, and resource summary.
- Go migration QA smoke covers health, routes, assets, registration validation/creation, login, session lookup, overview lookup, and method guards.
- Playwright headless visual E2E compares legacy PHP and Go/React public pages by screenshot diff plus key box geometry.

## Latest Verified Implementation

- Milestone: public page visual parity is guarded by headless Playwright comparison.

## Verified QA

- `bun run build && bun run check && bun test`: passing.
- `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`: passing.
- `testing/e2e/run-playwright-visual-e2e.sh`: passing for public desktop/mobile pages.
- Go internal coverage gate: `98.1% >= 97%`.
- Go smoke JSON: `all_pass: true`, including registration-created overview access.

Full legacy PHP E2E was not run for this Go migration step. Keep legacy PHP behavior as the oracle until each migrated flow has focused unit tests and E2E coverage.

## Remaining Work

- Implement activation confirmation, welcome mail/message, IP log, cleanup timer, and rank recalculation side effects.
- Add logout, expiry, and deeper session security behavior.
- Public legacy visual baseline is complete for the current public route set.
- Expand authenticated React game routes beyond overview with legacy PHP screen composition.
- Port current planet switching and full overview actions from legacy DB.
- Port resource production, queues, buildings, research, shipyard, defense, fleet, reports, messages, galaxy, alliance, admin, maintenance, options, recovery, deletion, vacation, bans, and permissions.
- Convert legacy E2E cases into Go compatibility checks as each flow is migrated.
- Run full legacy E2E before declaring any game-flow migration equivalent.
