# Admin Differential Coverage

Keep this under 4KB. PHP and Go start from the same restored DB state. Generated
IDs and wall-clock deltas are normalized; durable effects remain exact.

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
| Simulators | 20 | Rocket, Expedition and Battle form/result/report/message/debug semantics | PASS |
| CRON/cleanup/coupon | 12 | order/freeze/timers, cleanup, coupon mail and boundaries | PASS |
| Users | 8 | full edit, planet create/collision, stats, bot controls, reactivation SMTP | PASS |
| Planets | 13 | edit/delete, moon/debris, gates, fields/diameter, search | PASS |
| **Total** | **173** | deterministic PHP/Go DB, files and HTTP contracts | **PASS** |

Universe covers every mutable field, empty strings, news update/disable ordering,
max-user zero preservation, freeze/unfreeze, active-user VM forcing and rejection.
Audit/search exact-compares marker order and post-action table state without normalization.
Coupons preserves legacy unsigned failure, Moscow `mktime`, packed signed criteria and unrestricted queue removal.
Colony covers 15 values; Checksum 130 rows; Loca 2,090 rows.
Simulators compare every rendered Rocket value, all ten Expedition buckets, and
Battle attacker/defender/draw, defense, source import, rapid-fire, zero-round,
debug and Operator cases. Participant/source/battle/result diagnostics, report HTML,
link/losses, messages, retention and cleanup are exact after masking generated values.
CRON verifies PHP NULL coercion, planet/player cleanup and exemptions. Coupon CRON
covers one-off/periodic/frozen tasks, strict date bounds and localized SMTP.
Users covers every edit field, officer timers, rank recalculation, legacy bot queue
timing, generated colony invariants and normalized password/activation mail.
Planets exact-compares typed edits, home/colony deletion and queue cleanup, moon and
debris creation, gate timers, field/diameter recalculation, and both searches.

## Pending Groups

No concrete non-Mod Admin group remains in the registered inventory. New source
or state discovery must add a group before completion is reasserted.

PHP Mods are excluded by project policy. A group is removed from this list only
after normal, boundary, rejection, no-op and rollback/completion cases are added
where applicable.

## Commands

```sh
testing/e2e/run-golang-admin-planets-differential-e2e.sh
testing/e2e/run-golang-migration-qa.sh
```

All `run-golang-admin-*-differential-e2e.sh` runners are in the full QA gate.
