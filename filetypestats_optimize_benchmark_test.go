package filetypestats

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Rainc1oud/filetypestats/ftsdb"
)

// BenchmarkOptimizeConcurrentScanAndLiveUpdates reproduces, against a real
// directory tree, the exact scenario a strace on the target NAS pointed at:
// a background TreeStatsWatcher.ScanDir walk (batched, mutex-protected writes)
// running concurrently with a burst of live file-change events landing through
// the same unbatched, unlocked UpdateFileStats path onFileChanged uses. Set
// FILETYPESTATS_BENCH_ROOT to point at a real tree (see benchmarkScanRoot).
func BenchmarkOptimizeConcurrentScanAndLiveUpdates(b *testing.B) {
	root := benchmarkScanRoot(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dbfile := benchmarkDBFile(b, fmt.Sprintf("optimize-concurrent-%d", i))
		fdb, err := ftsdb.New(dbfile, true)
		if err != nil {
			b.Fatal(err)
		}

		tsw, err := NewTreeStatsWatcher(nil, fdb)
		if err != nil {
			fdb.Close()
			b.Fatal(err)
		}
		tsw.AddDir(root, true, tsw.eventHandler)

		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)

		go func() {
			defer wg.Done()
			if err := tsw.ScanDir(root); err != nil {
				errs <- err
			}
		}()

		go func() {
			defer wg.Done()
			// simulates a burst of live inotify events landing mid-scan
			for j := 0; j < 500; j++ {
				if err := fdb.UpdateFileStats(fmt.Sprintf("/bench/optimize/live-event/file-%06d.dat", j), "image", uint64(j)); err != nil {
					errs <- err
					return
				}
			}
		}()

		wg.Wait()
		close(errs)
		fdb.Close()
		// b.Fatal must be called from the benchmark's own goroutine, not from a
		// spawned worker, so errors are collected above and surfaced here.
		for err := range errs {
			b.Fatal(err)
		}
	}
}
