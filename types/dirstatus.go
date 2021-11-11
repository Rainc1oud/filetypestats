package types

import (
	"time"

	"github.com/ppenguin/gogenutils"
)

type TDirStatus struct {
	tstarted  time.Time
	tfinished time.Time
	dirty     bool
}

func newDirStatus() *TDirStatus {
	return &TDirStatus{
		tstarted:  time.Time{},
		tfinished: time.Time{},
		dirty:     false,
	}
}

func (t *TDirStatus) scanRunning() bool {
	return t.tstarted.After(t.tfinished)
}
func (t *TDirStatus) scanStart() {
	t.tstarted = time.Now()
}
func (t *TDirStatus) scanFinish() {
	t.tfinished = time.Now()
}
func (t *TDirStatus) scanStarted() time.Time {
	return t.tstarted
}
func (t *TDirStatus) scanFinished() time.Time {
	return t.tfinished
}
func (t *TDirStatus) isDirty() bool {
	return t.dirty
}

type tDSMap map[string]*TDirStatus

// TODO: this is a generic function for any map[string]interface{}, handle after generics support is here (go1.18)
func (tm *tDSMap) keys() []string {
	s := make([]string, len(*tm))
	i := 0
	for k := range *tm {
		s[i] = k
		i++
	}
	return s
}

// TODO: this is a generic function for any map[string]interface{}, handle after generics support is here (go1.18)
func (tm *tDSMap) hasElem(elem string) bool {
	return gogenutils.InSlice(elem, *tm)
}

type TDirsStatus struct {
	tDSMap
}

// NewDirsStatus constructor
func NewDirsStatus(dirs ...string) *TDirsStatus {
	tds := &TDirsStatus{tDSMap: make(tDSMap)}
	for _, d := range dirs {
		tds.AddDir(d)
	}
	return tds
}

func (ts *TDirsStatus) getItem(dir string) *TDirStatus {
	if v, ok := ts.tDSMap[dir]; ok {
		return v
	}
	return newDirStatus()
}

// AddDir adds dir to status
func (ts *TDirsStatus) AddDir(dir string) *TDirStatus {
	if v, ok := ts.tDSMap[dir]; ok {
		return v // ignore if exists
	}
	ts.tDSMap[dir] = newDirStatus()
	return ts.tDSMap[dir]
}

// RemoveDir removes dir from status
func (ts *TDirsStatus) RemoveDir(dir string) {
	delete(ts.tDSMap, dir) // no need to check existence, delete non-existing is no-op
}

// Dirs returns a slice of all registered dirs
func (ts *TDirsStatus) Dirs() []string {
	return ts.keys()
}

// Contains returns whether dir is contained in the registered dirs
func (ts *TDirsStatus) Contains(dir string) bool {
	return ts.hasElem(dir)
}

// ScanRunning reports whether a ssscan on dir is currently running
func (ts *TDirsStatus) ScanRunning(dir string) bool {
	return ts.getItem(dir).scanRunning()
}

// ScanFinish updates start time for dir
func (ts *TDirsStatus) ScanStart(dir string) {
	if v, ok := ts.tDSMap[dir]; ok {
		v.scanStart()
		v.dirty = true
	}
	// completely ignored if dir is not registered (= a pain for debugging?)
}

// ScanFinish updates finished time for dir
func (ts *TDirsStatus) ScanFinish(dir string) {
	if v, ok := ts.tDSMap[dir]; ok {
		v.scanFinish()
		v.dirty = false
	}
	// completely ignored if dir is not registered (= a pain for debugging?)
}

// ScanStarted returns the time the last scan was started
func (ts *TDirsStatus) ScanStarted(dir string) time.Time {
	return ts.getItem(dir).scanStarted()
}

// ScanFinished returns the time the last scan was started
func (ts *TDirsStatus) ScanFinished(dir string) time.Time {
	return ts.getItem(dir).scanFinished()
}

// IsDirty reports dirty status, i.e. if the DB for dir is up to date or being updated
func (ts *TDirsStatus) IsDirty(dir string) bool {
	return ts.getItem(dir).isDirty()
}
