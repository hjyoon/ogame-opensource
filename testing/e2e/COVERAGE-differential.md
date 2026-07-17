# PHP/Go Differential Coverage

Keep this under 4KB. Cases start from one DB snapshot, run PHP and Go separately,
compare normalized HTTP/DB effects, then restore.

## Current Cases

| Domain | Cases | Compared Effects | Status |
| --- | ---: | --- | --- |
| Public/view/economy | 69 | account, overview, officers, merchant, payment, empire, galaxy | PASS |
| Economy queues | 14 | resources, building/research/shipyard, demolition, defense limits | PASS |
| Fleet/colony/ACS | 24 | launch/recall/deploy/recycle/spy arrival, templates, colony, ACS/holding | PASS |
| Expedition | 10 | every result family, rewards/loss, combat reports, timing, logs | PASS |
| Missile/moon | 7 | interception/damage/report plus moon creation/destruction | PASS |
| Phalanx/Jump Gate | 24 | guards, debit, visibility, ship move, cooldown | PASS |
| Social | 39 | buddy, messages, notes, reports, caps and side effects | PASS |
| Alliance | 27 | create/apply/review, ranks, text/settings, rename, leave/kick/dismiss/transfer | PASS |
| Account options | 36 | settings, identity/mail, activation, vacation/deletion, Commander/feed/operator flags | PASS |
| Admin/runtime | 180 | [Admin 173](COVERAGE-admin-differential.md) plus runtime queue 7 | PASS |
| Combat engine | 4 | outcome, shots/power, absorption, survivors | PASS |
| **Total** | **434** | **48 differential result groups** | **PASS** |

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

No concrete non-Mod behavior group remains registered. Expand whenever source
audit, route discovery, or production reproduction finds a new state.

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
