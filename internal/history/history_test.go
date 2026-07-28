package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadLatestPicksNewest(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, Manifest{CreatedAt: 100, Items: []Item{{Original: "/a", Dest: "/t/a"}}}); err != nil {
		t.Fatalf("save 1: %v", err)
	}
	if _, err := Save(dir, Manifest{CreatedAt: 200, Items: []Item{{Original: "/b", Dest: "/t/b"}}}); err != nil {
		t.Fatalf("save 2: %v", err)
	}
	m, ok, err := LoadLatest(dir)
	if err != nil || !ok {
		t.Fatalf("LoadLatest = (%+v, %v, %v)", m, ok, err)
	}
	if m.CreatedAt != 200 || len(m.Items) != 1 || m.Items[0].Original != "/b" {
		t.Errorf("latest = %+v, want CreatedAt 200 with /b", m)
	}
}

func TestLoadLatestEmptyDir(t *testing.T) {
	_, ok, err := LoadLatest(t.TempDir())
	if err != nil || ok {
		t.Errorf("empty dir: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

func TestLoadLatestMissingDir(t *testing.T) {
	_, ok, err := LoadLatest(filepath.Join(t.TempDir(), "nope"))
	if err != nil || ok {
		t.Errorf("missing dir: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

func TestRestoreMovesBackAndSkips(t *testing.T) {
	base := t.TempDir()
	// 휴지통에 있는 파일(Dest 존재), 원본 자리는 비어 있음 → 복원되어야 함
	dest := filepath.Join(base, "trash-node_modules")
	if err := os.WriteFile(dest, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}
	original := filepath.Join(base, "node_modules")

	// 원본 자리에 이미 뭔가 있는 두 번째 항목 → 건너뛰어야 함
	occupied := filepath.Join(base, "occupied")
	if err := os.WriteFile(occupied, []byte("keep"), 0o644); err != nil {
		t.Fatalf("seed occupied: %v", err)
	}

	m := Manifest{Items: []Item{
		{Original: original, Dest: dest},
		{Original: occupied, Dest: filepath.Join(base, "missing-dest")},
	}}
	res := Restore(m)

	if len(res.Restored) != 1 || res.Restored[0] != original {
		t.Errorf("Restored = %v, want [%s]", res.Restored, original)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped = %v, want 1", res.Skipped)
	}
	if _, err := os.Lstat(original); err != nil {
		t.Errorf("original not restored: %v", err)
	}
}

// 원본 자리에 이미 파일이 있고 Dest 도 있으면, 덮어쓰지 않고 건너뛰며 둘 다 보존한다.
func TestRestoreDoesNotOverwriteExistingOriginal(t *testing.T) {
	base := t.TempDir()
	original := filepath.Join(base, "keep")
	if err := os.WriteFile(original, []byte("current"), 0o644); err != nil {
		t.Fatalf("seed original: %v", err)
	}
	dest := filepath.Join(base, "trash-keep")
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}
	res := Restore(Manifest{Items: []Item{{Original: original, Dest: dest}}})
	if len(res.Restored) != 0 || len(res.Skipped) != 1 {
		t.Fatalf("res = %+v, want 0 restored 1 skipped", res)
	}
	if data, _ := os.ReadFile(original); string(data) != "current" {
		t.Errorf("original overwritten: %q, want \"current\"", data)
	}
	if _, err := os.Lstat(dest); err != nil {
		t.Errorf("dest should be untouched: %v", err)
	}
}

// 원본은 비어 있지만 Dest(휴지통 파일)가 없으면 건너뛴다(두 번째 skip 분기 단독 검증).
func TestRestoreSkipsWhenDestMissing(t *testing.T) {
	base := t.TempDir()
	m := Manifest{Items: []Item{
		{Original: filepath.Join(base, "gone"), Dest: filepath.Join(base, "no-such-dest")},
	}}
	res := Restore(m)
	if len(res.Restored) != 0 {
		t.Errorf("Restored = %v, want none", res.Restored)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("Skipped = %v, want 1", res.Skipped)
	}
	if len(res.Failed) != 0 {
		t.Errorf("Failed = %v, want none", res.Failed)
	}
}
