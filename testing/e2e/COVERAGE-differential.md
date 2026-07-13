# PHP/Go Differential Coverage

Keep this file under 4KB. Differential cases start from the same DB state, run
one legacy PHP action, restore, run the equivalent Go action, compare normalized
HTTP/DB effects, and restore again.

## Current Cases

| Domain | Cases | Compared Effects | Status |
| --- | ---: | --- | --- |
| Resource production | 3 | HTTP status, `prod1/2/3/4/12/212` | PASS |

Resource cases cover partial production, 0-100 boundary values, and all-100
production. Each side logs in immediately before its action because login rotates
the private session cookie.

Run:

```sh
testing/e2e/run-golang-resource-differential-e2e.sh
```

The report is `.tmp/golang-resource-differential.json`. The fixture is generated
by `prepare-golang-smoke-fixture.php`; the full migration wrapper runs the case
after Go compatibility smoke.

## Expansion Order

1. Building/research queue create, cancel, resource debit/refund, completion.
2. Shipyard/defense batch queue and per-unit completion.
3. Fleet dispatch/recall, slots, fuel, cargo, arrival and return.
4. Combat, plunder, debris, repair, reports, moon creation/destruction.
5. Colony, missiles, expedition, phalanx and Jump Gate.
6. Account, alliance, messages, buddy and Admin mutations.

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
