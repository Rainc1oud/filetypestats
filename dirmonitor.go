package filetypestats

import (
	"fmt"
	"time"

	"github.com/ppenguin/filetypestats/notifywatch"
	ggu "github.com/ppenguin/gogenutils"
	"github.com/rjeczalik/notify"
)

type TDirMonitor struct {
	notifywatch.NotifyWatcher // embed NotifyWatcher, because TDirMonitor is just a Watcer with added state and access/info methods
	tstarted                  time.Time
	tfinished                 time.Time
	dirty                     bool
}

func newDirMonitor(dir string, recursive bool, handler notifywatch.NotifyHandlerFun, events ...notify.Event) *TDirMonitor {
	dm := &TDirMonitor{
		*notifywatch.NewNotifyWatcher(dir, recursive, handler, events...),
		time.Time{},
		time.Time{},
		false,
	}
	return dm
}

func (t *TDirMonitor) scanRunning() bool {
	return t.tstarted.After(t.tfinished)
}
func (t *TDirMonitor) scanStart() {
	t.tstarted = time.Now()
}
func (t *TDirMonitor) scanFinish() {
	t.tfinished = time.Now()
}
func (t *TDirMonitor) scanStarted() time.Time {
	return t.tstarted
}
func (t *TDirMonitor) scanFinished() time.Time {
	return t.tfinished
}
func (t *TDirMonitor) isDirty() bool {
	return t.dirty
}

type TDirMonitors map[string]*TDirMonitor

// TODO: this is a generic function for any map[string]interface{}, handle after generics support is here (go1.18)
func (dm *TDirMonitors) keys() []string {
	s := make([]string, len(*dm))
	i := 0
	for k := range *dm {
		s[i] = k
		i++
	}
	return s
}

// NewDirMonitors constructor
func NewDirMonitors() *TDirMonitors {
	tds := make(TDirMonitors)
	return &tds
}

func (dm *TDirMonitors) getItem(dir string) *TDirMonitor {
	if v, ok := (*dm)[dir]; ok {
		return v
	}
	return &TDirMonitor{
		notifywatch.NotifyWatcher{},
		time.Time{},
		time.Time{},
		false,
	}
}

// overlappedDirs returns all dirs that should be removed from the set {dir, Dirs()} because they are overlapped by a parent from the set (i.e. the returned list contains all entries that are under other entries in dir hierarchy)
func (dm *TDirMonitors) overlappedDirs(dir string) []string {
	alldirs := append(dm.Dirs(), dir)
	filtdirs := ggu.FilterCommonRootDirs(alldirs)
	rmdirs := []string{}
	for _, d := range alldirs {
		if !ggu.InSlice(d, filtdirs) {
			rmdirs = append(rmdirs, d)
		}
	}
	return rmdirs
}

// AddDir adds dir to the DirMonitors collection with a new DirMonitor instance, while removing all overlapping dirs
func (dm *TDirMonitors) AddDir(dir string, recursive bool, handler notifywatch.NotifyHandlerFun, events ...notify.Event) *TDirMonitor {
	unwanted := dm.overlappedDirs(dir)
	if ggu.InSlice(dir, unwanted) {
		unwanted = ggu.RemoveFromStringSlice(dir, unwanted)
	}
	if len(unwanted) > 0 {
		dm.RemoveDirs(unwanted...)
		return nil
	}
	if v, ok := (*dm)[dir]; ok {
		return v // ignore if exists
	}
	(*dm)[dir] = newDirMonitor(dir, recursive, handler, events...)
	return (*dm)[dir]
}

// RemoveDirs removes dirs from the container
func (dm *TDirMonitors) RemoveDirs(dirs ...string) error {
	errs := ggu.NewErrors()
	for _, d := range dirs {
		errs.AddIf(dm.RemoveDir(d))
	}
	return errs.Err()
}

// RemoveDir removes dir from the container
func (dm *TDirMonitors) RemoveDir(dir string) error {
	if _, ok := (*dm)[dir]; !ok {
		return fmt.Errorf("monitor for %s doesn't exist, watcher not removed", dir)
	}
	err := (*dm)[dir].Stop() // TBC: do we need to handle the error?
	delete(*dm, dir)         // no need to check existence, delete non-existing is no-op
	return err
}

// Dirs returns a slice of all registered dirs
func (dm *TDirMonitors) Dirs() []string {
	return dm.keys()
}

func (dm *TDirMonitors) hasElem(key string) bool {
	_, ok := (*dm)[key]
	return ok
}

// Contains returns whether dir is contained in the registered dirs
func (dm *TDirMonitors) Contains(dir string) bool {
	return dm.hasElem(dir)
}

// ScanRunning reports whether a ssscan on dir is currently running
func (dm *TDirMonitors) ScanRunning(dir string) bool {
	return dm.getItem(dir).scanRunning()
}

// ScanFinish updates start time for dir
func (dm *TDirMonitors) ScanStart(dir string) {
	if v, ok := (*dm)[dir]; ok {
		v.scanStart()
		v.dirty = true
	}
	// completely ignored if dir is not registered (= a pain for debugging?)
}

// ScanFinish updates finished time for dir
func (dm *TDirMonitors) ScanFinish(dir string) {
	if v, ok := (*dm)[dir]; ok {
		v.scanFinish()
		v.dirty = false
	}
	// completely ignored if dir is not registered (= a pain for debugging?)
}

// ScanStarted returns the time the last scan was started
func (dm *TDirMonitors) ScanStarted(dir string) time.Time {
	return dm.getItem(dir).scanStarted()
}

// ScanFinished returns the time the last scan was started
func (dm *TDirMonitors) ScanFinished(dir string) time.Time {
	return dm.getItem(dir).scanFinished()
}

// IsDirty reports dirty status, i.e. if the DB for dir is up to date or being updated
func (dm *TDirMonitors) IsDirty(dir string) bool {
	return dm.getItem(dir).isDirty()
}
