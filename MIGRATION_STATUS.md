# Migration Status

Updated: 2026-06-18 KST, branch `hjyoon/golang`.

React 19/Bun 1.3 + Go 1.25 native `net/http` tracker. Keep under 4KB; split linked docs first.

## Current State

- Clean Architecture baseline exists under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves `frontend/dist`, logs JSON, and runs from `compose.golang.yaml` on port `8890`.
- `/api/healthz` reports tool targets plus static/legacy asset readiness.
- Natural routes, legacy `.php` aliases, CSS bootstrap, smoke routes, and visual specs share manifests.
- Public routes and aliases match the legacy public compositions.
- Legacy public assets, evolution skin, and game images are copied into `frontend/dist/public-assets`.
- `/api/public/universes` reads the master DB with config fallback.
- Registration creates user, planet, reg IP log, greeting, TimeLimit, ranks, sessions, welcome mail, redirect.
- `/game/validate.php?ack=` and `/activation?ack=` activate accounts and sessions.
- Login/logout create and clear legacy sessions, private cookies, and `/game` redirects.
- `/api/game/session` validates public session plus private cookie, bans, and IP checks.
- `/api/game/{overview,buildings,resources,research,shipyard,fleet,galaxy,defense,technology,statistics,search,notes}` return models; overview/resources/notes mutate settings.
- Auth `/game/*` routes preserve sessions and persist `cp`; migrated game screens use the legacy `evolution` skin.
- Modernization: [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Verified Implementation

- Public parity, language flags, login/logout, and migrated game read models are guarded.
- Fleet covers `flotten1` summary, slots, expeditions, ships, speed/cargo/consumption; dispatch/recall/ACS/templates pending.
- Galaxy covers coordinate clamp, rows 1-15, player status, moon, debris, actions, slots, and deuterium warning; quick actions pending.
- Defense covers display state: shipyard gate, requirements, caps, busy state, costs, durations, counts, and max hints.
- Technology, statistics, search, and notes cover read models plus note mutations.
- Registration writes legacy side effects and sends SMTP/MailHog welcome mail with activation link/password lines.
- Activation clears `validatemd`, sets `validated=1`, copies `email` to `pemail`, redirects, and rejects link reuse.
- Overview covers legacy `cp`, `lgn` activity, rename/delete name rules, blockers, destroy markers, queue flush, stat/rank updates, and active restore.

## Verified QA

- `bun run build && bun run check && bun test`: passing.
- `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`: passing.
- Go smoke covers health, routes, assets, MailHog delivery, activation cleanup/reuse, auth/session/logout, reads/mutations, guards, and cookie privacy.
- Playwright visual/CSR E2E covers public pages and auth game routes in Chromium/Firefox.
- Auth visual contract passes in Chromium/Firefox; parity still misses (diff about 12.5-54.5%, box delta <=2).
- Go internal coverage gate: `97.0% >= 97%`.
- Go smoke JSON: `all_pass: true`.

Full legacy PHP E2E was not run for this Go step. Keep PHP as oracle until each migrated flow has focused checks.

## Remaining Work

- Add expiry and deeper session security behavior.
- Close authenticated visual diff for overview/buildings before claiming game-page parity.
- Port remaining overview legacy actions.
- Port queue mutations, research start/cancel, shipyard/defense orders, fleet actions, galaxy quick actions, reports, messages, alliance, admin, options, recovery, deletion, vacation, bans, permissions.
- Convert legacy E2E cases into Go compatibility checks per migrated flow.
- Run full legacy PHP E2E before declaring any game-flow migration equivalent.
