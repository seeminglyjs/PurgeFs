package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkAggregatesSizes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "abc") // 3 바이트
	mustMkdir(t, filepath.Join(root, "sub"))
	mustWrite(t, filepath.Join(root, "sub", "b.txt"), "hello") // 5 바이트

	r, werrs, err := Scan(root, DefaultRules())
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(werrs) != 0 {
		t.Fatalf("unexpected walk errors: %v", werrs)
	}
	if r.TotalSize != 8 {
		t.Errorf("root aggregated size = %d, want 8", r.TotalSize)
	}
	if len(r.Children) != 2 {
		t.Errorf("root children = %d, want 2", len(r.Children))
	}
	sub := childByPath(r, filepath.Join(resolved(t, root), "sub"))
	if sub == nil || sub.Size != 5 {
		t.Errorf("sub size = %v, want 5", sub)
	}
}

func TestWalkSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real")
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(target, "big.txt"), "0123456789") // 10 바이트
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	r, _, err := Scan(root, DefaultRules())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	linkChild := childByPath(r, filepath.Join(resolved(t, root), "link"))
	if linkChild == nil {
		t.Fatal("link entry missing")
	}
	// 링크 자체의 크기(대상 경로 문자열 길이)만 잡혀야 한다. 따라갔다면 대상의 10 바이트가
	// 링크 쪽에도 더해져 중복 집계된다.
	li, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if linkChild.Size != li.Size() {
		t.Errorf("symlink size = %d, want its own size %d (not the target's contents)", linkChild.Size, li.Size())
	}
	if want := 10 + li.Size(); r.TotalSize != want {
		t.Errorf("TotalSize = %d, want %d — the symlinked subtree is being counted twice", r.TotalSize, want)
	}
	if r.FileCount != 2 { // big.txt + 링크
		t.Errorf("FileCount = %d, want 2 (the symlink counts as one leaf)", r.FileCount)
	}
}

func TestWalkCollectsPermissionErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 0o000 does not restrict root; test is meaningless as root")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	mustMkdir(t, locked)
	mustWrite(t, filepath.Join(locked, "secret.txt"), "xyz")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, werrs, err := Scan(root, DefaultRules())
	if err != nil {
		t.Fatalf("Scan should not fail on a per-dir permission error: %v", err)
	}
	if len(werrs) == 0 {
		t.Error("expected a WalkError for the unreadable directory")
	}
}

// 못 읽는 root 는 순회를 중단시키지 않고 빈 결과가 된다(에러는 werrs 로).
func TestWalkUnreadableRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 0o000 does not restrict root")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	r, werrs, err := Scan(root, DefaultRules())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(werrs) != 1 || r.TotalSize != 0 {
		t.Errorf("werrs = %v, TotalSize = %d, want 1 error and 0 bytes", werrs, r.TotalSize)
	}
}

// root 가 디렉토리가 아니라 파일이어도 스캔된다.
func TestWalkFileRoot(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "a.txt")
	mustWrite(t, f, "abc")

	r, _, err := Scan(f, DefaultRules())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if r.TotalSize != 3 || r.FileCount != 1 || r.DirCount != 0 {
		t.Errorf("Report = %+v, want 3 bytes / 1 file / 0 dirs", r)
	}
}

func TestScanMissingRootErrors(t *testing.T) {
	_, _, err := Scan(filepath.Join(t.TempDir(), "nope"), DefaultRules())
	if err == nil {
		t.Error("scanning a missing path must return an error")
	}
}
