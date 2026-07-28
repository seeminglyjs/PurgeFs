package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"os"
)

func TestRunScanReportsTotal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	if err := runScan(&buf, root); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "3 B") {
		t.Errorf("output missing total size %q:\n%s", "3 B", out)
	}
	if !strings.Contains(out, "across 1 file, 1 dir") {
		t.Errorf("output missing exact count phrase:\n%s", out)
	}
}

// TestRunScanFollowsSymlinkedRoot ensures runScan resolves a symlinked root
// before handing it to engine.Scan. engine.Walk deliberately does not follow
// symlinks (including a symlinked root), so on macOS, where common roots such
// as /tmp and the t.TempDir() base are themselves symlinks, scanning the raw
// path would see the root as a symlink leaf with no children. runScan must
// resolve the root with filepath.EvalSymlinks first.
func TestRunScanFollowsSymlinkedRoot(t *testing.T) {
	base := t.TempDir()

	realDir := filepath.Join(base, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "big.bin"), make([]byte, 5000), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	linkPath := filepath.Join(base, "link")
	if err := os.Symlink(realDir, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var buf bytes.Buffer
	if err := runScan(&buf, linkPath); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "4.9 KB") {
		t.Errorf("output missing resolved child size %q (symlinked root not followed?):\n%s", "4.9 KB", out)
	}
}
