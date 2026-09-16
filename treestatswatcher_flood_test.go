package filetypestats

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rainc1oud/filetypestats/ftsdb"
	"github.com/Rainc1oud/filetypestats/utils"
)

// floodFileCount is chosen to overrun the event queue comfortably on any machine: the point of the
// test is the burst, not the volume, so the files stay tiny.
const floodFileCount = 3000

// TestWatcherSurvivesEventFlood writes a burst of files into a watched tree and requires the
// database to end up complete.
//
// This is what a bulk change looks like in production (a share being restored, synced or
// regenerated), and it used to be lost: notify drops events when the receiving channel is full,
// the handler ran inline in the receive loop -- reading and classifying every file, then writing a
// row -- so the channel stayed full, and nothing noticed the loss. Handling events off the receive
// loop absorbs the burst; whatever still overruns the queue marks the tree dirty and is repaired by
// the rescan that follows the burst.
func TestWatcherSurvivesEventFlood(t *testing.T) {
	root := t.TempDir()
	dbfile := filepath.Join(t.TempDir(), "flood.sqlite")

	fdb, err := ftsdb.New(dbfile, true)
	if err != nil {
		t.Fatal(err)
	}
	defer fdb.Close()

	tsw, err := NewTreeStatsWatcher([]string{root}, fdb)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	watchDone := make(chan error, 1)
	go func() { watchDone <- tsw.WatchAll(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-watchDone:
		case <-time.After(30 * time.Second):
			t.Error("watcher did not stop")
		}
	})

	waitForWatch(t, tsw, root)

	// The burst: files land as fast as the filesystem takes them, in nested dirs, so the watcher
	// also has to keep up with directories appearing under it.
	for i := range floodFileCount {
		dir := filepath.Join(root, fmt.Sprintf("dir_%02d", i%20))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		name := filepath.Join(dir, fmt.Sprintf("file-%05d.png", i))
		if err := os.WriteFile(name, testPNG, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Generous: the repair path waits out a quiet period and then rescans the tree.
	deadline := time.Now().Add(90 * time.Second)
	var count uint
	for time.Now().Before(deadline) {
		stats, err := fdb.FTStatsSum([]string{utils.DirStar(root)})
		if err != nil {
			t.Fatal(err)
		}
		if count = stats["image"].FileCount; count == floodFileCount {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("database holds %d of %d files after the burst; events were lost and never repaired", count, floodFileCount)
}

// waitForWatch waits until the monitor for dir is actually watching, so the burst below cannot
// start before the watch is in place (which would make the test pass for the wrong reason).
func waitForWatch(t *testing.T, tsw *TreeStatsWatcher, dir string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if monitor := tsw.get(dir); monitor != nil && monitor.IsWatching() {
			time.Sleep(200 * time.Millisecond) // let the recursive registration settle
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("watcher did not start")
}
