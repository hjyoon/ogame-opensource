# SQLite Mode

The Go/React runtime supports MySQL (default) and SQLite. SQLite uses the pure-Go `modernc.org/sqlite` driver, WAL for file databases, a busy timeout, and a small connection pool. The legacy PHP oracle remains MySQL-only.

## Docker

Start the persistent SQLite service:

```sh
docker compose -f compose.sqlite.yaml up -d --build goapp-sqlite
```

Open `http://localhost:8891`. A new volume is initialized with universe 1, Legor, Arakis, Mond, and the legacy rule tables.

- Login: `legor`
- Password: `admin`
- Volume: `ogame-opensource_sqlite_data`

Override the port or initial admin credentials before the first start:

```sh
OGAME_SQLITE_PORT=8892 OGAME_ADMIN_PASSWORD=secret \
  docker compose -f compose.sqlite.yaml up -d --build goapp-sqlite
```

Seed values are idempotent and do not overwrite an existing account. To intentionally create fresh databases, remove the SQLite volume with `docker compose -f compose.sqlite.yaml down -v` first.

## Configuration

Set `OGAME_DB_DRIVER=sqlite`. SQLite mode uses these variables:

| Variable | Default |
| --- | --- |
| `OGAME_SQLITE_MASTER_PATH` | `data/ogame-master.sqlite` |
| `OGAME_SQLITE_UNIVERSE_PATH` | `data/ogame-universe.sqlite` |
| `OGAME_SQLITE_AUTO_MIGRATE` | `1` |
| `OGAME_ADMIN_EMAIL` | `admin@example.local` |
| `OGAME_ADMIN_PASSWORD` | `admin` |

`OGAME_UNI_DB_PREFIX`, `OGAME_UNI_DB_SECRET`, `OGAME_UNI_NUMBER`, and `OGAME_PUBLIC_BASE_URL` remain common settings. MySQL host, user, password, and database variables are ignored in SQLite mode.

With auto-migrate enabled, startup creates missing master/universe tables and indexes, then idempotently installs required base rows. Disabling it requires compatible database files to already exist. This is schema bootstrap, not automatic transfer of existing MySQL data.

## Native Run

Build the React output first, then start Go from the repository root:

```sh
cd frontend && bun install && bun run build
cd ../backend
OGAME_DB_DRIVER=sqlite \
OGAME_SQLITE_MASTER_PATH=../.tmp/master.sqlite \
OGAME_SQLITE_UNIVERSE_PATH=../.tmp/universe.sqlite \
go run ./cmd/ogame-server
```

## QA

Run the SQLite HTTP smoke and mutation check:

```sh
testing/e2e/run-golang-sqlite-e2e.sh
```

It rebuilds the SQLite container and verifies health, React serving, Legor login, ten authenticated APIs, registration, resource-backed building enqueue, and persistence. Go integration tests additionally cover building/research/shipyard completion, rank updates, MCP token/OAuth storage, coupons, Admin cron cleanup, and Admin backup/restore.

The normal MySQL/PHP differential suite remains the behavior oracle. PHP Mod execution/readiness is intentionally disabled for SQLite; non-Mod Go base-product code uses the same domain and application layers in both modes.
