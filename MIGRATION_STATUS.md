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
- Buildings now ports legacy queue state, Commander queue rows, non-Commander active countdown cell, add/remove/demolish shortcuts, due-queue completion refresh, moon resource capacity behavior, and post-build resource header sync.
- Buildings resource deduction was compared against legacy after Build click: DB cost deduction, build queue/global queue insertion, and displayed resource values match the current DB state in Chromium and Firefox.
- Empire ports Commander-gated `imperium`, build queue markers, and legacy GET add/destroy/remove shortcuts.
- Fleet dispatch covers resource math, cargo/speed, hold fuel/clamps, ACS guards/sync, colonize/exp targets, templates, and recall; deeper mission restrictions remain.
- Galaxy covers clamp, rows, statuses, moon/debris/actions, slot/deut warnings, quick links, and target prefill; instant actions remain.
- Messages/report/notes/buddy/options follow legacy chrome; reports enforce owner/allied spy access.

## Verified QA

- Latest focused checks: `bun run --cwd frontend check`, `bun run --cwd frontend test`, focused Go tests for `mysqlgame`, `delivery/http`, `domain/game`, and `application/game`, `git diff --check`, and `8895 /health`.
- Playwright headless resource-deduction comparison passed in Chromium and Firefox.
- Prior migration QA passed with `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`.
- Prior visual/CSR E2E passed Chromium/Firefox with known auth-page diff tracking.
- Prior Go internal coverage gate: `97.0% >= 97%`.

Full legacy PHP E2E was not rerun in this step; PHP remains the oracle.

## Remaining Work

- Continue authenticated visual parity, starting with buildings post-action edge cases and then statistics/fleet high-diff pages.
- Add Go compatibility checks for migrated legacy E2E flows as they stabilize.
- Finish galaxy instant actions, alliance/admin/recovery/bans/permissions, deeper options mutations, and remaining mission restrictions.
- Run full legacy PHP E2E before declaring game-flow equivalence.
