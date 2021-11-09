# `filetypestats`

## Background

*TODO* update

Combines [`github.com/karrick/godirwalk`](https://github.com/karrick/godirwalk) with (a modified fork of) [`github.com/h2non/filetype`](https://github.com/h2non/filetype) to produce a dictionary with file classes ("video", "audio", ...) as keys and filecount and total size as values.

A slice of root folders to scan can be given as input (this list will be sanitized to remove overlap), and the statistics are returned as aggregated output per file class.

For performance reasons, scanning has been modified to store results in an `sqlite` database, and a normal query will be done on the DB, not perform a new scan.
To keep the DB up to date without doing frequent rescans, recursive `inotify` is used.

There are several `go` libs that wrap `inotify`:

- https://github.com/tywkeene/go-fsevents (`linux`, recursive, lean)
- https://github.com/illarion/gonotify (`linux`, recursive, lean)
- https://github.com/rjeczalik/notify (x-platform, recursive, large?)

For x-platform, we first try `notify`, if it is too resource-hungry, we may have to switch, since for now the main use case is NAS.


## Changelog (anecdotal)

### v0.3.4

Return struct changed from a map of dirs (which was not actually used as such) to `FileTypeStats`, which looks like this:

```go
type FTypeStat struct {
	Path      string
	FType     string
	NumBytes  uint64
	FileCount uint
}

// FileTypeStats is a map from type (same as FTypeStat.FType) to FTypeStat
type FileTypeStats map[string]*FTypeStat
```

The `FType` field contains one of `<filetype>` from [`h2non/filetype/kind.go `](https://github.com/h2non/filetype/blob/v1.1.1/kind.go) (in lowercase), plus the special keys '`dir`' and '`total`', which all are keys to `FileTypeStats`.

For '`dir`' `NumBytes` is always `0`.

`Path` has the following values:

- absolute path of a file for keys (`kind` or "category") where `FileCount == 1` *and* the query contains only one directory
- `<path>/*` if query contains only one directory
- otherwise `*` 


### Breaking Changes v0.3.0

Version 0.3.0 probably has breaking changes