# Dynamic Link Parity (Legacy ↔ Go/Bun)

Keep this file under 4KB.

This matrix tracks legacy-originated dynamic elements that create clickable links and whether exact visual parity is enabled in authenticated-game dynamic behavior.

## Automated Audit

`playwright-authenticated-game-dynamic-e2e.ts` now records visible route-changing
links before actions, action targets, and links after actions. It normalizes
legacy `index.php?page=...` and Go/Bun `/game/...` URLs, strips volatile
`session`/`cp` values, masks numeric IDs as `#`, and fails on unregistered
action or same-URL dynamic route targets. Link inventories are written to
`.tmp/playwright-authenticated-game-dynamic/<browser>/link-inventory/`.

## Legacy-Oracle Coverage Status

| Cases | Link Behavior | Evidence |
| --- | --- | --- |
| Message list/filter/reply/coordinates | folder `pm`, reply params, Galaxy coordinates | URL + exact diff |
| Combat/spy report links | report popup, spy coordinates, Attack fleet target | URL + popup/DOM + exact diff |
| Galaxy player actions | message, Buddy, statistics | URL + exact diff |
| Galaxy alliance actions | introduction popup, apply, statistics | URL + popup/DOM + exact diff |
| Galaxy planet/moon actions | fleet target, reports, Phalanx, missile | URL + tooltip/popup DOM |
| Overview event/player links | write message, position/Galaxy, rank/statistics | URL + exact diff |
| Research completion | completed queue `next` route | URL + DOM |
| Empire/Commander | building double-click enqueue and queue link | route + DOM/visual |
| Admin links | DB delete, simulator reports, BotEdit preview/export | route + popup/DOM |

## Notes

- The following link-like actions update inline widgets (not full-route page switches) and are validated by DOM/text assertions with masked dynamic fields; they are intentionally excluded from screenshot parity because no stable full-page visual transition is required:
  - `fleet-select-all-ships` (`maxShips`/`#all-ships`)
  - `fleet-residue`/`fleet-max-resources` max-links in ship/ressource fields
  - admin parsing action links (`spio`, `reset`) and popup action links
  - legacy `doit()` AJAX galaxy instant dispatch versus migrated fallback `href`
  - normal result-page links after admin filtering or building queue refresh

- All registered route-changing cases pass in Chromium and Firefox. New link
  surfaces must be registered and should enable exact visual checks once their
  fixture is deterministic.
