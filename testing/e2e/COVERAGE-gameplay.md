# E2E Coverage: Gameplay And Economy

Keep this file under 4KB. Add a new topic file when this grows.

## Economy And Queues

- Building, research, shipyard, defense, missile, and queue create/cancel/complete flows.
- Queue idempotency for repeated and parallel `UpdateQueue()` runs, same-tick arrivals, frozen tasks, and long scheduler drains.
- Concurrency/race checks for repeated clicks and multi-tab requests so resources cannot overspend and queues cannot duplicate.
- Resource production settings, storage caps, energy shortage, production-ratio ticks, and post-save resource header sync.
- Statistics and ranking recalculation, queued score adjustments, ordering, and page rendering.
- Performance baselines for overview, resources, galaxy, statistics, messages, and admin queue.

## Fleet

- Transport delivery/return, deploy arrival, recall, cargo returns, active fleet slot limits, and computer technology limits.
- Target restrictions for noob/strong score protection, vacation targets, operator/admin targets, temporary attack bans, and Galaxy AJAX errors.
- ACS attack/hold, union creation, invited participant join, participant recall before battle, battle resolution, report recipients, and return.
- Fleet templates, Commander template access, create/update/delete limits, and template use on dispatch pages.
- Fleet all-cases visual/contract coverage for initial, union, target, enemy planet, own colony, debris, moon destroy, expedition, probe-only, and colonize-empty previews.

## Combat And Reports

- Battle reports and espionage reports, including rapid-fire toggles, defense repair writeback, and report text.
- Plunder, debris creation, debris recycling, competing recycler collection, resource return, and defense writeback.
- Interplanetary missiles, anti-ballistic missiles, silo capacity, and defense destruction.
- Moon creation, moon destruction, moon-destruction failure paths, and destroyed-moon fleet retargeting/return cleanup.

## Planets, Galaxy, And Specials

- Owned/foreign/missing `cp` selection, moon selection, planet context isolation, spoofed fleet-origin rejection, and colony abandon fallback.
- Colony ship success, occupied-target failure, max-planet failure, and same-tick competition.
- Galaxy rows, statuses, moon/debris/actions, slot/deuterium warnings, quick links, target prefill, instant spy/recycle, and hover/click actions.
- Jump Gate target filtering, invalid source/target moons, missing gates, foreign moons, empty/oversized ship selections, cooldown rejection, same-moon rejection, and solar-satellite exclusion.
- Merchant, officer, paid/free Dark Matter spending order, invalid/insufficient premium purchases, lunar base, jump gates, and phalanx scans.
- Sensor Phalanx edge coverage for missing arrays, insufficient deuterium, own targets, out-of-range targets, exact 5,000 deuterium spend, and event rendering.
- Expedition flow and result cases.
