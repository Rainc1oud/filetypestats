package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ppenguin/filetypestats"
	"github.com/ppenguin/filetypestats/treestatsquery"
	"github.com/ppenguin/filetypestats/utils"
	"github.com/ppenguin/gogenutils"
)

var (
	testdirs = []string{"Documents", "Downloads"} // just Q&D, make sure to put existing dirs here
	dbfile   string
	tsw      *filetypestats.TreeStatsWatcher
)

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "FATAL: %s", err.Error())
	os.Exit(1)
}

func main() {

	dirs := getTestDirs()

	wd, err := os.Getwd()
	if err != nil {
		exitErr(err)
	}

	dbfile = filepath.Join(wd, "testdb.sqlite")
	tsw, err = filetypestats.NewTreeStatsWatcher(dirs, dbfile)
	if err != nil {
		exitErr(err)
	}

	// do some basic checks...

	// 1. We just initialised with dirs, that means normally a scan will be running for them...
	if err := checkInitialScan(tsw); err != nil {
		exitErr(err)
	}

	// 2. check selected files against query results
	// this is only approximate, because the selection criteria for the file-scan and the DB query are a bit different, and recursiveness also
	if err := checkFilesGlob("image", []string{"*.jpg", "*.png"}); err != nil {
		exitErr(err)
	}

	if err := checkFilesGlob("application", []string{"*.pdf", "*.*z*", "*.exe", "*.txt"}); err != nil {
		exitErr(err)
	}

	fmt.Println("Starting to watch, press ctrl+c to exit...")
	fmt.Printf("Manipulate the contents of %v to test inotify\n", tsw.Dirs())
	tsw.WatchAll()
	fmt.Println("All watchers finished")
}

func getTestDirs() []string {
	dirs := make([]string, 0)
	hdir, err := os.UserHomeDir()
	if err != nil {
		return dirs
	}
	for _, d := range testdirs {
		if fi, err := os.Lstat(filepath.Join(hdir, d)); err == nil && fi.IsDir() {
			dirs = append(dirs, filepath.Join(hdir, d))
		}
	}
	return dirs
}

func checkInitialScan(tsw *filetypestats.TreeStatsWatcher) error {
	errs := gogenutils.NewErrors()
	// ds := tsw.Dirs()
	anyscan := true
	for anyscan {
		anyscan = false
		for _, d := range tsw.Dirs() {
			stats, err := getStatSummaryStr(d)
			errs.AddIf(err)
			if tsw.ScanRunning(d) {
				fmt.Printf("scan for %s running, current stats: \n%s\n", d, stats)
				anyscan = true
			} else {
				anyscan = anyscan || false
				fmt.Printf("scan for %s finished (anyscan: %t), current stats: \n%s\n", d, anyscan, stats)
			}
		}
		time.Sleep(1 * time.Second)
	}
	return errs.Err()
}

func getStatSummaryStr(dir string) (string, error) {
	dirs := []string{utils.DirStar(dir)}
	stats, err := treestatsquery.FTStatsSum(dbfile, dirs)
	if err != nil {
		return "", err
	}
	return stats.ToString(), nil
}

func checkFilesGlob(ftype string, globs []string) error {
	// find some files and check their data against specific query data
	files := []string{}
	for _, d := range tsw.Dirs() {
		for _, g := range globs {
			files = append(files, getRelGlob(d, g)...)
		}
	}
	return checkFTStats(files, ftype)
}

func checkFTStats(files []string, ftype string) error {
	// find some files and check their data against specific query data
	if len(files) > 0 {
		ts, tc := getFileSizeCount(files)
		fmt.Printf("\nFound the following %s files: %v\ntotal size: %6s\ttotal count: %5d\n", ftype, files, gogenutils.ByteCountSI(ts), tc)
		qd := utils.StringSliceApply(tsw.Dirs(), utils.DirTrailSep)
		stats, err := treestatsquery.FTStatsSum(dbfile, qd) // now we basically expect that the images fount in the top-level dir correspond to those in the query result
		if err != nil {
			fmt.Printf("ERROR performing query: %s", err.Error())
			return err
		}
		if fst, ok := stats[ftype]; ok {
			fmt.Printf("\nQuery result for %s files: \ntotal size: %6s\ttotal count: %5d\n", ftype, gogenutils.ByteCountSI(fst.NumBytes), fst.FileCount)
		}
	}
	return nil
}

func getRelGlob(dir, pat string) []string {
	fs, err := filepath.Glob(filepath.Join(dir, pat))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err.Error())
		return []string{}
	}
	return fs

}

func getFileSizeCount(files []string) (uint64, uint) {
	var size uint64 = 0
	var count uint = 0
	for _, f := range files {
		if fi, err := os.Lstat(f); err == nil {
			if !fi.IsDir() {
				size += uint64(fi.Size())
				count += 1
			}
		}
	}
	return size, count
}
