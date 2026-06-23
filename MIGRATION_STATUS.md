# Migration Status

Updated: 2026-06-23 KST, branch `hjyoon/golang`.

React 19/Bun 1.3 + Go 1.25 `net/http` tracker. Keep this file <4KB; split details into another `.md` when needed.

## Current State

- Clean Architecture baseline is under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React production build, emits JSON logs, and runs from the current `compose.golang.yaml` `goapp` container.
- Natural routes and legacy `.php`/`page=` aliases share route manifests; visible chrome stays legacy-compatible.
- Public assets, `evolution` skin, and game images are copied into `frontend/dist/public-assets`.
- Registration, activation, login/logout, sessions, private cookies, IP/ban/session expiry, and `/game` redirects are ported.
- `/api/game/*` covers overview, buildings, empire, resources, merchant/officers, alliance/admin shell, research, shipyard, fleet/templates, galaxy, defense, technology, statistics, search, buddy, notes, messages, report, and options.
- Mutations exist for overview, buildings, resources, merchant call/trade, officer recruit, research, shipyard/defense queues, fleet templates/dispatch/recall, buddy, notes, messages, and options.
- Modernization candidates stay in [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Implementation

- Overview covers `cp`, `lgn`, notices, unread/build/incoming/missile/fleet pseudo-events, rename/delete blockers, queue markers, stats/ranks, and active planet restore.
- Buildings ports queue state, Commander rows, countdowns, shortcuts, due refresh, moon capacity, post-build header sync, and Chromium/Firefox parity for cost/refund/queue/timer states.
- Statistics/fleet authenticated visuals pass Chromium/Firefox at 0px for player/alliance statistics.
- Empire ports Commander-gated `imperium`, build queue markers, and legacy GET add/destroy/remove shortcuts.
- Resources ports the legacy production-percent form, premium bonus icon column, DB normalization, and post-save resource header sync.
- Merchant/officers/alliance/admin port DM spend/trade/timers, core alliance flows, 25 admin GET modes, and access guard.
- Research/Shipyard port aliases, chrome, colors, queues, completion refresh, start/cancel/build, and resource math.
- Fleet dispatch covers cargo/speed, fuel/clamps, ACS sync, colonize/exp targets, templates, and recall; deeper restrictions remain.
- Galaxy covers clamp, rows, statuses, moon/debris/actions, slot/deut warnings, quick links, and target prefill; instant actions remain.
- Messages/report/notes/buddy/options follow legacy chrome; report owner/allied access is enforced.
- Notes, Buddylist, and Options visual parity pass Chromium/Firefox at 0px; Options Commander rows use `com_until` like PHP.

## Verified QA

- Latest full check: `OGAME_GO_PORT=8895 OGAME_KEEP_GO_DOCKER=1 testing/e2e/run-golang-migration-qa.sh` passed.
- This includes full Docker PHP E2E, frontend Bun build/check/test, all Go tests, 97% Go coverage, Docker `goapp`, and Go compatibility smoke with `all_pass=true`.
- Playwright resources page actions passed Chromium/Firefox: percent save, DB `prod*`, selected values, totals, and visuals.
- Research/shipyard/defense actions and galaxy page pass Chromium/Firefox exact 0px; queues cover submit/partial/complete DB.
- CSR E2E covers Buddy/Options, Options save, Notes create/edit/delete, and logout; Chromium passes.
- Auth visual E2E passes Chromium/Firefox for Notes, Buddylist, Options, Merchant, and Statistics at 0px.
- Go internal coverage gate: `97.0% >= 97%`.

## Remaining Work

- Continue authenticated visual parity for remaining non-statistics pages.
- Add Go compatibility checks for migrated legacy E2E flows as they stabilize.
- Finish galaxy instant actions, alliance deep management, admin submodes/recovery/bans/permissions, options mutations, and mission restrictions.
- Run full legacy PHP E2E before declaring game-flow equivalence.
