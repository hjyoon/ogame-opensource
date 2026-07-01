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

Default-enabled screenshots cover the normal authenticated navigation surface:

- overview, rename planet, buildings, resources, merchant
- research, shipyard, fleet, technology, technology detail
- galaxy, galaxy hover tooltip, defense, alliance menu/create/search
- officers, statistics player/alliance
- search, messages, compose message
- buddy, options, notes, create note
- admin home plus admin modes

Default-enabled state snapshots also cover same-route UI changes without POST/GET mutations:

- resource production select edits
- shipyard, defense, and fleet quantity drafts
- alliance create/search drafts
- search, message, options, and notes form drafts
- admin bans, broadcast, and queue filter drafts

The registry also records fixture-gated screens that are disabled by default:

- Commander-only empire and fleet templates
- owned alliance management/member/rank/circular/application states
- report popup
- phalanx popup
- changelog direct screen

## Determinism

The runner stabilizes screenshots by:

- using fixed viewport, device scale factor, locale, timezone, and reduced motion
- installing a fixed `Date.now()` and deterministic `Math.random()`
- disabling animation, transition, smooth scroll, and caret rendering through screenshot CSS
- blurring focused elements and moving the mouse away before capture
- normalizing server time, queue countdowns, resource header values, rank/position text, and statistics timestamps
- masking known dynamic hover/tooltips unless a hover-state screen explicitly asks to capture them
- normalizing native checkbox rendering for Firefox/Chromium parity
- waiting for image decode, fonts, and stable paint before screenshot

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

Update legacy-oracle baselines:

```sh
OGAME_GAME_VISUAL_UPDATE_BASELINES=1 testing/e2e/run-playwright-authenticated-game-visual-e2e.sh
```

## Reports

Reports are written per browser under:

- `.tmp/playwright-authenticated-game-visual/chromium/report.json`
- `.tmp/playwright-authenticated-game-visual/chromium/report.md`
- `.tmp/playwright-authenticated-game-visual/firefox/report.json`
- `.tmp/playwright-authenticated-game-visual/firefox/report.md`

The final QA wrapper runs this suite when `OGAME_RUN_AUTH_GAME_VISUAL=1`, which is the default.
