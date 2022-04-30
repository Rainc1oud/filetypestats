package filetypestats

// legacy definitions for backwards compatibility
// recommended usage through TreeFileTypeStats

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/Rainc1oud/filetype"
	"github.com/karrick/godirwalk"

	"github.com/Rainc1oud/filetypestats/ftsdb"
	"github.com/Rainc1oud/filetypestats/types"
	"github.com/Rainc1oud/gogenutils"
)

func WalkFileTypeStatsDB(scanDirs []string, dbfile string) (types.FileTypeStats, error) {
	var err error
	var fdb *ftsdb.FileTypeStatsDB
	ftStats := make(types.FileTypeStats)
	pFtStats := &ftStats

	if fdb, err = ftsdb.New(dbfile, true); err != nil {
		return nil, err
	}

	sdirs := gogenutils.FilterCommonRootDirs(scanDirs)
	if len(sdirs) < 1 {
		return nil, fmt.Errorf("WalkFileTypeStats:: no scan path(s) specified")
	}

	for _, d := range sdirs {
		if err = fileTypeStatsDB(d, fdb); err != nil {
			return nil, err
		}
	}
	// TODO: do query and return stats in result
	fdb.Close()
	return *pFtStats, nil
}

func fileTypeStatsDB(scanRoot string, fdb *ftsdb.FileTypeStatsDB) error {

	if err := godirwalk.Walk(scanRoot, &godirwalk.Options{
		AllowNonDirectory: true,
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			var (
				err   error = nil
				fi    fs.FileInfo
				ftype string
			)

			if de.IsDir() {
				ftype = "dir"
				fdb.UpdateFileStats(osPathname+"/", ftype, 0) // add / to make filtering more consistent in SELECT queries
			} else if de.IsRegular() {
				fi, err = os.Stat(osPathname)
				if err == nil {
					if ftype, err = filetype.FileClass(osPathname); err == nil {
						fdb.UpdateFileStats(osPathname, ftype, uint64(fi.Size()))
						return nil
					}
				}
			}

			if err != nil {
				fmt.Fprint(os.Stderr, err.Error())
			}
			return nil
		},
		Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
		ErrorCallback: func(s string, e error) godirwalk.ErrorAction {
			fmt.Fprintf(os.Stderr, "warning: %s reading %s\n", e.Error(), s)
			return godirwalk.SkipNode
		},
	}); err != nil {
		return err
	}
	return nil
}
