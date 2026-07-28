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

// TestRunScanFollowsSymlinkedRoot 은 심볼릭링크인 root 가 순회 전에 resolve 되는지
// 확인한다. engine.Walk 는 심볼릭링크를(심볼릭링크인 root 까지) 일부러 따라가지 않으므로,
// /tmp 나 t.TempDir() 베이스처럼 흔한 root 가 심볼릭링크인 macOS 에서는 원본 경로를 그대로
// 스캔하면 root 가 자식 없는 심볼릭링크 leaf 로 보인다. engine.Scan 이 명명된 root 를 먼저
// filepath.EvalSymlinks 로 resolve 하고, runScan 은 그에 의존한다.
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
