package notifywatch

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
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
	if len(rootdirs) < 1 { // if no dirs, return an empty NWD instance to which we can add event handlers later
		return nwd
	}
	rdirs := gogenutils.FilterCommonRootDirs(rootdirs) // sanitise to only include disjoint roots
	for _, d := range rdirs {
		addWatcher(nwd, d, handler, events)
	}
	return nwd
}

func addWatcher(nwd *NotifyWatchDirs, dir string, handler NotifyHandlerFun, events []notify.Event) error {
	return nwd.AddWatcher(dir, handler, events)
}
func (nwd *NotifyWatchDirs) AddWatcher(dir string, handler NotifyHandlerFun, events []notify.Event) error {
	nwd.watchers[dir] = NewNotifyWatcher(dir, handler, events...)
	return nil
}

// WatchAll (blocking) starts watching (all watchers) and exits after the last watcher is terminated
func (nwd *NotifyWatchDirs) WatchAll() error {
	errl := []string{}
	for _, w := range nwd.watchers {
		nwd.wg.Add(1)
		go func(wg *sync.WaitGroup, watcher *NotifyWatcher) {
			err := watcher.Watch()
			errl = append(errl, err.Error())
			wg.Done()
		}(nwd.wg, w)
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
		if err := w.Stop(); err != nil {
			errl = append(errl, err.Error())
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
	eventInfo chan notify.EventInfo
	events    []notify.Event
	handler   NotifyHandlerFun
}

// NewNotifyWatcher watches the given dir and calls handler on inotify events
// a dir ending in "/*" will result in a recursive watch
func NewNotifyWatcher(dir string, handler NotifyHandlerFun, events ...notify.Event) *NotifyWatcher {
	wdir := dir
	if strings.HasSuffix(dir, "/*") { // TODO: this will not work on windows
		wdir = filepath.Join(strings.TrimSuffix(dir, "/*"), "...")
	}
	nw := &NotifyWatcher{
		eventInfo: make(chan notify.EventInfo, 1), // buffered to ensure no events are dropped
		events:    events,
		watchdir:  wdir,
		handler:   handler,
	}
	return nw
}

func (nw *NotifyWatcher) Watch() error {
	if err := notify.Watch(nw.watchdir, nw.eventInfo, nw.events...); err != nil {
		// log.Printf("error: %s", err.Error())
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
	return nil
}

func (nw *NotifyWatcher) Stop() error {
	if _, ok := <-nw.eventInfo; !ok {
		return fmt.Errorf("channel nw.EventInfo already closed")
	}
	close(nw.eventInfo) // can we do this? we probably need to be careful in the watch loop?
	return nil
}
