# Authenticated Game Visual Coverage

This page tracks deterministic visual regression coverage for logged-in game screens in the Go/React migration.

Keep Markdown files under 4KB. Split deeper per-page notes into a new linked file when needed.

## Tooling

- Runner: `frontend/scripts/playwright-authenticated-game-visual-e2e.ts`
- Registry: `frontend/scripts/visual/game-screen-registry.ts`
- Shared visual utilities: `frontend/scripts/visual/game-visual-utils.ts`
- Docker wrapper: `testing/e2e/run-playwright-authenticated-game-visual-e2e.sh`
- Package script: `bun run e2e:visual:game-auth`

The runner uses Playwright headless Chromium/Firefox. It compares legacy PHP and Go/React screenshots pairwise at an exact `0` default threshold.

## Auth

The wrapper prepares a deterministic `legor` session in the legacy PHP container with:

- validated account
- admin rights by default
- English language and `/evolution/` skin
- active home planet selected
- public session plus `prsess_*` private cookie
- active build/fleet/shipyard queues for that user cleared

The fixture JSON is written to `.tmp/authenticated-game-visual-fixture.json` and reused by both legacy and migrated contexts. Without the fixture file, the runner falls back to the existing login flow.

## Default Screens

Default-enabled screenshots cover the normal authenticated navigation surface: overview, rename planet, buildings, resources, merchant, research, shipyard, fleet, technology/detail, galaxy hovers, defense, alliance menu/create/search, officers, statistics, search, messages/compose, buddy, options, notes/create, admin home, and admin modes.

Default state snapshots cover non-mutating draft UI: resource production edits, shipyard/defense/fleet quantities, alliance create/search, search/message/options/notes forms, and admin bans/broadcast/queue filters.

Fixture-gated screens stay disabled by default because they need mutually exclusive account state:

- `OGAME_GAME_VISUAL_COMMANDER_FIXTURE=1`: empire, fleet templates, changelog
- `OGAME_GAME_VISUAL_ALLIANCE_FIXTURE=1`: owned alliance management/member/rank/circular/application states
- `OGAME_GAME_VISUAL_REPORT_FIXTURE=1`: seeded report popup
- `OGAME_GAME_VISUAL_PHALANX_FIXTURE=1`: seeded missing-sensor phalanx popup

## Determinism

The runner fixes viewport, scale, locale, timezone, `Date.now()`, and `Math.random()`. It disables animation/caret rendering, blurs focus, moves the mouse away, normalizes clocks/countdowns/ranks/resource headers/stat timestamps, masks known dynamic hover UI, normalizes checkbox rendering, and waits for images/fonts/stable paint.

## Commands

Run default authenticated game visual coverage:

```sh
testing/e2e/run-playwright-authenticated-game-visual-e2e.sh
```

Run only specific screens:

```sh
OGAME_GAME_VISUAL_SCREENS=game-overview,game-buildings testing/e2e/run-playwright-authenticated-game-visual-e2e.sh
```

Run an area:

```sh
OGAME_GAME_VISUAL_AREA=admin testing/e2e/run-playwright-authenticated-game-visual-e2e.sh
```

Run fixture-gated state examples:

```sh
OGAME_GAME_VISUAL_COMMANDER_FIXTURE=1 OGAME_GAME_VISUAL_SCREENS=game-empire,game-fleet-templates testing/e2e/run-playwright-authenticated-game-visual-e2e.sh
OGAME_GAME_VISUAL_REPORT_FIXTURE=1 OGAME_GAME_VISUAL_PHALANX_FIXTURE=1 OGAME_GAME_VISUAL_SCREENS=game-report,game-phalanx testing/e2e/run-playwright-authenticated-game-visual-e2e.sh
```

Update legacy-oracle baselines:

```sh
OGAME_GAME_VISUAL_UPDATE_BASELINES=1 testing/e2e/run-playwright-authenticated-game-visual-e2e.sh
```

## Reports

Reports are written per browser under `.tmp/playwright-authenticated-game-visual/<browser>/report.{json,md}`.

The final QA wrapper runs this suite when `OGAME_RUN_AUTH_GAME_VISUAL=1`, which is the default.
