# Modernization Options

This is a backlog, not permission to change behavior during migration. Preserve legacy parity first; modernize only after compatibility tests prove the old behavior.

## Current Evidence

- Public pages now use strict Playwright visual parity against PHP.
- Public CSR navigation, language flags, registration, and login are covered.
- Authenticated overview/buildings use the `evolution` skin but remain audit-only for visual diff.
- Login/session behavior still bridges legacy public session IDs and private session cookies.
- Game overview/buildings read legacy numeric DB columns and mirror legacy formatting such as `nicenum()` and storage caps.

## Guardrails

- Do not modernize anything that changes game math, timings, targeting, reports, permissions, or persisted state.
- Do not replace legacy layout until a page has visual E2E coverage and accepted diffs.
- Record any deliberate mismatch as a compatibility exception before merging it.
- Prefer adapter seams: keep legacy bridges at delivery/infrastructure edges, not in domain logic.

## Options

1. Public UI composition
   - After all public pages are parity-locked, collapse repeated `LegacyPublic*` components into shared typed templates.
   - Replace route-level CSS injection/body class toggles with scoped style loading or CSS layers.
   - Keep `.php` aliases as compatibility routes, but make natural routes the canonical internal model.

2. Language and preferences
   - Current flag clicks intentionally preserve legacy `ogamelang` cookie plus reload.
   - Later, add a React preference adapter that stores language without full reload while still syncing legacy cookies for PHP compatibility.

3. Session model
   - Current Go flow mirrors public session query params plus legacy private cookies.
   - Later, move to opaque server-side sessions with secure, HTTP-only, SameSite cookies, rotation, expiry, logout, and audit logs.
   - Keep a legacy bridge until PHP oracle traffic is fully retired.

4. Legacy DB access
   - Current repositories read numeric columns such as resources/buildings directly.
   - Later, hide numeric schema behind typed projections or database views.
   - Move formulas such as storage capacity from repository mapping into pure domain services once production/resource rules are ported.

5. Game shell rendering
   - Current auth screens intentionally mirror table-era HTML/CSS and fixed asset choices.
   - Later, extract header/menu/table primitives from verified DOM snapshots, then introduce design tokens only after parity is enforced.
   - Replace inline React styles used for legacy colors/states with typed presenter data.

6. Game mechanics
   - Keep formulas ported as pure domain functions with PHP-derived fixtures.
   - Later, consolidate duplicated price, duration, capacity, production, combat, queue, and fleet calculations into cohesive domain packages.

7. QA and tooling
   - Convert auth visual audit from advisory to enforced thresholds as each page converges.
   - Reduce flaky setup by creating deterministic test fixtures through Go test helpers instead of browser login where possible.
   - Keep browser comparison scripts as migration gates until the PHP UI is no longer the oracle.

8. Operations
   - Extend JSON logs with request IDs, session/user identifiers where safe, latency, and route names.
   - Add health/readiness details for DB connectivity and legacy bridge state.
