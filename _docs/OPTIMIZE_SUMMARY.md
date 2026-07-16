# ftsdb write-path optimization

## Background

A production `testcli scan` run on an armv7 NAS was profiled with `strace -c`. The
trace showed `read` (52.7%) and `futex` (27.9%) dominating syscall time, with
`fsync` a comparatively minor 4%. That shifted the priority list: lock/allocation
contention was costing ~7x what fsync overhead was, so the write path (not just
PRAGMA tuning) needed attention. A code review of `ftsdb/ftsdb.go` and
`ftsdb/ftsdb_batch.go` turned up six concrete issues:

1. No prepared statements / parameter binding anywhere — every query was built
   with `fmt.Sprintf` and manual quote-escaping, forcing SQLite to re-parse and
   re-plan on every call.
2. No PRAGMA tuning — default rollback journal + `synchronous=FULL`.
3. A per-row category subquery — every insert ran a correlated
   `(SELECT id FROM cats WHERE filecat=...)` for a fixed, 8-entry category set.
4. No connection pool limits — `sql.Open` never called `SetMaxOpenConns`.
5. The live-watch path (`onFileChanged` → `UpdateFileStats`) bypassed both the
   batch buffer and the `dbmutex` that batch commits used, so it could race a
   concurrent scan.
6. `DeleteOlderThan`/`DeleteOlderThanWithPrefix` filtered on `fileinfo.updated`,
   which had no index — a full table scan on every scan-completion cleanup.

## Benchmark suite added

New `BenchmarkOptimize*` functions were added specifically to reproduce these
issues, using the same result-handling pipeline as the existing sqlite3-vs-modernc
comparison suite (CSV via `scripts/bench-to-csv.awk`, SVG chart via
`scripts/bench-csv-chart.awk`), writing to a separate `_tests/benchmark-optimize/`
directory so it doesn't mix with the driver-comparison dataset:

- `ftsdb/ftsdb_optimize_benchmark_test.go` (package `ftsdb`, synthetic/controlled):
  - `BenchmarkOptimizeSingleRowUpsert` — isolates Sprintf + per-row subquery +
    autocommit/fsync cost of a single `UpdateFileStats` call.
  - `BenchmarkOptimizeBatchCommit200` — batch insert at the production batch
    size (200, matching `pathInfoBatchSize` in `treestatswatcher.go`), showing
    the allocation cost of building one giant `Sprintf`+`strings.Join` SQL string.
  - `BenchmarkOptimizeDeleteOlderThanFullScan` — seeds 10k rows all with
    `updated≈now`, then calls `DeleteOlderThan` with a threshold that matches
    zero rows every time, isolating the missing-index full-scan cost without
    ever shrinking the table.
  - `BenchmarkOptimizeConcurrentBatchAndSingleWriters` — runs a batched
    "scan" writer and an unbatched "live update" writer concurrently against
    the same DB, reproducing the futex/lock-contention scenario from the strace.
- `filetypestats_optimize_benchmark_test.go` (package `filetypestats`, real tree):
  - `BenchmarkOptimizeConcurrentScanAndLiveUpdates` — runs a real
    `TreeStatsWatcher.ScanDir` walk concurrently with a burst of live
    `UpdateFileStats` calls, reproducing the production scenario end-to-end.

Makefile targets `test-benchmark-optimize-csv` (parameterized by `BENCH_LABEL`,
defaulting to `baseline`) and `benchmark-optimize-chart` mirror the existing
`test-benchmark-csv`/`benchmark-chart` targets and reuse the same awk scripts
unmodified.

## Baseline run

```
make test-benchmark-optimize-csv BENCH_ROOT=../.. BENCH_LABEL=baseline
```

Both concurrency benchmarks **failed outright** with `SQLITE_BUSY` (`database is
locked`) — not a slowdown, an actual crash. This confirms issue #5 (inconsistent
locking) is a real production hazard: a live file-change event landing during a
background scan can abort the scan or drop the event.

## Optimizations implemented (`ftsdb/ftsdb.go`, `ftsdb/ftsdb_batch.go`)

- Prepared statements (`*sql.Stmt`, compiled once in `prepareStatements()`) for
  every hot-path mutation: upsert, delete, delete-older-than,
  delete-older-than-with-prefix, update-path.
- Batch insert (`upsertFileStatsMulti`) now runs the prepared upsert statement
  once per row inside a single `sql.Tx`, instead of building one large
  `Sprintf`/`strings.Join` SQL string.
- An in-memory `filecat -> cats.id` cache (`loadCatIDs`), populated once at
  init, removing the per-row correlated subquery.
- `PRAGMA journal_mode=WAL`, `PRAGMA synchronous=NORMAL`, `PRAGMA
  busy_timeout=5000`, and `db.SetMaxOpenConns(1)`, applied in a shared
  `configureConn` helper used by both `openDB` and `Open()`.
- Consistent `dbmutex` locking across all single-row mutation methods (not just
  batch commits), so single-row and batch writers serialize in Go instead of
  racing at the SQLite lock level.
- An index on `fileinfo(updated)`, added in `createTables()`.

Full `go test ./...` suite passes unchanged after these changes.

## Optimized run

```
make test-benchmark-optimize-csv BENCH_ROOT=../.. BENCH_LABEL=optimized
make benchmark-optimize-chart
```

All 5 benchmarks passed with zero errors (vs. 2 hard failures on baseline).
Results (see `_tests/benchmark-optimize/benchmark-comparison.svg` for the chart,
`benchmark-baseline-*.csv`/`benchmark-optimized-*.csv` for raw data):

| Benchmark | Baseline | Optimized | Change |
|---|---|---|---|
| Single-row upsert | 93.1µs | 43.1µs | 2.2x faster |
| 200-row batch commit | 190.7µs | 67.8µs | 2.8x faster |
| `DeleteOlderThan` full scan | 804.5µs | 24.8µs | 32x faster |
| Concurrent scan + live writes (synthetic) | crashed (`SQLITE_BUSY`) | 134.5ms, 0 errors | bug fixed |
| Concurrent scan + live writes (real tree, `../..`) | crashed (`SQLITE_BUSY`) | 3.08s, 0 errors | bug fixed |

## Files touched

- `Makefile` — new `BENCH_LABEL`/`BENCH_OPT_*` variables and
  `test-benchmark-optimize-csv`/`benchmark-optimize-chart` targets.
- `ftsdb/ftsdb.go`, `ftsdb/ftsdb_batch.go` — the optimizations above.
- `ftsdb/ftsdb_optimize_benchmark_test.go`,
  `filetypestats_optimize_benchmark_test.go` — new benchmark suite (untracked).
- `_tests/benchmark-optimize/` — generated CSV/txt/SVG benchmark artifacts
  (untracked).

Nothing here has been committed.
