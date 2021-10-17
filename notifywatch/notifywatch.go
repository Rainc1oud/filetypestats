package notifywatch

import (
	"log"
	"path/filepath"
	"sync"

	"github.com/ppenguin/gogenutils"
	"github.com/rjeczalik/notify"
)

type notifyWatchers map[string]*NotifyWatcher

type NotifyHandlerFun func(*notify.EventInfo) error

type NotifyWatchDirs struct {
	watchers notifyWatchers
	wg       *sync.WaitGroup
}

// NewNotifyWatchers instantiates the container dict for NotifyWatchers, which unifies listening for events over all (disjoint) dir trees in the collection
func NewNotifyWatchDirs(rootdirs []string, handler NotifyHandlerFun, events ...notify.Event) *NotifyWatchDirs {
	nwd := &NotifyWatchDirs{
		watchers: make(notifyWatchers),
		wg:       &sync.WaitGroup{},
	}
	if len(rootdirs) < 1 {
		return nwd
	}
	rdirs := gogenutils.FilterCommonRootDirs(rootdirs) // sanitise to only include disjoint roots
	for _, d := range rdirs {
		addWatcher(nwd, d, true, handler, events)
	}
	return nwd
}

func addWatcher(nwd *NotifyWatchDirs, dir string, recursive bool, handler NotifyHandlerFun, events []notify.Event) error {
	return nwd.AddWatcher(dir, recursive, handler, events)
}
func (nwd *NotifyWatchDirs) AddWatcher(dir string, recursive bool, handler NotifyHandlerFun, events []notify.Event) error {
	nwd.watchers[dir] = NewNotifyWatcher(dir, recursive, handler, events...)
	return nil
}

// Watch starts watching (all watchers)
func (nwd *NotifyWatchDirs) WatchAll() {
	for _, w := range nwd.watchers {
		nwd.wg.Add(1)
		go func(wg *sync.WaitGroup, watcher *NotifyWatcher) {
			watcher.Watch()
			wg.Done()
		}(nwd.wg, w)
	}
	nwd.wg.Wait()
}

func (nwd *NotifyWatchDirs) StopAll() {
	for _, w := range nwd.watchers {
		w.Stop()
	}
}

/*** inotify watcher with handler for one (recursive) file tree ***/

type NotifyWatcher struct {
	watchdir  string
	eventInfo chan notify.EventInfo
	events    []notify.Event
	handler   NotifyHandlerFun
}

func NewNotifyWatcher(dir string, recursive bool, handler NotifyHandlerFun, events ...notify.Event) *NotifyWatcher {
	wdir := dir
	if recursive {
		wdir = filepath.Join(dir, "...")
	}
	nw := &NotifyWatcher{
		eventInfo: make(chan notify.EventInfo, 1), // buffered to ensure no events are dropped
		events:    events,
		watchdir:  wdir,
		handler:   handler,
	}
	return nw
}

func (nw *NotifyWatcher) Watch() {
	if err := notify.Watch(nw.watchdir, nw.eventInfo, nw.events...); err != nil {
		log.Printf("error: %s", err.Error())
	}
	defer notify.Stop(nw.eventInfo)

	for {
		ei, ok := <-nw.eventInfo // this should exit the loop when we close the channel by executing nw.Stop()
		if ok {
			log.Printf("got event: %v; executing handler...", ei)
			if err := nw.handler(&ei); err != nil {
				log.Fatalf("failed executing handler for event: %v; %s", ei, err.Error())
			}
		} else {
			break
		}
	}
}

func (nw *NotifyWatcher) Stop() {
	close(nw.eventInfo) // can we do this? we probably need to be careful in the watch loop?
}
