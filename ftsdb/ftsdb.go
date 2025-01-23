package ftsdb

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Rainc1oud/filetypestats/types"
	"github.com/Rainc1oud/filetypestats/utils"
	_ "github.com/mattn/go-sqlite3"
)

// file categories are added when encountered, no need to hard-code and/or init in the DB
// => actually not such a good idea, because it forces us to do a "selsert" for every DB mutation, which is expensive!
// So: init the filetypes table when creating the DB

type FileTypeStatsDB struct {
	fileName string
	DB       *sql.DB
	IsOpened bool
	dbmutex  sync.Mutex
}

// New returns a DB instance to the sqlite db in existing file or creates it if it doesn't exist and create==true
func New(file string, create bool) (*FileTypeStatsDB, error) {
	var err error
	ftdb := new(FileTypeStatsDB)
	ftdb.fileName = file

	if ftdb.DB, err = openDB(file, create); err != nil {
		return nil, err
	}
	err = ftdb.initDB()
	ftdb.IsOpened = true
	return ftdb, err
}

// would a sensible strategy be to only init upon create?
// or should init include a check whether the DB (if exists) is indeed a valid initialised one?
// In that case we should evaluate the init() (return error)
func openDB(dbfile string, create bool) (*sql.DB, error) {
	var err error
	var db *sql.DB

	if _, err = os.Open(dbfile); err != nil {
		if os.IsNotExist(err) {
			if create {
				if _, err := os.Create(dbfile); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	if db, err = sql.Open("sqlite3", dbfile); err != nil {
		return nil, err
	}
	return db, nil
}

func (f *FileTypeStatsDB) Open() error {
	var err error
	if !f.IsOpened {
		if f.DB, err = sql.Open("sqlite3", f.fileName); err != nil {
			return err
		}
	}
	f.IsOpened = true
	return nil
}

func (f *FileTypeStatsDB) Close() {
	f.DB.Close()
	f.IsOpened = false
}

func (f *FileTypeStatsDB) initDB() error {
	if err := f.createTables(); err != nil {
		return err
	}

	if err := f.initCats(); err != nil {
		return err
	}

	return nil
}

func (f *FileTypeStatsDB) initCats() error {
	cats := types.FClassNames()
	qryl := make([]string, len(cats)+2)
	qryl[0] = "BEGIN TRANSACTION"
	i := 1
	for _, c := range cats {
		qryl[i] = fmt.Sprintf(
			`INSERT INTO cats(filecat) VALUES('%s')
				ON CONFLICT(filecat) DO NOTHING`,
			c,
		)
		i += 1
	}
	qryl[i] = "COMMIT;"
	qry := strings.Join(qryl, ";\n")

	if _, err := f.DB.Exec(qry); err != nil {
		return err
	}

	return nil
}

func (f *FileTypeStatsDB) createTables() error {

	// the updated field is INTEGER as unix time (sec), for efficientcy (https://stackoverflow.com/q/31667495/12771809)
	if _, err := f.DB.Exec(
		`CREATE TABLE IF NOT EXISTS fileinfo (
			path TEXT NOT NULL,
			size BIGINT,
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

// Paths are selected according to the following rules:
// Paths can be files or directories. The summary is counted like this for the respective path format
// path="/my/dir/*" => count /my/dir/ and below recursively
// path="/my/dir*/*" => count all dirs matching /my/dir*/ and below recursively
// path="/my/dir/" => count ony the contents of /my/dir/
// path="/my/dir*/" => count ony the contents of dirs matching /my/dir*/
// path="/my/file" => count only "/my/file"
// path="/my/file*" => count all files matching "/my/file*"

// FTDumpPaths returns all paths and raw info selected by the paths argument
// FIXME: this does not work (yet) of len(paths)>500, but this function is (should be!)
// only used in the testcli, so not a real issue
func (f *FileTypeStatsDB) FTDumpPaths(paths []string) (*[]types.FTypeStat, error) {
	wp := f.pathsWherePredicate(paths)
	fts := make([]types.FTypeStat, 0)
	rs, err := f.DB.Query(fmt.Sprintf(
		`SELECT fileinfo.path AS Path, cats.filecat AS Category, fileinfo.size AS Size FROM fileinfo,cats
			WHERE fileinfo.catid=cats.id AND (%s)`,
		wp,
	))
	if err != nil {
		return &fts, err
	}
	defer rs.Close()

	var (
		path     string
		filecat  string
		filesize uint64
	)

	for rs.Next() {
		if err := rs.Scan(&path, &filecat, &filesize); err != nil {
			return &fts, err
		}
		fts = append(fts, types.FTypeStat{Path: path, FType: filecat, NumBytes: filesize, FileCount: 0})
	}
	return &fts, nil
}

// FTStatsSum returns the summary FileTypeStats for the given paths as a map of FTypeStat per File Type
// To facilitate aggregation over multiple SELECTs (to circumvent the max WHERE conditions issue for >1000)
// The strategy becomes to concatenate SELECT results with UNION ALL into a CTE (Common Table Expression),
// then re-select from the CTE and add the "totals" record with UNION
func (f *FileTypeStatsDB) FTStatsSum(paths []string) (types.FileTypeStats, error) {
	const maxWhereCond = 500 // the maximum is 1000, but the where predicate has 2 conditions for each path
	ftstats := make(types.FileTypeStats)

	var qryParts []string
	for start := 0; start < len(paths); start += maxWhereCond {
		end := start + maxWhereCond
		if end > len(paths) {
			end = len(paths)
		}
		wp := f.pathsWherePredicate(paths[start:end])
		qryParts = append(
			qryParts,
			fmt.Sprintf(
				`SELECT cats.filecat AS fcat, fileinfo.path, COUNT(fileinfo.path) AS fcatcount, SUM(fileinfo.size) AS fcatsize FROM fileinfo, cats WHERE fileinfo.catid=cats.id AND (%s) GROUP BY cats.filecat`,
				wp),
		)
	}

	rs, err := f.DB.Query(fmt.Sprintf(
		`WITH CatSum(fcat, path, fcatcount, fcatsize) AS (%s) SELECT fcat, '' AS path, SUM(CatSum.fcatcount) AS fcatcount, SUM(CatSum.fcatsize) AS fcatsize FROM CatSum GROUP BY CatSum.fcat UNION SELECT 'total' AS fcat, '', SUM(CatSum.fcatcount), SUM(CatSum.fcatsize) FROM CatSum`,
		strings.Join(qryParts, ` UNION ALL `)))

	if err != nil {
		return ftstats, err
	}
	defer rs.Close()

	var (
		path       string
		fcat       string
		fcatcount  uint
		fcatsize   uint64
		pathN      sql.NullString
		fcatN      sql.NullString
		fcatcountN sql.NullInt32
		fcatsizeN  sql.NullInt64
	)

	for rs.Next() {
		if err := rs.Scan(&fcatN, &pathN, &fcatcountN, &fcatsizeN); err != nil {
			return ftstats, err
		}
		if !(pathN.Valid && fcatN.Valid && fcatcountN.Valid && fcatsizeN.Valid) { // we had NULL values, just return empty result without error
			return ftstats, nil
		}
		path = pathN.String
		fcat = fcatN.String
		fcatcount = uint(fcatcountN.Int32) // crappy that we don't have sql.NullUInt => will this be a problem???
		fcatsize = uint64(fcatsizeN.Int64)
		if len(paths) == 1 { // the query has specified a single directory pattern, so we use it for the path
			if fcatcount == 1 && fcat != "total" { // there's only one, so we can take the exact path, except for totals take the input path
				ftstats[fcat] = &types.FTypeStat{Path: path, FType: fcat, FileCount: fcatcount, NumBytes: fcatsize}
			} else { // use input pattern for path
				ftstats[fcat] = &types.FTypeStat{Path: paths[0], FType: fcat, FileCount: fcatcount, NumBytes: fcatsize}
			}
		} else {
			ftstats[fcat] = &types.FTypeStat{Path: "*", FType: fcat, FileCount: fcatcount, NumBytes: fcatsize}
		}
	}
	return ftstats, nil
}

// UpdateFileStats upserts the file in path with size
func (f *FileTypeStatsDB) UpdateFileStats(path, filecat string, size uint64) error {
	// upsert file type stats for dir
	if _, err := f.DB.Exec((fmt.Sprintf(
		`INSERT INTO fileinfo(path, size, catid, updated) VALUES('%s', %d, (SELECT id FROM cats WHERE filecat='%s'), %d)
			ON CONFLICT(path) DO
			UPDATE SET size=%d, catid=(SELECT id FROM cats WHERE filecat='%s'), updated=%d`,
		strings.Replace(path, "'", "''", -1), // escape single quotes for SQL
		size, filecat, time.Now().Unix(), size, filecat, time.Now().Unix(),
	))); err != nil {
		return err
	}
	return nil
}

// UpdateFilePath updates the file path(s), which needs to happen on a file move
// if path is a dir the update is recursive
func (f *FileTypeStatsDB) UpdateFilePath(from, to string) error {
	from = strings.Replace(from, "'", "''", -1) // escape single quotes for SQL
	to = strings.Replace(to, "'", "''", -1)
	if _, err := f.DB.Exec((fmt.Sprintf(
		`UPDATE fileinfo SET path=REPLACE(path, '%s', '%s'), updated=%d;`, from, to, time.Now().Unix()))); err != nil {
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

// DeleteOlderThanWithPrefix deletes all entries older than (i.e. not updated after) t
func (f *FileTypeStatsDB) DeleteOlderThanWithPrefix(t time.Time, prefix string) error {
	if _, err := f.DB.Exec((fmt.Sprintf(
		`DELETE FROM fileinfo
		WHERE fileinfo.updated < %d
			AND (fileinfo.path GLOB '%s/*' OR fileinfo.path='%s')`, t.Unix(), prefix, prefix))); err != nil {
		return err
	}
	return nil
}

// DeleteFileStats deletes the file/dir in path, if it's a dir, the delete is recursive
func (f *FileTypeStatsDB) DeleteFileStats(path string) error {
	// if we delete "<path>/*" OR "<path>" from the DB, we catch automatically the recursive case if it was a dir and existed, otherwise we delete just the file
	path = strings.Replace(path, "'", "''", -1) // escape single quotes for SQL

	if _, err := f.DB.Exec((fmt.Sprintf(
		`DELETE FROM fileinfo WHERE
			fileinfo.path GLOB '%s/*' OR fileinfo.path='%s'`, path, path))); err != nil {
		return err
	}
	return nil
}

func (f *FileTypeStatsDB) DbFileName() string {
	return f.fileName
}

// returns table.id where field==value, inserts value if not exist (id must be AUTOINCREMENT)
func (f *FileTypeStatsDB) selsertIdText(table, field, value string) (int, error) {
	value = strings.Replace(value, "'", "''", -1) // escape single quotes for SQL
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

// pathsWherePredicate returns the WHERE clause part selecting the paths according to input dir list
// we'll be using GLOB, translated from the path list to satisfy behaviour as described for FTStatsSum()
func (f *FileTypeStatsDB) pathsWherePredicate(paths []string) string {
	// we can (significantly) optimise the query by removing ineffective paths (duplicates and children of recursive globs) first
	paths = utils.OptimizePathsGlob(&paths)
	pred := make([]string, len(paths))
	for i, d := range paths {
		d = strings.Replace(d, "'", "''", -1)                          // escape single quotes for SQL
		if strings.HasSuffix(d, "*/*") || strings.HasSuffix(d, "/*") { // recursive directory
			pred[i] = fmt.Sprintf("(fileinfo.path GLOB '%s')", d)
		} else if strings.HasSuffix(d, "/") || strings.HasSuffix(d, "*/") { // specific directory or directory pattern
			pred[i] = fmt.Sprintf("(fileinfo.path GLOB '%s*' AND NOT fileinfo.path GLOB '%s*/*')", d, d)
		} else { // exact file path or file pattern
			pred[i] = fmt.Sprintf("(fileinfo.path GLOB '%s' AND NOT fileinfo.path GLOB '%s/*')", d, d)
		}
	}
	return strings.Join(pred, " OR ")
}
