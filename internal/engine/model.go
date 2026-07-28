// Package engine walks a filesystem tree, aggregates sizes, and (in later
// phases) matches junk rules. It is frontend-agnostic: CLI, TUI, and a future
// GUI all consume it.
package engine

// Entry is a file or directory discovered during a scan.
type Entry struct {
	Path     string
	Size     int64 // bytes; for a directory, the aggregated size of its subtree
	IsDir    bool
	ModTime  int64 // unix seconds; used for staleness in later phases
	Children []*Entry
}

// WalkError records a path that could not be processed during a walk.
type WalkError struct {
	Path string
	Err  error
}

// Report is the result of a Scan.
type Report struct {
	Root      *Entry
	TotalSize int64
	FileCount int
	DirCount  int
}
