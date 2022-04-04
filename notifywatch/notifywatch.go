package notifywatch

import (
	"fmt"

	"github.com/ppenguin/filetypestats/utils"
	"github.com/rjeczalik/notify"
)

type NotifyHandlerFun func(*notify.EventInfo) error

/*** inotify watcher with handler for one (recursive) file tree ***/

type NotifyWatcher struct {
	watchdir  string
	recursive bool
	watching  bool
	eventInfo chan notify.EventInfo
	events    []notify.Event
	handler   NotifyHandlerFun
}

// NewNotifyWatcher watches the given dir and calls handler on inotify events
// a dir ending in "/*" will result in a recursive watch
func NewNotifyWatcher(dir string, recursive bool, handler NotifyHandlerFun, events ...notify.Event) *NotifyWatcher {
	nw := &NotifyWatcher{
		eventInfo: make(chan notify.EventInfo, 1), // buffered to ensure no events are dropped
		events:    events,
		watchdir:  dir,
		recursive: recursive,
		watching:  false,
		handler:   handler,
	}
	return nw
}

// Watch starts an initialised notify watcher (blockings)
func (nw *NotifyWatcher) Watch() error {
	if nw.watchdir == "" {
		return fmt.Errorf("ERROR: refusing to start empty watcher")
	}
	var dir string
	var err error
	if nw.recursive {
		dir = utils.Dir3Dot(nw.watchdir)
	} else {
		dir = nw.watchdir
	}
	nw.watching = true
	if err = notify.Watch(dir, nw.eventInfo, nw.events...); err != nil { // blocking function
		// log.Printf("error: %s", err.Error())
		nw.watching = false
		return err
	}
	defer notify.Stop(nw.eventInfo)

	for {
		ei, ok := <-nw.eventInfo // this should exit the loop when we close the channel by executing nw.Stop()
		if ok {
			// log.Printf("got event: %v; executing handler...", ei) // FIXME: uncontrolled logging
			if err = nw.handler(&ei); err != nil {
				// log.Printf("failed executing handler for event: %v; %s", ei, err.Error()) // FIXME: uncontrolled logging
				// TODO: consider how to handle this error
			}
		} else {
			err = fmt.Errorf("watcher for %s terminated", nw.watchdir)
			break
		}
	}
	nw.watching = false
	return err
}

func (nw *NotifyWatcher) Stop() error {
	nw.watching = false
	select {
	case _, ok := <-nw.eventInfo:
		if !ok {
			return fmt.Errorf("channel nw.EventInfo already closed")
		} else {
			close(nw.eventInfo) // can we do this? we probably need to be careful in the watch loop?
			return nil
		}
	default:
		return nil
	}
}

func (nw *NotifyWatcher) IsWatching() bool {
	return nw.watching
}
