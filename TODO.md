## Notify Watcher

- if a (larger?) dir is deleted, it seems that arbitrary events arrive and are handled, leading to only deletion of a few children in the DB
- check whether all events are handled if many files are copied (i.e. many inotify events)