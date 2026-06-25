# E2E Coverage: Account, Security, Admin

Keep this file under 4KB. Add a new topic file when this grows.

## Account Flows

- Registration, activation links, duplicate username/email rejection, missing-field hardening, and welcome mail delivery.
- Login/logout, public/private session rotation, private-session cookie enforcement, session expiry, and existing-session admin downgrade.
- Account options for language, skin, sorting, notification settings, password/email change, vacation mode, and account deletion schedule/cancel.
- Notes, private messages, reports, report popup access control, message deletion/read state, operator PM reports, and report retention after source deletion.
- Password recovery through permanent/temporary email lookup, MailHog delivery, old password invalidation, and recovered-password login.

## Access Control

- Public/authenticated route matrix coverage for core game and admin pages.
- Direct-entry security for unsafe redirects, image proxy URLs, feed tokens, and GET requests that must not mutate state.
- Cross-user IDOR sweeps for messages, reports, notes, foreign `cp`, resource settings, missile silo demolition, and foreign planet deletion attempts.
- Stored text rendering and script-scheme URL rejection for private messages and alliance text.
- Universe-scoped private-session cookies and multi-universe isolation.

## Social And Alliance

- Alliance creation, applications, accept/reject, leave, dismiss, rank rights, circular messages, member management, settings, and text updates.
- Buddy request, reject, accept, delete, and hold-fleet access cases.
- Alliance Depot ACS fuel supply, hold extension, missing depot, and insufficient-fuel no-op behavior.

## Admin

- Admin/operator permission matrix for admin pages and mutation boundaries.
- User updates, ban/unban login blocking, vacation-mode blocking, and account-state rendering.
- Audit/log pages: UserLogs, Debug, Errors, Browse, Logins, Fleetlogs, seeded markers, regular-user denial, and operator delete boundaries.
- Tools: Bots, BotEdit, Mods, Checksum, DB pages, checksum baseline rendering, and regular-user denial.
- Operations: Broadcast, Reports, BattleSim, RakSim, Expedition simulator, marked report deletion, and Expedition settings admin-only changes.
- DB backup create/restore/delete and operator mutation denial.
- Destructive controls: queue freeze/unfreeze/complete/delete, account deletion scheduling, admin planet creation, universe freeze, and admin score recalculation.
