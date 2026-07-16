# Legacy Route Audit (2026-06-15)

Historical PHP-oracle snapshot from `hjyoon/fix`. Natural Go/React routes and current parity evidence are documented in [MIGRATION_STATUS.md](./MIGRATION_STATUS.md).

## Method

A temporary account was registered with `/game/reg/newredirect.php`. The resulting session and cookie jar were used with `/game/index.php?page=<page>&session=<session>&lgn=1`. Responses were classified by HTTP status, body size, redirect, and meta refresh.

## Rendered Documents

The following returned full `200` game documents in the tested fixture:

`overview`, `ainfo`, `allianzen`, `b_building`, `bericht`, `bewerben`, `bewerbungen`, `buddy`, `buildings`, `changelog`, `flotten1`, `galaxy`, `infos`, `messages`, `micropayment`, `notizen`, `options`, `payment`, `phalanx`, `pranger`, `renameplanet`, `resources`, `sprungtor`, `statistics`, `suche`, `techtree`, `techtreedetails`, `trader`, `writemessages`.

These covered Overview, alliance info/management/applications, construction, reports, buddy, research/shipyard/defense contexts, fleet dispatch, Galaxy, technology, messages, premium functions, notes, options, Jump Gate, rankings, search, merchant, and message compose.

## Redirected Or Stateful Routes

- `allianzdepot` redirected to item information when the required context was unavailable.
- `fleet_templates`, `flotten2`, `flotten3`, and `imperium` redirected without prerequisite state or entitlement.
- `flottenversand`, `logout`, and `/game/` returned legacy meta-refresh responses.
- `admin` returned full markup but began with a localhost meta refresh in this historical environment.

These results did not mean the destination feature was broken; several routes require the previous dispatch step, Commander, an alliance, or a specific planet/technology context.

## Login Entries

- `/game/`: legacy entry/meta refresh
- `/game/reg/login.php`: login form
- `/game/reg/login2.php`: valid credentials redirected to Overview with a session
- `/game/reg/logout.php`: absent direct filename in this snapshot

The current migrated logout contract is the game logout action plus `POST /api/game/logout`. Localhost host rewriting and natural route behavior were subsequently fixed and covered by browser E2E.

## Interpretation

This audit records what the 2026-06-15 fixture rendered. It is not a current route inventory. Current source-derived coverage tracks 37 game router keys, 26 Admin modes, 32 public PHP entrypoints, 14 Bot APIs, and the explicitly excluded optional Mod hooks.
