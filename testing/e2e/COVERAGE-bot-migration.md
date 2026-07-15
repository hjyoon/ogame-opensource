# Bot Migration Coverage

Keep this file under 4KB. This tracks the legacy bot system in the Go/Bun migration.

## Legacy Scope

- Runtime queue interpreter: `Start`, `End`, `Label`, `Branch`, `Cond`, and `Block` nodes.
- Runtime API calls: `BotIdle`, `BotStrategyExists`, `BotExec`, vars, resource checks, build, research, fleet build, and production settings.
- Admin Bots page: list, add by player name, stop selected bots.
- Admin BotEdit page: editor bootstrap, load, save, new, rename, preview, export, and multipart import.

## Migrated Implementation

- Runtime execution lives in Go infrastructure and is triggered from normal authenticated game loads, matching legacy queue behavior.
- Strategy functions call migrated building, research, shipyard, and resources use cases instead of PHP internals.
- Admin Bots and BotEdit are exposed through `/api/game/admin` and legacy aliases under `/game/index.php?page=admin&mode=...`.
- BotEdit import posts multipart data through `/game/index.php?page=admin&mode=BotEdit&action=import`, backs up the prior strategy to id `1`, persists uploaded source, and exports it through the legacy export alias.
- React renders the legacy BotEdit editor chrome and loads the legacy GoJS helper scripts for editor behavior.

## QA Evidence

- `bot-runtime-coverage-registry.json` maps all 14 discovered PHP `Bot*` APIs to native Go dispatch and the `runtime-queue-ai-full` direct differential case; `audit-bot-runtime-coverage.mjs` is a full-QA drift gate.
- `run-golang-runtime-queue-differential-e2e.sh` exact-compares all 7 maintenance/AI queue cases. Its isolated `_start` graph exercises every Bot API and compares branch transitions, variables, production factors, resource debit, building/research/shipyard queue levels, durations, and absolute starts.
- `go_bot_runtime_strategy_api` in `golang-compat-smoke.mjs` drains a natural `_start` strategy and verifies build, research, shipyard, resource settings, and BotEdit import/export persistence.
- `game-dynamic-behavior-registry.ts` covers BotEdit init, load, save, import, rename, new, preview popup, and export popup as authenticated dynamic cases.
- `playwright-user-type-e2e.ts` checks regular/operator denial and admin BotEdit access.
- Backend unit tests cover BotEdit mutation paths, import success, no-selected-strategy failure, and repository error branches.
- Legacy PHP admin smoke still verifies Bots/BotEdit page rendering as the oracle.
- `run-golang-admin-bots-differential-e2e.sh` exact-compares 18 list/add/stop and strategy CRUD/import/export cases, including whitespace names and access denial.

## Legacy Runtime Repairs

Direct comparison found and fixed legacy bot defects that prevented a valid oracle: AI child delays were treated as absolute timestamps twice, energy read a missing key, research used planet fields and undefined variables, and fleet build duplicated its queue row. PHP and Go now share relative scheduling and the same engine mutations.

## Remaining Rule

Bot migration should be considered incomplete only when a newly found legacy bot API, strategy node, admin action, or editor interaction lacks migrated implementation plus API/unit/E2E evidence. Add that case here before claiming closure again.
