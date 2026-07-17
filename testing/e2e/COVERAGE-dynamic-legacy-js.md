# Dynamic Legacy JS Coverage

Tracks legacy PHP/JS dynamic behavior. Exact screenshots prove pixels; masked regions need behavior tests too.

## Status Map

| Area | Legacy source | Go+Bun status | Evidence | Gap |
| --- | --- | --- | --- | --- |
| Game shell navigation/popups | `game/core/page.php` `showGalaxy`, `fenster`, planet selector, officer hovers | Registered parity | relative routes, report/phalanx pages, popup checks, five officer hover exact diffs | expand on discovery |
| Queue/countdown/event hovers | `overview.php`, `event_list.php`, `b_building.php`, `phalanx_events.php` | Registered parity | overview visual, tooltips, queue completion, phalanx countdown | expand on discovery |
| Galaxy hover/actions | `galaxy.php`, `galaxy_js.php` menus, `doit`, cursor keys | Registered parity | hover/action links, keyboard, instant success/failures | expand on discovery |
| Fleet selection/targeting | `flotten1/2/3.php` max links, `shortInfo`, remaining resources | Registered parity | controls, mission selection, cargo, attack/ACS/expedition | expand on discovery |
| Merchant calculator | `trader.php` calculator and hovers | Registered parity | clamps, rate tooltip, submit and HTTP edges | expand on discovery |
| Character counters | messages, notes, buddy, alliance textareas | Registered parity | compose/notes/buddy/alliance counters | expand on discovery |
| Statistics/empire hovers | `statistics.php`, `imperium.php` | Registered parity | player/alliance delta and empire average tooltips | expand on discovery |
| Admin tools | `pages_admin/*` simulators, filters, BotEdit JS | Registered parity | visual/HTTP, simulators, filters, BotEdit CRUD/popups | expand on discovery |
| Public auth/register | `wwwroot/*`, `registration.js` flags and polling | Registered parity | visual/CSR, login flags/recovery, registration behavior | expand on discovery |

## Masked Or Normalized Dynamic Regions

`frontend/scripts/visual/game-visual-utils.ts` normalizes server time,
countdowns, moving fleet timers, volatile resources, image URLs, tooltip
placement, and selected admin tables. Masked pixels need DOM/text assertions.

## Behavior Runner

`run-playwright-authenticated-game-dynamic-e2e.sh` enables commander/alliance/
report/phalanx/ACS fixtures by default and runs 112 cases: counters, galaxy
action/hover links/keyboard, fleet controls/launch errors, merchant clamps/
tooltips/submit, statistics/empire tooltips, overview event overLib, queue
countdowns, resource production draft/submit boundaries, all five header officer tooltips, technology rapid-fire and building
demolition links (including direct/new-tab start/cancel routes), popup sizing/body, admin filters/
search/Bans/Uni/Planets/Expedition/BattleSim debug/result/BotEdit, and empire
double-click enqueue routing.
`run-playwright-public-registration-dynamic-e2e.sh` separately compares public
register focus help, username polling, direct error URLs, and submit errors.
`run-playwright-public-login-dynamic-e2e.sh` compares invalid-login feedback and
language flags plus forgot-password behavior with/without universe selection.

Route-changing dynamic links are tracked separately here:

- [Dynamic Link Parity](./COVERAGE-dynamic-link-parity.md)
- [Bot Migration Coverage](./COVERAGE-bot-migration.md)

## Maintenance Rules

1. Keep DOM/text assertions for every masked selector.
2. Add isolated cases when unsupported legacy-only mutating JS is found.
3. Run both legacy PHP and Go+Bun where possible.

Conclusion: all 112 registered authenticated cases pass in both browsers. This
proves the registered inventory, not every theoretical JS state; discoveries
must expand the inventory before parity is reasserted.
