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

## Visual Pages

Authenticated visual equivalence covers default game pages in Chromium and Firefox. Page migration is not complete until the React page matches the legacy PHP screen layout, skin, table density, labels, images, and link/form contract.

Current deep visual areas include:

- overview with notices, unread messages, build rows, incoming/missile/fleet pseudo-events, active planet restore, and event clicks
- buildings, resources, research, shipyard, defense, completion refresh, resource deduction/refund, moon pages, and queue timers
- galaxy rows, actions, hover/click behavior, target prefill, and instant spy/recycle dispatch
- fleet continue and fleet all-cases dispatch previews
- statistics, search, messages, merchant, notes, buddy list, options, alliance, admin, and empire views

## Diff Policy

Final visual checks should run with enforced layout and diff. Where a script reports pixel diff, the target for migrated parity is exact `0` changed pixels unless the exception is documented before merging.

When comparing login or public pages, check behavior too: language flag clicks, redirects, form contracts, footer links, and resource loading must match, not just screenshots.

## Fixture Lifecycle

Visual fixtures may create active fleets, special target rows, moons, debris, build queues, and user states. They must either clean themselves up or remain DB-valid. The final wrapper runs `cleanup-golang-migration-fixtures.php` before legacy E2E to remove stale migration fixtures from previous runs.
