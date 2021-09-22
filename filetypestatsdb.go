package filetypestats

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/karrick/godirwalk"
	"github.com/ppenguin/filetype"

	"github.com/ppenguin/filetypestats/ftsdb"
	"github.com/ppenguin/filetypestats/types"
	"github.com/ppenguin/gogenutils"
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
		if pFtStats, err = fileTypeStatsDB(d, &ftStats, fdb); err != nil {
			return nil, err
		}
	}
	fdb.Close()
	return *pFtStats, nil
}

func fileTypeStatsDB(scanRoot string, statsData *types.FileTypeStats, fdb *ftsdb.FileTypeStatsDB) (*types.FileTypeStats, error) {

	if err := godirwalk.Walk(scanRoot, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			var (
				err   error = nil
				fi    fs.FileInfo
				ftype string
			)

			if de.IsDir() { // TODO: is it guaranteed that this happens before the files in this dir are scanned?
				fdb.ResetDirStats(osPathname) // set counters to 0 in DB for this dir
			} else if de.IsRegular() {
				// fullpath := osPathname + "/" + de.Name()
				fi, err = os.Stat(osPathname)
				if err == nil {
					if ftype, err = filetype.FileClass(osPathname); err == nil {
						fdb.UpdateDirStatsAdd(filepath.Dir(osPathname), ftype, &types.FTypeStat{FileCount: 1, NumBytes: uint64(fi.Size())})
						// statsData only for testing, can be removed later because all persistence should go through the DB
						if (*statsData)[ftype] == nil {
							(*statsData)[ftype] = new(types.FTypeStat)
						}
						(*statsData)[ftype].FileCount += 1
						(*statsData)[ftype].NumBytes += uint64(fi.Size())
						return err
					}
				}
			}

			if err != nil {
				fmt.Fprint(os.Stderr, err.Error())
			}
			return err
		},
		Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
		ErrorCallback: func(s string, e error) godirwalk.ErrorAction {
			fmt.Fprintf(os.Stderr, "warning: %s reading %s\n", e.Error(), s)
			return godirwalk.SkipNode
		},
	}); err != nil {
		return nil, err
	}
	return statsData, nil
}

// func WalkFileSizeCountDB(scanDirs []string) (*types.FTypeStat, error) {
// 	var err error
// 	pFStats := &types.FTypeStat{}

// 	sdirs := gogenutils.FilterCommonRootDirs(scanDirs)
// 	if len(sdirs) < 1 {
// 		return nil, fmt.Errorf("WalkFileTypeStats:: no scan path(s) specified")
// 	}

// 	for _, d := range sdirs {
// 		if pFStats, err = fileSizeCountDB(d, pFStats); err != nil {
// 			return nil, err
// 		}
// 	}
// 	return pFStats, nil
// }

// // fileSizeCount is the recursive callback that just counts number and size of files
// func fileSizeCountDB(scanRoot string, fstats *types.FTypeStat) (*types.FTypeStat, error) {

// 	if err := godirwalk.Walk(scanRoot, &godirwalk.Options{
// 		Callback: func(osPathname string, de *godirwalk.Dirent) error {
// 			var (
// 				err error = nil
// 				fi  fs.FileInfo
// 			)

// 			if de.IsRegular() {
// 				// fullpath := osPathname + "/" + de.Name()
// 				fi, err = os.Stat(osPathname)
// 				if err == nil {
// 					fstats.FileCount += 1
// 					fstats.NumBytes += uint64(fi.Size())
// 				}
// 			}

// 			if err != nil {
// 				fmt.Fprint(os.Stderr, err.Error())
// 			}
// 			return err
// 		},
// 		Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
// 		ErrorCallback: func(s string, e error) godirwalk.ErrorAction {
// 			fmt.Fprintf(os.Stderr, "warning: %s reading %s\n", e.Error(), s)
// 			return godirwalk.SkipNode
// 		},
// 	}); err != nil {
// 		return nil, err
// 	}
// 	return fstats, nil
// }
