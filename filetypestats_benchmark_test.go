package filetypestats

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rainc1oud/filetypestats/ftsdb"
	"github.com/Rainc1oud/filetypestats/treestatsquery"
	"github.com/Rainc1oud/filetypestats/utils"
)

const benchScanRootEnv = "FILETYPESTATS_BENCH_ROOT"

func benchmarkScanRoot(b *testing.B) string {
	b.Helper()

	root := os.Getenv(benchScanRootEnv)
	if root == "" {
		root = ".."
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		b.Fatal(err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		b.Fatalf("stat benchmark scan root %q: %v", abs, err)
	}
	if !info.IsDir() {
		b.Fatalf("benchmark scan root %q is not a directory", abs)
	}
	return abs
}

func benchmarkDBFile(b *testing.B, name string) string {
	b.Helper()

	dbfile := filepath.Join(os.TempDir(), fmt.Sprintf("filetypestats-%s-%d.sqlite", name, os.Getpid()))
	b.Cleanup(func() {
		if err := os.Remove(dbfile); err != nil && !os.IsNotExist(err) {
			b.Logf("cleanup %s: %v", dbfile, err)
		}
	})
	return dbfile
}

func buildBenchmarkDB(b *testing.B, root string) string {
	b.Helper()

	dbfile := benchmarkDBFile(b, "benchmark")
	if _, err := WalkFileTypeStatsDB([]string{root}, dbfile); err != nil {
		b.Fatal(err)
	}
	return dbfile
}

func BenchmarkWalkFileTypeStatsDBRealTree(b *testing.B) {
	root := benchmarkScanRoot(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dbfile := benchmarkDBFile(b, fmt.Sprintf("scan-%d", i))
		if _, err := WalkFileTypeStatsDB([]string{root}, dbfile); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTreeStatsWatcherScanDirRealTree(b *testing.B) {
	root := benchmarkScanRoot(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dbfile := benchmarkDBFile(b, fmt.Sprintf("watcher-scan-%d", i))
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
		if err := tsw.ScanDir(root); err != nil {
			fdb.Close()
			b.Fatal(err)
		}
		fdb.Close()
	}
}

func BenchmarkFTStatsSumRealTreeDB(b *testing.B) {
	root := benchmarkScanRoot(b)
	dbfile := buildBenchmarkDB(b, root)
	paths := []string{utils.DirStar(root)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats, err := treestatsquery.FTStatsSum(dbfile, paths)
		if err != nil {
			b.Fatal(err)
		}
		if len(stats) == 0 {
			b.Fatal("empty stats result")
		}
	}
}

func BenchmarkFTDumpPathsRealTreeDB(b *testing.B) {
	root := benchmarkScanRoot(b)
	dbfile := buildBenchmarkDB(b, root)

	fdb, err := ftsdb.New(dbfile, false)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(fdb.Close)

	paths := []string{utils.DirStar(root)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := fdb.FTDumpPaths(paths)
		if err != nil {
			b.Fatal(err)
		}
		if len(*rows) == 0 {
			b.Fatal("empty dump result")
		}
	}
}
