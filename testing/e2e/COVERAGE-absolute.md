# Absolute Legacy Coverage Target

Keep this file under 4KB. This is the conservative product-coverage model for the PHP-to-Go/Bun migration, separate from test pass rate or Go line coverage.

## Target

- Goal: 90% absolute coverage of legacy PHP functionality.
- Current estimate: 78%.
- QA pass rate inside the current registry may be 100%, but that only proves the inventoried cases.

## Denominator

The denominator is the legacy product surface, not file count alone:

| Area | Weight | Current | Notes |
| --- | ---: | ---: | --- |
| Public auth/site | 10 | 9 | login, registration, activation, recovery, public pages |
| Core game screens | 20 | 18 | authenticated route/visual/dynamic parity |
| Game mechanics | 30 | 26 | economy, queues, fleets, combat, reports, colony, moon, missiles, expedition, jump gate |
| Admin/ops tools | 15 | 10 | admin pages and core mutations covered; some tools remain shallow |
| Security/account/social | 10 | 9 | session, IDOR, options, messages, buddy, alliance |
| Runtime/infra/maintenance | 10 | 7 | cron, feed, backup, direct aliases, localization, performance, install/maintenance edges |
| Mods/extensibility | 5 | 2 | mod assets plus manifest-backed listing; runtime mod behavior is mostly not migrated |
| **Total** | **100** | **78** | conservative estimate |

## Evidence Already In QA

- Full migration QA wrapper passes with legacy PHP E2E plus Go/Bun checks.
- Go internal coverage gate: 97.0% >= 97%.
- Compatibility smoke covers 87 cases / 2186 checks.
- Strict navigation visual exact diff has 0 failures for the seeded public/game/admin inventory.
- Authenticated dynamic registry covers 55 listed legacy-JS cases.
- Jump Gate now has Go/Bun API, React route, legacy `sprungtor` POST compatibility, DB mutation tests, and edge validation tests.

## Work To Reach 90%

Prioritize gaps that add product coverage, not just more screenshots:

1. Deepen admin/ops behavior: Logins/Browse/Bots/Loca/Mods actions, simulator variants, destructive safety edges.
2. Expand runtime/infra: feed token variants, maintenance/install compatibility, DB backup restore failure cases, localization switching.
3. Add mod compatibility boundaries: manifest discovery, static asset aliases, disabled/available/active states, safe no-op install behavior.
4. Add long-tail mechanics: rare fleet/ACS timing, expedition depletion distributions, destroyed moon retarget edge cases.

## Counting Rule

A feature counts only when it has either:

- migrated Go/Bun implementation plus unit/API/E2E evidence, or
- an explicit compatibility decision documented as intentionally unsupported.

Passing the current registry does not raise this percentage unless the registry expands the legacy denominator.
