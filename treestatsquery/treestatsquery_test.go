package treestatsquery

import (
	"path/filepath"
	"testing"

	"github.com/Rainc1oud/filetypestats/ftsdb"
)

func newTestDB(t *testing.T) *ftsdb.FileTypeStatsDB {
	t.Helper()

	fdb, err := ftsdb.New(filepath.Join(t.TempDir(), "query.sqlite"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fdb.Close)
	return fdb
}

func TestFTStatsSumWrappers(t *testing.T) {
	fdb := newTestDB(t)
	if err := fdb.UpdateFileStats("/query/image.png", "image", 123); err != nil {
		t.Fatal(err)
	}

	stats, err := FTStatsSumDB(fdb, []string{"/query/*"})
	if err != nil {
		t.Fatal(err)
	}
	if stats["image"].FileCount != 1 || stats["image"].NumBytes != 123 {
		t.Fatalf("FTStatsSumDB image stats = %#v, want one 123-byte image", stats["image"])
	}

	stats, err = FTStatsSum(fdb.DbFileName(), []string{"/query/*"})
	if err != nil {
		t.Fatal(err)
	}
	if stats["image"].FileCount != 1 || stats["image"].NumBytes != 123 {
		t.Fatalf("FTStatsSum image stats = %#v, want one 123-byte image", stats["image"])
	}
}

func TestFTStatsSumDBRejectsInvalidConnections(t *testing.T) {
	if _, err := FTStatsSumDB(nil, []string{"/query/*"}); err == nil {
		t.Fatal("FTStatsSumDB(nil) succeeded, want error")
	}

	fdb := newTestDB(t)
	fdb.Close()
	if _, err := FTStatsSumDB(fdb, []string{"/query/*"}); err == nil {
		t.Fatal("FTStatsSumDB with closed DB succeeded, want error")
	}
}
