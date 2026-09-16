package notifywatch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Rainc1oud/filetypestats/utils"
	"github.com/rjeczalik/notify"
)

type EventHandler func(*notify.EventInfo) error

// ResyncHandler is called when the watcher cannot be trusted to have seen everything and the tree
// should be rescanned. Whoever registers it is expected to repair the gap.
//
// It fires for two reasons, both of which happen during a burst of changes:
//   - events had to be dropped because the queue was full;
//   - events arrived faster than burstRate, which is when inotify itself starts losing them -- a
//     file created in a directory that was itself just created can land before the recursive watch
//     covers it, and no amount of buffering on our side can catch that one.
type ResyncHandler func(reason string)

const (
	// notifyChanSize buffers what the kernel delivers. notify drops events when this channel is
	// full ("Drop event if receiver is too slow", rjeczalik/notify watchpoint.go) and tells nobody,
	// so the only defence is to take them off it quickly -- see the receive loop in Watch.
	notifyChanSize = 4096
	// queueSize buffers what is waiting to be handled. Handling an event reads and classifies a
	// file and writes a database row, which is orders of magnitude slower than receiving it, so the
	// two are separated by this queue. What does not fit is counted and reported as an overflow.
	queueSize = 16384
	// burstRate is the number of events per burstWindow above which the tree is treated as changing
	// in bulk, and a resync is asked for. Ordinary use (a file saved, a folder copied) stays far
	// below it, so this does not turn into a rescan on every change.
	burstRate   = 500
	burstWindow = time.Second
)

// NotifyWatcher owns one filesystem notification registration.
type NotifyWatcher struct {
	watchDir  string
	recursive bool
	events    []notify.Event
	handler   EventHandler

	mu        sync.RWMutex
	running   bool
	cancel    context.CancelFunc
	done      chan struct{}
	dropped   uint64
	lastErr   error
	resync    ResyncHandler
	reporting bool

	windowStart  time.Time
	windowEvents int
}

func NewNotifyWatcher(dir string, recursive bool, handler EventHandler, events ...notify.Event) *NotifyWatcher {
	return &NotifyWatcher{
		watchDir:  dir,
		recursive: recursive,
		events:    append([]notify.Event(nil), events...),
		handler:   handler,
	}
}

// OnResync registers the handler called when the tree should be rescanned. It must be set before
// Watch.
func (w *NotifyWatcher) OnResync(h ResyncHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.resync = h
}

// Dropped reports how many events were lost because the queue was full.
func (w *NotifyWatcher) Dropped() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.dropped
}

// LastError reports the last error a handler returned, if any. Handler errors do not stop the
// watch: one unreadable file must not blind the watcher to everything that follows.
func (w *NotifyWatcher) LastError() error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastErr
}

// Watch blocks until ctx is cancelled. It returns only after the underlying
// notify registration has been removed.
func (w *NotifyWatcher) Watch(ctx context.Context) error {
	if ctx == nil {
		return errors.New("refusing to start watcher with nil context")
	}
	if w.watchDir == "" {
		return errors.New("refusing to start empty watcher")
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher for %s is already running", w.watchDir)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	w.running = true
	w.cancel = cancel
	w.done = done
	w.mu.Unlock()

	defer func() {
		cancel()
		w.mu.Lock()
		w.running = false
		w.cancel = nil
		w.done = nil
		close(done)
		w.mu.Unlock()
	}()

	dir := w.watchDir
	if w.recursive {
		dir = utils.Dir3Dot(dir)
	}
	eventCh := make(chan notify.EventInfo, notifyChanSize)
	if err := notify.Watch(dir, eventCh, w.events...); err != nil {
		return err
	}
	defer notify.Stop(eventCh)

	// The queue separates receiving from handling: the receive loop below must return to the
	// channel as fast as it can, because everything it does not take is dropped by notify.
	queue := make(chan notify.EventInfo, queueSize)
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		w.handleQueue(queue)
	}()
	defer func() {
		close(queue)
		<-handlerDone
	}()

	for {
		select {
		case <-runCtx.Done():
			return nil
		case event := <-eventCh:
			w.countEvent()
			select {
			case queue <- event:
			default:
				// The handler cannot keep up. Count it and let the resync handler repair the gap;
				// dropping quietly is what made a lost burst invisible.
				w.reportDropped()
			}
		}
	}
}

// handleQueue calls the handler for queued events, in order (the handler pairs move-from with
// move-to events, so the order matters), until the queue is closed.
func (w *NotifyWatcher) handleQueue(queue <-chan notify.EventInfo) {
	for event := range queue {
		if w.handler == nil {
			continue
		}
		if err := w.handler(&event); err != nil {
			w.mu.Lock()
			w.lastErr = fmt.Errorf("handle event for %s: %w", w.watchDir, err)
			w.mu.Unlock()
		}
	}
}

// countEvent tracks the event rate and asks for a resync once it crosses burstRate.
func (w *NotifyWatcher) countEvent() {
	w.mu.Lock()
	now := time.Now()
	if now.Sub(w.windowStart) > burstWindow {
		w.windowStart = now
		w.windowEvents = 0
	}
	w.windowEvents++
	burst := w.windowEvents == burstRate
	w.mu.Unlock()

	if burst {
		w.requestResync("event rate above burst threshold")
	}
}

// reportDropped counts one dropped event and asks for a resync.
func (w *NotifyWatcher) reportDropped() {
	w.mu.Lock()
	w.dropped++
	w.mu.Unlock()
	w.requestResync("event queue full")
}

// requestResync calls the resync handler, coalesced: it is not called again while a call is still
// running, since a burst produces these by the thousand.
func (w *NotifyWatcher) requestResync(reason string) {
	w.mu.Lock()
	handler := w.resync
	if handler == nil || w.reporting {
		w.mu.Unlock()
		return
	}
	w.reporting = true
	w.mu.Unlock()

	go func() {
		handler(reason)
		w.mu.Lock()
		w.reporting = false
		w.mu.Unlock()
	}()
}

// Stop is idempotent and waits for Watch to release its registration.
func (w *NotifyWatcher) Stop() error {
	w.mu.RLock()
	cancel, done := w.cancel, w.done
	w.mu.RUnlock()
	if cancel == nil {
		return nil
	}
	cancel()
	<-done
	return nil
}

func (w *NotifyWatcher) IsWatching() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}
