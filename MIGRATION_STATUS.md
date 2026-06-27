# Migration Status

Updated: 2026-06-27 KST, branch `hjyoon/golang`.

React 19/Bun 1.3 + Go 1.25 `net/http` migration tracker. Keep this file under 4KB; split detail docs when needed.

## Current State

- Backend follows Clean Architecture under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React production build and legacy static aliases from the current `compose.golang.yaml` `goapp` container on port 8890.
- Natural routes and legacy `.php`/`page=` aliases share route manifests; the UI is CSR, while Go serves the built assets.
- Public assets, `evolution` skin, game CSS/images/js/mod assets, and `/game/css`, `/game/img`, `/evolution` aliases are served by Go.
- Registration, activation, login/logout, sessions, private cookies, IP/ban/session expiry, and `/game` redirects are ported.
- `/api/game/*` covers overview, buildings, resources, merchant/officers, research, shipyard/defense, fleet, galaxy, alliance, admin, statistics, search, messages, report, phalanx, notes, buddy, options, and logout.
- Mutations exist for overview, buildings, resources, merchant/officers, alliance, research, shipyard/defense, fleet, buddy, notes, messages, and options.
- Modernization candidates stay in [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Implementation

- Navigation visual E2E now scans all current seed screens for internal GET anchors, JS navigation, popup/open handlers, hover tooltip hrefs, select option URLs, and GET forms.
- The navigation wrapper continues across Chromium and Firefox even when exact visual diffs remain, then writes a combined report to [COVERAGE-navigation-visual.md](./testing/e2e/COVERAGE-navigation-visual.md).
- Fixed Firefox legacy host/session drift by keeping the configured legacy base URL instead of adopting a redirected `localhost` origin.
- Fixed route parity issues around legacy static asset aliases, planet selector URLs, statistics default `[Own position]`, register `linkuni` blank select display, and message reply subject prefill.
- Overview, buildings/resources/research/shipyard/defense, fleet, galaxy, statistics, search, messages, report, notes, buddy, options, merchant/officers, alliance, and admin use legacy chrome and route aliases where implemented.

## Verified QA

- Frontend: `bun run check` passes.
- Frontend route tests: `bun test src/routes.test.ts` passes, 10 tests / 61 expects.
- Backend focused tests: `go test ./internal/delivery/http ./internal/application/game ./internal/infrastructure/mysqlgame` passes.
- Docker Go app was rebuilt and restarted on `8890`; legacy PHP remains on `8888`.
- Strict navigation visual run, threshold `0`:
  - Chromium: 68 screens, 1910/1910 matched edges, 115 targets, 102 exact pass / 13 fail.
  - Firefox: 68 screens, 1910/1910 matched edges, 115 targets, 105 exact pass / 10 fail.
- Full details and screenshot paths are in `.tmp/playwright-navigation-visual/{chromium,firefox}/report.json` and report Markdown.

## Remaining Work

- Exact visual parity is not complete. Remaining gaps are concentrated in admin detail pages, alliance detail/application pages, changelog, password recovery, and technology info.
- Do not claim full navigation visual parity until both Chromium and Firefox report `Exact Fail` as 0 in `COVERAGE-navigation-visual.md`.
- Continue porting remaining legacy PHP E2E flows to Go+Bun compatibility where strict visual or flow parity still differs.
