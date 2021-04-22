# filetypestats

Combines [`github.com/karrick/godirwalk`](https://github.com/karrick/godirwalk) with (a modified fork of) [`github.com/h2non/filetype`](https://github.com/h2non/filetype) to produce a dictionary with file classes ("video", "audio", ...) as keys and filecount and total size as values.

A slice of root folders to scan can be given as input (this list will be sanitized to remove overlap), and the statistics are returned as aggregated output per file class.