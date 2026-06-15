# Post-Login Flow Check Report (2026-06-15)

Date: 2026-06-15

## Scope
- Environment: `docker-compose` stack is running
- Validation method: automatic test account creation inside the container, session acquisition through `reg/newredirect.php`, and route requests with cookie-based session context
- Target: login entry points and major `/game/index.php?page=...` routes

## Entry-point checks
- `/game/`: `200` + short meta refresh response (front-end redirect behavior)
- `/game/reg/login.php`: `200` + login form page
- `/game/reg/login2.php?login=...&pass=...`: `302` redirect to `overview`
- `/game/reg/logout.php`: `404` (route file is missing in current state)

## Post-login page results

### 1) Normal rendered documents (`200`, large body)
`overview`, `admin`, `ainfo`, `allianzen`, `b_building`, `bericht`, `bewerben`, `bewerbungen`, `buddy`, `buildings`, `changelog`, `flotten1`, `galaxy`, `infos`, `messages`, `micropayment`, `notizen`, `options`, `payment`, `phalanx`, `pranger`, `renameplanet`, `resources`, `sprungtor`, `statistics`, `suche`, `techtree`, `techtreedetails`, `trader`, `writemessages`

### 2) HTTP 302 redirect
- `allianzdepot` -> `index.php?page=infos&session=<session>&gid=34`
- `fleet_templates`
- `flotten2`
- `flotten3`
- `imperium`

### 3) Meta refresh responses
- `flottenversand`: `200` + meta refresh
- `logout`: `200` + meta refresh
- `/game/`: `200` + meta refresh on entry

## Conclusion
- Login flow itself is functioning (successful registration + `login2` redirect + valid session creation).
- Most routes are usable after login; a subset is intentionally redirected or requires additional state/conditions.
- `reg/logout.php` still returns `404` and remains a known issue that should be addressed separately.
