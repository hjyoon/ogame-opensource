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

| Case | Link Behavior | Legacy/Go URL Parity | Exact Diff Enabled |
| --- | --- | --- | --- |
| messages-summary-category-link-filter | `pm=1` filter link in message list | yes (`pm` query param) | yes |
| messages-personal-reply-link | reply link in personal PM row | yes (URL params preserved) | yes |
| messages-personal-galaxy-link | coordinates link in personal PM row | yes (`galaxy/system`) | yes |
| galaxy-action-message-compose-link | galaxy action icon → write-message | yes (`messageziel`) | yes |
| galaxy-action-buddy-request-link | galaxy action icon → buddy form | yes (`action/buddy_id`) | yes |
| galaxy-report-planet-popup-window | planet spy-report popup link | yes (`bericht`) | DOM/popup |
| galaxy-report-moon-popup-window | moon spy-report popup link | yes (`bericht`) | DOM/popup |
| galaxy-phalanx-name-popup-window | planet-name phalanx popup link | yes (`spid`) | DOM/popup |
| galaxy-phalanx-hover-popup-window | hover-menu phalanx popup link | yes (`spid`) | DOM/popup |
| galaxy-planet-hover-tooltip | hover-menu fleet/phalanx/missile links | optional by fixture; registered if present | DOM/popup |
| research-short-queue-completion-done | completed research `next` link | yes (`/game/research`) | DOM |
| overview-player-write-message-link | event-row write message icon | yes (legacy `showMessageMenu`, migrated `messageziel`) | yes |
| overview-planet-position-link | overview position link -> galaxy | yes (legacy `page=galaxy`, migrated `/game/galaxy`, coords preserved) | yes |
| overview-player-rank-link | overview rank link -> statistics | yes (legacy `page=statistics`, migrated `/game/statistics`, `start` preserved) | yes |

## Notes

- The following link-like actions update inline widgets (not full-route page switches) and are validated by DOM/text assertions with masked dynamic fields; they are intentionally excluded from screenshot parity because no stable full-page visual transition is required:
  - `fleet-select-all-ships` (`maxShips`/`#all-ships`)
  - `fleet-residue`/`fleet-max-resources` max-links in ship/ressource fields
  - admin parsing action links (`spio`, `reset`) and popup action links
  - legacy `doit()` AJAX galaxy instant dispatch versus migrated fallback `href`
  - normal result-page links after admin filtering or building queue refresh

- If additional legacy link surfaces are discovered (e.g., from new pages), add them as separate dynamic cases and enable `visual` once fixtures are deterministic.
