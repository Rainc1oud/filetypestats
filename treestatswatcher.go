package filetypestats

// TODO: allow to optionally only do direct scan, without db or inotify, to supersede legacy code

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/karrick/godirwalk"
	"github.com/ppenguin/filetype"
	"github.com/ppenguin/filetypestats/ftsdb"
	"github.com/ppenguin/filetypestats/notifywatch"
	"github.com/ppenguin/filetypestats/types"
	"github.com/ppenguin/filetypestats/utils"
	ggu "github.com/ppenguin/gogenutils"
	"github.com/rjeczalik/notify"
)

type TreeStatsWatcher struct {
	dirsWatcher *notifywatch.NotifyWatchDirs
	ftsDB       *ftsdb.FileTypeStatsDB
	dirsStatus  *types.TDirsStatus
}

// NewTreeStatsWatcher is the top level constructor featuring:
//  - a recursive watcher and scanner for all files in the given param dirs
//	- a sqlite DB session (param database: file name)
// An instance is always returned, even if an error occurred
// dirs will be trimmed of trailing suffixes and evaluated recursively
func NewTreeStatsWatcher(dirs []string, database string) (*TreeStatsWatcher, error) {
	var fdb *ftsdb.FileTypeStatsDB
	var err error
	fdb, err = ftsdb.New(database, true)
	tfts := new(TreeStatsWatcher)
	tfts.ftsDB = fdb
	tfts.dirsStatus = types.NewDirsStatus(ggu.FilterCommonRootDirs(utils.StringSliceApply(dirs, utils.JustDir))...)                                                                                     // init queue with all dirs
	tfts.dirsWatcher = notifywatch.NewNotifyWatchDirs(utils.StringSliceApply(tfts.dirsStatus.Dirs(), utils.DirStar), tfts.onFileChanged, []notify.Event{notify.Create, notify.Write, notify.Remove}...) // TODO: (maybe) add way to override/config event types?
	// defer tfts.ftsDB.Close() // this only closes the DB after this object is GC'd?
	return tfts, err // always return a valid watcher instance, we can add dirs and use other features later
}

// AddDirWatch adds a dir and starts watching it
// returns error if dir doesn't exist or is already watched
// make sure to suffix dir with "/*" for recursive watching
func (tfts *TreeStatsWatcher) AddDirWatch(dir string) error {
	if _, err := os.Lstat(utils.JustDir(dir)); err != nil {
		return err
	}
	if tfts.dirsStatus.Contains(utils.JustDir(dir)) { // ignore suffixes indicating recursive watch
		return fmt.Errorf("%s already in watched dirs (%v), refusing to add", dir, tfts.dirsStatus.Dirs())
	}
	tfts.dirsStatus.AddDir(utils.JustDir(dir))
	if err := tfts.dirsWatcher.AddWatcher(utils.DirStar(dir), tfts.onFileChanged, []notify.Event{notify.Create, notify.Write, notify.Remove}); err != nil {
		return err
	}
	return nil
}

func (tfts *TreeStatsWatcher) RemoveDirWatch(dir string) error {
	tfts.dirsStatus.RemoveDir(utils.JustDir(dir))
	tfts.ftsDB.DeleteFileStats(utils.JustDir(dir)) // we explicitly clean the database from dirs that we stop watching
	return tfts.dirsWatcher.RemoveWatcher(utils.DirStar(dir))
}

// Watch all registered dirs with the notify watcher
func (tfts *TreeStatsWatcher) Watch() error {
	err := tfts.dirsWatcher.WatchAll()
	tfts.ftsDB.Close() // TODO: close DB after all watchers finished... probably opening/closing should be one level deeper?
	return err
}

// ScanSync does a full scan over all registered dirs synchronously and updates the database
// This can take a long time (minutes to hours) to complete
func (tfts *TreeStatsWatcher) ScanSync() error {
	errl := []string{}
	for _, d := range tfts.dirsStatus.Dirs() {
		if err := tfts.ScanDir(d); err != nil {
			errl = append(errl, fmt.Sprintf("error [%s]: %s", d, err.Error()))
		}
	}
	// tfts.ftsDB.DeleteOlderThan(tfts.lastScanStarted) // delete all entries from before the scan (i.e. not updated during the scan, because this means they were deleted)

	if len(errl) > 0 {
		return fmt.Errorf(strings.Join(errl, "\n"))
	}
	return nil
}

// scanDir scans the given scanRoot recursively and updates the database
// This can take a long time (minutes to hours) to complete
func (tfts *TreeStatsWatcher) ScanDir(scanRoot string) error {

	if tfts.dirsStatus.ScanRunning(scanRoot) {
		return fmt.Errorf("warning: skipping scan of %s because it is already running", scanRoot)
	}

	tfts.dirsStatus.ScanStart(scanRoot)

	err := godirwalk.Walk(scanRoot, &godirwalk.Options{
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
	})

	tfts.ftsDB.DeleteOlderThanWithPrefix(tfts.dirsStatus.ScanStarted(scanRoot), scanRoot)
	tfts.dirsStatus.ScanFinish(scanRoot)

	if err != nil {
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
