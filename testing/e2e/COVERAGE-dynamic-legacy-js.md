# Dynamic Legacy JS Coverage

Tracks legacy PHP/JS dynamic behavior in the Go+Bun React frontend. Exact
screenshots prove pixels; masked regions need behavior tests too.

## Status Map

| Area | Legacy source | Go+Bun status | Evidence | Gap |
| --- | --- | --- | --- | --- |
| Game shell navigation/popups | `game/core/page.php` `showGalaxy`, `fenster`, planet selector, officer hovers | Mostly migrated | relative routes, report/phalanx pages, shell visual cases, report/phalanx popup sizing/body checks | add selector variants only if bugs appear |
| Queue/countdown/event hovers | `overview.php`, `event_list.php`, `b_building.php`, `phalanx_events.php` `bxx`, overLib | Mostly migrated | overview visual, event fleet/cargo tooltip checks, queue HTTP, completion and phalanx countdown checks | add more variants only if bugs appear |
| Galaxy hover/actions | `galaxy.php`, `galaxy_js.php` overLib menus, `doit`, cursor keys | Mostly migrated | hover, action nav, keyboard, instant success/failures incl. cargo, galaxy HTTP tests | add exotic target cases if bugs appear |
| Fleet selection/targeting | `flotten1.php`, `flotten2.php`, `flotten3.php` max links, `shortInfo`, `remainingresources` | Mostly migrated | fleet visual cases; dynamic all-ships, target `shortInfo`, maxResources, mission radio, residue/overcapacity, launch-submit attack/ACS/expedition/noob/vacation | add exotic fleet variants only if bugs appear |
| Merchant calculator | `trader.php` `checkValue`, `setMaxValue`, exchange hovers | Mostly migrated | max/negative/rate-tooltip/submit checks plus HTTP edges | more offer-ID variants can be added if bugs appear |
| Character counters | messages, notes, buddy, alliance textareas `cntChars` | Mostly migrated | compose/notes/buddy/alliance/application/settings counter checks | remaining counters should be added when found |
| Statistics/empire hovers | `statistics.php`, `imperium.php` overLib averages/deltas | Mostly migrated | player/alliance delta and empire average tooltip text checks | add more row variants only if bugs appear |
| Admin tools | `pages_admin/*` simulators, filters, bot editor JS | Mostly migrated | admin visual/HTTP, BattleSim slot-sync, Bans select-all, Planets parser/reset, Expedition chart, BotEdit checks | add more simulator variants only if bugs appear |
| Public auth/register | `wwwroot/*`, `registration.js` flags and polling validation | Mostly migrated | auth visual/CSR, login hover, forgot password click behavior, registration HTTP, focus/email/direct/submit-error checks | add exotic registration variants only if bugs appear |

## Masked Or Normalized Dynamic Regions

`frontend/scripts/visual/game-visual-utils.ts` normalizes server time,
countdowns, moving fleet timers, volatile resources, image URLs, tooltip
placement, and selected admin tables. Masked pixels need DOM/text assertions.

## Behavior Runner

`run-playwright-authenticated-game-dynamic-e2e.sh` enables the optional
commander/alliance/report/phalanx/ACS fixtures by default and runs 71 cases:
counters, galaxy action/hover/keyboard, fleet controls/launch errors, merchant
clamps/tooltips/submit, statistics/empire tooltips, overview event overLib,
queue countdowns, popup sizing/body, admin Bans/Planets/Expedition/BattleSim/
BotEdit, and empire double-click enqueue routing.
`run-playwright-public-registration-dynamic-e2e.sh` separately compares public
register focus help, username polling, direct error URLs, and submit errors.
`run-playwright-public-login-dynamic-e2e.sh` compares invalid-login feedback and
language flags plus forgot-password behavior with/without universe selection.

## Maintenance Rules

1. Keep DOM/text assertions for every masked selector.
2. Add isolated cases when unsupported legacy-only mutating JS is found.
3. Run both legacy PHP and Go+Bun where possible.

Current conclusion: the finite authenticated dynamic registry has 71 cases and
no listed remaining cases. Future additions are discovery-driven fine-grained
client behavior, not static page layout.
