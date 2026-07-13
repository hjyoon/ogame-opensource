# Admin Differential Coverage

Keep this under 4KB. PHP and Go start each case from the same restored DB state.
Generated IDs and wall-clock deltas are normalized; durable effects remain exact.

## Verified Groups

| Group | Cases | Exact Effects | Status |
| --- | ---: | --- | --- |
| Bans | 8 | access, ban/VM/attack flags, queues, pranger, scores and all ranks | PASS |
| Queue/Fleetlogs | 13 | access, complete/delete/freeze/unfreeze, single/ACS timing, recall fleet/queue/logs and retention | PASS |
| **Total** | **21** | deterministic PHP/Go DB and HTTP contracts | **PASS** |

Queue/Fleetlogs cases include missing-target no-ops and operator rejection. Recall
compares cargo, ships, fuel, mission, origin/target, duration, priority, fleetlog,
userlog text, and legacy two/four-week log cleanup. Only generated fleet/task IDs
and bounded request-time deltas are normalized.

## Pending Groups

- Broadcast, Reports, Expedition settings, Battle/Rocket simulators.
- Universe settings and CRON execution.
- Users and Planets full edit/create/destroy operations.
- Debug, Errors, UserLogs, Browse/Logins filters and cleanup actions.
- Database backup/restore/delete, Coupons, Bots/BotEdit, Loca, checksums and colony settings.

PHP Mods are excluded by project policy. A group is removed from this list only
after normal, boundary, rejection, no-op and rollback/completion cases are added
where applicable.

## Commands

```sh
testing/e2e/run-golang-admin-bans-differential-e2e.sh
testing/e2e/run-golang-admin-queue-differential-e2e.sh
```

Both are included in `testing/e2e/run-golang-migration-qa.sh`.
