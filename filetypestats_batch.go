package filetypestats

import (
	"github.com/Rainc1oud/filetypestats/ftsdb"
	"github.com/Rainc1oud/filetypestats/types"
)

// singleton batch buffer
var pFTSBatch *FTypeStatsBatch

const piBatchSize = 200

type FTypeStatsBatch struct {
	firstFreeIdx int
	ftStats      []types.FTypeStat
}

func (f *FTypeStatsBatch) GetInstance() *FTypeStatsBatch {
	if pFTSBatch == nil {
		pFTSBatch = &FTypeStatsBatch{
			firstFreeIdx: 0,
			ftStats:      make([]types.FTypeStat, piBatchSize),
		}
	}
	return pFTSBatch
}

func (f *FTypeStatsBatch) Append(path, filecat string, size uint64, fdb *ftsdb.FileTypeStatsDB) error {
	f.ftStats[f.firstFreeIdx] = types.FTypeStat{Path: path, FType: filecat, NumBytes: size}
	f.firstFreeIdx += 1
	if f.firstFreeIdx >= piBatchSize-1 {
		return f.Commit(fdb)
	}
	return nil
}

func (f *FTypeStatsBatch) Commit(fdb *ftsdb.FileTypeStatsDB) error {
	if f.firstFreeIdx < 1 {
		return nil
	}
	// fts := f.ftStats[:f.firstFreeIdx-1] // because we need this, it's probably the same effect as pass by value???
	err := fdb.UpdateMultiFileStats(f.ftStats[:f.firstFreeIdx-1])
	if err == nil { // we flush the buffer after a successful commit, so the next batch can start
		pFTSBatch = nil
	}
	return err
}
