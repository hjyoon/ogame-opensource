# Migration Status

Updated: 2026-07-04 KST, branch `hjyoon/golang`.

React 19/Bun 1.3 + Go 1.25 `net/http` migration tracker. Keep this file under 4KB; split detail docs when needed.

## Current State

- Backend follows Clean Architecture under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React build and legacy static aliases from `compose.golang.yaml` `goapp` on port 8890.
- Natural routes and legacy `.php`/`page=` aliases share route manifests; the UI is CSR, while Go serves the built assets.
- Public assets, `evolution` skin, game CSS/images/js/mod assets, and `/game/css`, `/game/img`, `/evolution` aliases are served by Go.
- Registration, activation, login/logout, sessions, private cookies, IP/ban/session expiry, and `/game` redirects are ported.
- `/api/game/*` covers overview, buildings, resources, merchant/officers, research, shipyard/defense, fleet, galaxy, alliance, admin, statistics, search, messages, report, phalanx, jump gate, notes, buddy, options, and logout.
- Mutations exist for overview, buildings, resources, merchant/officers, alliance, research, shipyard/defense, fleet, buddy, notes, messages, and options.
- Modernization candidates stay in [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Implementation

- Navigation visual E2E scans GET anchors, JS navigation, popups, hovers, select URLs, and GET forms.
- The navigation wrapper continues across Chromium and Firefox even when exact visual diffs remain, then writes a combined report to [COVERAGE-navigation-visual.md](./testing/e2e/COVERAGE-navigation-visual.md).
- Authenticated dynamic E2E now runs all 55 listed legacy-JS cases with commander, alliance, report, phalanx, and ACS fixtures enabled by default.
- Fixed Firefox legacy host/session drift by keeping the configured legacy base URL instead of adopting a redirected `localhost` origin.
- Fixed route parity issues around static aliases, planet selector URLs, statistics defaults, register blank selects, reply prefill, fleet union, commander folders, and galaxy hovers.
- Jump Gate, ACS 30% slowdown, expedition depletion min/med/max, options force-language, pranger, maintenance, feed GET/POST, DB backup safe restore, Mods state/heal/PHP hook policy, Logins/Browse, Loca, and Bots are migrated.
- Overview, buildings/resources/research/shipyard/defense, fleet, galaxy, statistics, search, messages, report, notes, buddy, options, merchant/officers, alliance, and admin use legacy chrome and route aliases where implemented.

## Verified QA

- Full migration QA wrapper passes.
- Legacy PHP Docker E2E passes before Go/Bun checks.
- Frontend build/typecheck/unit tests pass: 20 tests / 172 expects.
- Backend tests and the 97% internal coverage gate pass: `97.0% >= 97%`.
- Absolute legacy coverage target: 99%; current estimate: 99% in [COVERAGE-absolute.md](./testing/e2e/COVERAGE-absolute.md).
- Go compatibility smoke registry covers 87 cases / 2199 checks.
- User-type API and Chromium/Firefox Playwright QA pass.
- Auth visual, authenticated game visual, dynamic behavior, empire, alliance, overview fleet, overview all-cases, fleet continue, and fleet all-cases suites pass in Chromium and Firefox.
- Strict navigation visual threshold `0`: Chromium 170/170 and Firefox 169/169 pass.
- Full summary is in `.tmp/golang-migration-qa-summary.md`; navigation details are in [COVERAGE-navigation-visual.md](./testing/e2e/COVERAGE-navigation-visual.md).

## Remaining Work

- No current strict navigation visual gap remains in the seeded public/game/admin route inventory.
- No concrete listed authenticated dynamic E2E case remains in [COVERAGE-dynamic-legacy-js.md](./testing/e2e/COVERAGE-dynamic-legacy-js.md); add more only when new legacy-JS behavior is found.
- Highest post-99 absolute-coverage gap is broader stochastic expedition distribution evidence.
- Continue adding route/state/action inventory when new pages or unseeded legacy flows are migrated.
- Keep API endpoint inventory aligned with [Backend API Endpoints](./backend/API_ENDPOINTS.md).
