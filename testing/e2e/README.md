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
- Public and authenticated route matrix coverage for main game pages, including pages that need minimal GET/POST parameters.
- Render and asset smoke coverage for public/login pages and core authenticated pages, including referenced CSS/JS/image resources.
- Account action flows for notes, private messages, planet rename, options, resource settings, and building enqueue.
- Social and access-control flows for alliance creation/application/acceptance/leave/dismiss, buddy request/reject/accept/delete, unauthenticated private-page redirects, report ownership, note ownership, and foreign-planet build attempts.
- Admin and account-state flows for admin-area access control, operator write restrictions, admin user updates, ban/unban login blocking, and vacation-mode action blocking.
- Queue and fleet validation flows for building/research/shipyard queue create/cancel/complete, admin queue freeze/unfreeze/remove, active-queue vacation blocking, transport launch, and rejected fleet sends.
- Technology unlock gates and economy edge cases for building/research/shipyard requirements, energy-shortage production, storage caps, and production-ratio ticks.
- Fleet lifecycle flows for transport delivery/return, deploy arrival, and recalled transport return.
- ACS and hold flows for unauthorized hold rejection, buddy hold orbit/return, ACS union creation, invited participant join, battle resolution, and return.
- Trader, premium officer, and moon-tool flows for merchant calls/exchanges, officer purchase, lunar base construction, jump gates, and phalanx scans.
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
