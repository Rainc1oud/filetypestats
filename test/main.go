package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ppenguin/filetypestats"
	utils "github.com/ppenguin/gogenutils"
)

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "FATAL: %s", err.Error())
	os.Exit(1)
}

func main() {
	dirs := []string{"/usr/share"}

	ts := time.Now()
	ShowFileTypeStats(dirs)
	fmt.Printf("\nFileTypeStats took %v\n", time.Since(ts))

	ts = time.Now()
	ShowFileSizeCount(dirs)
	fmt.Printf("\nFileSizeCount took %v\n", time.Since(ts))
}

func ShowFileTypeStats(dirs []string) {
	var (
		totCount int   = 0
		totSize  int64 = 0
	)
	if ftStats, err := filetypestats.WalkFileTypeStats(dirs); err != nil {
		exitErr(err)
	} else {
		fmt.Println("Test with WalkFileTypeStats:")
		for k, v := range ftStats {
			fmt.Printf("%d %s files taking %s of space\n", v.FileCount, k, utils.ByteCountSI(v.NumBytes))
			totCount += v.FileCount
			totSize += v.NumBytes
		}
		fmt.Printf("\nTotal %d files taking %s of space\n", totCount, utils.ByteCountSI(totSize))
	}
}

func ShowFileSizeCount(dirs []string) {
	if fStats, err := filetypestats.WalkFileSizeCount(dirs); err != nil {
		exitErr(err)
	} else {
		fmt.Printf("Test with WalkFileSizeCount:\n%d files taking %s of space\n", fStats.FileCount, utils.ByteCountSI(fStats.NumBytes))
	}
}
