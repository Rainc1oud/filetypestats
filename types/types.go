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
			s += fmt.Sprintf("\t%s.sum{size: %8s, count: %5d}\n", k, gogenutils.ByteCountSI(st.NumBytes), st.FileCount)
		}
	}
	return s
}

// FTypeStatsBatch is a "stack like" buffer with a pointer to the next free slot
type FTypeStatsBatch struct {
	cap      int
	lastElem int
	ftStats  []FTypeStat
}

func NewFTypeStatsBatch(capacity int) *FTypeStatsBatch {
	return &FTypeStatsBatch{
		cap:      capacity,
		lastElem: 0,
		ftStats:  make([]FTypeStat, capacity),
	}
}

func (fb *FTypeStatsBatch) IsFull() bool {
	return fb.lastElem >= fb.cap-2
}

func (fb *FTypeStatsBatch) IsEmpty() bool {
	return fb.lastElem < 1
}

// Push pushes an item in the buffer and returns false if the buffer is full
// (For convenience false will still handle the current pushif possible, it's just the last one possible, so flushing should be handled by the user)
func (fb *FTypeStatsBatch) Push(elem FTypeStat) bool {
	fb.lastElem += 1
	fb.ftStats[fb.lastElem] = elem
	return fb.IsFull()
}

func (fb *FTypeStatsBatch) AllElem() []FTypeStat {
	return fb.ftStats[:fb.lastElem]
}

func (fb *FTypeStatsBatch) Reset() {
	fb.lastElem = 0
	fb.ftStats = make([]FTypeStat, fb.cap)
}
