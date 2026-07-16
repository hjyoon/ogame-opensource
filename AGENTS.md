# AGENTS

## Scope

This branch maintains the migration from legacy PHP to:

- Frontend: React 19/Bun 1.3.
- Backend: Go 1.25 `net/http`.
- Oracle: Docker PHP app and `testing/e2e`.

Do not weaken legacy behavior. Check each migrated flow against existing E2E or a new compatibility case.

## Migration Interpretation

Do not translate PHP files one-for-one. Reinterpret APIs, state, and modules naturally for React and Go. New routes do not need `.php` suffixes. Preserve legacy URLs only as compatibility entry points.

Visible pages must match legacy layout, skin, density, labels, and assets unless an exception is documented. Preserve parity first; record later cleanup in `MODERNIZATION_OPTIONS.md`.

Resource math, timings, combat, queues, economy, targeting, reports, and permissions must match legacy. Prove this with unit tests and PHP-oracle E2E.

## Architecture Rule

Follow Clean Architecture.

- Domain rules must not depend on HTTP, SQL, React, files, clocks, or external services.
- Application/use-case code coordinates domain rules through explicit interfaces.
- Infrastructure code implements those interfaces for MySQL/SQLite, HTTP, files, mail, queues, and legacy adapters.
- Delivery code is only transport/UI: Go handlers, React components, request parsing, response shaping.
- Dependencies point inward: `delivery -> application -> domain`; infrastructure is wired at the edge.
- Do not put game rules in React components, HTTP handlers, SQL rows, or migration glue.
- Cover new ports with domain/application unit tests and boundary E2E compatibility tests.

## Layout

- `backend/internal/domain`: pure game rules and value objects.
- `backend/internal/application`: use cases and ports.
- `backend/internal/infrastructure`: MySQL, files, runtime, legacy adapters.
- `backend/internal/delivery`: HTTP handlers and other delivery adapters.
- `frontend/`: React shell built with Bun.
- `game/`, `wwwroot/`, `download/`: legacy runtime and assets.
- `testing/e2e/`: regression suite. Prefer extending this before replacing it.

## QA Rules

Legacy PHP E2E is the baseline:

```sh
testing/e2e/run-docker-e2e.sh
```

Final migration QA:

```sh
OGAME_RUN_LEGACY_E2E=1 OGAME_GO_PORT=8890 OGAME_KEEP_GO_DOCKER=1 testing/e2e/run-golang-migration-qa.sh
```

Keep PHP as oracle; keep one current Go `goapp` container only.

During page migration, extend Playwright visual E2E before claiming parity. Public: `testing/e2e/run-playwright-visual-e2e.sh`; auth: `testing/e2e/run-playwright-auth-visual-e2e.sh`. Keep exact diff enforced unless an exception is documented.

Use `OGAME_RUN_LEGACY_E2E=0` only for local smoke work. Port HTTP black-box checks to Go with the same JSON shape.

Behavior baseline drift must be reviewed, never blindly regenerated. Add snapshot/restore PHP-Go differential cases for state-changing ports; compare HTTP, DB, queue, and report effects.

Go internal package coverage must stay at or above 97%:

```sh
backend/scripts/test-coverage.sh
```

## Backend Rules

- Use Go 1.25.
- Use the standard library HTTP stack first: `net/http`, `http.ServeMux`, `httptest`.
- Serve the React production build from Go. Bun is the build tool, not the runtime server.
- Runtime logs must be JSON. Use Go `log/slog` JSON handlers at the edge.
- Keep repository SQL portable across MySQL and SQLite or isolate differences in infrastructure dialect adapters.
- Keep route handlers small and push game rules into package-level services.
- Preserve legacy URLs until a compatibility redirect or replacement is covered by tests.

## Frontend Rules

- Use Bun 1.3 commands and lockfiles.
- Use React 19.
- Reuse legacy visual assets and page composition during the transition.
- Keep screens dense and game-operational; do not replace legacy screens with marketing or console-style UI.

## Markdown Limit

Keep each Markdown file <=4KB; split larger docs by topic and link them from a short index.

## Status Tracking

Keep `MIGRATION_STATUS.md` current. Update it with every migration milestone, QA result, skipped validation, and remaining-work change.
