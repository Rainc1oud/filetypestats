package ftsdb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rainc1oud/filetypestats/types"
)

const benchRows = 10000
const benchBatchCap = 500

func reportRowsPerSecond(b *testing.B, start time.Time, rows int) {
	b.Helper()

	elapsed := time.Since(start).Seconds()
	if elapsed > 0 {
		b.ReportMetric(float64(rows)/elapsed, "rows/s")
	}
}

func benchDB(b *testing.B, name string) *FileTypeStatsDB {
	b.Helper()

	dbfile := filepath.Join(os.TempDir(), fmt.Sprintf("filetypestats-ftsdb-%s-%d.sqlite", name, os.Getpid()))
	if err := os.Remove(dbfile); err != nil && !os.IsNotExist(err) {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := os.Remove(dbfile); err != nil && !os.IsNotExist(err) {
			b.Logf("cleanup %s: %v", dbfile, err)
		}
	})

	fdb, err := New(dbfile, true)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(fdb.Close)
	return fdb
}

func seedBenchRows(b *testing.B, fdb *FileTypeStatsDB, root string, rows int) {
	b.Helper()

	batch := types.NewFTypeStatsBatch(benchBatchCap)
	classes := []string{"image", "application", "audio", "video", "document", "archive", "other"}
	for i := 0; i < rows; i++ {
		filecat := classes[i%len(classes)]
		if err := fdb.UpdateFileStatsMulti(fmt.Sprintf("%s/file-%06d.dat", root, i), filecat, uint64(100+i), batch); err != nil {
			b.Fatal(err)
		}
	}
	if err := fdb.CommitBatch(batch); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkUpdateFileStatsUpsert(b *testing.B) {
	fdb := benchDB(b, "upsert")

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		if err := fdb.UpdateFileStats(fmt.Sprintf("/bench/upsert/file-%06d.dat", i), "image", uint64(i)); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(1, "target_rows/op")
	b.ReportMetric(float64(b.N), "final_rows")
	reportRowsPerSecond(b, start, b.N)
}

func BenchmarkUpdateFileStatsMultiCommitBatch(b *testing.B) {
	fdb := benchDB(b, "batch")
	batch := types.NewFTypeStatsBatch(benchBatchCap)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		if err := fdb.UpdateFileStatsMulti(fmt.Sprintf("/bench/batch/file-%06d.dat", i), "application", uint64(i), batch); err != nil {
			b.Fatal(err)
		}
	}
	if err := fdb.CommitBatch(batch); err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	b.ReportMetric(benchBatchCap, "batch_cap")
	b.ReportMetric(1, "target_rows/op")
	b.ReportMetric(float64(b.N), "final_rows")
	reportRowsPerSecond(b, start, b.N)
}

func BenchmarkUpdateFilePathSingleRow(b *testing.B) {
	fdb := benchDB(b, "move-single")
	seedBenchRows(b, fdb, "/bench/move-single", benchRows)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		var from, to string
		if i%2 == 0 {
			from = fmt.Sprintf("/bench/move-single/file-%06d.dat", i%benchRows)
			to = fmt.Sprintf("/bench/move-single/moved-%06d.dat", i%benchRows)
		} else {
			from = fmt.Sprintf("/bench/move-single/moved-%06d.dat", i%benchRows)
			to = fmt.Sprintf("/bench/move-single/file-%06d.dat", i%benchRows)
		}
		if err := fdb.UpdateFilePath(from, to); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(1, "target_rows/op")
	b.ReportMetric(benchRows, "table_rows")
	reportRowsPerSecond(b, start, b.N)
}

func BenchmarkUpdateFilePathDirectoryPrefix(b *testing.B) {
	fdb := benchDB(b, "move-prefix")
	seedBenchRows(b, fdb, "/bench/move-prefix/from", benchRows)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		var from, to string
		if i%2 == 0 {
			from = "/bench/move-prefix/from"
			to = "/bench/move-prefix/to"
		} else {
			from = "/bench/move-prefix/to"
			to = "/bench/move-prefix/from"
		}
		if err := fdb.UpdateFilePath(from, to); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(benchRows, "target_rows/op")
	b.ReportMetric(benchRows, "table_rows")
	reportRowsPerSecond(b, start, b.N*benchRows)
}

func BenchmarkDeleteFileStatsSingleRow(b *testing.B) {
	fdb := benchDB(b, "delete")
	tableRows := max(b.N, benchRows)
	seedBenchRows(b, fdb, "/bench/delete", tableRows)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		if err := fdb.DeleteFileStats(fmt.Sprintf("/bench/delete/file-%06d.dat", i)); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.ReportMetric(1, "target_rows/op")
	b.ReportMetric(float64(tableRows), "initial_rows")
	reportRowsPerSecond(b, start, b.N)
}

func BenchmarkFTStatsSumSyntheticDB(b *testing.B) {
	fdb := benchDB(b, "sum")
	seedBenchRows(b, fdb, "/bench/sum", benchRows)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		stats, err := fdb.FTStatsSum([]string{"/bench/sum/*"})
		if err != nil {
			b.Fatal(err)
		}
		if len(stats) == 0 {
			b.Fatal("empty stats result")
		}
	}
	b.StopTimer()
	b.ReportMetric(benchRows, "selected_rows/op")
	b.ReportMetric(benchRows, "table_rows")
	reportRowsPerSecond(b, start, b.N*benchRows)
}

func BenchmarkFTDumpPathsSyntheticDB(b *testing.B) {
	fdb := benchDB(b, "dump")
	seedBenchRows(b, fdb, "/bench/dump", benchRows)

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		rows, err := fdb.FTDumpPaths([]string{"/bench/dump/*"})
		if err != nil {
			b.Fatal(err)
		}
		if len(*rows) == 0 {
			b.Fatal("empty dump result")
		}
	}
	b.StopTimer()
	b.ReportMetric(benchRows, "selected_rows/op")
	b.ReportMetric(benchRows, "table_rows")
	reportRowsPerSecond(b, start, b.N*benchRows)
}
