# Docker E2E Tests

This suite treats the Docker PHP app as the behavior oracle for the Go/React migration. It runs real HTTP requests and in-container queue handlers against the installed game.

Keep this file under 4KB. Split details into linked Markdown files.

## Legacy Oracle

Start the legacy stack:

```sh
docker compose up -d --build
```

Run all legacy PHP E2E cases:

```sh
testing/e2e/run-docker-e2e.sh
```

The wrapper copies current `wwwroot`, `download`, `game`, and `testing/e2e` files into the `server` container, cleans stale Go migration fixtures by default, runs all PHP cases, and writes results under `/tmp/ogame-e2e-results`.

Result files:

- `/tmp/ogame-e2e-results/summary.md`
- `/tmp/ogame-e2e-results/summary.json`
- one JSON file per case group
- one `.stderr` file per case; non-empty stderr fails the case

`OGAME_CLEAN_MIGRATION_FIXTURES=1` is the default. Set it to `0` only when debugging fixture lifecycle problems, because stale visual fixtures can leave active fleets pointing at removed special targets and break DB invariants before the real test starts.

## Final Migration QA

Run final equivalence with the legacy oracle included:

```sh
OGAME_KEEP_GO_DOCKER=1 testing/e2e/run-golang-migration-qa.sh
```

This wrapper runs:

- full legacy PHP Docker E2E
- legacy behavior-surface drift audit and PHP-Go DB differential checks
- Go smoke fixture preparation
- frontend `bun install`, build, typecheck, and unit tests
- backend tests plus the 97% internal coverage gate
- Go Docker app rebuild/start
- Go compatibility smoke and user-type API QA
- Chromium/Firefox Playwright CSR and visual equivalence checks
- overview/fleet deep visual and click-contract checks
- navigation visual discovery for all migrated public/game `GET` paths
- final `.tmp/golang-migration-qa-summary.{json,md}` aggregation

The wrapper includes legacy PHP E2E by default and starts the migrated Go app on the default Go port, currently `8890`. Do not set `OGAME_RUN_LEGACY_E2E=0` for final validation; it is only for local smoke work while iterating on frontend/backend code.

## Visual Parity

Default final visual checks enforce exact parity where the scripts support it:

- auth visual pages: Chromium/Firefox
- empire and alliance visual pages: Chromium/Firefox
- overview fleet view and countdown behavior
- overview all-cases event surface and clicks
- fleet continue and fleet all-cases dispatch previews
- navigation path discovery and exact diff reports
- authenticated game visual registry with fixed clock/CSS/masks
- authenticated game dynamic behavior registry for masked JS actions

Reports are written under `.tmp/playwright-*`, plus `.tmp/golang-migration-qa-summary.{json,md}`. Treat old `.tmp` reports as stale unless produced by the current run.

Direct registry run:

```sh
testing/e2e/run-playwright-authenticated-game-visual-e2e.sh
```

## Coverage Index

Detailed coverage is split by topic:

- [Account, Security, and Admin](./COVERAGE-account-admin.md)
- [Gameplay and Economy](./COVERAGE-gameplay.md)
- [Migration Visual Equivalence](./COVERAGE-migration.md)
- [Navigation Visual Coverage](./COVERAGE-navigation-visual.md)
- [Authenticated Game Visual Coverage](./COVERAGE-authenticated-game-visual.md)
- [Public Auth Dynamic Coverage](./COVERAGE-public-auth-dynamic.md)
- [Dynamic Legacy JS Coverage](./COVERAGE-dynamic-legacy-js.md)
- [Bot Migration Coverage](./COVERAGE-bot-migration.md)
- [Dynamic Link Parity](./COVERAGE-dynamic-link-parity.md)
- [Infrastructure and Invariants](./COVERAGE-infra.md)
- [Legacy Behavior Surface](./COVERAGE-legacy-behavior-surface.md)
- [PHP/Go Differential Coverage](./COVERAGE-differential.md)

Navigation visual scans seeded public/game/admin links and compares Chromium/Firefox targets at exact threshold `0`; its combined report updates `COVERAGE-navigation-visual.md`.

## Notes

The suite mutates the local Docker database. Fixture scripts must either remove their rows or leave them in a valid state. DB invariant audit is intentionally strict and should not be weakened to hide stale fixture state.
