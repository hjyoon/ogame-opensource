# Absolute Legacy Coverage Target

Keep this file under 4KB. This is the conservative product-coverage model for the PHP-to-Go/Bun migration, separate from test pass rate or Go line coverage.

## Target

- Goal: 100% absolute coverage of legacy PHP functionality.
- Current absolute estimate: not asserted; the repository has no exhaustive legacy denominator proving a percentage.
- Current inventoried QA pass rate: 100% (32/32 suites in the 2026-07-12 full run).
- A passing registry proves only listed cases, not every reachable legacy behavior.

## Denominator

The denominator is the legacy product surface, not file count alone:

| Area | Weight | Current | Notes |
| --- | ---: | ---: | --- |
| Public auth/site | 10 | 10 | login, registration, activation, recovery, public pages, registration aliases |
| Core game screens | 20 | 20 | authenticated route/visual/dynamic parity |
| Game mechanics | 30 | 30 | economy, queues, fleets, combat, reports, colony, moon, missiles, expedition, jump gate |
| Admin/ops tools | 15 | 15 | admin pages, bans, audit, DB, simulators, queues, bots, localization |
| Security/account/social | 10 | 10 | session, IDOR, options, email queue timing, messages, buddy, alliance |
| Runtime/infra/maintenance | 10 | 10 | cron, feed GET/POST, aliases, maintenance, backups, localization/perf edges |
| Mods/extensibility | 5 | Partial | assets/manifests/actions migrated; PHP runtime hooks are not executable in Go |
| **Total** | **100** | **Not scored** | weights define the target, not measured completion |

## Evidence Already In QA

- Full migration QA wrapper passes with legacy PHP E2E plus Go/Bun checks.
- Go internal coverage gate: 97.0% >= 97%.
- Compatibility smoke covers 90 cases / 2254 checks.
- Strict navigation visual exact diff has 0 failures for the seeded public/game/admin inventory.
- Auth/game/navigation visual QA covers the normal authenticated route surface, route-discovered targets, and page-state exact diff in Chromium and Firefox.
- Authenticated dynamic registry covers 91 listed legacy-JS cases.
- Jump Gate, pranger, maintenance mode, feed GET/POST, registration aliases, DB backup safe failures, and unsafe restore rejection have Go/Bun implementation plus handler/DB/API evidence.
- Account options preserve password/email/vacation/deletion behavior, including 7-day email confirmation queue timing and forced-universe language.
- Admin Logins/Browse, Loca, Bots, Mods, DB, simulators, destructive actions, and queue operations are migrated with API/unit/smoke evidence.
- Admin Mods now reads/heals `uni.modlist`, mirrors active/available/installed states, handles install/remove/move, detects legacy PHP runtime hooks, and marks them as unsupported native-adapter work instead of executing PHP.
- Expedition due-queue result selection now includes legacy success/event roll buckets, hold-time success, depletion min/med/max thresholds, and far-space visit counter evidence.
- ACS attack launch now covers the legacy 30% slowdown boundary and queue resync to the later union arrival.

## Remaining Closure Work

Keep this model honest as new legacy-only surfaces are found:

1. Replace each PHP mod runtime hook with a native Go adapter, or remove it through an explicit product decision; unsupported behavior is not functional parity.
2. Treat any reachable migration-pending fallback as an implementation gap.
3. Audit legacy routes, POST actions, cron paths, bot strategies, admin modes, and client-side handlers against registries.
4. Keep screenshots, smoke checks, and Go tests aligned when inventory expands.

## Counting Rule

A feature counts as functionally migrated only when it has Go/Bun implementation plus proportionate unit/API/E2E evidence. An intentionally unsupported decision is documented scope reduction, not migrated functionality.

Passing the current registry does not raise this percentage unless the registry expands the legacy denominator.
