# Operations and Hardening Notes

This project now applies a small core schema migration at runtime after a configured database connection is opened. The migration is idempotent and records itself in `<prefix>schema_migrations`.

## Runtime Security Baseline

- Game and public start pages send `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, and a `frame-ancestors 'self'` content security policy.
- Private session cookies are set through `SetGameCookie()` with `HttpOnly` and `SameSite=Lax`. `Secure` is enabled automatically when the incoming request is HTTPS or forwarded as HTTPS.
- Public login sessions use random bytes when available. The legacy MD5 password storage remains for compatibility and needs a separate schema migration before replacing it with `password_hash()`.
- SQL errors are logged server-side instead of being printed into HTTP responses.

## Database Hot Paths

The core migration adds indexes for the highest-traffic paths:

- `queue`: due task scans, owner/type scans, and fleet sub-task lookups.
- `fleet`: owner mission, target mission, and origin planet lookups.
- `planets`: coordinate/type lookups, owner/type scans, and removed-planet cleanup.
- `users`: login/session, feed, email, disabled-account cleanup, and inactivity cleanup lookups.
- `messages`, `reports`, `notes`, `buddy`, `allyapps`, `template`, `userlogs`, and `fleetlogs`: owner/date or relationship lookups used by common pages and cleanup jobs.

## Game Rule Baseline

The current implementation follows the repository's legacy OGame-like rules rather than a named upstream OGame version. The actively covered rule areas are:

- Economy production, storage caps, energy shortage behavior, trader exchanges, and premium DM spending order.
- Build, research, shipyard, queue freeze/unfreeze, and multi-day queue completion.
- Fleet slots from computer technology, transport/deploy/recall, ACS hold, alliance depot fuel support, and jump gate restrictions.
- Battle reports, espionage reports, rapid fire toggles, defense repair, plunder, debris creation, and recycler collection.
- IPM/ABM behavior and defense destruction.
- Colony ship success/failure/max-planet/competition cases.
- Moon creation, moon destruction, destroyed-moon retargeting, and lunar buildings.
- Expedition result families and configurable expedition caps.

Rule changes should be documented with the exact setting or code path they alter, then covered by an E2E case before the default behavior changes.

## Remaining Hardening Work

- Add a real CSRF token system across authenticated mutation forms. This is intentionally not enabled globally yet because every legacy form and AJAX endpoint must receive and validate the token consistently.
- Migrate passwords from the legacy 32-character MD5 column to a longer modern hash column with backward-compatible login upgrade.
- Convert high-risk SQL construction paths to shared escaping/prepared helpers in small batches.
