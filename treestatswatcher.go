package filetypestats

// TODO: allow to optionally only do direct scan, without db or inotify, to supersede legacy code

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"sync"

	"github.com/karrick/godirwalk"
	"github.com/ppenguin/filetype"
	"github.com/ppenguin/filetypestats/ftsdb"
	"github.com/ppenguin/filetypestats/notifywatch"
	"github.com/ppenguin/filetypestats/utils"
	ggu "github.com/ppenguin/gogenutils"
	"github.com/rjeczalik/notify"
	"golang.org/x/sys/unix"
)

var defaultNotifyEvents = []notify.Event{notify.InCreate, notify.InModify, notify.InMovedFrom, notify.InMovedTo, notify.Remove}

type tMoveInfo struct {
	From string
	To   string
}
type tMoveMap map[uint32]*tMoveInfo
type TreeStatsWatcher struct {
	TDirMonitors // embed this map, because a TreeStatsWatcher is just TDirMonitors with added state
	moves        tMoveMap
	ftsDB        *ftsdb.FileTypeStatsDB
	eventHandler notifywatch.NotifyHandlerFun
	wg           *sync.WaitGroup
}

// NewTreeStatsWatcher is the top level constructor featuring:
//  - a recursive watcher and scanner for all files in the given param dirs
//	- a sqlite DB session (param database: file name)
// An instance is always returned, even if an error occurred
// dirs will be trimmed of trailing suffixes and evaluated recursively
// If dirs is empty, you can add watches later with AddWatch() or AddDir()
func NewTreeStatsWatcher(dirs []string, database string) (*TreeStatsWatcher, error) {
	var fdb *ftsdb.FileTypeStatsDB
	var err error
	if database != "" {
		fdb, err = ftsdb.New(database, true)
	} else {
		fdb = nil
	}
	tsw := &TreeStatsWatcher{
		*NewDirMonitors(),
		make(tMoveMap),
		fdb,
		nil,
		&sync.WaitGroup{},
	}
	tsw.eventHandler = tsw.onFileChanged // set default event handler
	tsw.AddWatch(dirs...)
	return tsw, err // always return a valid watcher instance, we can add dirs and use other features later
}

// AddWatch adds a (default) watch for the given dirs
// Default means: recursive and for events notify.InCreate, notify.InModify, notify.InMovedFrom, notify.InMovedTo, notify.Remove
// For a customised watch, use AddDir()
func (tsw *TreeStatsWatcher) AddWatch(dirs ...string) error {
	errs := ggu.NewErrors()
	for _, d := range dirs {
		tsw.AddDir(d, true, tsw.onFileChanged, defaultNotifyEvents...) // TBC: do we need to make this configurable on a higher level?
		errs.AddIf(tsw.ScanDirAsync(d))
	}
	return errs.Err()
}

// WatchAll starts all registered dirs with the notify watcher (ignoring already started ones)
func (tsw *TreeStatsWatcher) WatchAll() error {
	errs := ggu.NewErrors()
	for _, d := range tsw.Dirs() {
		errs.AddIf(tsw.StartWatcher(d))
	}
	tsw.wg.Wait() // wait until last watcher finishes
	return errs.Err()
}

// StopAll stops all registered dirs with the notify watcher
func (tsw *TreeStatsWatcher) StopWatchAll() error {
	errs := ggu.NewErrors()
	for _, v := range tsw.TDirMonitors {
		errs.AddIf(v.Stop())
	}
	return errs.Err()
}

// ScanSync does a full scan over all registered dirs synchronously and updates the database
// This can take a long time (minutes to hours) to complete
func (tsw *TreeStatsWatcher) ScanAllSync() error {
	errs := ggu.NewErrors()
	for _, d := range tsw.Dirs() {
		if err := tsw.ScanDir(d); err != nil {
			errs.AddIf(fmt.Errorf("error [%s]: %s", d, err.Error()))
		}
	}
	// tsw.ftsDB.DeleteOlderThan(tsw.lastScanStarted) // delete all entries from before the scan (i.e. not updated during the scan, because this means they were deleted)
	return errs.Err()
}

// ScanDirAsync scans dir asynchronously
// TODO: add channel to make interuption possible?
func (tsw *TreeStatsWatcher) ScanDirAsync(dir string) error {
	if tsw.ScanRunning(dir) {
		return fmt.Errorf("warning: skipping scan of %s because it is already running", dir)
	}
	go func() {
		tsw.ScanDir(dir)
	}()
	return nil
}

