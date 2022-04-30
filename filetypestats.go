package filetypestats

// legacy definitions for backwards compatibility
// recommended usage through TreeFileTypeStats

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/Rainc1oud/filetype"

	"github.com/Rainc1oud/filetypestats/types"
)

func getFTStat(path string) (*types.FTypeStat, error) {
	var (
		err error = nil
		fi  fs.FileInfo
	)

	if fi, err = os.Lstat(path); err != nil {
		return nil, err
	}

	fts := &types.FTypeStat{}

	if fi.IsDir() {
		fts.FType = "dir"
		fts.Path = path + "/" // add / to make filtering more consistent in SELECT queries
		fts.FileCount = 0
		fts.NumBytes = 0
		return fts, nil
	}

	if fts.FType, err = filetype.FileClass(path); err == nil {
		fts.Path = path
		fts.NumBytes = uint64(fi.Size())
		fts.FileCount = 1 // unnecessary, we may need to optimise the handling
		return fts, nil
	}
	return nil, fmt.Errorf("no info could be obtained for %v", fi)
}
