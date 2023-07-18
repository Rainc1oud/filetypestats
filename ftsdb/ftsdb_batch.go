package ftsdb

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rainc1oud/filetypestats/types"
)

const piBatchSize = 200

type FTypeStatsBatch struct {
	firstFreeIdx int
	ftStats      []types.FTypeStat
}

func (fb *FTypeStatsBatch) Reset() {
	fb.firstFreeIdx = 0
	fb.ftStats = make([]types.FTypeStat, piBatchSize)
}

func (f *FileTypeStatsDB) UpdateFileStatsMulti(path, filecat string, size uint64) error {
	f.pFTSBatch.ftStats[f.pFTSBatch.firstFreeIdx] = types.FTypeStat{Path: path, FType: filecat, NumBytes: size}
	f.pFTSBatch.firstFreeIdx += 1
	if f.pFTSBatch.firstFreeIdx >= piBatchSize-1 {
		return f.CommitBatch()
	}
	return nil
}

func (f *FileTypeStatsDB) CommitBatch() error {
	if f.pFTSBatch.firstFreeIdx < 1 {
		return nil
	}
	// fts := f.ftStats[:f.firstFreeIdx-1] // because we need this, it's probably the same effect as pass by value???
	err := f.upsertFileStatsMulti(f.pFTSBatch.ftStats[:f.pFTSBatch.firstFreeIdx-1])
	if err == nil { // we flush the buffer after a successful commit, so the next batch can start
		f.pFTSBatch.Reset()
	}
	return err
}

// upsertFileStatsMulti upserts the file in path with size
// best done with transactions: https://stackoverflow.com/a/5009740
func (f *FileTypeStatsDB) upsertFileStatsMulti(pathsInfo []types.FTypeStat) error {
	qryl := make([]string, len(pathsInfo)+2)
	qryl[0] = "BEGIN TRANSACTION"
	i := 1
	for _, pi := range pathsInfo {
		qryl[i] = (fmt.Sprintf(
			`INSERT INTO fileinfo(path, size, catid, updated) VALUES('%s', %d, (SELECT id FROM cats WHERE filecat='%s'), %d)
				ON CONFLICT(path) DO
				UPDATE SET size=%d, catid=(SELECT id FROM cats WHERE filecat='%s'), updated=%d`,
			strings.Replace(pi.Path, "'", "''", -1), pi.NumBytes, pi.FType, time.Now().Unix(),
			pi.NumBytes, pi.FType, time.Now().Unix(),
		))
		i += 1
	}
	qryl[i] = "COMMIT;"
	qry := strings.Join(qryl, ";\n")
	if _, err := f.DB.Exec(qry); err != nil {
		return err
	}
	return nil
}
