# Dynamic Legacy JS Coverage

Tracks legacy PHP/JS dynamic behavior in the Go+Bun React frontend. Exact
screenshots prove pixels; masked regions need behavior tests too.

## Status Map

| Area | Legacy source | Go+Bun status | Evidence | Gap |
| --- | --- | --- | --- | --- |
| Game shell navigation/popups | `game/core/page.php` `showGalaxy`, `fenster`, planet selector, officer hovers | Mostly migrated | relative game routes, report/phalanx pages, shell visual cases | popup window sizing and every selector action need behavior coverage |
| Queue countdowns | `overview.php`, `buildings.php`, `b_building.php`, `phalanx_events.php` `bxx`, `setTimeout` | Mostly migrated | overview/building/research/shipyard/defense visual cases; queue HTTP tests; building queue completion dynamic check | add research/shipyard completion variants if bugs appear |
| Galaxy hover/actions | `galaxy.php`, `galaxy_js.php` overLib menus, `doit`, cursor keys | Mostly migrated | hover, action nav, keyboard, instant success/failures incl. cargo, galaxy HTTP tests | add exotic target cases if bugs appear |
| Fleet selection/targeting | `flotten1.php`, `flotten2.php`, `flotten3.php` max links, `shortInfo`, `remainingresources` | Mostly migrated | fleet visual cases; dynamic all-ships, target `shortInfo`, maxResources, mission radio, residue/overcapacity, launch-submit noob/vacation errors | add success-page variants if bugs appear |
| Merchant calculator | `trader.php` `checkValue`, `setMaxValue`, exchange hovers | Mostly migrated | max/negative/rate-tooltip/submit checks plus HTTP edges | more offer-ID variants can be added if bugs appear |
| Character counters | messages, notes, buddy, alliance textareas `cntChars` | Mostly migrated | compose/notes/buddy/alliance/application counter checks | remaining counters should be added when found |
| Statistics/empire hovers | `statistics.php`, `imperium.php` overLib averages/deltas | Mostly migrated | player/alliance delta and empire average tooltip text checks | add more row variants only if bugs appear |
| Admin tools | `pages_admin/*` simulators, filters, bot editor JS | Mostly migrated | admin visual/HTTP, BattleSim slot-sync, BotEdit init/palette checks | BotEdit SACK load/new/rename/save remains isolated follow-up |
| Public auth/register | `wwwroot/*`, `registration.js` flags and polling validation | Mostly migrated | auth visual/CSR, registration HTTP, public registration focus/polling/submit-error checks | add exotic registration availability errors if bugs appear |

## Masked Or Normalized Dynamic Regions

`frontend/scripts/visual/game-visual-utils.ts` normalizes server time,
countdowns, moving fleet timers, volatile resources, image URLs, tooltip
placement, and selected admin tables. Masked pixels need DOM/text assertions.

## Behavior Runner

`run-playwright-authenticated-game-dynamic-e2e.sh` runs 39 shared-fixture cases:
message/notes/buddy/alliance/application counters, galaxy tooltip/action
navigation/keyboard/instant dispatch, fleet all-ships, target metrics, cargo/mission controls,
launch-submit errors, merchant clamps/tooltips/submit, statistics/empire tooltips,
building queue completion, BattleSim slot sync, and BotEdit init.
`run-playwright-public-registration-dynamic-e2e.sh` separately compares public
register focus help, username polling, direct error URLs, and submit errors.

## Required Follow-up

1. Keep DOM/text assertions for every masked selector.
2. Add isolated cases for unsupported legacy-only mutating JS.
3. Run both legacy PHP and Go+Bun where possible.

Current conclusion: core visible dynamics are substantially migrated. Remaining
work is fine-grained client behavior, not static page layout.
