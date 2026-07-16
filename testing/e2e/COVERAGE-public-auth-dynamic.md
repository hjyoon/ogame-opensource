# Public Auth Dynamic Coverage

This document tracks public-side legacy-compatibility dynamics for login + registration.

## Public Login Dynamic (`run-playwright-public-login-dynamic-e2e.sh`)

Coverage:

- Invalid login flow:
  - same error page path across legacy and migrated (`/game/reg/errorpage.php`)
  - same inline feedback behavior ordering before navigation
- Language flag click behavior:
  - per-language locale refresh
  - post-click URL/path hash/cookie parity
  - localized text surface parity (top menu/login text blocks)
- Forgot password click behavior:
  - with-universe:
    - click target does not hardcode unexpected origin
    - navigates to `/game/reg/mail.php` on same origin
    - no unexpected dialog on submit path
  - without-universe:
    - does not navigate
    - shows universe-required alert path and keeps both sides in parity

## Public Registration Dynamic (`run-playwright-public-registration-dynamic-e2e.sh`)

- focus/help text lifecycle and username/email polling
- direct-error URL matrix
- submit/validation error parity
- legacy/new registration form structural parity (`/game/reg/new.php`)

All public auth dynamic scripts run side-by-side against
`OGAME_LEGACY_BASE_URL` and `OGAME_GO_BASE_URL`, and emit:

- `report.json`
- `report.md`
- screenshot evidence under `.tmp/playwright-public-*/`

The final migration wrapper runs the login dynamic suite. Registration dynamic
is a standalone gate and must be run when registration UI behavior changes.
