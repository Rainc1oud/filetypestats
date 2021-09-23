package ftsdb

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ppenguin/filetypestats/types"
)

type FileTypeStatsDB struct {
	// self *FileTypeStatsDB
	fileName string
	DB       *sql.DB
}

var FileCategories = func() []string { return []string{"Audio", "Video", "Image", "Application", "Other"} }

// New returns a DB instance to the sqlite db in existing file or creates it if it doesn't exist and create==true
func New(file string, create bool) (*FileTypeStatsDB, error) {
	var err error
	ftdb := new(FileTypeStatsDB)
	ftdb.fileName = file

	if _, err = os.Open(file); err != nil {
		if os.IsNotExist(err) {
			if create {
				if _, err := os.Create(file); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	if ftdb.DB, err = sql.Open("sqlite3", file); err != nil {
		return nil, err
	}

	if err = ftdb.init(); err != nil {
		return nil, err
	}
	return ftdb, nil
}

func (f *FileTypeStatsDB) Close() {
	f.DB.Close()
}

func (f *FileTypeStatsDB) init() error {
	if err := f.createTables(); err != nil {
		return err
	}
	// initialise cats table with same categories as used in the scanning functions
	// not found out yet how to query filetypes for the main categories registered, so we use a constant for now
	// vp := make([]string, len(FileCategories()))
	// for i, cat := range FileCategories() {
	// 	vp[i] = fmt.Sprintf("('%s')", cat)
	// }
	// if err := f.qryCUD(fmt.Sprintf("INSERT INTO cats(filecat) VALUES %s ON CONFLICT(filecat) DO NOTHING", strings.Join(vp, ","))); err != nil {
	// 	return err
	// }
	return nil
}

// qryCUD is a simple Create/Update/Delete query with only error return
func (f *FileTypeStatsDB) qryCUD(query string) error {
	var qry *sql.Stmt
	var err error
	if qry, err = f.DB.Prepare(query); err != nil {
		return err
	}
	defer qry.Close()

	if _, err = qry.Exec(); err != nil {
		return err
	}
	return nil
}

func (f *FileTypeStatsDB) createTables() error {

	if err := f.qryCUD(
		`CREATE TABLE IF NOT EXISTS dirs (
			id  INTEGER PRIMARY KEY,
			dir TEXT UNIQUE,
			count UNSIGNED INT,
			size UNSIGNED BIGINT
		);`); err != nil {
		return err
	}

	if err := f.qryCUD(
		`CREATE TABLE IF NOT EXISTS dircatstats (
			dirid INTEGER NOT NULL,
			catid INTEGER NOT NULL,
			count UNSIGNED INT,
			size UNSIGNED BIGINT,
			PRIMARY KEY (dirid, catid)
		);`); err != nil {
		return err
	}

	if err := f.qryCUD(
		`CREATE TABLE IF NOT EXISTS cats (
			id INTEGER PRIMARY KEY,
			filecat TEXT UNIQUE
		);`); err != nil {
		return err
	}

	return nil
}

// FTStatsDirsSum returns the FileTypeStats summed over the given dirs
// call with dir="/my/dir/*" to get the recursive totals under that dir
func (f *FileTypeStatsDB) FTStatsDirsSum(dirs []string) (types.FileTypeStats, error) {
	pred := make([]string, len(dirs))
	for i, d := range dirs {
		if strings.Contains(d, "*") {
			pfx := strings.TrimSuffix(strings.Split(d, "*")[0], "/") // somehow we need the extra trim for the numbers to add up, as well as the exact match
			pred[i] = fmt.Sprintf("dirs.dir LIKE '%s%%' OR dirs.dir='%s'", pfx, pfx)
		} else {
			pred[i] = fmt.Sprintf("dirs.dir='%s'", d)
		}
	}
	wpreds := strings.Join(pred, " OR")
	rs, err := f.DB.Query(fmt.Sprintf(
		"SELECT cats.filecat, SUM(dircatstats.count) AS fcatcount, SUM(dircatstats.size) AS fcatsize"+
			" FROM dirs, cats, dircatstats"+
			" WHERE dircatstats.catid=cats.id AND dircatstats.dirid=dirs.id AND (%s)"+
			" GROUP BY filecat", wpreds))
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	var (
		filecat   string
		fcatcount uint
		fcatsize  uint64
		fstats    = make(types.FileTypeStats)
	)
	for rs.Next() {
		if err := rs.Scan(&filecat, &fcatcount, &fcatsize); err != nil {
			return nil, err
		}
		fstats[filecat] = &types.FTypeStat{FileCount: fcatcount, NumBytes: fcatsize}
	}
	return fstats, nil
}

// ResetDirStats sets all counters for this dir to zero
func (f *FileTypeStatsDB) ResetDirStats(dir string) error {
	rs, err := f.DB.Query(fmt.Sprintf("SELECT * FROM dirs WHERE dir='%s'", dir))
	if err != nil {
		return nil
	}
	defer rs.Close()
	if rs.Next() {
		var dirid int
		var dirpath string
		rs.Scan(&dirid, &dirpath)
		// set all categories for this dir to 0
		if err := f.qryCUD(fmt.Sprintf("UPDATE dircatstats SET count=0, size=0 WHERE dirid=%d", dirid)); err != nil {
			return nil
		}
		if err := f.qryCUD(fmt.Sprintf("UPDATE dirs SET count=0, size=0 WHERE id=%d", dirid)); err != nil {
			return nil
		}

	}
	return nil
}

// UpdateDirStatsAdd adds the count and size of the parameter fstats to the values for filecat for dir dir
func (f *FileTypeStatsDB) UpdateDirStatsAdd(dir, filecat string, fstats *types.FTypeStat) error {
	catid, err := f.selsertIdText("cats", "filecat", filecat)
	if err != nil {
		return err
	}
	dirid, err := f.selsertIdText("dirs", "dir", dir) // should combine with last query for performance
	if err != nil {
		return err
	}
	// upsert file type stats for dir
	if err := f.qryCUD(fmt.Sprintf(
		"INSERT INTO dircatstats(dirid, catid, count, size) VALUES(%d, %d, %d, %d)"+
			" ON CONFLICT(dirid, catid) DO"+
			" UPDATE SET count=count+%d, size=size+%d", dirid, catid, fstats.FileCount, fstats.NumBytes, fstats.FileCount, fstats.NumBytes)); err != nil {
		return err
	}
	// update dir totals for dir
	return f.qryCUD(fmt.Sprintf("UPDATE dirs SET count=count+%d, size-size+%d WHERE id=%d", fstats.FileCount, fstats.NumBytes, dirid))
}

// returns table.id where field==value, inserts value if not exist (id must be AUTOINCREMENT)
func (f *FileTypeStatsDB) selsertIdText(table, field, value string) (int, error) {
	var id int
	rs, err := f.DB.Query(fmt.Sprintf("SELECT id FROM %s WHERE %s='%s'", table, field, value))
	if err != nil {
		return -1, err
	}
	defer rs.Close() // important, otherwise later we get "locked" errors
	if rs.Next() {
		if err := rs.Scan(&id); err != nil {
			return -1, err
		}
		return id, nil
	}
	r := f.DB.QueryRow(fmt.Sprintf("INSERT INTO %s(%s) VALUES('%s') RETURNING id", table, field, value))
	if err := r.Scan(&id); err != nil {
		return -1, err
	}
	return id, nil
}

// RDirStats reads stats for dir
func RDirStats(dir string) *types.FileTypeStats {
	return nil
}
