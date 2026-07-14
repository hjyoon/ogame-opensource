# Migration Status

Updated: 2026-07-13 KST, branch `hjyoon/golang`.

React/Bun + Go migration tracker. Keep under 4KB; split details.

## Current State

- Backend follows Clean Architecture under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React build and legacy static aliases from `compose.golang.yaml` `goapp` on port 8890.
- Natural routes and legacy `.php`/`page=` aliases share route manifests; the UI is CSR, while Go serves the built assets.
- Public assets, `evolution` skin, game CSS/images/js/mod assets, and `/img`, `/game/css`, `/game/img`, `/evolution` aliases are served by Go.
- Registration, activation, login/logout, sessions, private cookies, IP/ban/session expiry, and `/game` redirects are ported.
- `/api/game/*` covers overview, buildings, resources, merchant/officers, research, shipyard/defense, fleet, galaxy, alliance, admin, statistics, search, messages, report, phalanx, jump gate, notes, buddy, options, and logout.
- Mutations exist for overview, buildings, resources, merchant/officers, alliance, research, shipyard/defense, fleet, buddy, notes, messages, and options.
- Modernization candidates stay in [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Implementation

- Navigation visual E2E scans links, JS navigation, popups, hovers, select URLs, and GET forms in both browsers; results are in [COVERAGE-navigation-visual.md](./testing/e2e/COVERAGE-navigation-visual.md).
- Authenticated dynamic E2E runs 95 listed legacy-JS cases with commander, alliance, report, phalanx, and ACS fixtures enabled by default.
- Fixed Firefox legacy host/session drift by keeping the configured legacy base URL instead of adopting a redirected `localhost` origin.
- Fixed known route parity defects in aliases, selectors, statistics, registration, messages, fleet, commander folders, and galaxy hovers.
- Jump Gate, ACS slowdown, full expedition lifecycle, option locale, pranger, maintenance, feed, DB restore, Mods hook policy, Logins/Browse, Loca, Bots, and BotEdit import are migrated.
- Inventoried game and admin screens use legacy chrome and route aliases.

## Verified QA

- Full migration QA wrapper passes.
- Legacy PHP Docker E2E passes before Go/Bun checks.
- Frontend build/typecheck/unit tests pass: 24 tests.
- Backend tests and the 97% internal coverage gate pass: `97.0% >= 97%`.
- Inventoried QA fully passes; absolute legacy coverage is not claimed as 100%. See [COVERAGE-absolute.md](./testing/e2e/COVERAGE-absolute.md).
- Go compatibility smoke registry covers 90 cases / 2254 checks.
- User-type API and Chromium/Firefox Playwright QA pass.
- Auth visual, authenticated game visual, dynamic behavior, empire, alliance, overview fleet, overview all-cases, fleet continue, and fleet all-cases suites pass in Chromium and Firefox.
- Strict navigation visual threshold `0`: Chromium 172/172 and Firefox 171/171 pass.
- Full summary is in `.tmp/golang-migration-qa-summary.md`; navigation details are in [COVERAGE-navigation-visual.md](./testing/e2e/COVERAGE-navigation-visual.md).

## Remaining Work

- No current strict navigation visual gap remains in the seeded public/game/admin route inventory.
- No concrete listed authenticated dynamic E2E case remains in [COVERAGE-dynamic-legacy-js.md](./testing/e2e/COVERAGE-dynamic-legacy-js.md); add more only when new legacy-JS behavior is found.
- Source audit inventories routes, Admin modes, public entrypoints, Bot APIs, and optional Mod hooks in [COVERAGE-legacy-inventory.md](./testing/e2e/COVERAGE-legacy-inventory.md).
- Behavior baseline tracks drift and 275 PHP/Go differential cases; Admin evidence is tracked in [COVERAGE-admin-differential.md](./testing/e2e/COVERAGE-admin-differential.md).
- PHP Mod execution is excluded: installs are rejected and active Mods fail readiness.
- Migration-pending game/admin fallback text has been removed.
- Continue adding route/state/action inventory when new pages or unseeded legacy flows are migrated.
- Keep API endpoint inventory aligned with [Backend API Endpoints](./backend/API_ENDPOINTS.md).
