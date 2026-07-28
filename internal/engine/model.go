// Package engine walks a filesystem tree, aggregates sizes, and (in later
// phases) matches junk rules. It is frontend-agnostic: CLI, TUI, and a future
// GUI all consume it.
package engine

// Entry is one node (a file or a directory) in the scanned tree. Directories
// hold their children in Children and their Size is the sum of the whole
// subtree; files have a nil Children and Size is their own byte count. This one
// shape is what every frontend renders — both the size-tree view and the
// category view are built from the same Entry graph.
type Entry struct {
	Path     string
	Size     int64    // bytes. File: own size. Directory: aggregated size of its subtree.
	IsDir    bool     // true for directories; false for files and symlinks.
	ModTime  int64    // last-modified time, unix seconds. Used for staleness scoring in later phases.
	Children []*Entry // child entries for a directory; nil for a file or symlink (never descended).
}

// WalkError records a single path the walk could not process (e.g. permission
// denied). It is non-fatal by design: the walk collects these and keeps going
// so one unreadable directory never aborts the whole scan.
type WalkError struct {
	Path string // the path that failed
	Err  error  // the underlying os error
}

// Report is the finished result of a Scan: the root of the tree plus rolled-up
// totals the frontend prints without re-walking.
type Report struct {
	Root      *Entry // root of the scanned tree; Root.Size == TotalSize
	TotalSize int64  // total bytes under the root
	FileCount int    // number of file nodes in the tree
	DirCount  int    // number of directory nodes in the tree (the root counts)
}
