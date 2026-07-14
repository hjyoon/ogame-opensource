# PHP/Go Differential Coverage

Keep this under 4KB. Cases start from one DB snapshot, run PHP and Go separately,
compare normalized HTTP/DB effects, then restore.

## Current Cases

| Domain | Cases | Compared Effects | Status |
| --- | ---: | --- | --- |
| Resource production | 3 | HTTP status, `prod1/2/3/4/12/212` | PASS |
| Building queue | 2 | debit/refund, normalized queues, duration, completion, level/fields/score | PASS |
| Research queue | 2 | debit/refund, duration/priority, completion, level/score/ranks | PASS |
| Shipyard/defense | 2 | batch debit, per-unit completion, queue timing, units/score/ranks | PASS |
| Advanced building | 2 | demolition, Commander queue cancel/level shift/propagation | PASS |
| Defense limits | 3 | dome uniqueness, missile capacity, mixed queue order/completion | PASS |
| Fleet lifecycle | 23 | transport/recall/deploy/recycle, combat, moon, colony and missile lifecycle | PASS |
| Expedition | 10 | every result family, rewards/loss, combat reports, timing, logs | PASS |
| Phalanx | 11 | guards, debit, fleet visibility, ACS hold/grouping, state restore | PASS |
| Jump Gate | 13 | target filters, guards, ship move, satellite exclusion, cooldown | PASS |
| Buddy | 12 | request views, guards, text limits, lifecycle, PM side effects | PASS |
| Messages | 18 | read/retention, delete modes, reports, flags, send/cap | PASS |
| Alliance | 27 | create/apply/review, ranks, text/settings, rename, leave/kick/dismiss/transfer | PASS |
| Account options | 36 | settings, identity/mail, activation, vacation/deletion, Commander/feed/operator flags | PASS |
| Admin | 172 | [Mutation matrix](COVERAGE-admin-differential.md) | PASS |
| Combat engine | 4 | outcome, shots/power, absorption, survivors | PASS |

Resource cases cover partial production and 0/100 boundaries. Each side logs in
before its action because login rotates the private cookie.

Run all cases through `testing/e2e/run-golang-migration-qa.sh`, or run an
individual `testing/e2e/run-golang-*-differential-e2e.sh` script. For example:

```sh
testing/e2e/run-golang-jump-gate-differential-e2e.sh
```

Reports use `.tmp/golang-*-differential.json`. Queue cases normalize generated
IDs/timestamps but preserve duration. Fixtures come from
`prepare-golang-smoke-fixture.php`; every script restores changed state.
The combat engine oracle exact-compares four deterministic round outcomes, shot
totals, absorbed power, and survivors. Guarded DB cases also compare repair,
losses, debris, report HTML/link messages, planet units, scores, and cleanup.
Moon creation retries a deterministic 20% opportunity to success on each runtime;
only generated diameter/temperature are range-checked instead of exact-compared.
Colonization exact-compares success/return, consumed ship, occupied-race and
nine-planet-limit states; random diameter/temperature are range-checked.
Missile QA exact-compares full/partial interception, targeted and sweep damage,
moon ABM use, defense state, rank effects, cleanup, and legacy report HTML.

## Expansion Order

1. Battle simulator debug and post-action visual states.

## Comparison Contract

- Compare status, redirect/cookie semantics, selected response fields, changed DB
  rows, queue rows, reports/messages/mail, and next scheduler effects.
- Normalize only generated IDs, session secrets, wall-clock values, and seeded
  randomness explicitly listed by a case.
- A scope decision or an existing E2E is not automatically differential proof.
- Failed cases must leave the DB restored through a trap/finalizer.
- Founder transfer scopes owner updates to its alliance; PHP's unscoped global update is a documented security correction.

## Completion Rule

Differential completion requires every core state-changing behavior group in the
legacy behavior baseline to have at least one normal, boundary, rejection, and
completion/rollback case where applicable. Optional PHP Mods remain excluded.
