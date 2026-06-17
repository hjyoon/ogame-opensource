# AGENTS

## Scope

This branch starts a staged migration from the legacy PHP game to:

- Frontend: React 19, Bun 1.3.
- Backend: Go 1.25, native `net/http`.
- Compatibility oracle: the existing Docker PHP app and `testing/e2e`.

Do not remove or weaken legacy behavior while porting. Each migrated flow must be checked against the existing E2E coverage or a new compatibility case.

## Migration Interpretation

Do not translate PHP files one-for-one. Reinterpret APIs, state, and modules naturally for React and Go. New routes do not need `.php` suffixes. Preserve legacy URLs only as compatibility entry points.

Visible page composition is not free-form: every migrated public, game, and admin page must match the corresponding legacy PHP screen layout, skin, table density, labels, and asset choices unless a documented compatibility exception exists.

Game mechanics are different: resource math, timings, combat, queues, economy, targeting, reports, and permissions must behave exactly like the legacy game. Prove equivalence with focused unit tests plus E2E checks against the PHP oracle.

## Architecture Rule

All newly migrated code must follow Clean Architecture. This is mandatory, not advisory.

- Domain rules must not depend on HTTP, SQL, React, files, clocks, or external services.
- Application/use-case code coordinates domain rules through explicit interfaces.
- Infrastructure code implements those interfaces for MySQL, HTTP, files, mail, queues, and legacy adapters.
- Delivery code is only transport/UI: Go handlers, React components, request parsing, response shaping.
- Dependencies must point inward: `delivery -> application -> domain`; infrastructure is wired at the edge.
- Do not put game rules in React components, HTTP handlers, SQL rows, or migration glue.
- New ports should be covered with unit tests at the domain/application layer and E2E compatibility tests at the boundary.

## Layout

- `backend/internal/domain`: pure game rules and value objects.
- `backend/internal/application`: use cases and ports.
- `backend/internal/infrastructure`: MySQL, files, runtime, and legacy adapters.
- `backend/internal/delivery`: HTTP handlers and other delivery adapters.
- `frontend/`: React shell built with Bun.
- `game/`, `wwwroot/`, `download/`: legacy runtime and assets.
- `testing/e2e/`: regression suite. Prefer extending this before replacing it.

## QA Rules

Use the existing PHP E2E suite as the baseline:

```sh
testing/e2e/run-docker-e2e.sh
```

For migration smoke checks:

```sh
testing/e2e/run-golang-migration-qa.sh
```

Set `OGAME_RUN_LEGACY_E2E=0` only for local frontend/backend smoke work. Do not use that skip for final validation of migrated game behavior.
Reuse existing E2E behavior as the oracle: full PHP E2E stays on the legacy stack, and HTTP black-box checks are ported to Go with the same JSON result shape.

Go internal package coverage must stay at or above 97%:

```sh
backend/scripts/test-coverage.sh
```

## Backend Rules

- Use Go 1.25.
- Use the standard library HTTP stack first: `net/http`, `http.ServeMux`, `httptest`.
- Serve the React production build from Go. Bun is the build tool, not the runtime server.
- Runtime logs must be JSON. Use Go `log/slog` JSON handlers at the edge.
- Keep route handlers small and push game rules into package-level services.
- Preserve legacy URLs until a compatibility redirect or replacement is covered by tests.

## Frontend Rules

- Use Bun 1.3 commands and lockfiles.
- Use React 19.
- Reuse legacy visual assets and page composition during the transition.
- Keep screens dense and game-operational; do not replace legacy screens with marketing or console-style UI.

## Markdown Limit

Keep every Markdown file at or below 4KB. If a document needs more detail, split it by topic and link the parts from a short index.

## Status Tracking

Keep `MIGRATION_STATUS.md` current. Update it with every migration milestone, QA result, skipped validation, and remaining-work change.
