package notifywatch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rjeczalik/notify"
	"github.com/stretchr/testify/assert"
)

func mktemp(dir string) string {
	tdir, err := os.MkdirTemp(dir, ".tmp-XXXXX")
	if err != nil {
		panic(err)
	}
	return tdir
}

// not really a test, but more function observation...
func TestNotifyWatcher_Watch(t *testing.T) {
	wd, err := os.Getwd()
	assert.Nil(t, err)
	tdir := mktemp(filepath.Join(wd))
	defer os.RemoveAll(tdir)

	assert.Nil(t, os.WriteFile(filepath.Join(tdir, "tmpfile1.txt"), []byte("Hahaha, this is the content of tmpfile1"), 0644))

	h := func(ei *notify.EventInfo) error {
		fmt.Printf("handler says: ei: %v %v\n", (*ei).Path(), (*ei).Event())
		return nil
	}

	watch := NewNotifyWatcher(filepath.Join(tdir, "..", ".."), true, h, notify.Create, notify.Remove, notify.InModify)
	go func() {
		watch.Watch()
	}()
	assert.Nil(t, os.WriteFile(filepath.Join(tdir, "tmpfile2.txt"), []byte("Hahaha, this is the content of tmpfile2"), 0644))
	time.Sleep(2 * time.Second)
	assert.Nil(t, os.WriteFile(filepath.Join(tdir, "tmpfile3.txt"), []byte("Hahaha, this is the content of tmpfile2"), 0644))
	time.Sleep(1 * time.Second)
	assert.Nil(t, os.Remove(filepath.Join(tdir, "tmpfile1.txt")))
	assert.Nil(t, os.Mkdir(filepath.Join(tdir, "tmpdir"), 0755))
	time.Sleep(1 * time.Second)
	assert.Nil(t, os.WriteFile(filepath.Join(tdir, "tmpdir", "tmpfile11.txt"), []byte("Hahaha, this is the content of tmpfile11"), 0644))
	time.Sleep(2 * time.Second)
}
