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
OGAME_RUN_LEGACY_E2E=1 OGAME_GO_PORT=8895 OGAME_KEEP_GO_DOCKER=1 testing/e2e/run-golang-migration-qa.sh
```

This wrapper runs:

- full legacy PHP Docker E2E
- Go smoke fixture preparation
- frontend `bun install`, build, typecheck, and unit tests
- backend tests plus the 97% internal coverage gate
- Go Docker app rebuild/start
- Go compatibility smoke and user-type API QA
- Chromium/Firefox Playwright CSR and visual equivalence checks
- overview/fleet deep visual and click-contract checks

Do not set `OGAME_RUN_LEGACY_E2E=0` for final validation. It is only for local smoke work while iterating on frontend/backend code.

## Visual Parity

Default final visual checks enforce exact parity where the scripts support it:

- auth visual pages: Chromium/Firefox
- empire and alliance visual pages: Chromium/Firefox
- overview fleet view and countdown behavior
- overview all-cases event surface and clicks
- fleet continue and fleet all-cases dispatch previews

Reports are written under `.tmp/playwright-*`. Treat old `.tmp` reports as stale unless they were produced by the current run.

## Coverage Index

Detailed coverage is split by topic:

- [Account, Security, and Admin](./COVERAGE-account-admin.md)
- [Gameplay and Economy](./COVERAGE-gameplay.md)
- [Migration Visual Equivalence](./COVERAGE-migration.md)
- [Infrastructure and Invariants](./COVERAGE-infra.md)

## Notes

The suite mutates the local Docker database. Fixture scripts must either remove their rows or leave them in a valid state. DB invariant audit is intentionally strict and should not be weakened to hide stale fixture state.
