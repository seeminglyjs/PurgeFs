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

// undo 를 두 번 하면 두 번째는 그 이전 purge 를 되돌려야 한다. 매니페스트를 소비하지 않으면
// 최신 것이 영원히 최신으로 남아 이전 기록에 도달할 수 없다.
func TestRunUndoTwiceReachesPreviousPurge(t *testing.T) {
	base := t.TempDir()
	histDir := filepath.Join(base, "history")

	// 두 번의 purge 를 흉내낸다: 각각 휴지통 파일 하나씩.
	seed := func(createdAt int64, name string) string {
		dest := filepath.Join(base, "trash-"+name)
		if err := os.WriteFile(dest, []byte(name), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
		original := filepath.Join(base, name)
		if _, err := history.Save(histDir, history.Manifest{
			CreatedAt: createdAt,
			Items:     []history.Item{{Original: original, Dest: dest}},
		}); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
		return original
	}
	older := seed(100, "older")
	newer := seed(200, "newer")

	var buf bytes.Buffer
	if err := runUndoDir(&buf, histDir); err != nil {
		t.Fatalf("first undo: %v", err)
	}
	if _, err := os.Lstat(newer); err != nil {
		t.Fatalf("first undo did not restore the newest purge: %v", err)
	}
	if _, err := os.Lstat(older); err == nil {
		t.Fatal("first undo restored the older purge too")
	}

	if err := runUndoDir(&buf, histDir); err != nil {
		t.Fatalf("second undo: %v", err)
	}
	if _, err := os.Lstat(older); err != nil {
		t.Errorf("second undo did not reach the previous purge: %v", err)
	}
}

// 복원에 실패한 항목이 있으면 매니페스트를 소비하지 않아, 원인을 고친 뒤 다시 시도할 수 있다.
func TestRunUndoKeepsManifestWhenRestoreFails(t *testing.T) {
	base := t.TempDir()
	histDir := filepath.Join(base, "history")
	dest := filepath.Join(base, "trash-nm")
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// 원본의 부모 디렉토리가 없어 rename 이 실패한다.
	original := filepath.Join(base, "gone-parent", "nm")
	if _, err := history.Save(histDir, history.Manifest{
		CreatedAt: 5, Items: []history.Item{{Original: original, Dest: dest}},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	var buf bytes.Buffer
	if err := runUndoDir(&buf, histDir); err != nil {
		t.Fatalf("runUndoDir: %v", err)
	}
	if _, ok, _ := history.LoadLatest(histDir); !ok {
		t.Error("manifest was consumed despite a failed restore; the retry is now impossible")
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
