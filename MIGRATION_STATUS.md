# Migration Status

Last updated: 2026-06-18 KST, branch `hjyoon/golang`.

Living tracker for the React 19 + Bun 1.3 frontend and Go 1.25 native `net/http` backend migration. Keep this file under 4KB; split details into linked docs before exceeding that limit.

## Current State

- Clean Architecture baseline exists under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React production build from `frontend/dist`, logs JSON through `log/slog`, and runs locally from `compose.golang.yaml` on port `8890`.
- `/api/healthz` reports tool targets plus static and legacy asset readiness.
- Natural public routes and legacy `.php` aliases resolve through the React shell.
- Public aliases, early CSS bootstrap, Go smoke routes, and public visual specs derive from one route manifest.
- `/home`, `/register`, `/about`, `/story`, `/screenshots`, `/rules`, `/legal`, `/universes`, and aliases match the legacy public compositions.
- Legacy public assets, evolution skin, and game images are copied into `frontend/dist/public-assets`.
- `/api/public/universes` reads the master DB with config fallback.
- Registration validates drafts, checks duplicates/capacity, then creates a legacy-compatible unvalidated user, activation code, home planet, sessions, and `/game/overview` redirect.
- Login validates credentials, creates public/private sessions, updates legacy session fields, sets the private cookie, and redirects to `/game/overview`.
- `/api/game/session` validates public session plus private cookie, including banned and IP checks.
- `/api/game/{overview,buildings,resources,research,shipyard,defense,technology}` return game read models; resources also stores production settings.
- Auth `/game/*` routes preserve sessions; overview/buildings/resources/research/shipyard/defense/technology use the legacy `evolution` skin.
- Modernization: [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Verified Implementation

- Milestone: public parity, language flags, resource header caps, login redirect, research, shipyard, defense, and technology read models are guarded.
- Defense currently covers display state only: shipyard gate, availability requirements, shield dome cap, missile silo cap, busy state, costs, durations, counts, and max build hints.
- Technology covers the main `techtree` requirements table; recursive `techtreedetails` is still pending.

## Verified QA

- `bun run build && bun run check && bun test`: passing.
- `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`: passing.
- Go smoke covers health, routes, assets, registration, login, session lookup, migrated game read models, method guards, and private-cookie non-disclosure.
- Playwright visual/CSR E2E covers public pages, language flags, game menu navigation, and auth overview/buildings/resources/research/shipyard/defense/technology in Chromium and Firefox.
- Auth visual contract passes in Chromium/Firefox; parity still misses (diff about 12.5-54.5%, box delta <=2).
- Go internal coverage gate: `97.2% >= 97%`.
- Go smoke JSON: `all_pass: true`, including registration-created overview access.

Full legacy PHP E2E was not run for this Go migration step. Keep legacy PHP behavior as the oracle until each migrated flow has focused unit tests and E2E coverage.

## Remaining Work

- Implement activation confirmation, welcome mail/message, IP log, cleanup timer, and rank recalculation side effects.
- Add logout, expiry, and deeper session security behavior.
- Close authenticated visual diff for overview/buildings before claiming game-page parity.
- Port current planet switching and full overview actions from legacy DB.
- Port queue mutations, research start/cancel, shipyard orders, defense orders, technology details, fleet, reports, messages, galaxy, alliance, admin, maintenance, options, recovery, deletion, vacation, bans, and permissions.
- Convert legacy E2E cases into Go compatibility checks as each flow is migrated.
- Run full legacy PHP E2E before declaring any game-flow migration equivalent.
