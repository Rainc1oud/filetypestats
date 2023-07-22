package ftsdb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestFileTypeStatsDB_New(t *testing.T) {

	cwd, _ := os.Getwd()
	dtmp, err := os.MkdirTemp(cwd, ".tmp-")
	defer os.RemoveAll(dtmp)
	if err != nil {
		t.Fatal(err.Error())
	}

	fdb, err := New(filepath.Join(dtmp, "testdb.sqlite"), true)
	if err != nil {
		t.Fatal(err.Error())
	}

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
