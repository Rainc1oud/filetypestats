package filetypestats

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Rainc1oud/filetypestats/ftsdb"
	"github.com/Rainc1oud/filetypestats/treestatsquery"
	"github.com/Rainc1oud/filetypestats/utils"
)

var (
	testPNG = []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00,
	}
	testPDF = []byte("%PDF-1.4\n% test pdf\n")
)

func writeTestTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{
		filepath.Join(root, "image.png"):      testPNG,
		filepath.Join(root, "document.pdf"):   testPDF,
		filepath.Join(nested, "nested.png"):   testPNG,
		filepath.Join(nested, "nested.pdf"):   testPDF,
		filepath.Join(nested, "unknown.data"): []byte("not a magic file type"),
	}
	for path, data := range files {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func assertStat(t *testing.T, gotCount uint, gotSize uint64, wantCount uint, wantSize uint64) {
	t.Helper()
	if gotCount != wantCount || gotSize != wantSize {
		t.Fatalf("got count=%d size=%d, want count=%d size=%d", gotCount, gotSize, wantCount, wantSize)
	}
}

func TestWalkFileTypeStatsDBScansFilesAndQueriesDB(t *testing.T) {
	root := writeTestTree(t)
	dbfile := filepath.Join(t.TempDir(), "scan.sqlite")

	if _, err := WalkFileTypeStatsDB([]string{root}, dbfile); err != nil {
		t.Fatal(err)
	}

	fdb, err := ftsdb.New(dbfile, false)
	if err != nil {
		t.Fatal(err)
	}
	defer fdb.Close()

	stats, err := fdb.FTStatsSum([]string{utils.DirStar(root)})
	if err != nil {
		t.Fatal(err)
	}

	assertStat(t, stats["image"].FileCount, stats["image"].NumBytes, 2, uint64(len(testPNG)*2))
	assertStat(t, stats["application"].FileCount, stats["application"].NumBytes, 2, uint64(len(testPDF)*2))
	assertStat(t, stats["dir"].FileCount, stats["dir"].NumBytes, 2, 0)

	dump, err := fdb.FTDumpPaths([]string{filepath.Join(root, "image.png")})
	if err != nil {
		t.Fatal(err)
	}
	if len(*dump) != 1 || (*dump)[0].FType != "image" || (*dump)[0].NumBytes != uint64(len(testPNG)) {
		t.Fatalf("unexpected dump result: %#v", *dump)
	}
}

func TestTreeStatsWatcherScanDirAndQueryWrappers(t *testing.T) {
	root := writeTestTree(t)
	dbfile := filepath.Join(t.TempDir(), "watcher.sqlite")

	fdb, err := ftsdb.New(dbfile, true)
	if err != nil {
		t.Fatal(err)
	}
	defer fdb.Close()

	tsw, err := NewTreeStatsWatcher(nil, fdb)
	if err != nil {
		t.Fatal(err)
	}
	tsw.AddDir(root, true, tsw.eventHandler)

	if err := tsw.ScanDir(root); err != nil {
		t.Fatal(err)
	}

	statsFromDB, err := treestatsquery.FTStatsSumDB(fdb, []string{utils.DirStar(root)})
	if err != nil {
		t.Fatal(err)
	}
	assertStat(t, statsFromDB["image"].FileCount, statsFromDB["image"].NumBytes, 2, uint64(len(testPNG)*2))
	assertStat(t, statsFromDB["application"].FileCount, statsFromDB["application"].NumBytes, 2, uint64(len(testPDF)*2))

	statsFromFile, err := treestatsquery.FTStatsSum(dbfile, []string{filepath.Join(root, "nested") + string(os.PathSeparator)})
	if err != nil {
		t.Fatal(err)
	}
	assertStat(t, statsFromFile["image"].FileCount, statsFromFile["image"].NumBytes, 1, uint64(len(testPNG)))
	assertStat(t, statsFromFile["application"].FileCount, statsFromFile["application"].NumBytes, 1, uint64(len(testPDF)))
}
