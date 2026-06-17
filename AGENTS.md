# AGENTS

## Scope

This branch starts a staged migration from the legacy PHP game to:

- Frontend: React 19, Bun 1.3.
- Backend: Go 1.25, native `net/http`.
- Compatibility oracle: the existing Docker PHP app and `testing/e2e`.

Do not remove or weaken legacy behavior while porting. Each migrated flow must be checked against the existing E2E coverage or a new compatibility case.

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
- Reuse legacy visual assets during the transition.
- Keep screens dense and game-operational, not marketing-oriented.

## Markdown Limit

Keep every Markdown file at or below 4KB. If a document needs more detail, split it by topic and link the parts from a short index.
