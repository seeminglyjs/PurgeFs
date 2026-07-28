package engine

import (
	"path/filepath"
	"testing"
)

func TestScanCounts(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "abc")          // file
	mustMkdir(t, filepath.Join(root, "sub"))                   // dir
	mustWrite(t, filepath.Join(root, "sub", "b.txt"), "hello") // file

	r, werrs, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(werrs) != 0 {
		t.Fatalf("unexpected walk errors: %v", werrs)
	}
	if r.TotalSize != 8 {
		t.Errorf("TotalSize = %d, want 8", r.TotalSize)
	}
	if r.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", r.FileCount)
	}
	if r.DirCount != 2 { // root + sub
		t.Errorf("DirCount = %d, want 2", r.DirCount)
	}
}
