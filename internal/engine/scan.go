package engine

import "path/filepath"

// Scan walks root and produces a Report with aggregated size and node counts.
// The named root is resolved through filepath.EvalSymlinks first (Walk does not
// follow symlinks, including a symlinked root, and common macOS roots such as
// /tmp are symlinks). If resolution fails (e.g. root does not exist), the
// original root is walked so Walk surfaces the real error.
func Scan(root string) (*Report, []WalkError, error) {
	scanRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		scanRoot = resolved
	}
	rootEntry, werrs, err := Walk(scanRoot)
	if err != nil {
		return nil, werrs, err
	}
	r := &Report{
		Root:      rootEntry,
		TotalSize: rootEntry.Size,
	}
	// TotalSize is already on the root (Walk aggregated it). Counts still need
	// one pass over the tree.
	countEntries(rootEntry, r)
	return r, werrs, nil
}

// countEntries tallies file and directory nodes into r by walking the tree the
// same shape Walk produced (the root itself is counted as a directory).
func countEntries(e *Entry, r *Report) {
	if e.IsDir {
		r.DirCount++
	} else {
		r.FileCount++
	}
	for _, c := range e.Children {
		countEntries(c, r)
	}
}
