package types

type FTypeStat struct {
	Path      string
	FType     string
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
