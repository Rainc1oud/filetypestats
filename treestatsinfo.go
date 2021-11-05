package filetypestats

import (
	"fmt"
	"path/filepath"

	"github.com/ppenguin/filetypestats/ftsdb"
	"github.com/ppenguin/filetypestats/types"
)

// query functions for the file info DB generated and maintained by TreeStatsWatcher

type TreeStatsInfo struct {
	dbFile string
	ftsDB  *ftsdb.FileTypeStatsDB // maybe not needed: we do direct opening and closing for each query
}

// NewTreeStatsInfo instantiates a TreeStatsInfo, a convenience object providing query functions for a file info DB
func NewTreeStatsInfo(filedb string) (*TreeStatsInfo, error) {
	fdb, err := ftsdb.NewNoOpen(filedb)
	return &TreeStatsInfo{
		dbFile: filedb,
		ftsDB:  fdb,
	}, err // we always return an object, because the open call for the DB is basically just a test, if no DB it can come into existence later
}

// FTStatsDirs returns the FileTypeStats per dir
// call with dir="/my/dir/*" to get the recursive totals under that dir, or set recursive=true
func (tsi *TreeStatsInfo) FTStatsDirs(dirs []string, recursive bool) (types.FileTypeDirStats, error) {
	if err := tsi.openDB(); err != nil {
		return types.FileTypeDirStats{}, err
	}
	if recursive {
		for i, d := range dirs {
			dirs[i] = filepath.Join(d, "*")
		}
	}
	res, err := tsi.ftsDB.FTStatsDirs(dirs)
	tsi.closeDB()
	return res, err
}

func (tsi *TreeStatsInfo) openDB() error {
	if tsi.ftsDB != nil {
		tsi.ftsDB.Open()
	}
	return fmt.Errorf("TreeStatsInfo: no DB assigned, no file info available")
}

func (tsi *TreeStatsInfo) closeDB() {
	if tsi.ftsDB != nil {
		tsi.ftsDB.Close()
	}
}
