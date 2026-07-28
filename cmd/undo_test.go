package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seeminglyjs/PurgeFs/internal/history"
	"github.com/seeminglyjs/PurgeFs/internal/trash"
)

func TestRecordHistoryWritesManifest(t *testing.T) {
	dir := t.TempDir()
	res := trash.Result{Moved: []trash.Moved{{Original: "/p/node_modules", Dest: "/t/node_modules"}}}
	if err := recordHistory(dir, res, 1234); err != nil {
		t.Fatalf("recordHistory: %v", err)
	}
	m, ok, err := history.LoadLatest(dir)
	if err != nil || !ok {
		t.Fatalf("LoadLatest = (%+v, %v, %v)", m, ok, err)
	}
	if len(m.Items) != 1 || m.Items[0].Original != "/p/node_modules" {
		t.Errorf("manifest = %+v", m)
	}
}

func TestRecordHistoryNoMovedIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := recordHistory(dir, trash.Result{Trashed: []string{"/x"}}, 1); err != nil {
		t.Fatalf("recordHistory: %v", err)
	}
	if _, ok, _ := history.LoadLatest(dir); ok {
		t.Error("no Moved should write no manifest")
	}
}

func TestRunUndoRestores(t *testing.T) {
	base := t.TempDir()
	histDir := filepath.Join(base, "history")
	dest := filepath.Join(base, "trash-nm")
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	original := filepath.Join(base, "nm")
	if _, err := history.Save(histDir, history.Manifest{CreatedAt: 5, Items: []history.Item{{Original: original, Dest: dest}}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	var buf bytes.Buffer
	if err := runUndoDir(&buf, histDir); err != nil {
		t.Fatalf("runUndoDir: %v", err)
	}
	if _, err := os.Lstat(original); err != nil {
		t.Errorf("original not restored: %v", err)
	}
	if !strings.Contains(buf.String(), "복원") {
		t.Errorf("output missing restore notice:\n%s", buf.String())
	}
}

func TestRunUndoNoHistory(t *testing.T) {
	var buf bytes.Buffer
	if err := runUndoDir(&buf, filepath.Join(t.TempDir(), "empty")); err != nil {
		t.Fatalf("runUndoDir: %v", err)
	}
	if !strings.Contains(buf.String(), "복원할 기록이 없습니다") {
		t.Errorf("output = %q", buf.String())
	}
}
