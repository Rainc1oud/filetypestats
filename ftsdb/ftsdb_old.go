package ftsdb

import (
	"database/sql"
	"fmt"
	"github.com/Rainc1oud/filetypestats/types"
	_ "github.com/mattn/go-sqlite3"
)

// FTStatsSum returns the summary FileTypeStats for the given paths as a map of FTypeStat per File Type
func (f *FileTypeStatsDB) FTStatsSum_(paths []string) (types.FileTypeStats, error) {
	wp := f.pathsWherePredicate(paths)
	ftstats := make(types.FileTypeStats)
	rs, err := f.DB.Query(fmt.Sprintf(
		`SELECT cats.filecat AS fcat, fileinfo.path, COUNT(fileinfo.path) AS fcatcount, SUM(fileinfo.size) AS fcatsize FROM fileinfo, cats
			WHERE fileinfo.catid=cats.id AND (%s)
			GROUP BY cats.filecat
		 UNION ALL
		 SELECT 'total', '', COUNT(fileinfo.path), SUM(fileinfo.size) FROM cats, fileinfo
		 	WHERE fileinfo.catid=cats.id AND (cats.filecat IS NOT 'dir') AND (%s)
		 ORDER BY fileinfo.path
			`, wp, wp))
	if err != nil {
		return ftstats, err
	}
	defer rs.Close()

	var (
		path       string
		fcat       string
		fcatcount  uint
		fcatsize   uint64
		pathN      sql.NullString
		fcatN      sql.NullString
		fcatcountN sql.NullInt32
		fcatsizeN  sql.NullInt64
	)

	for rs.Next() {
		if err := rs.Scan(&fcatN, &pathN, &fcatcountN, &fcatsizeN); err != nil {
			return ftstats, err
		}
		if !(pathN.Valid && fcatN.Valid && fcatcountN.Valid && fcatsizeN.Valid) { // we had NULL values, just return empty result without error
			return ftstats, nil
		}
		path = pathN.String
		fcat = fcatN.String
		fcatcount = uint(fcatcountN.Int32) // crappy that we don't have sql.NullUInt => will this be a problem???
		fcatsize = uint64(fcatsizeN.Int64)
		if len(paths) == 1 { // the query has specified a single directory pattern, so we use it for the path
			if fcatcount == 1 && fcat != "total" { // there's only one, so we can take the exact path, except for totals take the input path
				ftstats[fcat] = &types.FTypeStat{Path: path, FType: fcat, FileCount: fcatcount, NumBytes: fcatsize}
			} else { // use input pattern for path
				ftstats[fcat] = &types.FTypeStat{Path: paths[0], FType: fcat, FileCount: fcatcount, NumBytes: fcatsize}
			}
		} else {
			ftstats[fcat] = &types.FTypeStat{Path: "*", FType: fcat, FileCount: fcatcount, NumBytes: fcatsize}
		}
	}
	return ftstats, nil
}
