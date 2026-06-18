# Migration Status

Last updated: 2026-06-18 KST, branch `hjyoon/golang`.

Living tracker for the React 19 + Bun 1.3 frontend and Go 1.25 native `net/http` backend migration. Keep this file under 4KB; split details into linked docs before exceeding that limit.

## Current State

- Clean Architecture baseline exists under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React production build from `frontend/dist` and logs JSON through `log/slog`.
- `compose.golang.yaml` runs the Go app; default local port is `8890`.
- `/api/healthz` reports tool targets and static/legacy asset readiness.
- Natural public routes and legacy `.php` aliases resolve through the React shell.
- Public aliases, early CSS bootstrap, Go smoke routes, and public visual specs derive from one route manifest.
- `/home`, `/register`, `/about`, `/story`, `/screenshots`, `/rules`, `/legal`, `/universes`, and legacy aliases target their PHP public compositions.
- Legacy public, evolution skin, and game images are copied into `frontend/dist/public-assets`.
- `/api/public/universes` exposes the universe catalog from the master DB with config fallback.
- `/api/public/registration/validate` performs draft validation and duplicate/capacity checks against the legacy universe DB.
- `/api/public/registration` creates a legacy-compatible unvalidated user, activation code, home planet, private/public session, and `/game/overview` redirect.
- `/api/public/login/validate` and `/api/public/login` validate credentials, create public/private sessions, update legacy session fields, set the private cookie, and return a natural `/game/overview` redirect.
- `/api/game/session` validates public session plus private cookie, including banned and IP checks.
- `/api/game/{overview,resources}` return session-guarded planet/resource data from legacy DB.
- Authenticated `/game/*` routes preserve sessions; overview/buildings/resources use the legacy `evolution` skin.
- Go migration QA smoke covers health, routes, assets, registration, login, session lookup, overview/buildings/resources, and guards.
- Playwright visual/CSR E2E compares public pages, language flags, game menu navigation, and auth overview/buildings/resources.
- Modernization backlog: [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Verified Implementation

- Milestone: public parity, language flags, resource header caps, login redirect, and evolution auth audit are guarded.

## Verified QA

- `bun run build && bun run check && bun test`: passing.
- `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`: passing.
- Playwright visual, authenticated visual audit, and CSR E2E: passing in Chromium and Firefox.
- Auth visual contract passes in Chromium/Firefox; parity still misses (diff about 24-55%, box delta <=2).
- Go internal coverage gate: `97.4% >= 97%`.
- Go smoke JSON: `all_pass: true`, including registration-created overview access.

Full legacy PHP E2E was not run for this Go migration step. Keep legacy PHP behavior as the oracle until each migrated flow has focused unit tests and E2E coverage.

## Remaining Work

- Implement activation confirmation, welcome mail/message, IP log, cleanup timer, and rank recalculation side effects.
- Add logout, expiry, and deeper session security behavior.
- Public legacy visual baseline is complete for the current public route set.
- Close authenticated visual diff for overview/buildings before claiming game-page parity.
- Port remaining authenticated game screens beyond buildings with legacy PHP screen composition.
- Port current planet switching and full overview actions from legacy DB.
- Port queues, research, shipyard, defense, fleet, reports, messages, galaxy, alliance, admin, maintenance, options, recovery, deletion, vacation, bans, and permissions.
- Convert legacy E2E cases into Go compatibility checks as each flow is migrated.
- Run full legacy E2E before declaring any game-flow migration equivalent.
