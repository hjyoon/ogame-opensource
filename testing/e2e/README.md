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
- Account security flows for private-session cookie enforcement, public/private session rotation, existing-session admin downgrade, logout invalidation, password change and re-login, email validation, and account deletion schedule/cancel.
- Localization and force-language edge flows for missing translation fallback, user language option persistence, forced universe language override, and invalid user-language fallback.
- Password recovery flows for the forgot-password form, missing/unknown email rejection, permanent/temporary email lookup, MailHog delivery, old password invalidation, and recovered-password login.
- Registration validation flows for `new.php`/`newredirect.php` input rejection, duplicate username/email handling, missing-field hardening, welcome-mail delivery, and activation-link verification.
- Message and report lifecycle flows for inbox read state, selected/displayed deletion, PM operator reports, report popup access control, deleted report links, and expiry cleanup.
- Report retention and ownership edge flows for reported PM audit-row retention after source deletion, crafted foreign report POST rejection, owner-scoped expiry cleanup, and admin/operator message retention.
- Fleet template and galaxy action flows for Commander template access, template create/update/delete limits, template use on fleet dispatch screens, galaxy quick-action links, remote-system deuterium cost, IPM form opening, and AJAX spy/recycle dispatch.
- Target restriction flows for noob/strong score protection, vacation targets, operator/admin targets, temporary attack bans, Galaxy AJAX espionage errors, and IPM restriction handling.
- Planet context flows for owned/foreign/missing `cp` selection, moon selection, per-planet resource/build queue isolation, spoofed fleet-origin rejection, and colony abandon fallback.
- Planet cleanup edge flows for removed-planet cleanup recalling inbound fleets before deletion and debris cleanup preserving active recycler targets while removing inactive empty debris.
- Social and access-control flows for alliance creation/application/acceptance/leave/dismiss, buddy request/reject/accept/delete, unauthenticated private-page redirects, report ownership, note ownership, and foreign-planet build attempts.
- Cross-user IDOR sweeps for message deletion/reporting, foreign `cp` resource-setting and missile-silo demolition attempts, and direct foreign planet deletion attempts.
- Input hardening sweeps for malformed numeric POST fields in resource settings, options, shipyard orders, missile demolition, fleet dispatch, and AJAX quick dispatch.
- Alliance management flows for rank creation/rights/assignment/deletion, direct-URL permission denial, rank-scoped circular messages, and alliance text/settings updates.
- Admin and account-state flows for admin-area access control, operator write restrictions, admin user updates, ban/unban login blocking, and vacation-mode action blocking.
- Admin permission matrix edge flows for regular-user denial across admin modes and operator-vs-admin mutation boundaries for queue controls, universe settings, coupon creation, and planet actions.
- Admin audit/log flows for UserLogs, Debug, Errors, Browse, Logins, Fleetlogs rendering, seeded audit marker visibility, regular-user denial, and operator delete-boundary checks.
- Admin tool smoke flows for Bots, BotEdit, Mods, Checksum, and DB pages, including regular-user denial and checksum baseline rendering.
- Coupon and Dark Matter payment flows for admin coupon creation/listing/deletion, invalid/used coupon rejection, paid-DM redemption, duplicate redemption prevention, and periodic coupon queue creation/removal.
- Queue and fleet validation flows for building/research/shipyard queue create/cancel/complete, admin queue freeze/unfreeze/remove, active-queue vacation blocking, transport launch, and rejected fleet sends.
- Queue/event idempotency flows for repeated `UpdateQueue()` runs across building, research, shipyard, transport fleet arrival/return completion, recalc-points, and a multi-day long scheduler drain.
- Vacation/freeze timing edge flows for vacation enable rejection with active build/fleet queues, vacation-mode build/shipyard mutation blocking, and universe-freeze pause/resume behavior for due queues.
- Global maintenance queue flows for user state timers, score recalculation, old-score snapshots, debris cleanup, removed-planet cleanup, and disabled-player cleanup.
- Concurrency/race-condition flows for parallel building, research, shipyard, and fleet-dispatch requests so repeated clicks or multi-tab requests cannot duplicate queues, overspend resources, or duplicate fleet rows.
- Technology unlock gates and economy edge cases for building/research/shipyard requirements, energy-shortage production, storage caps, and production-ratio ticks.
- Statistics and ranking flows for recalculated asset scores, queue-completion score adjustments, rank ordering, and statistics page rendering.
- Fleet lifecycle flows for transport delivery/return, deploy arrival, and recalled transport return.
- Fleet recall edge flows for invalid fleet ids, foreign fleet recall rejection, already-returning/completed fleet recall no-ops, transport/deploy cargo returns, and orbiting ACS hold recall.
- ACS and hold flows for unauthorized hold rejection, buddy hold orbit/return, ACS union creation, invited participant join, battle resolution, and return.
- Alliance Depot ACS fuel supply flows for hold-fleet rendering, successful hold extension with deuterium spending, and no-op handling without a depot or enough fuel.
- Admin destructive flows for queue freeze/unfreeze/complete/delete controls, account deletion scheduling, admin planet creation, universe freeze, and admin-triggered score recalculation.
- Trader, premium officer, and moon-tool flows for merchant calls/exchanges, officer purchase, paid/free DM spending order, insufficient/invalid premium purchases, lunar base construction, jump gates, and phalanx scans.
- Jump Gate edge flows for target filtering, invalid source/target moons, missing gates, foreign moons, empty/oversized ship selections, cooldown direct-POST rejection, same-moon rejection, and solar-satellite exclusion.
- Battle reports and espionage reports.
- Plunder, debris creation, debris recycling, resource return, and defense writeback.
- Interplanetary missile and anti-ballistic missile cases.
- Computer technology fleet-slot limits.
- Colony ship colonization success and failure paths.
- Moon creation, moon destruction, and moon-destruction failure paths.
- Expedition flow and expedition result cases.
- Database invariant audit coverage for non-negative resources/counts, gameplay-critical orphaned references, queue/fleet/buildqueue consistency, coordinate uniqueness, alliance/buddy references, fixture runtime cleanup, and stale fleet lock files.

## Public Host Strict Mode

By default, the HTTP test records login redirect host behavior without failing a local Docker setup whose `StartPage` is `http://localhost:8888`.

To fail when login redirects or page resources leak loopback hosts, run:

```sh
OGAME_E2E_STRICT_PUBLIC_HOST=1 testing/e2e/run-docker-e2e.sh
```

## Notes

These are integration/E2E tests, not PHPUnit unit tests. They mutate the local Docker database while running and restore or remove their own fixture data afterward.
