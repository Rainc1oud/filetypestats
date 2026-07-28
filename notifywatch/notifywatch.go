package notifywatch

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Rainc1oud/filetypestats/utils"
	"github.com/rjeczalik/notify"
)

type EventHandler func(*notify.EventInfo) error

// NotifyWatcher owns one filesystem notification registration.
type NotifyWatcher struct {
	watchDir  string
	recursive bool
	events    []notify.Event
	handler   EventHandler

	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewNotifyWatcher(dir string, recursive bool, handler EventHandler, events ...notify.Event) *NotifyWatcher {
	return &NotifyWatcher{
		watchDir:  dir,
		recursive: recursive,
		events:    append([]notify.Event(nil), events...),
		handler:   handler,
	}
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
	eventCh := make(chan notify.EventInfo, 64)
	if err := notify.Watch(dir, eventCh, w.events...); err != nil {
		return err
	}
	defer notify.Stop(eventCh)

	for {
		select {
		case <-runCtx.Done():
			return nil
		case event := <-eventCh:
			if w.handler == nil {
				continue
			}
			if err := w.handler(&event); err != nil {
				return fmt.Errorf("handle event for %s: %w", w.watchDir, err)
			}
		}
	}
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
