package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Rainc1oud/filetypestats"
	"github.com/Rainc1oud/filetypestats/ftsdb"
	"github.com/Rainc1oud/filetypestats/treestatsquery"
	"github.com/Rainc1oud/filetypestats/types"
	utils "github.com/Rainc1oud/gogenutils"
)

var (
	dbinstance *ftsdb.FileTypeStatsDB
)

func main() {
	pscandirs := flag.String("dirs", "./", "directories to scan, comma-separated")
	dbfile := flag.String("db", "scandb.sqlite", "database in which the scan result is stored")

	rm := flag.Bool("rm", false, "remove database if exists")
	flag.Parse()

	if len(flag.Args()) == 0 {
		usage()
	}

	scandirs := strings.Split(*pscandirs, ",")

	switch flag.Arg(0) {
	case "scan":
		if *rm {
			os.Remove(*dbfile)
		}
		scan(scandirs, *dbfile)
	case "show":
		show(scandirs, *dbfile)
	case "summary":
		summary(scandirs, *dbfile)
	case "dump":
		dump(scandirs, *dbfile)
	case "watch":
		watch(scandirs, *dbfile)
	default:
		usage()
	}
}

func usage() {
	fmt.Printf(
		"Usage: %s [ --dirs=dir1,dir2 ] [ --db=scandb.sqlite ] [ scan | show | summary | dump ]\n"+
			"\tscan: scans all dirs given recursively and stores statistics per dir in scandb\n"+
			"\tshow: gets the totals from scandb for the given dirs.\n"+
			"\t\tTo show totals under a dir, use the special form --dir='/dir/to/*' (remember quoting if necessary)\n"+
			"\tsummary: show sum totals for all selected dirs\n"+
			"\tdump: dump the paths and info for the selected dirs\n"+
			"\twatch: watch selected dirs for modification (blocking)\n\nFlags:\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(0)
}

func exiterr(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %s", err.Error())
	os.Exit(1)
}

func scan(dirs []string, file string) {
	fmt.Printf("Scanning %v to database %s...\n", dirs, file)
	ts := time.Now()
	if _, err := filetypestats.WalkFileTypeStatsDB(dirs, file); err != nil {
		exiterr(err)
	} else {
		fmt.Printf("Scanning took %s\n\n", time.Since(ts))
		fmt.Println("Scan totals:")
		// printstats(fstats)
	}
}

func show(dirs []string, file string) {
	ts := time.Now()

	var err error
	var fdb *ftsdb.FileTypeStatsDB

	if fdb, err = ftsdb.New(file, false); err != nil {
		exiterr(err)
	}
	defer fdb.Close()

	for _, d := range dirs {
		fstats, err := fdb.FTStatsSum(dirs)
		if err != nil {
			exiterr(err)
		}

		fmt.Printf("%s: query took %s\n\n", d, time.Since(ts))
		printstats(fstats)
	}
}

func summary(dirs []string, file string) {
	ts := time.Now()
	fstats, err := treestatsquery.FTStatsSum(file, dirs)
	if err != nil {
		exiterr(err)
	}
	fmt.Printf("Query took %s\n\n", time.Since(ts))
	fmt.Println("Query totals:")
	printstats(fstats)
}

func dump(dirs []string, file string) {
	var err error
	var fdb *ftsdb.FileTypeStatsDB

	ts := time.Now()
	if fdb, err = ftsdb.New(file, false); err != nil {
		exiterr(err)
	}
	defer fdb.Close()

	flist, err := fdb.FTDumpPaths(dirs)
	if err != nil {
		exiterr(err)
	}
	te := time.Since(ts)
	printflist(flist)
	fmt.Printf("\n\nQuery took %s\n", te)
}

func watch(dirs []string, file string) {
	var fts *filetypestats.TreeStatsWatcher
	var err error
	if fts, err = filetypestats.NewTreeStatsWatcher(dirs, getDB(file)); err != nil {
		exiterr(err)
	}
	fmt.Printf("Watching dirs %v for changes (blocking), press ctrl-c to stop; open a second instance to query the database (read-only)", dirs)
	fts.WatchAll()
}

func printstats(ftstats types.FileTypeStats) {
	fmt.Printf("%10s: \t%30s %8s \t%5s\n%75s\n", "Type", "Path", "Size", "Count", strings.Repeat("-", 75))
	for _, catstat := range ftstats {
		fmt.Printf("%10s: \t%30s (%8s) \t%5d files\n", catstat.FType, catstat.Path, utils.ByteCountSI(catstat.NumBytes), catstat.FileCount)
	}
}

func printflist(flist *[]types.FTypeStat) {
	fmt.Printf("%60s\t%10s\t%10s\t%80s\n", "Path", "Type", "Size", strings.Repeat("-", 75))
	for _, pathinfo := range *flist {
		fmt.Printf("%60s\t%10s\t%10s\n", pathinfo.Path, pathinfo.FType, utils.ByteCountSI(pathinfo.NumBytes))
	}
}

// getDB returns a connection to the DB file (reuses an exising connection to the same file or overwrites the connection if the filename is different)
func getDB(dbfile string) *ftsdb.FileTypeStatsDB {
	if dbinstance != nil && dbinstance.DbFileName() == dbfile {
		return dbinstance
	}

	var err error

	if dbinstance == nil {
		dbinstance, err = ftsdb.New(dbfile, true)
	}
	if err != nil {
		log.Fatalf("couldn't read or create database file: %s", err.Error())
	}
	return dbinstance
}
