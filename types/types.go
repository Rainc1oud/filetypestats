package types

type FTypeStat struct {
	NumBytes  uint64
	FileCount uint
}

type FileTypeStats map[string]*FTypeStat

type FTypeDirStat struct {
	// dir string
	FTypeStats FileTypeStats
	TotCount   uint
	TotSize    uint64
}

type FileTypeDirStats map[string]*FTypeDirStat

// type DirTreesInfo struct {
// 	statsDB        *FileTypeStatsDB
// 	watchDirs      []string
// 	notifyWatchers *NotifyWatchers
// }
