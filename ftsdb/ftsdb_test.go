package ftsdb

import (
	"fmt"
	rand "math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/Rainc1oud/filetypestats/types"
	"github.com/google/go-cmp/cmp"
)

func tmpDB(t *testing.T) (fdb *FileTypeStatsDB) {
	cwd, _ := os.Getwd()
	dtmp, err := os.MkdirTemp(cwd, ".tmp-")
	if err != nil {
		t.Fatal(err.Error())
	}
	fdb, err = New(filepath.Join(dtmp, "testdb.sqlite"), true)
	if err != nil {
		t.Fatal(err.Error())
	}
	return fdb
}

func TestFileTypeStatsDB_New(t *testing.T) {

	fdb := tmpDB(t)
	defer os.RemoveAll(path.Dir(fdb.DbFileName()))

	rows, err := fdb.DB.Query(`SELECT * FROM cats`)
	if err != nil {
		t.Fatal(err.Error())
	}

	var (
		id      int
		filecat string
	)
	for rows.Next() {
		if err := rows.Scan(&id, &filecat); err != nil {
			t.Fatal(err.Error())
		}
		fmt.Printf("id: %d\tfilecat: %s\n", id, filecat)
	}
	rows.Close()
}

func fstatsSum(fts types.FileTypeStats) (ftss types.FileTypeStats) {
	ftss = make(types.FileTypeStats)
	ftss["total"] = &types.FTypeStat{FType: "total", NumBytes: 0, FileCount: 0}
	for _, s := range fts {
		if v, ok := ftss[s.FType]; ok {
			v.FileCount += s.FileCount
			v.NumBytes += s.NumBytes
		} else {
			ftss[s.FType] = &types.FTypeStat{FType: s.FType, NumBytes: s.NumBytes, FileCount: s.FileCount}
		}
		ftss["total"].NumBytes += s.NumBytes
		ftss["total"].FileCount += s.FileCount
	}
	return ftss
}

func TestFileTypeStatsDB_FTStatsSum(t *testing.T) {

	fdb := tmpDB(t)
	defer os.RemoveAll(path.Dir(fdb.DbFileName()))

	var (
		totalFileSize uint64 = 0
		totalFiles    int    = 2789
	)

	allFiles := make(types.FileTypeStats)
	selFiles := make(types.FileTypeStats)
	selPaths := make([]string, 0)
	rng := rand.New(rand.NewPCG(1, 2))
	fclasses := types.FClassNames()

	// fill database with test data
	for n := 0; n < totalFiles; n++ {
		fsize := uint64(rng.Int64N(9000000))
		fpath := fmt.Sprintf("/somedir/file%04d.tmp", n)
		fcat := fclasses[rng.IntN(len(fclasses))]
		fdb.UpdateFileStats(fpath, fcat, fsize)
		allFiles[fpath] = &types.FTypeStat{Path: fpath, FType: fcat, NumBytes: fsize, FileCount: 1}
		totalFileSize += fsize
		if rng.IntN(9) > 1 { // add to a selection based on random
			selFiles[fpath] = &types.FTypeStat{Path: fpath, FType: fcat, NumBytes: fsize, FileCount: 1}
			selPaths = append(selPaths, fpath)
		}
	}

	type args struct {
		paths []string
	}
	tests := []struct {
		name    string
		args    args
		want    types.FileTypeStats
		wantErr bool
	}{
		{
			name:    "all files with wildcard",
			args:    args{paths: []string{"/somedir/**"}},
			want:    fstatsSum(allFiles),
			wantErr: false,
		},
		{
			name:    fmt.Sprintf("%d selected paths", len(selPaths)),
			args:    args{paths: selPaths},
			want:    fstatsSum(selFiles),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fdb.FTStatsSum(tt.args.paths)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileTypeStatsDB.FTStatsSum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// tweak got since for a deep compare path is spurious info (basically a "bonus" representing the original query)
			for _, v := range got {
				v.Path = ""
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("FileTypeStatsDB.FTStatsSum() = \n%v, want %v", got.ToString(), tt.want.ToString())
			}
			t.Logf("Test data successfully compared equal:\nFileTypeStatsDB.FTStatsSum() = \n%v", got.ToString())
		})
	}
}

func TestFileTypeStatsDB_BatchDumpMoveAndDelete(t *testing.T) {
	fdb := tmpDB(t)
	defer os.RemoveAll(path.Dir(fdb.DbFileName()))

	batch := types.NewFTypeStatsBatch(3)
	if err := fdb.UpdateFileStatsMulti("/tree/", "dir", 0, batch); err != nil {
		t.Fatal(err)
	}
	if err := fdb.UpdateFileStatsMulti("/tree/image.png", "image", 42, batch); err != nil {
		t.Fatal(err)
	}
	if err := fdb.CommitBatch(batch); err != nil {
		t.Fatal(err)
	}
	if !batch.IsEmpty() {
		t.Fatal("batch should be empty after commit")
	}

	dump, err := fdb.FTDumpPaths([]string{"/tree/*"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*dump) != 2 {
		t.Fatalf("dump length = %d, want 2: %#v", len(*dump), *dump)
	}

	if err := fdb.UpdateFilePath("/tree/image.png", "/tree/moved.png"); err != nil {
		t.Fatal(err)
	}
	stats, err := fdb.FTStatsSum([]string{"/tree/moved.png"})
	if err != nil {
		t.Fatal(err)
	}
	if stats["image"].FileCount != 1 || stats["image"].NumBytes != 42 {
		t.Fatalf("moved file stats = %#v, want one 42-byte image", stats["image"])
	}

	if err := fdb.DeleteFileStats("/tree/moved.png"); err != nil {
		t.Fatal(err)
	}
	stats, err = fdb.FTStatsSum([]string{"/tree/*"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stats["image"]; ok {
		t.Fatalf("image stats still present after delete: %#v", stats["image"])
	}
}
