package types

import (
	"fmt"

	"github.com/Rainc1oud/gogenutils"
)

var (
	// "const" string slice to enforce the field order for pretty printing
	FTypeNames  = func() []string { return []string{"dir", "audio", "application", "image", "other", "total"} }
	FClassNames = func() []string {
		return []string{"dir", "other", "application", "archive", "audio", "document", "image", "video"}
	}
	// TODO: somehow the categories seem not to cover all posible types, this might be an issue with h2non/filetype?
	// var FileCategories = func() []string { return []string{"Audio", "Video", "Image", "Application", "Other"} }
)

// FTypeStat contains:
//
//	either a summary for one filetype (Path is wildcard and FileCount >= 1)
//	or the type and size of one file (Path is regular file and FileCount == 1)
type FTypeStat struct {
	Path      string
	FType     string
	NumBytes  uint64
	FileCount uint
}

// FileTypeStats is a map from type (same as FTypeStat.FType) to FTypeStat
type FileTypeStats map[string]*FTypeStat

func FileTypeStatsToString(self *FileTypeStats) { self.ToString() }
func (f *FileTypeStats) ToString() string {
	s := ""
	for _, k := range FTypeNames() {
		if st, ok := (*f)[k]; ok {
			s += fmt.Sprintf("\t%s.sum{size: %8s, count: %5d, path: %-16s}\n", k, gogenutils.ByteCountSI(st.NumBytes), st.FileCount, st.Path)
		}
	}
	return s
}

// FTypeStatsBatch is a "stack like" buffer with a pointer to the next free slot
type FTypeStatsBatch struct {
	cap     int
	ftStats []FTypeStat
}

func NewFTypeStatsBatch(capacity int) *FTypeStatsBatch {
	return &FTypeStatsBatch{
		cap:     capacity,
		ftStats: make([]FTypeStat, 0, capacity),
	}
}

func (fb *FTypeStatsBatch) IsFull() bool {
	return len(fb.ftStats) >= cap(fb.ftStats)-1
}

func (fb *FTypeStatsBatch) IsEmpty() bool {
	return len(fb.ftStats) < 1
}

// Push pushes an item in the buffer and returns false if the buffer is full
// (For convenience false will still handle the current pushif possible, it's just the last one possible, so flushing should be handled by the user)
func (fb *FTypeStatsBatch) Push(elem FTypeStat) bool {
	fb.ftStats = append(fb.ftStats, elem)
	return !fb.IsFull()
}

func (fb *FTypeStatsBatch) AllElem() []FTypeStat {
	return fb.ftStats[:]
}

func (fb *FTypeStatsBatch) Reset() {
	fb.ftStats = make([]FTypeStat, 0, fb.cap)
}
