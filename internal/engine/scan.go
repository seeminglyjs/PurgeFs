package engine

// Scan walks root and produces a Report with aggregated size and node counts.
func Scan(root string) (*Report, []WalkError, error) {
	rootEntry, werrs, err := Walk(root)
	if err != nil {
		return nil, werrs, err
	}
	r := &Report{
		Root:      rootEntry,
		TotalSize: rootEntry.Size,
	}
	countEntries(rootEntry, r)
	return r, werrs, nil
}

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
