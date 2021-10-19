package filetypestats

// TODO: allow to optionally only do direct scan, without db or inotify, to supersede legacy code

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/karrick/godirwalk"
	"github.com/ppenguin/filetype"
	"github.com/ppenguin/filetypestats/ftsdb"
	"github.com/ppenguin/filetypestats/notifywatch"
	"github.com/ppenguin/gogenutils"
	"github.com/rjeczalik/notify"
)

type TreeStatsWatcher struct {
	dirs            []string // not strictly necessary (because also held in child objects), but a convenience to check dirs later
	dirsWatcher     *notifywatch.NotifyWatchDirs
	ftsDB           *ftsdb.FileTypeStatsDB
	scanning        bool
	lastScanStarted time.Time
}

// NewTreeStatsWatcher is the top level constructor featuring:
//  - a recursive watcher and scanner for all files in the given param dirs
//	- a sqlite DB session (param database: file name)
func NewTreeStatsWatcher(dirs []string, database string) (*TreeStatsWatcher, error) {
	var fdb *ftsdb.FileTypeStatsDB
	var err error

	if fdb, err = ftsdb.New(database, true); err != nil {
		return nil, err
	}
	tfts := &TreeStatsWatcher{
		dirs:     gogenutils.FilterCommonRootDirs(dirs),
		ftsDB:    fdb,
		scanning: false,
	}
	tfts.dirsWatcher = notifywatch.NewNotifyWatchDirs(dirs, tfts.onFileChanged, []notify.Event{notify.Create, notify.Write, notify.Remove}...)
	// defer tfts.ftsDB.Close() // this only closes the DB after this object is GC'd?
	return tfts, nil
}

// Watch all registered dirs with the notify watcher
func (tfts *TreeStatsWatcher) Watch() error {
	err := tfts.dirsWatcher.WatchAll()
	tfts.ftsDB.Close() // TODO: close DB after all watchers finished... probably opening/closing should be one level deeper?
	return err
}

// ScanFullSync does a full scan over all registered dirs synchronously and updates the database
// This can take a long time (minutes to hours) to complete
func (tfts *TreeStatsWatcher) ScanSync() error {
	tfts.scanning = true
	tfts.lastScanStarted = time.Now()
	var errl []string
	for _, d := range tfts.dirs {
		if err := tfts.scanDir(d); err != nil {
			errl = append(errl, fmt.Sprintf("error [%s]: %s", d, err.Error()))
		}
	}

	tfts.ftsDB.DeleteOlderThan(tfts.lastScanStarted) // delete all entries from before the scan (i.e. not updated during the scan, because this means they were deleted)
	tfts.scanning = false

	if len(errl) > 0 {
		return fmt.Errorf(strings.Join(errl, "\n"))
	}
	return nil
}

// scanDir scans the given scanRoot recursively and updates the database
// This can take a long time (minutes to hours) to complete
func (tfts *TreeStatsWatcher) scanDir(scanRoot string) error {

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
				tfts.ftsDB.UpdateFileStats(osPathname+"/", ftype, 0) // add / to make filtering more consistent in SELECT queries
			} else if de.IsRegular() {
				fi, err = os.Stat(osPathname)
				if err == nil {
					if ftype, err = filetype.FileClass(osPathname); err == nil {
						tfts.ftsDB.UpdateFileStats(osPathname, ftype, uint64(fi.Size()))
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

// onFileChanged is the inotify event handler passed to the notify watcher
// for now we handle create, remove, write (this is like modify but guaranteed on all platforms)
func (tfts *TreeStatsWatcher) onFileChanged(eventInfo *notify.EventInfo) error {
	switch (*eventInfo).Event() {
	case notify.Create, notify.Write:
		if fts, err := getFTStat((*eventInfo).Path()); err == nil {
			return tfts.ftsDB.UpdateFileStats(fts.Path, fts.FType, fts.NumBytes)
		} // any stat errors are simply ignored
	case notify.Remove:
		return tfts.ftsDB.DeleteFileStats((*eventInfo).Path()) // we can't know for sure whether it's a dir or regular file?
	}
	return fmt.Errorf("unhandled event %v for %s", eventInfo, (*eventInfo).Path())
}
