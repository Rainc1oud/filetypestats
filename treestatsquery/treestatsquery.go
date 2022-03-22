package treestatsquery

import (
	"fmt"

	"github.com/ppenguin/filetypestats/ftsdb"
	"github.com/ppenguin/filetypestats/types"
)

// query functions for the file info DB generated and maintained by TreeStatsWatcher
// this is a static package (no object instantiation), since we rely on one-off on-demand queries
// and don't need/want to hold state

// FTStatsSum returns the summary FileTypeStats for the given paths as a map of FTypeStat per File Type
// Paths can be files or directories. The summary is counted like this for the respective path format
// path="/my/dir/*" => count /my/dir/ and below recursively
// path="/my/dir*/*" => count all dirs matching /my/dir*/ and below recursively
// path="/my/dir/" => count ony the contents of /my/dir/
// path="/my/dir*/" => count ony the contents of dirs matching /my/dir*/
// path="/my/file" => count only "/my/file"
// path="/my/file*" => count all files matching "/my/file*"
func FTStatsSum(dbfile string, paths []string) (types.FileTypeStats, error) {
	var err error
	var fdb *ftsdb.FileTypeStatsDB

	if fdb, err = ftsdb.New(dbfile, false); err != nil {
		return types.FileTypeStats{}, err
	}
	defer fdb.Close()

	res, err := fdb.FTStatsSum(paths)
	return res, err
}

func FTStatsSumDB(dbconn *ftsdb.FileTypeStatsDB, paths []string) (types.FileTypeStats, error) {
	var err error
	if dbconn == nil {
		err = fmt.Errorf("invalid: dbconn==nil")
	} else if !dbconn.IsOpened {
		err = fmt.Errorf("dbconn is not open")
	}
	if err != nil {
		return types.FileTypeStats{}, err
	}

	res, err := dbconn.FTStatsSum(paths)
	return res, err
}
