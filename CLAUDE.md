# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

pgsnap is a Go library that provides a fake PostgreSQL server for unit testing, using
snapshot testing (inspired by Jest UI snapshots). On first run (or when no snapshot file
exists), it proxies real client<->PostgreSQL traffic and records it to a `pgsnap_<testname>.txt`
file. On subsequent runs it replays that recorded traffic without touching a real database.

The repo has two Go modules:
- `.` (`github.com/egon12/pgsnap`) — the core library, no Docker dependency.
- `./docker` (`github.com/egon12/pgsnap/docker`) — optional helper that spins up a real
  Postgres in Docker (via `dockertest`) to generate snapshots, and to run migrations
  before recording. It depends on the root module via a `replace` directive to `../`.

## Common commands

Root module:
```sh
go build -v ./...
go test -v ./...
```

Docker module (run from `./docker`):
```sh
cd docker && go build -v ./...
cd docker && go test -v ./...
```

Run a single test:
```sh
go test -run TestSnap_RunScript ./...
```

Skip tests that require Docker/a real Postgres (docker module tests call `testing.Short()`):
```sh
go test -short ./...
```

Force re-recording a snapshot (deletes/ignores the existing snapshot file and re-proxies
against a real Postgres at the given URL):
```sh
PGSNAP_FORCE_WRITE=true go test ./...
```

Verbose proxy/server logging during a test run:
```sh
PGSNAP_DEBUG=true go test -v -run TestName ./...
```

CI runs `go build -v ./...` and `go test -v ./...` on Go 1.19 (see `.github/workflows`).

## Architecture

### Recording vs. replay (`snap.go`)
`Snap` is the entry point (`NewSnap`/`NewSnapWithConfig`). It opens a TCP listener on
`127.0.0.1:0` and exposes it as a `postgres://` URL via `Addr()`. Then it decides between
two modes based on whether a snapshot file exists and is non-empty:
- **Proxy mode** (`proxy.go`): connects to a real Postgres (the `url` passed in), and
  relays messages bidirectionally between the test's DB client and the real server,
  writing every message to the snapshot file as it goes.
- **Server mode** (`fake_server.go`): reads the snapshot file into a `pgmock.Script` and
  replays it against the test's DB client — no real Postgres involved.

`shouldRunProxy` triggers proxy mode when the file doesn't exist (`os.IsNotExist`) or is
empty (fewer steps than the base connection handshake, `EmptyScript`), or when
`Config.ForceWrite` is set (via `PGSNAP_FORCE_WRITE=true`).

### Snapshot file format (`script.go`)
Each line is `F <json>` (frontend/client message) or `B <json>` (backend/server message),
JSON-encoded `pgproto3` message structs prefixed by their wire message type. Filenames are
derived from `t.Name()`: `Test` prefix stripped, `/` replaced with `__`, non-alphanumerics
replaced with `_`, lowercased, prefixed `pgsnap_` (e.g. `TestSnap/RunScript` ->
`pgsnap_snap_runscript.txt`). `readScript` rebuilds a `pgmock.Script` from these lines,
always starting from `pgmock.AcceptUnauthenticatedConnRequestSteps()`.

### Message comparison (`custom_expectation.go`)
`pgmock`'s default `ExpectMessage` step does exact struct comparison. Because prepared
statement/portal names in `Parse`/`Describe`/`Bind` messages are non-deterministic across
runs (differ between `lib/pq` and `jackc/pgx`, and can vary run-to-run), pgsnap uses custom
`Step` implementations (`expectParseMessage`, `expectDescribeMessage`, `expectBindMessage`)
that compare everything except the name/portal identifiers.

### Driver support
Only `github.com/lib/pq` is supported as the client driver for `sql.Open` — see the
"Known Bugs" note in README.md about why `jackc/pgx`'s named-transaction behavior doesn't
round-trip consistently through recording/replay. Tests exercise both drivers against the
proxy/server, but the public `NewDB`/`NewDBWithConfig` helpers hardcode `sql.Open("postgres", ...)`.

### Docker module (`docker/`)
`NewPostgreInDocker` (via `ory/dockertest`) starts a Postgres container, mounting a SQL
migration directory (`docker-entrypoint-initdb.d`) so schema exists before recording.
`docker.SetMigrationPath` / `docker.WithMigrationPath` configure that mount path (defaults
to `./sqlschema/`). `NewSnapWithDocker`/`NewPgSnapDocker` only start a container when a
snapshot doesn't already exist (`pgsnap.IsSnapshotExists`) or `ForceWrite` is set — so CI
runs against committed snapshots without ever touching Docker. The container's `Finish`
func is registered via `Snap.AddFinishFunc` so `snap.Finish()` also tears down the
container.

### Concurrency/finish semantics
Both proxy and server modes run message loops in goroutines and signal completion via a
`done` channel. `Snap.setFailAfter` races the test against `Config.TestTimeout` (default
5s) and calls `t.FailNow()` if a script never reaches Sync (see the "Known Bugs" note about
hangs when expectations don't match). Always call `defer snap.Finish()` in tests to close
the listener, signal the proxy/server, wait for goroutines, and run any registered
finish funcs (e.g. Docker container cleanup).
