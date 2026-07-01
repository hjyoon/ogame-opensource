# Dynamic Legacy JS Coverage

This audit tracks whether legacy PHP/JS dynamic behavior is migrated to the
Go+Bun React frontend. Exact screenshots prove visual output only when the
state is represented in the registry; masked or normalized regions still need
behavior tests.

## Status Map

| Area | Legacy source | Go+Bun status | Evidence | Gap |
| --- | --- | --- | --- | --- |
| Game shell navigation/popups | `game/core/page.php` `showGalaxy`, `fenster`, planet selector, officer hovers | Mostly migrated | relative game routes, report/phalanx pages, shell visual cases | popup window sizing and every selector action need behavior coverage |
| Queue countdowns | `overview.php`, `buildings.php`, `b_building.php`, `phalanx_events.php` `bxx`, `setTimeout` | Migrated and normalized for screenshots | overview/building/research/shipyard/defense visual cases; queue HTTP tests | every queue completion transition is not covered by one visual suite |
| Galaxy hover/actions | `galaxy.php`, `galaxy_js.php` overLib menus, `doit`, cursor keys | Partially migrated | planet/moon/debris/player/alliance hover visual cases; galaxy HTTP tests | keyboard navigation and all instant action variants need behavior assertions |
| Fleet selection/targeting | `flotten1.php`, `flotten2.php`, `flotten3.php` max links, `shortInfo`, `remainingresources` | Partially migrated | fleet visual draft/continue cases; fleet lifecycle HTTP tests | all speed/mission/cargo combinations need deterministic behavior tests |
| Merchant calculator | `trader.php` `checkValue`, `setMaxValue`, exchange hovers | Partially migrated | merchant visual cases and trader HTTP tests | client-side clamping/math needs explicit DOM behavior tests |
| Character counters | messages, notes, buddy, alliance textareas `cntChars` | Visual shell migrated | compose/draft visual cases | live keyup counter behavior is not proven everywhere |
| Statistics/empire hovers | `statistics.php`, `imperium.php` overLib averages/deltas | Partially migrated | statistics tooltip and empire visual cases | tooltip text parity should be asserted for representative rows |
| Admin tools | `pages_admin/*` simulators, filters, bot editor JS | Partially migrated | admin visual draft cases, admin HTTP tests | battle sim/bot editor dynamic JS remains highest-risk behavior gap |
| Public auth/register | `wwwroot/*`, `registration.js` flags and polling validation | Partially migrated | auth visual/CSR cases, registration HTTP tests | username/email polling parity is not an exact visual guarantee |

## Masked Or Normalized Dynamic Regions

`frontend/scripts/visual/game-visual-utils.ts` normalizes server time,
countdowns, moving fleet timers, volatile resource values, dynamic image URLs,
statistics/galaxy tooltip placement, and selected admin tables. These masks are
intentional for stable screenshots, but they mean screenshot success alone does
not prove timer, hover, or calculation logic.

## Required Follow-up

1. Add a behavior registry beside `game-screen-registry.ts` for dynamic actions
   that should not be judged by pixels: fleet `shortInfo`, merchant calculator,
   text counters, galaxy instant actions, and admin simulators.
2. For every masked selector, keep at least one DOM/text assertion that proves
   the underlying dynamic value is updated.
3. Run the behavior suite against both legacy PHP and Go+Bun where possible,
   then document unsupported legacy-only cases here instead of hiding them in
   visual masks.

Current conclusion: core visible dynamics are substantially migrated, but the
legacy dynamic surface is not yet fully proven. The remaining work is mostly
fine-grained client behavior, not static page layout.
