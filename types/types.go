package types

type FTypeStat struct {
	NumBytes  uint64
	FileCount uint
}

type FileTypeStats map[string]*FTypeStat
