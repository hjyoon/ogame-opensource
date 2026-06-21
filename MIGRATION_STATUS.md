# Migration Status

Updated: 2026-06-21 KST, branch `hjyoon/golang`.

React 19/Bun 1.3 + Go 1.25 `net/http` tracker. Keep this file <4KB; split details into another `.md` when needed.

## Current State

- Clean Architecture baseline is under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React production build, emits JSON logs, and runs from the current `compose.golang.yaml` `goapp` container.
- Natural routes and legacy `.php`/`page=` aliases share route manifests; migrated pages use natural React/Go routes while visible chrome stays legacy-compatible.
- Public assets, `evolution` skin, and game images are copied into `frontend/dist/public-assets`.
- Registration, activation, login/logout, sessions, private cookies, IP/ban/session expiry, and `/game` redirects are ported.
- `/api/game/*` covers overview, buildings, empire, resources, research, shipyard, fleet/templates, galaxy, defense, technology, statistics, search, buddy, notes, messages, report, and options models.
- Mutations exist for overview, buildings, resources, research, shipyard/defense queues, fleet templates/dispatch/recall, buddy, notes, messages, and options.
- Modernization candidates stay in [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Implementation

- Overview covers `cp`, `lgn`, notices, unread/build/incoming/missile/fleet pseudo-events, rename/delete blockers, queue markers, stats/ranks, and active planet restore.
- Buildings ports queue state, Commander queue rows, countdowns, add/remove/demolish shortcuts, info-page demolish links, due completion refresh, moon capacity behavior, and post-build header sync.
- Buildings Build/Cancel/Commander queue/Demolish/Build-to-Overview were compared against legacy: DB cost/refund, queue rows, level/fields/score, resources, active timer, and timer-ended state pass Chromium/Firefox.
- Statistics/fleet authenticated visual contracts pass Chromium/Firefox; fleet is near-exact, while player statistics remains a tracked text-rendering diff.
- Empire ports Commander-gated `imperium`, build queue markers, and legacy GET add/destroy/remove shortcuts.
- Resources ports the legacy production-percent form, premium bonus icon column, DB normalization, and post-save resource header sync.
- Research/Shipyard port legacy aliases, table chrome, action colors, countdowns, queue panels, completion refresh, start/cancel/build, and cost/refund/resource math.
- Fleet dispatch covers resource math, cargo/speed, hold fuel/clamps, ACS guards/sync, colonize/exp targets, templates, and recall; deeper mission restrictions remain.
- Galaxy covers clamp, rows, statuses, moon/debris/actions, slot/deut warnings, quick links, and target prefill; instant actions remain.
- Messages/report/notes/buddy/options follow legacy chrome; reports enforce owner/allied spy access.

## Verified QA

- Latest focused checks: `bun run --cwd frontend check`, `bun run --cwd frontend test`, focused Go tests for `mysqlgame`, `delivery/http`, `domain/game`, and `application/game`, `git diff --check`, and `8895 /health`.
- Playwright resources page actions passed Chromium/Firefox: percent save, DB `prod*`, selected values, totals, and visuals.
- Research/shipyard actions pass Chromium/Firefox exact 0px; shipyard covers submit, partial Expected tasks, and complete DB.
- Prior migration QA passed with `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`.
- Prior visual/CSR E2E passed Chromium/Firefox with known auth-page diff tracking.
- Prior Go internal coverage gate: `97.0% >= 97%`.

Full legacy PHP E2E was not rerun in this step; PHP remains the oracle.

## Remaining Work

- Continue authenticated visual parity for remaining statistics text rendering and then other high-diff pages.
- Add Go compatibility checks for migrated legacy E2E flows as they stabilize.
- Finish galaxy instant actions, alliance/admin/recovery/bans/permissions, deeper options mutations, and remaining mission restrictions.
- Run full legacy PHP E2E before declaring game-flow equivalence.
