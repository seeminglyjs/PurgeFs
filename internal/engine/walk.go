package engine

import (
	"os"
	"path/filepath"
)

// Walk traverses root and returns the root Entry with subtree sizes aggregated.
// Symlinks are recorded as leaf entries (their own size) and never followed.
// Per-path errors (e.g. permission denied) are collected in the returned slice
// and traversal continues. The error return is non-nil only when root itself
// cannot be stat'd.
func Walk(root string) (*Entry, []WalkError, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, nil, err
	}
	var werrs []WalkError
	e := walkEntry(root, info, &werrs)
	return e, werrs, nil
}

// walkEntry builds the Entry for path and, if it is a directory, recurses into
// its children. Size bubbles up: a directory's Size is accumulated from every
// child as the recursion returns, so the root ends up holding the total. errs
// is threaded by pointer so failures deep in the tree land in one shared slice.
func walkEntry(path string, info os.FileInfo, werrs *[]WalkError) *Entry {
	e := &Entry{
		Path:    path,
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Unix(),
	}
	// Base case: a file is a leaf — record its own size and stop.
	if !info.IsDir() {
		e.Size = info.Size()
		return e
	}

	// Listing the directory can fail (permissions). Record it and return the
	// directory as an empty node rather than aborting the whole walk.
	dirents, err := os.ReadDir(path)
	if err != nil {
		*werrs = append(*werrs, WalkError{Path: path, Err: err})
		return e
	}

	for _, de := range dirents {
		childPath := filepath.Join(path, de.Name())
		// de.Info() is lstat-based, so a symlink reports as a symlink here
		// instead of the file it points at — that is how we avoid following it.
		ci, err := de.Info()
		if err != nil {
			*werrs = append(*werrs, WalkError{Path: childPath, Err: err})
			continue
		}
		// Symlink: record the link itself as a leaf (its own tiny size) and do
		// NOT descend. Following it could escape the scan root, double-count a
		// real subtree, or later delete something outside what the user chose.
		if ci.Mode()&os.ModeSymlink != 0 {
			child := &Entry{
				Path:    childPath,
				IsDir:   false,
				Size:    ci.Size(),
				ModTime: ci.ModTime().Unix(),
			}
			e.Children = append(e.Children, child)
			e.Size += child.Size
			continue
		}
		// Regular file or directory: recurse, then fold the child's size in.
		child := walkEntry(childPath, ci, werrs)
		e.Children = append(e.Children, child)
		e.Size += child.Size
	}
	return e
}
