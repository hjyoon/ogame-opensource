# E2E Coverage: Infrastructure And Invariants

Keep this file under 4KB. Add a new topic file when this grows.

## Install And Routing

- Docker auto-install must skip installer screens when environment variables provide master and universe DB settings.
- Root/login pages, post-login redirects, resource URLs, internal assets, and public host behavior are covered.
- Public host strict mode can fail loopback host leaks:

```sh
OGAME_E2E_STRICT_PUBLIC_HOST=1 testing/e2e/run-docker-e2e.sh
```

## Data Integrity

The DB invariant audit checks:

- core schema migrations and hot-path indexes
- non-negative resources, buildings, ships, defenses, fields, research, fleet cargo, and queue levels
- fleet owner/origin/target references
- queue/fleet/buildqueue consistency
- planet coordinate uniqueness
- alliance, buddy, message, report, and template references
- self-cleaning fixture user isolation
- stale fleet lock files and battle scratch cleanup

Do not weaken these checks to hide fixture leftovers. Fix cleanup or the underlying game behavior.

## Maintenance And Recovery

- Removed-planet cleanup recalls inbound fleets before deletion.
- Debris cleanup preserves active recycler targets and removes inactive empty debris.
- User state timers, disabled-player cleanup, score recalculation, old-score snapshots, and global maintenance queue scheduling are covered.
- Cron resilience covers browser access denial, due task idempotency, unknown task debug auditing, frozen task preservation, and universe-freeze queue blocking.
- Fresh-process queue recovery and bounded-load rendering are covered through recovery/bulk journey tests.

## Final Results

Legacy PHP E2E writes authoritative summaries inside the `server` container:

- `/tmp/ogame-e2e-results/summary.md`
- `/tmp/ogame-e2e-results/summary.json`

Go/React migration QA writes local artifacts under `.tmp`, including:

- `.tmp/golang-compat-smoke.json`
- `.tmp/golang-user-type-qa.json`
- `.tmp/playwright-*/<browser>/report.json`

Old `.tmp` reports are not authoritative for the current run. Use timestamps or rerun the wrapper when in doubt.
