package types

import (
	"database/sql"
)

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

type FileTypeStatsDB struct {
	// self *FileTypeStatsDB
	fileName string
	DB       *sql.DB
}

type DirTreeInfo struct {
	statsDB   *FileTypeStatsDB
	watchDirs []string
	// notifyWatcher *inotify_blabla
}
