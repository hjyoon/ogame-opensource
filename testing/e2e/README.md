# Docker E2E Tests

These tests exercise the Docker-installed OGame runtime through real HTTP requests and in-container game queue handlers.

## Run

Start the stack first:

```sh
docker compose up -d --build
```

Then run:

```sh
testing/e2e/run-docker-e2e.sh
```

The wrapper copies `testing/e2e` into the `server` container, creates disposable fixture users, runs every case, and removes the fixture users at exit.

JSON result files are written inside the container under `/tmp/ogame-e2e-results`.

## Covered Areas

- Docker auto-install smoke check: the root page must not show the Master Database Settings installer.
- HTTP registration, login, core post-login pages, and internal asset URL checks.
- Battle reports and espionage reports.
- Plunder, debris creation, debris recycling, resource return, and defense writeback.
- Interplanetary missile and anti-ballistic missile cases.
- Computer technology fleet-slot limits.
- Colony ship colonization success and failure paths.
- Moon creation, moon destruction, and moon-destruction failure paths.
- Expedition flow and expedition result cases.

## Public Host Strict Mode

By default, the HTTP test records login redirect host behavior without failing a local Docker setup whose `StartPage` is `http://localhost:8888`.

To fail when login redirects or page resources leak loopback hosts, run:

```sh
OGAME_E2E_STRICT_PUBLIC_HOST=1 testing/e2e/run-docker-e2e.sh
```

## Notes

These are integration/E2E tests, not PHPUnit unit tests. They mutate the local Docker database while running and restore or remove their own fixture data afterward.
