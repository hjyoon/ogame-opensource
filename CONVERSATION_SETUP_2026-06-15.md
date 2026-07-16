# PHP Setup And Branch History (2026-06-15)

Historical snapshot from `hjyoon/fix`. Current migration status is in [MIGRATION_STATUS.md](./MIGRATION_STATUS.md).

## Branch Work

The conversation began by comparing `master` and `hjyoon/fix`, then placing the fix branch on current master and pushing it. Confirmed historical commits included:

- `a7abfd8e` Fix bug
- `4d186e49` Update `apache-remoteip.conf`
- `99e4e0d7` Add Docker auto-install for master database
- `fe0491e0` Add Docker auto-install for universe setup
- `b45f2d7a` Add login-flow report
- `364da1dd` Add full conversation documentation

At that point local and remote `hjyoon/fix` matched at `364da1dd`.

## Environment Installation

Commit `99e4e0d7` added `docker/auto-install.php` and entrypoint integration. `OGAME_AUTO_INSTALL` with `OGAME_MDB_*` settings creates the Master DB configuration; `OGAME_AUTO_INSTALL_OVERWRITE` controls replacement.

Commit `fe0491e0` added Universe installation. `OGAME_UNI_AUTO_INSTALL` and `OGAME_UNI_*` configure URL, start page, DB/prefix/secret, language/number, game and fleet speed, galaxies/systems/users, starting Dark Matter, ACS, debris, rapid fire, moons, battle engine and limits, forced language, feed age, external links, and initial Admin credentials.

Exact defaults belong to `.env.example`, `docker-compose.yml`, and `wiki/en/install_docker.md`; this historical summary does not duplicate them.

## Runtime Checks

The PHP stack used:

- legacy server on `8888`
- MySQL on `3306`
- phpMyAdmin on `8080`
- MailHog on `8026`

The homepage returned HTML, and in-container registration through `/game/reg/newredirect.php` created a valid session and redirected to Overview. Temporary random accounts were used; cookie jars were reused across authenticated requests.

## Documentation And Git

The login report was committed as `b45f2d7a` and the original combined report as `364da1dd`, both pushed to `origin/hjyoon/fix`. The combined report was later split into this file, the route audit, and the current conversation index to satisfy the 4KB Markdown policy.
