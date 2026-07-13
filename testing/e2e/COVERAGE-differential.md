# PHP/Go Differential Coverage

Keep this file under 4KB. Differential cases start from the same DB state, run
one legacy PHP action, restore, run the equivalent Go action, compare normalized
HTTP/DB effects, and restore again.

## Current Cases

| Domain | Cases | Compared Effects | Status |
| --- | ---: | --- | --- |
| Resource production | 3 | HTTP status, `prod1/2/3/4/12/212` | PASS |
| Building queue | 2 | debit/refund, normalized queues, duration, completion, level/fields/score | PASS |
| Research queue | 2 | debit/refund, duration/priority, completion, level/score/ranks | PASS |
| Shipyard/defense | 2 | batch debit, per-unit completion, queue timing, units/score/ranks | PASS |
| Advanced building | 2 | demolition, Commander queue cancel/level shift/propagation | PASS |
| Defense limits | 3 | dome uniqueness, missile capacity, mixed queue order/completion | PASS |
| Fleet lifecycle | 11 | transport/recall/deploy/recycle, attack/repair, ACS hold/attack/recall | PASS |
| Combat engine | 4 | outcome, shots/power, absorption, survivors | PASS |

Resource cases cover partial production, 0-100 boundary values, and all-100
production. Each side logs in immediately before its action because login rotates
the private session cookie.

Run:

```sh
testing/e2e/run-golang-resource-differential-e2e.sh
testing/e2e/run-golang-building-differential-e2e.sh
testing/e2e/run-golang-research-differential-e2e.sh
testing/e2e/run-golang-shipyard-defense-differential-e2e.sh
testing/e2e/run-golang-building-advanced-differential-e2e.sh
testing/e2e/run-golang-defense-limits-differential-e2e.sh
testing/e2e/run-golang-fleet-differential-e2e.sh
testing/e2e/run-golang-acs-attack-differential-e2e.sh
testing/e2e/run-golang-combat-engine-differential-e2e.sh
```

Reports use `.tmp/golang-*-differential.json`. Queue cases
normalize generated IDs/timestamps, preserve duration, and restore the original
planet, score/ranks, queue, log, production, vacation, and premium state. Fixtures
come from `prepare-golang-smoke-fixture.php`; the wrapper runs differential QA
after Go compatibility smoke and before fixture cleanup.
The combat engine oracle exact-compares four deterministic round outcomes, shot
totals, absorbed power, and survivors. Guarded DB cases also compare repair,
losses, debris, report HTML/link messages, planet units, scores, and cleanup.

## Expansion Order

1. ACS holding defenders and moon creation/destruction.
2. Colony, missiles, expedition, phalanx and Jump Gate.
3. Account, alliance, messages, buddy and Admin mutations.

## Comparison Contract

- Compare status, redirect/cookie semantics, selected response fields, changed DB
  rows, queue rows, reports/messages/mail, and next scheduler effects.
- Normalize only generated IDs, session secrets, wall-clock values, and seeded
  randomness explicitly listed by a case.
- A scope decision or an existing E2E is not automatically differential proof.
- Failed cases must leave the DB restored through a trap/finalizer.

## Completion Rule

Differential completion requires every core state-changing behavior group in the
legacy behavior baseline to have at least one normal, boundary, rejection, and
completion/rollback case where applicable. Optional PHP Mods remain excluded.
