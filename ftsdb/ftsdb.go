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
	vp := make([]string, len(FileCategories()))
	for i, cat := range FileCategories() {
		vp[i] = fmt.Sprintf("('%s')", cat)
	}
	if err := f.qryCUD(fmt.Sprintf("INSERT INTO cats(filecat) VALUES %s ON CONFLICT(filecat) DO NOTHING", strings.Join(vp, ","))); err != nil {
		return err
	}
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

// DbUDirStats updates stats for dir or creates if not exists
func UDirStats(dir string, stats *types.FileTypeStats) error {
	return nil
}

// RDirStats reads stats for dir
func RDirStats(dir string) *types.FileTypeStats {
	return nil
}
