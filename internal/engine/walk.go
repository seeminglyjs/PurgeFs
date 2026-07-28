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

func walkEntry(path string, info os.FileInfo, werrs *[]WalkError) *Entry {
	e := &Entry{
		Path:    path,
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Unix(),
	}
	if !info.IsDir() {
		e.Size = info.Size()
		return e
	}

	dirents, err := os.ReadDir(path)
	if err != nil {
		*werrs = append(*werrs, WalkError{Path: path, Err: err})
		return e
	}

	for _, de := range dirents {
		childPath := filepath.Join(path, de.Name())
		ci, err := de.Info() // lstat-based: does not follow symlinks
		if err != nil {
			*werrs = append(*werrs, WalkError{Path: childPath, Err: err})
			continue
		}
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
		child := walkEntry(childPath, ci, werrs)
		e.Children = append(e.Children, child)
		e.Size += child.Size
	}
	return e
}
