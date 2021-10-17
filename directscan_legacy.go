package filetypestats

// legacy definitions for backwards compatibility
// recommended usage through TreeFileTypeStats

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/karrick/godirwalk"
	"github.com/ppenguin/filetype"

	"github.com/ppenguin/filetypestats/types"
	"github.com/ppenguin/gogenutils"
)

func WalkFileTypeStats(scanDirs []string) (types.FileTypeStats, error) {
	var err error
	ftStats := make(types.FileTypeStats)
	pFtStats := &ftStats

	sdirs := gogenutils.FilterCommonRootDirs(scanDirs)
	if len(sdirs) < 1 {
		return nil, fmt.Errorf("WalkFileTypeStats:: no scan path(s) specified")
	}

	for _, d := range sdirs {
		if pFtStats, err = fileTypeStats(d, &ftStats); err != nil {
			return nil, err
		}
	}
	return *pFtStats, nil
}

func fileTypeStats(scanRoot string, statsData *types.FileTypeStats) (*types.FileTypeStats, error) {

	if err := godirwalk.Walk(scanRoot, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			var (
				err   error = nil
				fi    fs.FileInfo
				ftype string
			)

			if de.IsRegular() {
				// fullpath := osPathname + "/" + de.Name()
				fi, err = os.Stat(osPathname)
				if err == nil {
					if ftype, err = filetype.FileClass(osPathname); err == nil {
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

func WalkFileSizeCount(scanDirs []string) (*types.FTypeStat, error) {
	var err error
	pFStats := &types.FTypeStat{}

	sdirs := gogenutils.FilterCommonRootDirs(scanDirs)
	if len(sdirs) < 1 {
		return nil, fmt.Errorf("WalkFileTypeStats:: no scan path(s) specified")
	}

	for _, d := range sdirs {
		if pFStats, err = fileSizeCount(d, pFStats); err != nil {
			return nil, err
		}
	}
	return pFStats, nil
}

// fileSizeCount is the recursive callback that just counts number and size of files
func fileSizeCount(scanRoot string, fstats *types.FTypeStat) (*types.FTypeStat, error) {

	if err := godirwalk.Walk(scanRoot, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			var (
				err error = nil
				fi  fs.FileInfo
			)

			if de.IsRegular() {
				// fullpath := osPathname + "/" + de.Name()
				fi, err = os.Stat(osPathname)
				if err == nil {
					fstats.FileCount += 1
					fstats.NumBytes += uint64(fi.Size())
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
	return fstats, nil
}
