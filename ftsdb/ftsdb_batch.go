package ftsdb

import (
	"fmt"
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

// upsertFileStatsMulti upserts every path in the batch inside a single transaction,
// reusing the prepared upsert statement per row instead of building one large
// Sprintf/strings.Join'd SQL string (which was expensive to parse/plan and
// allocation-heavy at production batch sizes).
func (f *FileTypeStatsDB) upsertFileStatsMulti(batchBuffer *types.FTypeStatsBatch) error {
	pathsInfo := batchBuffer.AllElem()
	updated := time.Now().Unix()

	f.dbmutex.Lock() // make sure the transaction is executed exclusive
	defer f.dbmutex.Unlock()

	tx, err := f.DB.Begin()
	if err != nil {
		return err
	}
	stmt := tx.Stmt(f.stmtUpsert)
	defer stmt.Close()

	for _, pi := range pathsInfo {
		catid, ok := f.catIDs[pi.FType]
		if !ok {
			tx.Rollback()
			return fmt.Errorf("unknown file category %q", pi.FType)
		}
		if _, err := stmt.Exec(pi.Path, pi.NumBytes, catid, updated); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
