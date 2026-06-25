# Migration Status

Updated: 2026-06-26 KST, branch `hjyoon/golang`.

React 19/Bun 1.3 + Go 1.25 `net/http` tracker. Keep this file <4KB; split details when needed.

## Current State

- Clean Architecture baseline is under `backend/internal/{domain,application,infrastructure,delivery}`.
- Go serves the React production build, emits JSON logs, and runs from the current `compose.golang.yaml` `goapp` container.
- Natural routes and legacy `.php`/`page=` aliases share manifests.
- Public assets, `evolution` skin, and game images are copied into `frontend/dist/public-assets`.
- Registration, activation, login/logout, sessions, private cookies, IP/ban/session expiry, and `/game` redirects are ported.
- `/api/game/*` covers main game pages: overview through options, plus alliance/admin, fleet, galaxy, notes, messages, report, and phalanx.
- Mutations exist for overview, buildings, resources, merchant/officers, alliance, research, shipyard/defense, fleet, buddy, notes, messages, and options.
- Modernization candidates stay in [MODERNIZATION_OPTIONS.md](./MODERNIZATION_OPTIONS.md).

## Latest Implementation

- Overview covers `cp`, `lgn`, notices, unread/build/incoming/missile/fleet pseudo-events, rename/delete blockers, queue markers, stats/ranks, and active planet restore.
- Buildings ports queue state, Commander rows, countdowns, shortcuts, due refresh, moon capacity, post-build header sync, and Chromium/Firefox parity for cost/refund/queue/timer states.
- Statistics/fleet authenticated visuals pass Chromium/Firefox at 0px for player/alliance statistics.
- Empire ports Commander-gated `imperium`, build queue markers, and legacy GET add/destroy/remove shortcuts.
- Resources ports the legacy production-percent form, premium bonus icon column, DB normalization, and post-save resource header sync.
- Merchant/officers/alliance/admin port DM spend/trade/timers, alliance home/apply/management/ranks/members/circular, admin modes, Broadcast, Reports, Bans/Expedition/Queue/Fleetlogs mutations, Fleetlogs return recall, and guards.
- Research/Shipyard port aliases, chrome, colors, queues, completion refresh, start/cancel/build, and resource math.
- Fleet dispatch covers cargo/speed, fuel/clamps, ACS sync, colonize/exp targets, templates, and recall.
- Galaxy covers clamp, rows, statuses, moon/debris/actions, slot/deut warnings, quick links, target prefill, and instant spy/recycle dispatch.
- Messages/report/notes/buddy/options follow legacy chrome; escaped legacy message text, report access, and options mutations are enforced.
- Notes, Buddylist, and Options visual parity pass Chromium/Firefox at 0px; Commander rows use `com_until` like PHP.

## Verified QA

- Full migration QA wrapper on 8890 passed: 20 passed, 0 failed; includes PHP E2E, Bun, Go, coverage, smoke, user-type, visuals.
- User-type QA covers regular, operator, admin, unvalidated, vacation, banned, deletion-queued, and options vacation/password/email mutations.
- Resources actions pass Chromium/Firefox: percent save, DB `prod*`, selected values, totals, and visuals.
- Research/shipyard/defense, galaxy, admin pages, and Empire Commander/redirect pass Chromium/Firefox 0px; queues cover submit/partial/complete DB.
- Fleet all-cases passes Chrome/Firefox 0px for initial, union, target, dispatch previews.
- CSR E2E covers Buddy/Options, Options save, Notes create/edit/delete, and logout; Chromium passes.
- Auth visual E2E passes Chromium/Firefox 0px for all default page specs.
- Go smoke covers routes/assets, auth, reg/recovery, acct/options/localization, admin/Fleetlogs, premium/merchant/vacation/moon/phalanx, fleet, galaxy cost/IPM, buddy/social, report/msg lifecycle/resources/input/feed, aliases (58 cases/1643 checks).
- Legacy PHP E2E latest summary: 59 result files, 340 cases, 2428 checks, failed groups 0.
- Go internal coverage gate: `97.0% >= 97%`.

## Remaining Work

- Extend deep-state visuals for pages whose base views already pass.
- Add Go compatibility checks for migrated legacy E2E flows as they stabilize.
- Finish admin mutations and remaining mission edges.
