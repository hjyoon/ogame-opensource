# Dynamic Link Parity (Legacy ↔ Go/Bun)

Keep this file under 4KB.

This matrix tracks legacy-originated dynamic elements that create clickable links and whether exact visual parity is enabled in authenticated-game dynamic behavior.

## Legacy-Oracle Coverage Status

| Case | Link Behavior | Legacy/Go URL Parity | Exact Diff Enabled |
| --- | --- | --- | --- |
| messages-summary-category-link-filter | `pm=1` filter link in message list | yes (`pm` query param) | yes |
| messages-personal-reply-link | reply link in personal PM row | yes (URL params preserved) | yes |
| messages-personal-galaxy-link | coordinates link in personal PM row | yes (`galaxy/system`) | yes |
| galaxy-action-message-compose-link | galaxy action icon → write-message | yes (`messageziel`) | yes |
| galaxy-action-buddy-request-link | galaxy action icon → buddy form | yes (`action/buddy_id`) | yes |
| overview-player-write-message-link | event-row write message icon | yes (`messageziel`) | yes |
| overview-planet-position-link | overview position link → galaxy | yes (`/game/galaxy` and coords) | yes |
| overview-player-rank-link | overview rank link → statistics | yes (`/game/statistics` and `start`) | yes |

## Notes

- The following link-like actions update inline widgets (not full-route page switches) and are validated by DOM/text assertions with masked dynamic fields; they are intentionally excluded from screenshot parity because no stable full-page visual transition is required:
  - `fleet-select-all-ships` (`maxShips`/`#all-ships`)
  - `fleet-residue`/`fleet-max-resources` max-links in ship/ressource fields
  - admin parsing action links (`spio`, `reset`) and popup action links

- If additional legacy link surfaces are discovered (e.g., from new pages), add them as separate dynamic cases and enable `visual` once fixtures are deterministic.
