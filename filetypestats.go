package filetypestats

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/karrick/godirwalk"
	"github.com/ppenguin/filetype"
	"github.com/ppenguin/gogenutils"
)

type ftypeStat struct {
	NumBytes  int64
	FileCount int
}

type FileTypeStats map[string]*ftypeStat

func WalkFileTypeStats(scanDirs []string) (FileTypeStats, error) {
	var err error
	ftStats := make(FileTypeStats)
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

func fileTypeStats(scanRoot string, statsData *FileTypeStats) (*FileTypeStats, error) {

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
							(*statsData)[ftype] = new(ftypeStat)
						}
						(*statsData)[ftype].FileCount += 1
						(*statsData)[ftype].NumBytes += fi.Size()
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
