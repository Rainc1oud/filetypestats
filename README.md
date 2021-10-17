# `filetypestats`

## Breaking Changes v0.3.0

Version 0.3.0 probably has breaking changes

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