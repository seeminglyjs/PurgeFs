package trash

import (
	"os"
	"path/filepath"
	"testing"
)

// newMacTrasherAt 는 테스트용으로 지정한 디렉토리를 휴지통으로 쓰는 macTrasher 를 만든다.
func newMacTrasherAt(t *testing.T, dir string) *macTrasher {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir trash: %v", err)
	}
	return &macTrasher{trashDir: dir}
}

func TestMacTrasherMovesToTrash(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "junk.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	trashDir := filepath.Join(base, "Trash")
	tr := newMacTrasherAt(t, trashDir)

	res := tr.Trash([]string{src})

	if len(res.Trashed) != 1 || len(res.Failed) != 0 {
		t.Fatalf("Result = %+v, want 1 trashed 0 failed", res)
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Errorf("source still exists: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(trashDir, "junk.txt")); err != nil {
		t.Errorf("not moved into trash: %v", err)
	}
}

func TestMacTrasherRenamesOnCollision(t *testing.T) {
	base := t.TempDir()
	trashDir := filepath.Join(base, "Trash")
	tr := newMacTrasherAt(t, trashDir)
	// 휴지통에 이미 "dup" 이 있음
	if err := os.WriteFile(filepath.Join(trashDir, "dup"), []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := filepath.Join(base, "dup")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	res := tr.Trash([]string{src})

	if len(res.Trashed) != 1 {
		t.Fatalf("Result = %+v, want 1 trashed", res)
	}
	if _, err := os.Lstat(filepath.Join(trashDir, "dup 2")); err != nil {
		t.Errorf("collision not renamed to \"dup 2\": %v", err)
	}
}

func TestMacTrasherRecordsFailure(t *testing.T) {
	base := t.TempDir()
	trashDir := filepath.Join(base, "Trash")
	tr := newMacTrasherAt(t, trashDir)

	res := tr.Trash([]string{filepath.Join(base, "does-not-exist")})

	if len(res.Trashed) != 0 || len(res.Failed) != 1 {
		t.Fatalf("Result = %+v, want 0 trashed 1 failed", res)
	}
}

func TestHardDeleterRemoves(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "cache")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "f"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	res := NewHardDeleter().Trash([]string{dir})

	if len(res.Trashed) != 1 || len(res.Failed) != 0 {
		t.Fatalf("Result = %+v, want 1 trashed", res)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("dir still exists: %v", err)
	}
}
