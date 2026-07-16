# Backend Compatibility Endpoints

Source: `backend/internal/delivery/http/server.go`. Updated 2026-07-16. Keep under 4KB.

Natural React routes and `/api/*` are canonical. These handlers preserve links, forms, feeds, and assets expected by the PHP UI and external clients.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET/POST` | `/game/reg/newredirect.php`, `/game/reg/login2.php`, `/game/reg/check_registration.php` | Registration, login, and availability aliases |
| `GET` | `/game/reg/login.php`, `/game/reg/errorpage.php` | Legacy login/error pages |
| `GET` | `/game/validate.php`, `/activation` | Account activation |
| `GET/HEAD/POST` | `/game/index.php` | Legacy game entry and `page`/Admin action bridge |
| `GET` | `/game/pranger.php`, `/game/maintenance.php` | Pillory and maintenance pages |
| `GET` | `/game/redir.php`, `/game/pic.php` | Safe redirect and image proxy |
| any | `/game/cron.php` | Explicitly forbidden browser script |
| `GET` | `/game/reg/mail.php` | Password recovery form |
| `POST` | `/game/reg/fa_pass.php` | Password recovery submit |
| `GET/POST` | `/game/feed/show.php`, `/game/feed/viewitem.php` | RSS/Atom feed compatibility |
| `GET` | `/game/css/*`, `/game/img/*`, `/game/js/*`, `/game/mods/*` | Legacy game static assets |
| `GET` | `/evolution/*`, `/img/*`, `/legacy-assets/*` | Skin, public image, and asset aliases |
| `GET` | `/` and non-API paths | React production build and CSR fallback |

Compatibility URLs are not the internal architecture. Preserve them until route, form, cookie, redirect, and visual behavior have replacement coverage.

Primary API inventory: [API_ENDPOINTS.md](./API_ENDPOINTS.md).
