package notifywatch

import (
	"fmt"
	"log"
	"strings"
	"sync"

	utils "github.com/ppenguin/filetypestats/utils"
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
	if len(rootdirs) < 1 { // if no dirs, return an empty NWD instance to which we can add event handlers later
		return nwd
	}
	rdirs := gogenutils.FilterCommonRootDirs(rootdirs) // sanitise to only include disjoint roots
	for _, d := range rdirs {
		nwd.AddWatcher(d, handler, events)
	}
	return nwd
}

func (nwd *NotifyWatchDirs) startWatcher(dir string) {
	w := nwd.getWatcher(dir)
	if !w.watching { // avoid starting a watcher that is already watching
		nwd.wg.Add(1)
		go func(wg *sync.WaitGroup, watcher *NotifyWatcher) {
			_ = watcher.Watch() // TODO: error handling?
			wg.Done()
		}(nwd.wg, w)
	}
}

func (nwd *NotifyWatchDirs) AddWatcher(dir string, handler NotifyHandlerFun, events []notify.Event) error {
	nwd.watchers[dir] = NewNotifyWatcher(dir, handler, events...)
	nwd.startWatcher(dir)
	return nil // TODO: error handling (?) async => how?
}

func (nwd *NotifyWatchDirs) RemoveWatcher(dir string) error {
	w, ok := nwd.watchers[dir]
	if !ok {
		return nil // ignore non-existing
	}
	err := w.Stop()
	nwd.watchers[dir] = nil
	return err
}

func (nwd *NotifyWatchDirs) getWatcher(dir string) *NotifyWatcher {
	v, ok := nwd.watchers[dir]
	if !ok {
		return &NotifyWatcher{watchdir: "", eventInfo: make(chan notify.EventInfo, 1)} // what happens down the line if we try to use an empty watcher?
	}
	return v
}

// WatchAll (blocking) starts watching (all watchers) and exits after the last watcher is terminated
func (nwd *NotifyWatchDirs) WatchAll() error {
	errl := []string{}
	for _, w := range nwd.watchers {
		nwd.startWatcher(w.watchdir) // automatically ignores already running watchers
	}
	nwd.wg.Wait()
	if len(errl) > 0 {
		return fmt.Errorf(strings.Join(errl, "\n"))
	}
	return nil
}

func (nwd *NotifyWatchDirs) StopAll() error {
	errl := []string{}
	for _, w := range nwd.watchers {
		if w.watching {
			if err := w.Stop(); err != nil {
				errl = append(errl, err.Error())
			}
		}
	}
	if len(errl) > 0 {
		return fmt.Errorf(strings.Join(errl, "\n"))
	}
	return nil
}

/*** inotify watcher with handler for one (recursive) file tree ***/

type NotifyWatcher struct {
	watchdir  string
	watching  bool
	eventInfo chan notify.EventInfo
	events    []notify.Event
	handler   NotifyHandlerFun
}

// NewNotifyWatcher watches the given dir and calls handler on inotify events
// a dir ending in "/*" will result in a recursive watch
func NewNotifyWatcher(dir string, handler NotifyHandlerFun, events ...notify.Event) *NotifyWatcher {
	wdir := dir
	if utils.IsDirRecursive(dir) {
		wdir = utils.Dir3Dot(dir)
	}
	nw := &NotifyWatcher{
		eventInfo: make(chan notify.EventInfo, 1), // buffered to ensure no events are dropped
		events:    events,
		watchdir:  wdir,
		watching:  false,
		handler:   handler,
	}
	return nw
}

func (nw *NotifyWatcher) Watch() error {
	if nw.watchdir == "" {
		return fmt.Errorf("ERROR: refusing to start empty watcher")
	}
	nw.watching = true
	if err := notify.Watch(nw.watchdir, nw.eventInfo, nw.events...); err != nil {
		// log.Printf("error: %s", err.Error())
		nw.watching = false
		return err
	}
	defer notify.Stop(nw.eventInfo)

	for {
		ei, ok := <-nw.eventInfo // this should exit the loop when we close the channel by executing nw.Stop()
		if ok {
			log.Printf("got event: %v; executing handler...", ei) // FIXME: uncontrolled logging
			if err := nw.handler(&ei); err != nil {
				log.Fatalf("failed executing handler for event: %v; %s", ei, err.Error())
			}
		} else {
			break
		}
	}
	nw.watching = false
	return nil
}

func (nw *NotifyWatcher) Stop() error {
	if _, ok := <-nw.eventInfo; !ok {
		return fmt.Errorf("channel nw.EventInfo already closed")
	}
	close(nw.eventInfo) // can we do this? we probably need to be careful in the watch loop?
	nw.watching = false
	return nil
}
