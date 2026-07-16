# E2E Coverage: Migration Visual Equivalence

Keep this file under 4KB. Add a new topic file when this grows.

## Go/React Scope

The migration target is React 19 on Bun 1.3 served by a Go 1.25 `net/http` backend. PHP remains the oracle until a migrated flow is covered by equivalent Go tests.

Natural routes are preferred for new code. Legacy `.php` paths and `page=` URLs remain compatibility aliases while tests prove parity.

## User Types

Go user-type QA covers:

- regular player
- operator
- administrator
- unvalidated account
- vacation-enable and active-vacation states
- banned account
- deletion-queued account
- credentials and options mutations

Both API-level QA and Chromium/Firefox Playwright CSR checks are part of final migration QA.
Source-derived route, Admin, public entrypoint, Bot API, and Mod hook coverage is tracked in [Legacy Migration Inventory](./COVERAGE-legacy-inventory.md).
Request inputs, actions, queue constants, SQL mutations, and navigation handlers are frozen by [Legacy Behavior Surface](./COVERAGE-legacy-behavior-surface.md). Snapshot/restore comparison progress is tracked in [PHP/Go Differential Coverage](./COVERAGE-differential.md).

## Visual Pages

Authenticated visual equivalence covers default game pages in Chromium and Firefox. Page migration is not complete until the React page matches the legacy PHP screen layout, skin, table density, labels, images, and link/form contract.

Current deep visual areas include:

- overview with notices, unread messages, build rows, incoming/missile/fleet pseudo-events, active planet restore, event clicks, active-event reload persistence, and due-fleet return transitions
- buildings, resources, research, shipyard, defense, completion refresh, resource deduction/refund, moon pages, and queue timers
- galaxy rows, actions, hover/click behavior, target prefill, and instant spy/recycle dispatch
- fleet continue, all-cases dispatch previews, and Recall button timing in Chromium/Firefox; recall return duration must equal outbound elapsed time, including accelerated fleet-speed universes
- statistics, search, messages, merchant, notes, buddy list, options, alliance, admin, and empire views

Runtime queue QA requires due combat to create its battle report in the first Messages API response, without an overview visit or duplicate report on reload.

## Diff Policy

Final visual checks should run with enforced layout and diff. Where a script reports pixel diff, the target for migrated parity is exact `0` changed pixels unless the exception is documented before merging.

When comparing login or public pages, check behavior too: language flag clicks, redirects, form contracts, footer links, and resource loading must match, not just screenshots.

## Fixture Lifecycle

Visual fixtures may create active fleets, special target rows, moons, debris, build queues, and user states. They must either clean themselves up or remain DB-valid. The final wrapper runs `cleanup-golang-migration-fixtures.php` before legacy E2E to remove stale migration fixtures from previous runs.
