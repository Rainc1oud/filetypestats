package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/ppenguin/filetypestats"
	utils "github.com/ppenguin/gogenutils"
)

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "FATAL: %s", err.Error())
	os.Exit(1)
}

func main() {
	usr, _ := user.Current()
	home := usr.HomeDir

	var (
		scanRoot       = []string{home + "/Documents", home + "/Downloads", home + "/.local"}
		totCount int   = 0
		totSize  int64 = 0
	)

	if ftStats, err := filetypestats.WalkFileTypeStats(scanRoot); err != nil {
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
