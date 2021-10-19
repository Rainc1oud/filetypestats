package filetypestats

import (
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
	var fdb *ftsdb.FileTypeStatsDB
	var err error

	if fdb, err = ftsdb.NewNoOpen(filedb); err != nil {
		return nil, err
	}
	return &TreeStatsInfo{
		dbFile: filedb,
		ftsDB:  fdb,
	}, nil
}

// FTStatsDirs returns the FileTypeStats per dir
// call with dir="/my/dir/*" to get the recursive totals under that dir, or set recursive=true
func (tsi *TreeStatsInfo) FTStatsDirs(dirs []string, recursive bool) (types.FileTypeDirStats, error) {
	tsi.ftsDB.Open()
	if recursive {
		for i, d := range dirs {
			dirs[i] = filepath.Join(d, "*")
		}
	}
	res, err := tsi.ftsDB.FTStatsDirs(dirs)
	tsi.ftsDB.Close()
	return res, err
}
