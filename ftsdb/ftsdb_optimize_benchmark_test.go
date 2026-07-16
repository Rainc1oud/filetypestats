package ftsdb

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Rainc1oud/filetypestats/types"
)

// prodBatchSize mirrors pathInfoBatchSize in treestatswatcher.go (the value actually
// used by scan/watch in production), kept as a local constant here to avoid an
// import cycle (the root package already imports ftsdb).
const prodBatchSize = 200

// BenchmarkOptimizeSingleRowUpsert isolates the cost of a single UpdateFileStats call:
// Sprintf-built SQL text, a correlated "(SELECT id FROM cats WHERE filecat=...)"
// subquery per row, and an implicit autocommit transaction/fsync per call.
func BenchmarkOptimizeSingleRowUpsert(b *testing.B) {
	fdb := benchDB(b, "optimize-single-upsert")

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		if err := fdb.UpdateFileStats(fmt.Sprintf("/bench/optimize/single/file-%06d.dat", i), "image", uint64(i)); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(1, "target_rows/op")
	b.ReportMetric(float64(b.N), "final_rows")
	reportRowsPerSecond(b, start, b.N)
}

// BenchmarkOptimizeBatchCommit200 mirrors the production batch size (200, see
// pathInfoBatchSize in treestatswatcher.go) rather than the 500 used by the
// existing driver-comparison suite, to make the Sprintf+strings.Join allocation
// cost of building one giant multi-statement transaction string visible at a
// realistic scale.
func BenchmarkOptimizeBatchCommit200(b *testing.B) {
	fdb := benchDB(b, "optimize-batch-200")
	batch := types.NewFTypeStatsBatch(prodBatchSize)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		if err := fdb.UpdateFileStatsMulti(fmt.Sprintf("/bench/optimize/batch200/file-%06d.dat", i), "application", uint64(i), batch); err != nil {
			b.Fatal(err)
		}
	}
	if err := fdb.CommitBatch(batch); err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	b.ReportMetric(prodBatchSize, "batch_cap")
	b.ReportMetric(1, "target_rows/op")
	b.ReportMetric(float64(b.N), "final_rows")
	reportRowsPerSecond(b, start, b.N)
}

// BenchmarkOptimizeDeleteOlderThanFullScan seeds a table and repeatedly calls
// DeleteOlderThan with a threshold that matches zero rows every time (all seeded
// rows are "now", the threshold is an hour in the past). Since fileinfo.updated
// has no index, every call still does a full table scan to find nothing to
// delete - this isolates that scan cost without ever shrinking the table.
func BenchmarkOptimizeDeleteOlderThanFullScan(b *testing.B) {
	fdb := benchDB(b, "optimize-delete-scan")
	seedBenchRows(b, fdb, "/bench/optimize/delete-scan", benchRows)
	threshold := time.Now().Add(-1 * time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		if err := fdb.DeleteOlderThan(threshold); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(benchRows, "table_rows")
	reportRowsPerSecond(b, start, b.N*benchRows)
}

// concurrentBurstRows is a fixed per-iteration workload (rather than scaled by
// b.N) so this benchmark still produces real contention under the Makefile's
// default -benchtime 1x, where b.N would otherwise be 1 and each writer would
// only ever issue a single op with essentially nothing to contend over.
const concurrentBurstRows = 2000

// BenchmarkOptimizeConcurrentBatchAndSingleWriters reproduces the production
// scenario a strace on the target NAS pointed at: a background scan doing
// mutex-protected batched commits, running concurrently with live inotify-style
// events that go through the unbatched, unlocked UpdateFileStats path. The high
// futex time observed in that trace is expected to show up here as elevated
// ns/op and allocs/op from lock/scheduler contention between the two writers.
func BenchmarkOptimizeConcurrentBatchAndSingleWriters(b *testing.B) {
	fdb := benchDB(b, "optimize-concurrent")

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()

	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)

		go func(iter int) {
			defer wg.Done()
			batch := types.NewFTypeStatsBatch(prodBatchSize)
			for i := 0; i < concurrentBurstRows; i++ {
				if err := fdb.UpdateFileStatsMulti(fmt.Sprintf("/bench/optimize/concurrent/scan/%d/file-%06d.dat", iter, i), "application", uint64(i), batch); err != nil {
					errs <- err
					return
				}
			}
			if err := fdb.CommitBatch(batch); err != nil {
				errs <- err
			}
		}(n)

		go func(iter int) {
			defer wg.Done()
			for i := 0; i < concurrentBurstRows; i++ {
				if err := fdb.UpdateFileStats(fmt.Sprintf("/bench/optimize/concurrent/watch/%d/file-%06d.dat", iter, i), "image", uint64(i)); err != nil {
					errs <- err
					return
				}
			}
		}(n)

		wg.Wait()
		close(errs)
		// b.Fatal must be called from the benchmark's own goroutine, not from a
		// spawned worker, so errors are collected above and surfaced here.
		for err := range errs {
			b.Fatal(err)
		}
	}

	b.StopTimer()
	reportRowsPerSecond(b, start, b.N*concurrentBurstRows*2)
}
