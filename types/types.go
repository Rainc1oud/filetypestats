package types

import (
	"fmt"

	"github.com/ppenguin/gogenutils"
)

var (
	// "const" string slice to enforce the field order for pretty printing
	FTypeNames = func() []string { return []string{"dir", "audio", "application", "image", "other", "total"} }
)

// FTypeStat contains:
//		either a summary for one filetype (Path is wildcard and FileCount >= 1)
//		or the type and size of one file (Path is regular file and FileCount == 1)
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
