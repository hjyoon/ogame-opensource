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
| Bots/BotEdit | 18 | list/add/stop, names, strategy CRUD/import/export, access | PASS |
| Colony/Checksum/Loca | 13 | all colony values, serialized baselines, all localization rows/order/colors | PASS |
| Simulators | 19 | Rocket, Expedition and Battle form/result/report/message semantics | PASS |
| **Total** | **139** | deterministic PHP/Go DB, files and HTTP contracts | **PASS** |

Queue/Fleetlogs covers no-ops, access, recall state/logs and retention. Only
generated fleet/task IDs and bounded request-time deltas are normalized.

Operations covers recipients, BBCode, message cap, reports, all 33 expedition
settings, malformed requests and access rejection.

Universe covers every mutable field, empty strings, news update/disable ordering,
max-user zero preservation, freeze/unfreeze, active-user VM forcing and rejection.
Audit/search exact-compares marker order and post-action table state without normalization.
Coupons preserves legacy unsigned failure, Moscow `mktime`, packed signed criteria and unrestricted queue removal.
Database runs one non-empty backup at a time, restores a post-backup marker, and
deletes the file immediately. Go rejects incomplete schemas before destructive SQL.
Bots compares account, planet, IP log, variables, AI queue, strategy source and
user-count effects; generated IDs, passwords, IPs, times and temperature are normalized.
Colony covers all 15 values and unsigned boundaries. Checksum compares all 130 rows
and exact PHP serialization. Loca compares 2,090 English rows and missing JP files.
Simulators compare every rendered Rocket value, all ten Expedition buckets, and
Battle attacker/defender/draw, defense, source import, rapid-fire, zero-round and
Operator cases. Battle report HTML, link style/losses, message metadata, retention
count and battledata cleanup are exact after masking time, random coordinates and IDs.

## Pending Groups

- Battle simulator debug diagnostics and post-action screenshot state.
- CRON execution.
- Users and Planets full edit/create/destroy operations.

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
testing/e2e/run-golang-admin-bots-differential-e2e.sh
testing/e2e/run-golang-admin-colony-settings-differential-e2e.sh
testing/e2e/run-golang-admin-checksum-differential-e2e.sh
testing/e2e/run-golang-admin-loca-differential-e2e.sh
testing/e2e/run-golang-admin-simulators-differential-e2e.sh
```

All are included in `testing/e2e/run-golang-migration-qa.sh`.
