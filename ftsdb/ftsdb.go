package ftsdb

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ppenguin/filetypestats/types"
)

// file categories are added when encountered, no need to hard-code and/or init in the DB
// TODO: somehow the categories seem not to cover all posible types, this might be an issue with h2non/filetype?
// var FileCategories = func() []string { return []string{"Audio", "Video", "Image", "Application", "Other"} }

type FileTypeStatsDB struct {
	// self *FileTypeStatsDB
	fileName string
	DB       *sql.DB
}

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

// NewNoOpen instantiates a FileTypeStatsDB object without opening the DB (but just checking existence of the file)
func NewNoOpen(file string) (*FileTypeStatsDB, error) {
	var err error
	ftdb := new(FileTypeStatsDB)
	ftdb.fileName = file
	if _, err = os.Open(file); err != nil {
		return nil, err
	}
	return ftdb, nil
}

func (f *FileTypeStatsDB) Open() error {
	var err error
	if f.DB, err = sql.Open("sqlite3", f.fileName); err != nil {
		return err
	}
	return nil
}

func (f *FileTypeStatsDB) Close() {
	f.DB.Close()
}

func (f *FileTypeStatsDB) init() error {
	if err := f.createTables(); err != nil {
		return err
	}
	return nil
}

func (f *FileTypeStatsDB) createTables() error {

	// the updated field is INTEGER as unix time (sec), for efficientcy (https://stackoverflow.com/q/31667495/12771809)
	if _, err := f.DB.Exec(
		`CREATE TABLE IF NOT EXISTS fileinfo (
			path TEXT NOT NULL,
			size UNSIGNED BIGINT,
			catid INTEGER NOT NULL,
			updated INTEGER,
			PRIMARY KEY (path)
		);`); err != nil {
		return err
	}

	if _, err := f.DB.Exec(
		`CREATE TABLE IF NOT EXISTS cats (
			id INTEGER PRIMARY KEY,
			filecat TEXT UNIQUE
		);`); err != nil {
		return err
	}

	return nil
}

// FTStatsDirs returns the FileTypeStats per dir
// call with dir="/my/dir/*" to get the recursive totals under that dir
func (f *FileTypeStatsDB) FTStatsDirs(dirs []string) (types.FileTypeDirStats, error) {
	// TODO: maybe nicer solution to get the "top level" path for each listed category?
	wp := f.dirsWherePredicate(dirs)
	rs, err := f.DB.Query(fmt.Sprintf(
		`SELECT fileinfo.path, cats.filecat, COUNT(fileinfo.path) AS fcatcount, SUM(fileinfo.size) AS fcatsize FROM fileinfo, cats
			WHERE fileinfo.catid=cats.id AND (%s)
			GROUP BY cats.filecat
			ORDER BY fileinfo.path`, wp))
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	var (
		dir       string
		path      string
		filecat   string
		fcatcount uint
		fcatsize  uint64
		fdstats   = make(types.FileTypeDirStats)
	)
	for rs.Next() {
		if err := rs.Scan(&path, &filecat, &fcatcount, &fcatsize); err != nil {
			return nil, err
		}
		if strings.HasSuffix(path, "/") { // since we order by path, we can be sure that the first "new" path will be assigned to dir before categories
			dir = path // TODO (maybe): get the query result to list only the dir also for other file categories
			// now the returned dir path is inaccurate if more than 1 dirs are scanned
		}
		if fdstats[dir] == nil {
			ftstats := make(types.FileTypeStats)
			fdstats[dir] = &types.FTypeDirStat{FTypeStats: ftstats, TotCount: 0, TotSize: 0}
		}
		fdstats[dir].FTypeStats[filecat] = &types.FTypeStat{FileCount: fcatcount, NumBytes: fcatsize}
		fdstats[dir].TotCount += fcatcount
		fdstats[dir].TotSize += fcatsize
	}
	return fdstats, nil
}

// UpdateFileStats upserts the file in path with size
func (f *FileTypeStatsDB) UpdateFileStats(path, filecat string, size uint64) error {
	catid, err := f.selsertIdText("cats", "filecat", filecat)
	if err != nil {
		return err
	}
	// upsert file type stats for dir

	if _, err := f.DB.Exec((fmt.Sprintf(
		`INSERT INTO fileinfo(path, size, catid, updated) VALUES('%s', %d, %d, %d) 
			ON CONFLICT(path) DO 
			UPDATE SET size=%d, catid=%d, updated=%d`, path, size, catid, time.Now().Unix(), size, catid, time.Now().Unix()))); err != nil {
		return err
	}
	return nil
}

// DeleteOlderThan deletes all entries older than (i.e. not updated after) t
func (f *FileTypeStatsDB) DeleteOlderThan(t time.Time) error {
	if _, err := f.DB.Exec((fmt.Sprintf(
		`DELETE FROM fileinfo WHERE fileinfo.updated < %d`, t.Unix()))); err != nil {
		return err
	}
	return nil
}

// DeleteFileStats deletes the file/dir in path, if it's a dir, the delete is recursive
func (f *FileTypeStatsDB) DeleteFileStats(path string) error {
	// if we delete "<path>/*" OR "<path>" from the DB, we catch automatically the recursife case if it was a dir and existed, otherwise we delete just the file
	if _, err := f.DB.Exec((fmt.Sprintf(
		`DELETE FROM fileinfo WHERE 
			fileinfo.path GLOB "%s/*" OR fileinfo.path="%s"`, path, path))); err != nil {
		return err
	}
	return nil
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

// dirsWherePredicate returns the WHERE clause part selecting the dirs according to input dir list
// we'll be using GLOB, so the following behaviour is "translated"
// '/path/to/subdir' || '/path/to/subdir/' => GLOB '/path/to/subdir/*' AND NOT GLOB '/path/to/subdir/*/*' => this gives the totals of the FILES in '/path/to/subdir/'
// '/path/to/subdir*' || '/path/to/subdir*/' => GLOB '/path/to/subdir*/*' AND NOT GLOB '/path/to/subdir*/*/*' => this gives the totals of the FILES in '/path/to/subdir*/'
// '/path/to/subdir/*' => GLOB '/path/to/subdir/*' => this gives the totals of the FILES in '/path/to/subdir/' AND BELOW
// '/path/to/subdir*/*' => GLOB '/path/to/subdir*/*' => this gives the totals of the FILES in '/path/to/subdir*/' AND BELOW
func (f *FileTypeStatsDB) dirsWherePredicate(dirs []string) string {
	pred := make([]string, len(dirs))
	for i, d := range dirs {
		if strings.HasSuffix(d, "*/*") || strings.HasSuffix(d, "/*") {
			pred[i] = fmt.Sprintf("(fileinfo.path GLOB '%s')", d)
		} else if strings.HasSuffix(d, "*") || strings.HasSuffix(d, "*/") {
			pred[i] = fmt.Sprintf("(fileinfo.path GLOB '%s*/*' AND NOT fileinfo.path GLOB '%s*/*/*')", d, d)
		} else {
			pred[i] = fmt.Sprintf("(fileinfo.path GLOB '%s/*' AND NOT fileinfo.path GLOB '%s*/*')", d, d)
		}
	}
	return strings.Join(pred, " OR ")
}
