# Absolute Legacy Coverage Target

Keep this file under 4KB. This is the conservative product-coverage model for the PHP-to-Go/Bun migration, separate from test pass rate or Go line coverage.

## Target

- Goal: 97% absolute coverage of legacy PHP functionality.
- Current estimate: 97%.
- QA pass rate inside the current registry may be 100%, but only proves inventoried cases.

## Denominator

The denominator is the legacy product surface, not file count alone:

| Area | Weight | Current | Notes |
| --- | ---: | ---: | --- |
| Public auth/site | 10 | 10 | login, registration, activation, recovery, public pages, registration aliases |
| Core game screens | 20 | 20 | authenticated route/visual/dynamic parity |
| Game mechanics | 30 | 28 | economy, queues, fleets, combat, reports, colony, moon, missiles, expedition, jump gate |
| Admin/ops tools | 15 | 15 | admin pages, bans, audit, DB, simulators, queues, bots, localization |
| Security/account/social | 10 | 10 | session, IDOR, options, email queue timing, messages, buddy, alliance |
| Runtime/infra/maintenance | 10 | 9 | cron, feed GET/POST, aliases, maintenance, backups, localization/perf edges |
| Mods/extensibility | 5 | 5 | assets, manifests, state columns, stale-heal, modlist actions, PHP hook policy |
| **Total** | **100** | **97** | conservative estimate |

## Evidence Already In QA

- Full migration QA wrapper passes with legacy PHP E2E plus Go/Bun checks.
- Go internal coverage gate: 97.0% >= 97%.
- Compatibility smoke covers 87 cases / 2199 checks.
- Strict navigation visual exact diff has 0 failures for the seeded public/game/admin inventory.
- Auth/game/navigation visual QA covers the normal authenticated route surface, route-discovered targets, and page-state exact diff in Chromium and Firefox.
- Authenticated dynamic registry covers 55 listed legacy-JS cases.
- Jump Gate, pranger, maintenance mode, feed GET/POST, registration aliases, and DB backup safe failures have Go/Bun implementation plus handler/DB/API evidence.
- Account options preserve password/email/vacation/deletion behavior, including 7-day email confirmation queue timing and forced-universe language.
- Admin Logins/Browse, Loca, Bots, Mods, DB, simulators, destructive actions, and queue operations are migrated with API/unit/smoke evidence.
- Admin Mods now reads/heals `uni.modlist`, mirrors active/available/installed states, handles install/remove/move, detects legacy PHP runtime hooks, and marks them as unsupported native-adapter work instead of executing PHP.
- Expedition due-queue result selection now includes legacy hold-time success, depletion min/med/max thresholds, and far-space visit counter evidence.

## Work Beyond 97%

Prioritize gaps that add product coverage, not just more screenshots:

1. Raise game mechanics from 28/30 with rare fleet/ACS timing and broader stochastic expedition distributions.
2. Raise runtime/infra from 9/10 with deeper install/upgrade recovery drills.
3. Convert any new legacy-only surface found during inventory into migrated code or an explicit unsupported decision.

## Counting Rule

A feature counts only when it has migrated Go/Bun implementation plus unit/API/E2E evidence, or an explicit compatibility decision documented as intentionally unsupported.

Passing the current registry does not raise this percentage unless the registry expands the legacy denominator.
