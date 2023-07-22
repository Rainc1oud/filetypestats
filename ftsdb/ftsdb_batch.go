package ftsdb

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rainc1oud/filetypestats/types"
)

func (f *FileTypeStatsDB) UpdateFileStatsMulti(path, filecat string, size uint64, batchBuffer *types.FTypeStatsBatch) error {
	var err error
	if !batchBuffer.Push(types.FTypeStat{Path: path, FType: filecat, NumBytes: size}) { // push returns false if this push filled the buffer to capacity
		err = f.CommitBatch(batchBuffer) // commit resets lastElem and empties the batch buffer
	}
	return err
}

func (f *FileTypeStatsDB) CommitBatch(batchBuffer *types.FTypeStatsBatch) error {
	if batchBuffer.IsEmpty() {
		return nil
	}
	err := f.upsertFileStatsMulti(batchBuffer)
	// we flush the buffer after a commit, so the next batch can start
	// (if there is an error, the data is not saved, which is "acceptable" because an error for an upsert means we couldn't save the data in the DB anyway)
	batchBuffer.Reset()

	return err
}

// upsertFileStatsMulti upserts the file in path with size
// best done with transactions: https://stackoverflow.com/a/5009740
func (f *FileTypeStatsDB) upsertFileStatsMulti(batchBuffer *types.FTypeStatsBatch) error {
	pathsInfo := batchBuffer.AllElem()
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

	f.dbmutex.Lock() // make sure the transaction block in the query is executed exclusive
	defer f.dbmutex.Unlock()

	if _, err := f.DB.Exec(qry); err != nil {
		return err
	}

	return nil
}
