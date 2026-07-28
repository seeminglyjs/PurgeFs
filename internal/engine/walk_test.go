package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkAggregatesSizes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "abc") // 3 bytes
	mustMkdir(t, filepath.Join(root, "sub"))
	mustWrite(t, filepath.Join(root, "sub", "b.txt"), "hello") // 5 bytes

	e, werrs, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	if len(werrs) != 0 {
		t.Fatalf("unexpected walk errors: %v", werrs)
	}
	if !e.IsDir {
		t.Errorf("root should be a dir")
	}
	if e.Size != 8 {
		t.Errorf("root aggregated size = %d, want 8", e.Size)
	}
	if len(e.Children) != 2 {
		t.Errorf("root children = %d, want 2", len(e.Children))
	}
	sub := childByPath(e, filepath.Join(root, "sub"))
	if sub == nil || sub.Size != 5 {
		t.Errorf("sub size = %v, want 5", sub)
	}
}

func TestWalkSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real")
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(target, "big.txt"), "0123456789") // 10 bytes
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	e, _, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	linkEntry := childByPath(e, link)
	if linkEntry == nil {
		t.Fatal("link entry missing")
	}
	if linkEntry.IsDir {
		t.Error("symlink must not be treated as a directory")
	}
	if len(linkEntry.Children) != 0 {
		t.Error("symlink must not be descended into")
	}
}

func TestWalkCollectsPermissionErrors(t *testing.T) {
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	mustMkdir(t, locked)
	mustWrite(t, filepath.Join(locked, "secret.txt"), "xyz")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, werrs, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk should not fail on a per-dir permission error: %v", err)
	}
	if len(werrs) == 0 {
		t.Error("expected a WalkError for the unreadable directory")
	}
}
