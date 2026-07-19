package filetypestats

// TODO: allow to optionally only do direct scan, without db or inotify, to supersede legacy code

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/Rainc1oud/filetype"
	"github.com/Rainc1oud/filetypestats/ftsdb"
	"github.com/Rainc1oud/filetypestats/notifywatch"
	"github.com/Rainc1oud/filetypestats/types"
	"github.com/Rainc1oud/filetypestats/utils"
	ggu "github.com/Rainc1oud/gogenutils"
	"github.com/karrick/godirwalk"
	"github.com/rjeczalik/notify"
	"golang.org/x/sys/unix"
)

var defaultNotifyEvents = []notify.Event{notify.InCreate, notify.InModify, notify.InMovedFrom, notify.InMovedTo, notify.Remove}

const pathInfoBatchSize = 200

type tMoveInfo struct {
	From string
	To   string
}
type tMoveMap map[uint32]*tMoveInfo
type TreeStatsWatcher struct {
	*DirMonitors
	mu               sync.RWMutex
	lastScanDuration time.Duration
	moves            tMoveMap
	ftsDB            *ftsdb.FileTypeStatsDB
	eventHandler     notifywatch.EventHandler
}

// NewTreeStatsWatcher is the top level constructor featuring:
//   - a recursive watcher and scanner for all files in the given param dirs
//   - a sqlite DB session (param database: file name)
//
// An instance is always returned, even if an error occurred
// dirs will be trimmed of trailing suffixes and evaluated recursively
// If dirs is empty, you can add watches later with AddWatch() or AddDir()
func NewTreeStatsWatcher(dirs []string, dbconn *ftsdb.FileTypeStatsDB) (*TreeStatsWatcher, error) {
	tsw := &TreeStatsWatcher{
		DirMonitors: NewDirMonitors(),
		moves:       make(tMoveMap),
		ftsDB:       dbconn,
	}
	tsw.eventHandler = tsw.onFileChanged // set default event handler
	err := tsw.AddWatch(dirs...)
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

// WatchAll runs all currently registered watchers until ctx is cancelled.
func (tsw *TreeStatsWatcher) WatchAll(ctx context.Context) error {
	if ctx == nil {
		return errors.New("watch all: nil context")
	}
	monitors := tsw.Monitors()
	if len(monitors) == 0 {
		<-ctx.Done()
		return nil
	}
	errCh := make(chan error, len(monitors))
	var wg sync.WaitGroup
	for _, monitor := range monitors {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := monitor.Watch(ctx); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// StopAll stops all registered dirs with the notify watcher
func (tsw *TreeStatsWatcher) StopWatchAll() error {
	var errs []error
	for _, monitor := range tsw.Monitors() {
		errs = append(errs, monitor.Stop())
	}
	return errors.Join(errs...)
}

// ScanSync does a full scan over all registered dirs synchronously and updates the database
// This can take a long time (minutes to hours) to complete
func (tsw *TreeStatsWatcher) ScanAllSync() error {
	return tsw.ScanAll(context.Background())
}

func (tsw *TreeStatsWatcher) ScanAll(ctx context.Context) error {
	errs := ggu.NewErrors()
	tb := time.Now()
	for _, d := range tsw.Dirs() {
		if err := tsw.ScanDirContext(ctx, d); err != nil {
			errs.AddIf(fmt.Errorf("error [%s]: %s", d, err.Error()))
		}
	}
	tsw.mu.Lock()
	tsw.lastScanDuration = time.Since(tb)
	tsw.mu.Unlock()
	// tsw.ftsDB.DeleteOlderThan(tsw.lastScanStarted) // delete all entries from before the scan (i.e. not updated during the scan, because this means they were deleted)
	return errs.Err()
}

// ScanDirAsync scans dir asynchronously
// TODO: add channel to make interuption possible?
func (tsw *TreeStatsWatcher) ScanDirAsync(dir string) error {
	go func() {
		_ = tsw.ScanDirContext(context.Background(), dir)
	}()
	return nil
}

// scanDir scans the given dir recursively and updates the database
// This can take a long time (minutes to hours) to complete
func (tsw *TreeStatsWatcher) ScanDir(dir string) error {
	return tsw.ScanDirContext(context.Background(), dir)
}

func (tsw *TreeStatsWatcher) ScanDirContext(ctx context.Context, dir string) error {
	if ctx == nil {
		return errors.New("scan dir: nil context")
	}

	if !tsw.tryScanStart(dir) {
		return fmt.Errorf("warning: skipping scan of %s because it is already running", dir)
	}
	defer tsw.ScanFinish(dir)
	batchBuffer := types.NewFTypeStatsBatch(pathInfoBatchSize)

	err := godirwalk.Walk(dir, &godirwalk.Options{
		AllowNonDirectory: true,
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			var (
				err   error = nil
				fi    fs.FileInfo
				ftype string
			)

			if de.IsDir() {
				ftype = "dir"
				tsw.ftsDB.UpdateFileStatsMulti(osPathname+"/", ftype, 0, batchBuffer) // add / to make filtering more consistent in SELECT queries
			} else if de.IsRegular() {
				fi, err = os.Stat(osPathname)
				if err == nil {
					if ftype, err = filetype.FileClass(osPathname); err == nil {
						tsw.ftsDB.UpdateFileStatsMulti(osPathname, ftype, uint64(fi.Size()), batchBuffer)
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
			// fmt.Fprintf(os.Stderr, "warning: %s reading %s\n", e.Error(), s)
			return godirwalk.SkipNode
		},
	})

	if err := tsw.ftsDB.CommitBatch(batchBuffer); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	tsw.ftsDB.DeleteOlderThanWithPrefix(tsw.ScanStarted(dir), dir)
	return nil
}

// onFileChanged is the inotify event handler passed to the notify watcher
// for now we handle create, remove, write (this is like modify but guaranteed on all platforms)
func (tsw *TreeStatsWatcher) onFileChanged(eventInfo *notify.EventInfo) error {
	tsw.mu.Lock()
	defer tsw.mu.Unlock()
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
		// log.Printf("updating DB for file move %s -> %s", minfo.From, minfo.To) // FIXME: uncontrolled logging
		err = tsw.ftsDB.UpdateFilePath(minfo.From, minfo.To)
		delete(tsw.moves, cookie)
		return err
	}
	if minfo.From != "" || minfo.To != "" { // we're in the middle of a move op, continue and await the second event
		return nil
	}
	return fmt.Errorf("unhandled event %v for %s", eventInfo, (*eventInfo).Path())
}

// StopWatcher stops and removes the watcher for dir. It is idempotent for a
// registered watcher because NotifyWatcher.Stop is idempotent.
func (tsw *TreeStatsWatcher) StopWatcher(dir string) error {
	if !tsw.Contains(dir) {
		return fmt.Errorf("refusing to stop non-existing watcher for %s", dir)
	}
	return tsw.RemoveDir(dir)
}

func (tsw *TreeStatsWatcher) ScanDurationLast() time.Duration {
	tsw.mu.RLock()
	duration := tsw.lastScanDuration
	tsw.mu.RUnlock()
	for _, monitor := range tsw.Monitors() {
		_, _, scanDuration, _ := monitor.status()
		if duration < scanDuration {
			duration = scanDuration
		}
	}
	return duration
}
