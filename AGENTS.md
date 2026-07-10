# AGENTS.md

Guidance for agents working in this repository.

## Project Overview

`filetypestats` is a Go library for recursively scanning file trees, classifying files with `github.com/Rainc1oud/filetype`, and storing/querying aggregate file type statistics in SQLite. It can also keep the database up to date with recursive filesystem notifications through `github.com/rjeczalik/notify`.

Important areas:

- Root package: scanner and watcher orchestration (`treestatswatcher.go`, `dirmonitor.go`, legacy helpers in `filetypestats.go` and `filetypestats_legacy.go`).
- `ftsdb/`: SQLite schema, queries, mutations, batching, and DB tests.
- `notifywatch/`: thin notify watcher wrapper and observation-style tests.
- `treestatsquery/`: query-facing package.
- `types/`: shared data structures and file class lists.
- `internal/cmd/testcli/`: manual CLI for scanning, querying, dumping, and watching.

## Build And Test

This module targets Go 1.20 and uses CGO through `github.com/mattn/go-sqlite3`.

Common commands:

```sh
go test ./...
go test -v ./...
make test
make testcli
```

Notes:

- `CGO_ENABLED=1` is required for normal builds and tests.
- `make testcli` builds `build/linux-amd64/testcli` by default.
- Cross-architecture `make testcli` targets use Podman and an internal container image; do not assume they work outside the author's environment.
- Tests create temporary SQLite files/directories under package directories and should clean them up. Investigate leftover `.tmp-*` or `*.sqlite` files before deleting them.
- `notifywatch` tests depend on platform filesystem notification behavior and contain sleeps. They are more integration/observation oriented than deterministic unit tests.

## Development Conventions

- Keep changes small and package-local where possible.
- Run `gofmt` on edited Go files.
- Prefer Go's `testing` package with existing dependencies (`testify`, `go-cmp`) over adding new test frameworks.
- Do not introduce new dependencies unless they are clearly needed and fit the existing Go 1.20 module.
- Preserve path handling conventions: directory records commonly use a trailing slash so SQL path filters can distinguish directories from similarly named files.
- Be careful with watcher concurrency. `TreeStatsWatcher` coordinates scans, notify handlers, DB updates, and `TDirMonitors`; changes can easily affect lifecycle and cleanup behavior.
- Be careful with SQLite batching and query chunking in `ftsdb`; large path selections are intentionally split to avoid SQLite predicate limits.

## Manual CLI

After building `make testcli`, example usage:

```sh
build/linux-amd64/testcli --dirs=/path/to/tree --db=scandb.sqlite --rm scan
build/linux-amd64/testcli --dirs='/path/to/tree/*' --db=scandb.sqlite summary
build/linux-amd64/testcli --dirs=/path/to/tree --db=scandb.sqlite watch
```

The CLI can create or mutate SQLite databases in the working directory. Avoid committing generated databases or scan output.

## Repository Hygiene

- Do not modify `.nix/flake.nix` or `.nix/flake.lock` unless the task is explicitly about the Nix environment.
- Do not revert user changes in the worktree. Check `git status --short` before editing and preserve unrelated changes.
- Avoid destructive cleanup commands. If generated files need cleanup, remove only files you created or confirm intent first.
- Keep public API compatibility in mind; the README documents legacy behavior and historical breaking changes.
