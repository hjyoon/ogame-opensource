# Migration Status

Updated: 2026-06-22 KST, branch `hjyoon/golang`.

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
- Statistics/fleet authenticated visual contracts pass Chromium/Firefox; player statistics keeps a tracked text-rendering diff.
- Empire ports Commander-gated `imperium`, build queue markers, and legacy GET add/destroy/remove shortcuts.
- Resources ports the legacy production-percent form, premium bonus icon column, DB normalization, and post-save resource header sync.
- Merchant/officers/alliance/admin port DM spend/trade/timers, core alliance flows, 6 admin GET modes, and access guard.
- Research/Shipyard port aliases, chrome, colors, queues, completion refresh, start/cancel/build, and resource math.
- Fleet dispatch covers cargo/speed, fuel/clamps, ACS sync, colonize/exp targets, templates, and recall; deeper restrictions remain.
- Galaxy covers clamp, rows, statuses, moon/debris/actions, slot/deut warnings, quick links, and target prefill; instant actions remain.
- Messages/report/notes/buddy/options follow legacy chrome; report owner/allied access is enforced.
- Notes, Buddylist, and Options visual parity pass Chromium/Firefox at 0px; Options Commander rows use `com_until` like PHP.

## Verified QA

- Latest focused checks: frontend check, focused Go tests, route tests, Docker `goapp` on `8895`.
- Playwright resources page actions passed Chromium/Firefox: percent save, DB `prod*`, selected values, totals, and visuals.
- Research/shipyard/defense actions and galaxy page pass Chromium/Firefox exact 0px; queues cover submit/partial/complete DB.
- Prior migration QA passed with `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`.
- CSR E2E covers Buddy/Options, Options save, Notes create/edit/delete, and logout; Chromium passes.
- Auth visual E2E passes Chromium/Firefox for Notes, Buddylist, Options at 0px; Merchant call/exchange has ~0.12% select diff.
- Prior Go internal coverage gate: `97.0% >= 97%`.

Full legacy PHP E2E was not rerun in this step; PHP remains the oracle.

## Remaining Work

- Continue authenticated visual parity for remaining statistics text rendering and then remaining pages.
- Add Go compatibility checks for migrated legacy E2E flows as they stabilize.
- Finish merchant edge visuals, galaxy instant actions, alliance deep management, admin submodes/recovery/bans/permissions, options mutations, and mission restrictions.
- Run full legacy PHP E2E before declaring game-flow equivalence.
