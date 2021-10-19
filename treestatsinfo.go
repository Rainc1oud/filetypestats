package filetypestats

import "github.com/ppenguin/filetypestats/ftsdb"

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
