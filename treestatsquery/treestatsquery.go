package treestatsquery

import (
	"github.com/ppenguin/filetypestats/ftsdb"
	"github.com/ppenguin/filetypestats/types"
)

// query functions for the file info DB generated and maintained by TreeStatsWatcher
// this is a static package (no object instantiation), since we rely on one-off on-demand queries
// and don't need/want to hold state

// FTStatsDirs returns the FileTypeStats per dir
// call with dir="/my/dir/*" to get the recursive totals under that dir, or set recursive=true
func FTStatsDirs(dbfile string, dirs []string) (types.FileTypeStats, error) {
	var err error
	var fdb *ftsdb.FileTypeStatsDB

	if fdb, err = ftsdb.New(dbfile, false); err != nil {
		return types.FileTypeStats{}, err
	}
	defer fdb.Close()

	res, err := fdb.FTStatsDirs(dirs)
	return res, err
}
