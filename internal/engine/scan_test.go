package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanCounts(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "abc")          // 파일
	mustMkdir(t, filepath.Join(root, "sub"))                   // 디렉토리
	mustWrite(t, filepath.Join(root, "sub", "b.txt"), "hello") // 파일

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
	if r.DirCount != 2 { // root + sub 둘
		t.Errorf("DirCount = %d, want 2", r.DirCount)
	}
}

func TestScanResolvesSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	mustMkdir(t, real)
	mustWrite(t, filepath.Join(real, "f.txt"), "0123456789") // 10 바이트
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	r, _, err := Scan(link)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.TotalSize != 10 {
		t.Errorf("TotalSize = %d, want 10 (symlinked root must be resolved)", r.TotalSize)
	}
	if r.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", r.FileCount)
	}
}