// scanDir scans the given dir recursively and updates the database
// This can take a long time (minutes to hours) to complete
func (tsw *TreeStatsWatcher) ScanDir(dir string) error {

	if tsw.ScanRunning(dir) {
		return fmt.Errorf("warning: skipping scan of %s because it is already running", dir)
	}

	tsw.ScanStart(dir)

	err := godirwalk.Walk(dir, &godirwalk.Options{
		AllowNonDirectory: true,
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			var (
				err   error = nil
				fi    fs.FileInfo
				ftype string
			)

			if de.IsDir() {
				ftype = "dir"
				tsw.ftsDB.UpdateFileStats(osPathname+"/", ftype, 0) // add / to make filtering more consistent in SELECT queries
			} else if de.IsRegular() {
				fi, err = os.Stat(osPathname)
				if err == nil {
					if ftype, err = filetype.FileClass(osPathname); err == nil {
						tsw.ftsDB.UpdateFileStats(osPathname, ftype, uint64(fi.Size()))
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

	tsw.ftsDB.DeleteOlderThanWithPrefix(tsw.ScanStarted(dir), dir)
	tsw.ScanFinish(dir)

	if err != nil {
		return err
	}
	return nil
}

// onFileChanged is the inotify event handler passed to the notify watcher
// for now we handle create, remove, write (this is like modify but guaranteed on all platforms)
func (tsw *TreeStatsWatcher) onFileChanged(eventInfo *notify.EventInfo) error {
	cookie := (*eventInfo).Sys().(*unix.InotifyEvent).Cookie // this is a kind of hash to relate the From event to the To event
	minfo, ok := tsw.moves[cookie]
	if !ok {
		minfo = &tMoveInfo{}
		tsw.moves[cookie] = minfo
	}
	switch (*eventInfo).Event() {
	case notify.InCreate, notify.InModify:
		if minfo.From == "" && minfo.To == "" { // only execute create if not already moving
			if fts, err := getFTStat((*eventInfo).Path()); err == nil {
				return tsw.ftsDB.UpdateFileStats(fts.Path, fts.FType, fts.NumBytes)
			}
		} // any stat errors are simply ignored
	case notify.InMovedFrom:
		minfo.From = (*eventInfo).Path()
	case notify.InMovedTo:
		minfo.To = (*eventInfo).Path()
	case notify.Remove: // TODO: it is a real problem that we don't know whether it is a dir or a file?
		if minfo.From == "" && minfo.To == "" { // only execute remove if not already moving
			return tsw.ftsDB.DeleteFileStats((*eventInfo).Path())
		}
	}

	if cookie != 0 && minfo.From != "" && minfo.To != "" {
		// verrry important to make sure that a dir gets a trailing /, otherwise a file with a similar naome (or all dirs starting with the same name) would also be renamed in the DB!
		// since we have no way to find out from the event whether the target is a dir, we have to stat it
		fi, err := os.Lstat(minfo.To)
		if err != nil {
			return fmt.Errorf("couldn't get file info for moved target %s in event %v, not handling move", minfo.To, eventInfo)
		}
		if fi.IsDir() {
			minfo.From = utils.DirTrailSep(minfo.From)
			minfo.To = utils.DirTrailSep(minfo.To)
		}
		log.Printf("updating DB for file move %s -> %s", minfo.From, minfo.To) // FIXME: uncontrolled logging
		err = tsw.ftsDB.UpdateFilePath(minfo.From, minfo.To)
		delete(tsw.moves, cookie)
		return err
	}
	if minfo.From != "" || minfo.To != "" { // we're in the middle of a move op, continue and await the second event
		return nil
	}
	return fmt.Errorf("unhandled event %v for %s", eventInfo, (*eventInfo).Path())
}

// StartWatcher starts the dir watcher in the background (or returns an error if not available)
func (tsw *TreeStatsWatcher) StartWatcher(dir string) error {
	w, ok := tsw.TDirMonitors[dir]
	if !ok {
		return fmt.Errorf("refusing to start non-existing watcher for %s", dir)
	}
	if w.IsWatching() { // avoid starting a watcher that is already watching
		return fmt.Errorf("refusing to start already running watcher for %s", dir)
	}
	tsw.wg.Add(1)
	go func() { // we can do without passing wg because it's a pointer we don't change?
		_ = w.Watch() // TODO: error handling?
		tsw.wg.Done()
		delete(tsw.TDirMonitors, dir)
	}()
	return nil
}

// StopWatcher stops and removes the watcher for dir
// (The DirMonitor is removed entirely, because we have no way to re-start a stopped watcher, so its existence becomes meaningless after stopping)
func (tsw *TreeStatsWatcher) StopWatcher(dir string) error {
	w, ok := tsw.TDirMonitors[dir]
	if !ok {
		return fmt.Errorf("refusing to stop non-existing watcher for %s", dir)
	}
	if !w.IsWatching() { // avoid starting a watcher that is already watching
		return fmt.Errorf("refusing to stop already stopped watcher for %s", dir)
	}
	return tsw.RemoveDir(dir)
}
