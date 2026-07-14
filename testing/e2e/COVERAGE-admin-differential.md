# Admin Differential Coverage

Keep this under 4KB. PHP and Go start each case from the same restored DB state.
Generated IDs and wall-clock deltas are normalized; durable effects remain exact.

## Verified Groups

| Group | Cases | Exact Effects | Status |
| --- | ---: | --- | --- |
| Bans | 8 | access, ban/VM/attack flags, queues, pranger, scores and all ranks | PASS |
| Queue/Fleetlogs | 13 | access, complete/delete/freeze/unfreeze, single/ACS timing, recall fleet/queue/logs and retention | PASS |
| Operations | 16 | broadcast recipients/BBCode/cap, report deletion, all expedition settings | PASS |
| Universe | 9 | all settings/links/news/max users, freeze VM effects, access | PASS |
| Audit/search | 20 | Debug/Errors windows, filters, UserLogs periods/types, Logins/Browse order | PASS |
| Coupons | 12 | list order, create/delete, periodic queue values/timing, access | PASS |
| Database | 11 | create/restore lifecycle, delete, invalid/partial/path guards, access | PASS |
| **Total** | **89** | deterministic PHP/Go DB and HTTP contracts | **PASS** |

Queue/Fleetlogs cases include missing-target no-ops and operator rejection. Recall
compares cargo, ships, fuel, mission, origin/target, duration, priority, fleetlog,
userlog text, and legacy two/four-week log cleanup. Only generated fleet/task IDs
and bounded request-time deltas are normalized.

Operations covers all recipient categories, empty/whitespace fields, every legacy
broadcast BBCode family, the 127-message cap, marked/all/empty reports, all 33
expedition settings, malformed partial requests and access rejection.

Universe covers every mutable field, empty strings, news update/disable ordering,
max-user zero preservation, freeze/unfreeze, active-user VM forcing and rejection.
Audit/search exact-compares marker order and post-action table state without normalization.
Coupons preserves legacy unsigned failure, Moscow `mktime`, packed signed criteria and unrestricted queue removal.
Database runs one non-empty backup at a time, restores a post-backup marker, and
deletes the file immediately. Go rejects incomplete schemas before destructive SQL.

## Pending Groups

- Battle/Rocket/Expedition simulator calculations.
- CRON execution.
- Users and Planets full edit/create/destroy operations.
- Bots/BotEdit, Loca, checksums and colony settings.

PHP Mods are excluded by project policy. A group is removed from this list only
after normal, boundary, rejection, no-op and rollback/completion cases are added
where applicable.

## Commands

```sh
testing/e2e/run-golang-admin-bans-differential-e2e.sh
testing/e2e/run-golang-admin-queue-differential-e2e.sh
testing/e2e/run-golang-admin-operations-differential-e2e.sh
testing/e2e/run-golang-admin-universe-differential-e2e.sh
testing/e2e/run-golang-admin-audit-differential-e2e.sh
testing/e2e/run-golang-admin-coupons-differential-e2e.sh
testing/e2e/run-golang-admin-database-differential-e2e.sh
```

All are included in `testing/e2e/run-golang-migration-qa.sh`.
