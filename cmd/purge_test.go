package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seeminglyjs/PurgeFs/internal/trash"
)

// fakeTrasher 는 실제로 지우지 않고 받은 경로만 기록한다.
type fakeTrasher struct {
	got []string
}

func (f *fakeTrasher) Trash(paths []string) trash.Result {
	f.got = append(f.got, paths...)
	return trash.Result{Trashed: paths}
}

// junkDir 는 node_modules(파일 포함)와 .DS_Store 가 있는 임시 디렉토리를 만든다.
func junkDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	nm := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nm, "big.bin"), make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return root
}

func TestRunPurgeConfirmedTrashes(t *testing.T) {
	root := junkDir(t)
	f := &fakeTrasher{}
	var buf bytes.Buffer

	err := runPurge(&buf, strings.NewReader("y\n"), root, f, false, false)
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) == 0 {
		t.Error("nothing was sent to the trasher after confirmation")
	}
	if !strings.Contains(buf.String(), "node_modules") {
		t.Errorf("summary missing node_modules:\n%s", buf.String())
	}
}

func TestRunPurgeCancelledDoesNothing(t *testing.T) {
	root := junkDir(t)
	f := &fakeTrasher{}
	var buf bytes.Buffer

	err := runPurge(&buf, strings.NewReader("n\n"), root, f, false, false)
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) != 0 {
		t.Errorf("trasher was called despite cancel: %v", f.got)
	}
	if !strings.Contains(buf.String(), "취소") {
		t.Errorf("output missing cancel notice:\n%s", buf.String())
	}
}

func TestRunPurgeAssumeYesSkipsPrompt(t *testing.T) {
	root := junkDir(t)
	f := &fakeTrasher{}
	var buf bytes.Buffer

	// 입력이 비어도 --yes 면 진행해야 한다.
	err := runPurge(&buf, strings.NewReader(""), root, f, false, true)
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) == 0 {
		t.Error("--yes should proceed without input")
	}
}

func TestRunPurgeNoJunk(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f := &fakeTrasher{}
	var buf bytes.Buffer

	err := runPurge(&buf, strings.NewReader(""), root, f, false, true)
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) != 0 {
		t.Errorf("clean dir should trash nothing: %v", f.got)
	}
}
