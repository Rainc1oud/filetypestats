package filetypestats

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Rainc1oud/filetypestats/notifywatch"
	ggu "github.com/Rainc1oud/gogenutils"
	"github.com/rjeczalik/notify"
)

type DirMonitor struct {
	*notifywatch.NotifyWatcher

	mu       sync.RWMutex
	started  time.Time
	finished time.Time
	duration time.Duration
	dirty    bool
}

func newDirMonitor(dir string, recursive bool, handler notifywatch.EventHandler, events ...notify.Event) *DirMonitor {
	return &DirMonitor{NotifyWatcher: notifywatch.NewNotifyWatcher(dir, recursive, handler, events...)}
}

func (m *DirMonitor) scanRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started.After(m.finished)
}

func (m *DirMonitor) tryScanStart() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started.After(m.finished) {
		return false
	}
	m.started = time.Now()
	m.dirty = true
	return true
}

func (m *DirMonitor) scanFinish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finished = time.Now()
	m.duration = m.finished.Sub(m.started)
	m.dirty = false
}

func (m *DirMonitor) status() (time.Time, time.Time, time.Duration, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started, m.finished, m.duration, m.dirty
}

type DirMonitors struct {
	mu       sync.RWMutex
	monitors map[string]*DirMonitor
}

type DirMonitorsStatus struct {
	Dirty            bool
	ScanStartedLast  time.Time
	ScanFinishedLast time.Time
	ScanLongestLast  time.Duration
}

func NewDirMonitors() *DirMonitors {
	return &DirMonitors{monitors: make(map[string]*DirMonitor)}
}

func (m *DirMonitors) snapshot() map[string]*DirMonitor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*DirMonitor, len(m.monitors))
	for dir, monitor := range m.monitors {
		result[dir] = monitor
	}
	return result
}

func (m *DirMonitors) Monitors() []*DirMonitor {
	snapshot := m.snapshot()
	result := make([]*DirMonitor, 0, len(snapshot))
	for _, monitor := range snapshot {
		result = append(result, monitor)
	}
	return result
}

func (m *DirMonitors) Status() *DirMonitorsStatus {
	result := &DirMonitorsStatus{}
	for _, monitor := range m.Monitors() {
		started, finished, duration, dirty := monitor.status()
		if result.ScanStartedLast.Before(started) {
			result.ScanStartedLast = started
		}
		if result.ScanFinishedLast.Before(finished) {
			result.ScanFinishedLast = finished
		}
		if result.ScanLongestLast < duration {
			result.ScanLongestLast = duration
		}
		result.Dirty = result.Dirty || dirty
	}
	return result
}

func (m *DirMonitors) dirsLocked() []string {
	dirs := make([]string, 0, len(m.monitors))
	for dir := range m.monitors {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	return dirs
}

func (m *DirMonitors) overlappedDirsLocked(dir string) []string {
	allDirs := append(m.dirsLocked(), dir)
	filtered := ggu.FilterCommonRootDirs(allDirs)
	var result []string
	for _, candidate := range allDirs {
		if !ggu.InSlice(candidate, filtered) {
			result = append(result, candidate)
		}
	}
	return result
}

func (m *DirMonitors) AddDir(dir string, recursive bool, handler notifywatch.EventHandler, events ...notify.Event) *DirMonitor {
	m.mu.Lock()
	defer m.mu.Unlock()
	unwanted := m.overlappedDirsLocked(dir)
	if ggu.InSlice(dir, unwanted) {
		unwanted = ggu.RemoveFromStringSlice(dir, unwanted)
	}
	for _, unwantedDir := range unwanted {
		if monitor := m.monitors[unwantedDir]; monitor != nil && monitor.IsWatching() {
			return nil
		}
		delete(m.monitors, unwantedDir)
	}
	if existing := m.monitors[dir]; existing != nil {
		return existing
	}
	monitor := newDirMonitor(dir, recursive, handler, events...)
	m.monitors[dir] = monitor
	return monitor
}

func (m *DirMonitors) RemoveDirs(dirs ...string) error {
	var errs []error
	for _, dir := range dirs {
		errs = append(errs, m.RemoveDir(dir))
	}
	return errors.Join(errs...)
}

func (m *DirMonitors) RemoveDir(dir string) error {
	monitor := m.get(dir)
	if monitor == nil {
		return fmt.Errorf("monitor for %s doesn't exist, watcher not removed", dir)
	}
	if err := monitor.Stop(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.monitors[dir] == monitor {
		delete(m.monitors, dir)
	}
	m.mu.Unlock()
	return nil
}

func (m *DirMonitors) Dirs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dirsLocked()
}

func (m *DirMonitors) get(dir string) *DirMonitor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.monitors[dir]
}

func (m *DirMonitors) Contains(dir string) bool { return m.get(dir) != nil }

func (m *DirMonitors) ScanRunning(dir string) bool {
	monitor := m.get(dir)
	return monitor != nil && monitor.scanRunning()
}

func (m *DirMonitors) tryScanStart(dir string) bool {
	monitor := m.get(dir)
	return monitor != nil && monitor.tryScanStart()
}

func (m *DirMonitors) ScanFinish(dir string) {
	if monitor := m.get(dir); monitor != nil {
		monitor.scanFinish()
	}
}

func (m *DirMonitors) ScanStarted(dir string) time.Time {
	if monitor := m.get(dir); monitor != nil {
		started, _, _, _ := monitor.status()
		return started
	}
	return time.Time{}
}

func (m *DirMonitors) ScanFinished(dir string) time.Time {
	if monitor := m.get(dir); monitor != nil {
		_, finished, _, _ := monitor.status()
		return finished
	}
	return time.Time{}
}

func (m *DirMonitors) IsDirty(dir string) bool {
	if monitor := m.get(dir); monitor != nil {
		_, _, _, dirty := monitor.status()
		return dirty
	}
	return false
}

// Deprecated compatibility aliases. New code should use the idiomatic names.
type TDirMonitor = DirMonitor
type TDirMonitors = DirMonitors
type TDirMonitorsStatus = DirMonitorsStatus
