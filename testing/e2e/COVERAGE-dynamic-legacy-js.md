# Dynamic Legacy JS Coverage

Tracks legacy PHP/JS dynamic behavior in the Go+Bun React frontend. Exact
screenshots prove pixels; masked regions need behavior tests too.

## Status Map

| Area | Legacy source | Go+Bun status | Evidence | Gap |
| --- | --- | --- | --- | --- |
| Game shell navigation/popups | `game/core/page.php` `showGalaxy`, `fenster`, planet selector, officer hovers | Mostly migrated | relative routes, report/phalanx pages, shell visual cases, report/phalanx popup sizing/body checks | add selector variants only if bugs appear |
| Queue countdowns | `overview.php`, `buildings.php`, `b_building.php`, `phalanx_events.php` `bxx`, `setTimeout` | Mostly migrated | overview/building/research/shipyard/defense visual cases; queue HTTP tests; building/research/shipyard completion and phalanx event countdown checks | add more countdown variants only if bugs appear |
| Galaxy hover/actions | `galaxy.php`, `galaxy_js.php` overLib menus, `doit`, cursor keys | Mostly migrated | hover, action nav, keyboard, instant success/failures incl. cargo, galaxy HTTP tests | add exotic target cases if bugs appear |
| Fleet selection/targeting | `flotten1.php`, `flotten2.php`, `flotten3.php` max links, `shortInfo`, `remainingresources` | Mostly migrated | fleet visual cases; dynamic all-ships, target `shortInfo`, maxResources, mission radio, residue/overcapacity, launch-submit attack/ACS/expedition/noob/vacation | add exotic fleet variants only if bugs appear |
| Merchant calculator | `trader.php` `checkValue`, `setMaxValue`, exchange hovers | Mostly migrated | max/negative/rate-tooltip/submit checks plus HTTP edges | more offer-ID variants can be added if bugs appear |
| Character counters | messages, notes, buddy, alliance textareas `cntChars` | Mostly migrated | compose/notes/buddy/alliance/application counter checks | remaining counters should be added when found |
| Statistics/empire hovers | `statistics.php`, `imperium.php` overLib averages/deltas | Mostly migrated | player/alliance delta and empire average tooltip text checks | add more row variants only if bugs appear |
| Admin tools | `pages_admin/*` simulators, filters, bot editor JS | Mostly migrated | admin visual/HTTP, BattleSim slot-sync, BotEdit init/load/save/rename/new/preview/export checks | add more simulator variants only if bugs appear |
| Public auth/register | `wwwroot/*`, `registration.js` flags and polling validation | Mostly migrated | auth visual/CSR, registration HTTP, public registration focus/email non-poll/direct errors/submit-error checks | add exotic registration variants only if bugs appear |

## Masked Or Normalized Dynamic Regions

`frontend/scripts/visual/game-visual-utils.ts` normalizes server time,
countdowns, moving fleet timers, volatile resources, image URLs, tooltip
placement, and selected admin tables. Masked pixels need DOM/text assertions.

## Behavior Runner

`run-playwright-authenticated-game-dynamic-e2e.sh` enables the optional
commander/alliance/report/phalanx/ACS fixtures by default and runs 55 cases:
message/notes/buddy/alliance/application counters, galaxy tooltip/action
navigation/keyboard/instant dispatch, fleet all-ships, target metrics, cargo/mission controls,
launch-submit success/errors, merchant clamps/tooltips/submit, statistics/empire tooltips,
building/research/shipyard/phalanx queue countdowns, report/phalanx popup sizing/body,
BattleSim slot sync, and BotEdit init/load/save/rename/new/preview/export.
`run-playwright-public-registration-dynamic-e2e.sh` separately compares public
register focus help, username polling, direct error URLs, and submit errors.

## Maintenance Rules

1. Keep DOM/text assertions for every masked selector.
2. Add isolated cases when unsupported legacy-only mutating JS is found.
3. Run both legacy PHP and Go+Bun where possible.

Current conclusion: the finite authenticated dynamic registry has 55 cases and
no listed remaining cases. Future additions are discovery-driven fine-grained
client behavior, not static page layout.
