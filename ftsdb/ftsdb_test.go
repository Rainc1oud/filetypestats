package ftsdb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// func TestTypeCategories(t *testing.T) {
// 	// var ms = matchers.Matchers
// 	var mk = matchers.MatcherKeys
// 	// for _, m := range ms {
// 	// 	fmt.Printf("key: %v", m)
// 	// }
// 	cats := []string{}
// 	for _, k := range mk {
// 		fmt.Printf("key: %v", k.MIME.Value)
// 		if !gogenutils.InSlice(k.MIME.Type, cats) {
// 			cats = append(cats, k.MIME.Type)
// 		}
// 	}
// 	fmt.Printf("key: %v", cats)
// }

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
	// fmt.Printf("res: %v\n", res)
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
