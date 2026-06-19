# Migration Status

Updated: 2026-06-19 KST, branch `hjyoon/golang`.

React 19/Bun 1.3 + Go 1.25 native `net/http` tracker. Keep under 4KB; split linked docs first.

## Current State

- Clean Architecture baseline exists under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves `frontend/dist`, logs JSON, and runs from `compose.golang.yaml` on port `8890`.
- `/api/healthz` reports tool targets plus static/legacy asset readiness.
- Natural routes, `.php` aliases, CSS bootstrap, smoke routes, and visual specs share manifests.
- Public routes and aliases match the legacy public compositions.
- Public assets, evolution skin, and game images copy into `frontend/dist/public-assets`.
- `/api/public/universes` reads the master DB with config fallback.
- Registration creates user, planet, IP log, greeting, TimeLimit, ranks, sessions, mail, redirect.
- `/game/validate.php?ack=` and `/activation?ack=` activate accounts and sessions.
- Login/logout create/clear sessions, private cookies, home `aktplanet`, and `/game` redirects.
- `/api/game/session` validates public session plus private cookie, bans/IP with expiry, preserves vacation/deletion state, and touches `lastclick`.
- `/api/game/*` covers overview, build/resource/research/ship/fleet/templates/galaxy/defense/tech/stat/search/buddy/notes/messages/report models; overview/build/resources/research/ship/defense/fleet templates/buddy/notes/messages mutate.
- Auth `/game/*` preserves sessions and `cp`; screens use legacy `evolution` skin.
- Modernization: [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Verified Implementation

- Public parity, language flags, login/logout, and migrated game read models are guarded.
- Fleet covers summary, slots, expeditions, ships, speed/cargo, Commander templates, and recall; dispatch/ACS pending.
- Galaxy covers clamp, rows, status, moon/debris/actions, slots, deut warning, quick links/prefill; instant sends pending.
- Shipyard/defense cover display plus POST orders into legacy `Shipyard` queue with due partial/full completion.
- Technology/stat/search/buddy/notes/messages/report layouts follow legacy chrome; messages send/delete/report PMs; `bericht` owner/allied-spy access is ported.
- Registration writes side effects and sends SMTP/MailHog activation/password mail.
- Activation clears `validatemd`, sets `validated=1`, copies `email` to `pemail`, redirects, and rejects link reuse.
- Overview covers legacy `cp`, `lgn` activity, admin notice, header/menu/table layout parity work, rename/delete name rules, blockers, destroy markers, queue flush, stat/rank updates, and active restore.
- Buildings layout matches legacy row geometry; add/remove/demolish/finish writes queues/stats.
- Resource accrual updates metal/crystal/deuterium from `lastpeek` with caps before overview/resources/building writes.
- Research start/cancel/finish writes global queue, resources, active state, stats/ranks; UI cancels active queue.

## Verified QA

- `bun run build && bun run check && bun test`: passing.
- `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`: passing.
- Go smoke covers health, routes, assets, MailHog, activation cleanup/reuse, auth/session cookie expiry/security, logout, reads/mutations, guards, and privacy.
- Playwright visual/CSR E2E passes Chromium/Firefox for public/auth routes; auth `lgn` is consumed once.
- Chromium diff remains highest on stats/tech/overview/resources/fleet.
- Go internal coverage gate: `97.0% >= 97%`.
- Go smoke JSON: `all_pass: true`.

Full legacy PHP E2E was not run for this step; keep PHP as oracle.

## Remaining Work

- Close authenticated visual diff for statistics and remaining game-page nits before claiming parity.
- Port remaining overview legacy actions.
- Port fleet dispatch/ACS, galaxy instant actions, alliance/admin/options/recovery/deletion/vacation/bans/permissions.
- Convert legacy E2E cases into Go compatibility checks per migrated flow.
- Run full legacy PHP E2E before declaring any game-flow migration equivalent.
