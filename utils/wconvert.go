package utils

import (
	"path/filepath"
	"strings"

	ggu "github.com/Rainc1oud/gogenutils"
)

// wconvert contains util functions to convert wildcards between different formats used by this lib and included libs
// notably: recursive directory is:
// - "dir/..." in notify
// - "dir/*" in sqlite GLOB
// TODO: it is not clear how the behaviour on windows is (not yet supported)

func JustDir(path string) string {
	s := strings.TrimRight(path, string(filepath.Separator))
	s = strings.TrimSuffix(s, string(filepath.Separator)+"*")
	s = strings.TrimSuffix(s, string(filepath.Separator)+"...")
	return s
}

func DirStar(path string) string {
	return filepath.Join(JustDir(path), "*")
}

func Dir3Dot(path string) string {
	return filepath.Join(JustDir(path), "...")
}

func DirTrailSep(path string) string {
	return JustDir(path) + string(filepath.Separator)
}

// HarmonizePathStar returns "path/" => "path/*" or "path///" => "path/*" or "pathxxxyyy" => "pathxxxyyy"
func CleanPath(path string) string {
	if strings.HasSuffix(path, "//") { // TODO: somehow this occurs sometimes, the extra / might be due to over-correction somewhere? TODO: better fix root cause
		path = strings.TrimRight(path, "/") + "/" // remove all trailing / and re-add one /
	}
	return path
}

// CleanPathStar returns "path/" => "path/*" or "path///" => "path/*" or "pathxxxyyy" => "pathxxxyyy"
func CleanPathStar(path string) string {
	path = CleanPath(path)
	if strings.HasSuffix(path, "/") {
		path += "*"
	}
	return path
}

func IsDirRecursive(path string) bool {
	return strings.HasSuffix(path, string(filepath.Separator)) || strings.HasSuffix(path, string(filepath.Separator)+"*") || strings.HasSuffix(path, string(filepath.Separator)+"...")
}

func StringSliceApply(slice []string, fun func(string) string) []string {
	for i, v := range slice {
		slice[i] = fun(v)
	}
	return slice
}

// TODO: queries could be significantly optimised if we would smartly filter a path list to:
// - exlude all entries if an item "/my/dir/.../myfile" with a parent dir "/my/dir*/*"" or "/my/dir/*" exists
// This is a variation of "FilterCommonRootdirs" with glob support
// For now, our simpler implementation just filters duplicates
// OptimizePathsGlob returns an optimised paths list with duplicates removed (TODO: remove children that are included by parent glob)
func OptimizePathsGlob(paths *[]string) []string {
	return *ggu.StringSliceUniq(paths)
}
