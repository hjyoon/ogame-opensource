# Migration Status

Updated: 2026-07-03 KST, branch `hjyoon/golang`.

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

- Navigation visual E2E scans all current seed screens for internal GET anchors, JS navigation, popup/open handlers, hover tooltip hrefs, select option URLs, and GET forms.
- The navigation wrapper continues across Chromium and Firefox even when exact visual diffs remain, then writes a combined report to [COVERAGE-navigation-visual.md](./testing/e2e/COVERAGE-navigation-visual.md).
- Authenticated dynamic E2E now runs all 55 listed legacy-JS cases with commander, alliance, report, phalanx, and ACS fixtures enabled by default.
- Fixed Firefox legacy host/session drift by keeping the configured legacy base URL instead of adopting a redirected `localhost` origin.
- Fixed route parity issues around static asset aliases, planet selector URLs, statistics default `[Own position]`, register `linkuni` blank select display, message reply subject prefill, fleet union state, commander message folders, and galaxy tooltip hover normalization.
- Overview, buildings/resources/research/shipyard/defense, fleet, galaxy, statistics, search, messages, report, notes, buddy, options, merchant/officers, alliance, and admin use legacy chrome and route aliases where implemented.

## Verified QA

- Full migration QA: `OGAME_KEEP_GO_DOCKER=1 testing/e2e/run-golang-migration-qa.sh` passes.
- Legacy PHP Docker E2E passes before Go/Bun checks.
- Frontend build/typecheck/unit tests pass: 20 tests / 144 expects.
- Backend tests and the 97% internal coverage gate pass: `97.0% >= 97%`.
- Go compatibility smoke passes: 87 cases / 2186 checks.
- User-type API and Chromium/Firefox Playwright QA pass.
- Auth visual, authenticated game visual, dynamic behavior, empire, alliance, overview fleet, overview all-cases, fleet continue, and fleet all-cases suites pass in Chromium and Firefox.
- Strict navigation visual run, threshold `0`: Chromium 170/170 targets pass, Firefox 169/169 targets pass, exact fail 0.
- Full summary is in `.tmp/golang-migration-qa-summary.md`; navigation details are in [COVERAGE-navigation-visual.md](./testing/e2e/COVERAGE-navigation-visual.md).

## Remaining Work

- No current strict navigation visual gap remains in the seeded public/game/admin route inventory.
- No concrete listed authenticated dynamic E2E case remains in [COVERAGE-dynamic-legacy-js.md](./testing/e2e/COVERAGE-dynamic-legacy-js.md); add more only when new legacy-JS behavior is found.
- Continue adding route/state/action inventory when new pages or unseeded legacy flows are migrated.
- Keep API endpoint inventory aligned with [Backend API Endpoints](./backend/API_ENDPOINTS.md).
