# Time handling

The application boundary is UTC. Go, legacy PHP, MySQL, SQLite game logic, admin schedules, and generated fallback timestamps all use UTC. `OGAME_TIMEZONE` is no longer a supported override, so an old value left in `.env` cannot move the game clock away from UTC.

The Go API exposes absolute instants as Unix seconds. The overview keeps the legacy `serverTime` string for compatibility and also exposes `serverTimeUnix`, which is the canonical value for display. No timestamp data migration is required.

The Go/React UI converts every structured absolute timestamp with the browser's IANA timezone from `Intl.DateTimeFormat`. This automatically follows the visitor's local timezone and daylight-saving transitions. New battle and espionage report bodies carry their UTC Unix timestamp as `<time>` metadata; historical reports without that metadata use their stored message timestamp as the localization fallback. The direct pillory page uses the same browser-local behavior. Countdown durations are not timezone-converted.

The legacy PHP stack remains the compatibility oracle and runs entirely in UTC; browser-local rendering is a Go/React production feature.

To apply this change later, rebuild the selected image during the user's manual deployment. A container restart without rebuilding continues to use the old frontend and runtime configuration.
