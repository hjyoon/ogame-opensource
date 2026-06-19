# Migration Status

Updated: 2026-06-20 KST, branch `hjyoon/golang`.

React 19/Bun 1.3 + Go 1.25 `net/http` tracker. Keep <4KB; split docs first.

## Current State

- Clean Architecture baseline exists under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves `frontend/dist`, logs JSON, and runs from `compose.golang.yaml` on port `8890`.
- `/api/healthz` reports tool targets plus static/legacy asset readiness.
- Natural routes, `.php` aliases, CSS bootstrap, smoke routes, visual specs, public routes, and aliases share manifests.
- Legacy game `page=` aliases route to migrated screens or explicit pending screens instead of falling back to overview.
- Public assets, evolution skin, and game images copy into `frontend/dist/public-assets`.
- `/api/public/universes` reads the master DB with config fallback.
- Registration creates user, planet, IP log, greeting, TimeLimit, ranks, sessions, mail, redirect.
- `/game/validate.php?ack=` and `/activation?ack=` activate accounts and sessions.
- Login/logout create/clear sessions, private cookies, home `aktplanet`, and `/game` redirects.
- `/api/game/session` validates public+private cookies, bans/IP expiry, vacation/deletion state, and `lastclick`.
- `/api/game/*` covers overview, build/empire/resource/research/ship/fleet/templates/galaxy/defense/tech/stat/search/buddy/notes/messages/report/options models; overview/build/resources/research/ship/defense/fleet templates/buddy/notes/messages/options mutate.
- Auth `/game/*` preserves sessions/`cp` and uses legacy `evolution` skin.
- Modernization: [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Verified Implementation

- Public parity, language flags, login/logout, legacy route aliases, and game read models are guarded.
- Fleet covers summary, slots, exp, ships, cargo/speed, templates, recall, dispatch; deeper restrictions pending.
- Galaxy covers clamp, rows, status, moon/debris/actions, slots, deut warning, quick links/prefill; instant sends pending.
- Shipyard/defense cover display plus POST orders into legacy `Shipyard` queue.
- Tech/stat/search/buddy/notes/messages/report/options follow legacy chrome; messages send/delete/report PMs; `bericht` owner/allied-spy access is ported.
- Registration/activation side effects, SMTP/MailHog mail, redirects, and link reuse rejection are ported.
- Overview covers `cp`, `lgn`, notices, unread/build/incoming/missile/fleet pseudo-events, chrome/link/event names+DOM, rename/delete/blockers/destroy/queue/stats/ranks, and active restore.
- Buildings/research/resources write legacy queues, resources, stats/ranks, caps, and active queue state.
- Empire ports the Commander-gated `imperium` table, build queue markers, and legacy GET add/destroy/remove shortcuts.
- Fleet dispatch covers resources/math, hold fuel/clamps, target/ACS guards, ACS sync, and colonize/exp targets.
- Options reads account/settings flags and saves language, skin path/toggle, IP check, sort, spy/fleet counts, and deletion queue toggle.

## Verified QA

- `bun run build && bun run check && bun test`: passing.
- `OGAME_RUN_LEGACY_E2E=0 testing/e2e/run-golang-migration-qa.sh`: passing.
- Go smoke covers health, routes, assets, MailHog, activation cleanup/reuse, auth/session expiry/security, reads/mutations, guards, and privacy.
- Playwright visual/CSR E2E passes Chromium/Firefox; auth supports page filters and consumes `lgn` once.
- Chromium diff remains highest on stats/tech/overview/resources/fleet.
- Go internal coverage gate: `97.0% >= 97%`.
- Go smoke JSON: `all_pass: true`.

Full legacy PHP E2E was not run for this step; keep PHP as oracle.

## Remaining Work

- Close authenticated visual diff for statistics and remaining game-page nits before claiming parity.
- Audit overview parity.
- Finish mission restrictions, galaxy instant actions, alliance/admin/recovery/bans/permissions, and deeper options mutations.
- Convert legacy E2E cases into Go compatibility checks per migrated flow.
- Run full legacy PHP E2E before declaring any game-flow migration equivalent.
