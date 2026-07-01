# Dynamic Legacy JS Coverage

This tracks legacy PHP/JS dynamic behavior in the Go+Bun React frontend. Exact
screenshots prove pixels only for registry states; masked regions also need
behavior tests.

## Status Map

| Area | Legacy source | Go+Bun status | Evidence | Gap |
| --- | --- | --- | --- | --- |
| Game shell navigation/popups | `game/core/page.php` `showGalaxy`, `fenster`, planet selector, officer hovers | Mostly migrated | relative game routes, report/phalanx pages, shell visual cases | popup window sizing and every selector action need behavior coverage |
| Queue countdowns | `overview.php`, `buildings.php`, `b_building.php`, `phalanx_events.php` `bxx`, `setTimeout` | Migrated and normalized for screenshots | overview/building/research/shipyard/defense visual cases; queue HTTP tests | every queue completion transition is not covered by one visual suite |
| Galaxy hover/actions | `galaxy.php`, `galaxy_js.php` overLib menus, `doit`, cursor keys | Mostly migrated | hover, action nav, keyboard, instant success/failures incl. cargo, galaxy HTTP tests | add exotic target cases if bugs appear |
| Fleet selection/targeting | `flotten1.php`, `flotten2.php`, `flotten3.php` max links, `shortInfo`, `remainingresources` | Mostly migrated | fleet visual cases; dynamic all-ships, target `shortInfo`, maxResources, mission radio, residue/overcapacity | launch-submit mission variants still need more isolated cases |
| Merchant calculator | `trader.php` `checkValue`, `setMaxValue`, exchange hovers | Mostly migrated | max/negative/rate-tooltip/submit checks plus HTTP edges | more offer-ID variants can be added if bugs appear |
| Character counters | messages, notes, buddy, alliance textareas `cntChars` | Mostly migrated | compose/notes/buddy/alliance/application counter checks | remaining counters should be added when found |
| Statistics/empire hovers | `statistics.php`, `imperium.php` overLib averages/deltas | Mostly migrated | player/alliance delta and empire average tooltip text checks | add more row variants only if bugs appear |
| Admin tools | `pages_admin/*` simulators, filters, bot editor JS | Partially migrated | admin visual/HTTP plus BattleSim slot-sync checks | bot editor dynamic JS remains highest-risk behavior gap |
| Public auth/register | `wwwroot/*`, `registration.js` flags and polling validation | Partially migrated | auth visual/CSR cases, registration HTTP tests | username/email polling parity is not an exact visual guarantee |

## Masked Or Normalized Dynamic Regions

`frontend/scripts/visual/game-visual-utils.ts` normalizes server time,
countdowns, moving fleet timers, volatile resource values, dynamic image URLs,
statistics/galaxy tooltip placement, and selected admin tables. These masks are
intentional for stable screenshots, but they mean screenshot success alone does
not prove timer, hover, or calculation logic.

## Behavior Runner

`run-playwright-authenticated-game-dynamic-e2e.sh` runs 35 shared-fixture cases:
message/notes/buddy/alliance/application counters, galaxy tooltip/action
navigation/keyboard/instant dispatch, fleet all-ships, target metrics, cargo/mission controls,
merchant clamps/tooltips/submit, statistics/empire tooltips, and BattleSim slot sync. Merchant text allows
+/-1 for live production drift; action results are still compared numerically.

## Required Follow-up

1. Expand `game-dynamic-behavior-registry.ts` for dynamic actions that should
   not be judged by pixels: fleet `shortInfo`, merchant calculator, text
   counters, galaxy instant actions, and admin simulators.
2. For every masked selector, keep at least one DOM/text assertion that proves
   the underlying dynamic value is updated.
3. Run the behavior suite against both legacy PHP and Go+Bun where possible,
   then document unsupported legacy-only cases here instead of hiding them in
   visual masks.

Current conclusion: core visible dynamics are substantially migrated. Remaining
work is fine-grained client behavior, not static page layout.
