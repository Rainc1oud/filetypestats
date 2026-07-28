package notifywatch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rjeczalik/notify"
)

func TestNotifyWatcherHandlesEventAndStops(t *testing.T) {
	dir := t.TempDir()
	eventSeen := make(chan struct{}, 1)
	watcher := NewNotifyWatcher(dir, true, func(*notify.EventInfo) error {
		select {
		case eventSeen <- struct{}{}:
		default:
		}
		return nil
	}, notify.Create, notify.InModify)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- watcher.Watch(ctx) }()

	deadline := time.Now().Add(time.Second)
	for !watcher.IsWatching() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !watcher.IsWatching() {
		t.Fatal("watcher did not start")
	}
	if err := os.WriteFile(filepath.Join(dir, "event.txt"), []byte("event"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case <-eventSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not handle filesystem event")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watch() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop")
	}
}

func TestNotifyWatcherStopIsConcurrentAndIdempotent(t *testing.T) {
	watcher := NewNotifyWatcher(t.TempDir(), true, nil, notify.Create)
	done := make(chan error, 1)
	go func() { done <- watcher.Watch(context.Background()) }()

	deadline := time.Now().Add(time.Second)
	for !watcher.IsWatching() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	var failures atomic.Int32
	stopped := make(chan struct{}, 2)
	for range 2 {
		go func() {
			if err := watcher.Stop(); err != nil {
				failures.Add(1)
			}
			stopped <- struct{}{}
		}()
	}
	<-stopped
	<-stopped
	if failures.Load() != 0 {
		t.Fatalf("Stop failures = %d", failures.Load())
	}
	if err := <-done; err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	if err := watcher.Stop(); err != nil {
		t.Fatalf("repeated Stop() error = %v", err)
	}
}
