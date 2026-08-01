# Migration Status

Updated: 2026-08-02 KST, branch `hjyoon/golang`.

React/Bun + Go migration tracker. Keep under 4KB; split details by topic.

## Current State

- React 19 CSR is built with Bun 1.3; Go 1.25 `net/http` serves the build and legacy static assets on port 8890.
- Backend dependencies follow Clean Architecture under `backend/internal/{domain,application,infrastructure,delivery}`.
- Natural routes and legacy `.php`/`page=` aliases share route manifests.
- Registration, activation, recovery, login/logout, session expiry, private cookies, IP/ban checks, and `/game` redirects are migrated.
- `/api/game/*` implements overview, economy and queues, fleet/combat, galaxy, social/account, reports, officers/payment, and Admin/Bot operations.
- Game mutations preserve math, permissions, effects, reports, and scheduler behavior through PHP/Go differential cases; due queues are atomically settled by a configurable background worker.
- MCP supports scoped tokens, Commander-compatible category-filtered/cursor-paged messages with summaries, zero-percent production, projected option affordability, explicit empire aggregates, queue-worker health, and per-token throttling with IP guard and expiry.
- The Go runtime supports MySQL by default and persistent SQLite as an optional pure-Go mode; see [SQLite](./SQLITE.md).
- SQLite due-queue settlement shares the mutation lock; bootstrap honors legacy universe environment settings, and Overview failures use JSON logs.
- One multi-target `Dockerfile` and one `docker-compose.yml` define the PHP oracle and both Go database modes.
- Modernization candidates stay in [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Source Inventory

The drift gates currently map 37/37 game router keys, 26/26 Admin modes, 32/32 public PHP entrypoints, and 14/14 Bot API functions. The non-Mod behavior baseline freezes 463 request inputs, 162 action values, 19 queue constants, 349 SQL mutation sites, and 149 navigation handlers.

Four optional PHP Mods and their 37 executable hooks are outside product scope. Go rejects new PHP Mod installs and fails readiness if one is active; it never silently executes arbitrary PHP.

## Latest Full QA

The 2026-07-16 clean wrapper run completed with `89 passed, 0 failed, 0 skipped`:

```sh
OGAME_RUN_LEGACY_E2E=1 OGAME_GO_PORT=8890 OGAME_KEEP_GO_DOCKER=1 testing/e2e/run-golang-migration-qa.sh
```

- The legacy PHP Docker oracle passed all 59 groups plus its DB invariant.
- Frontend build, TypeScript, and 24 Bun tests passed.
- Backend tests passed the required internal coverage gate at exactly 97.0%.
- Compatibility smoke passed 90 cases / 2249 checks; user-type API QA passed 8 cases / 43 checks.
- All 48 registered PHP/Go DB and HTTP differential groups passed, totaling 433 result cases including dedicated one-case flows.
- Public, authenticated, Commander, dynamic, alliance, empire, overview, fleet, expiry, and navigation Playwright suites passed in Chromium and Firefox.
- Navigation exact diff threshold `0` passed 1,947/1,947 edges and 162/162 target representatives in each browser.
- Authenticated dynamic registry passed all 98 listed cases in each browser.

SQLite focused QA passes bootstrap, authenticated HTTP, registration/build mutation, queue completion, MCP/OAuth, coupons, Admin cron, and backup/restore. The full PHP differential baseline above remains MySQL-backed.

The 2026-08-02 focused backend run passed `go test ./...` and the 97.0% internal coverage gate.

## Completion Statement

All currently discovered and registered non-Mod base-product features are migrated and pass the proportional QA registry. There is no known migration-pending route, screen, action group, or dynamic case in that inventory.

This is not proof of every theoretically possible legacy runtime state. A newly found route, handler, DB-state combination, or behavior expands the denominator and must add implementation plus unit/API/differential/visual evidence before the same statement remains valid. Optional PHP Mod execution remains an explicit exclusion.
