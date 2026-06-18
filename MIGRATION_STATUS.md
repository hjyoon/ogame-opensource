# Migration Status

Updated: 2026-06-18 KST, branch `hjyoon/golang`.

Tracker for React 19/Bun 1.3 + Go 1.25 native `net/http` migration. Keep under 4KB; split linked docs first.

## Current State

- Clean Architecture baseline exists under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves `frontend/dist`, logs JSON, and runs from `compose.golang.yaml` on port `8890`.
- `/api/healthz` reports tool targets plus static and legacy asset readiness.
- Natural routes and legacy `.php` aliases resolve through the React shell.
- Aliases, CSS bootstrap, smoke routes, and visual specs share manifests.
- Public routes and aliases match the legacy public compositions.
- Legacy public assets, evolution skin, and game images are copied into `frontend/dist/public-assets`.
- `/api/public/universes` reads the master DB with config fallback.
- Registration validates drafts, checks duplicates/capacity, then creates legacy-compatible user, home planet, sessions, and `/game/overview` redirect.
- Login/logout create and clear legacy sessions, private cookies, and `/game` redirects.
- `/api/game/session` validates public session plus private cookie, bans, and IP checks.
- `/api/game/{overview,buildings,resources,research,shipyard,fleet,galaxy,defense,technology,statistics,search,notes}` return models; overview/resources/notes store settings.
- Auth `/game/*` routes preserve sessions and persist `cp`; migrated game screens use the legacy `evolution` skin.
- Modernization: [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Verified Implementation

- Milestone: public parity, language flags, login/logout, and migrated game read models are guarded.
- Fleet covers `flotten1` summary, slots, expeditions, ship rows, and speed/cargo/consumption. Dispatch/recall/ACS/templates pending.
- Galaxy covers the `galaxy` read screen: coordinate clamp, rows 1-15, player status, moon, debris, actions, slots, and deuterium warning. Quick actions and cost mutation pending.
- Defense covers display state only: shipyard gate, requirements, shield dome/missile caps, busy state, costs, durations, counts, and max hints.
- Technology covers `techtree` and recursive details; statistics covers player/alliance rankings; search covers player/planet/alliance results.
- Notes covers `notizen` list, create/edit forms, and add/update/delete mutations.
- Current planet switching follows legacy owned/foreign/missing/moon `cp` behavior.
- Overview rename/delete covers legacy name rules, password/home/fleet blockers, destroy markers, queue flush, and active-planet restore.

## Verified QA

- `bun run build && bun run check && bun test`: passing.
- `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`: passing.
- Go smoke covers health, routes, assets, auth/session/logout, migrated reads/mutations incl. planet rename, guards, and cookie non-disclosure.
- Playwright visual/CSR E2E covers public pages and auth game routes in Chromium/Firefox.
- Auth visual contract passes in Chromium/Firefox; parity still misses (diff about 12.5-54.5%, box delta <=2).
- Go internal coverage gate: `97.0% >= 97%`.
- Go smoke JSON: `all_pass: true`.

Full legacy PHP E2E was not run for this Go step. Keep legacy PHP as oracle until each flow has focused tests and E2E.

## Remaining Work

- Implement activation confirmation, welcome mail/message, IP log, cleanup timer, and rank recalculation side effects.
- Add expiry and deeper session security behavior.
- Close authenticated visual diff for overview/buildings before claiming game-page parity.
- Port overview stat/rank side effects and remaining legacy actions.
- Port queue mutations, research start/cancel, shipyard/defense orders, fleet dispatch/recall/ACS/templates, galaxy quick actions, reports, messages, alliance, admin, maintenance, options, recovery, deletion, vacation, bans, and permissions.
- Convert legacy E2E cases into Go compatibility checks as each flow is migrated.
- Run full legacy PHP E2E before declaring any game-flow migration equivalent.
