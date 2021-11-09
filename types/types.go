package types

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

// Obsoleted: totals are returned from normal queries in FileTypeStats
// type FTypeDirStat struct {
// 	// dir string
// 	FTypeStats FileTypeStats
// 	TotCount   uint
// 	TotSize    uint64
// }

// type FileTypeDirStats map[string]*FTypeDirStat
