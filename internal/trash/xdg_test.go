package trash

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newXDGTrasherAt 는 테스트용으로 지정한 디렉토리를 휴지통 루트로 쓰는 xdgTrasher 를 만든다.
func newXDGTrasherAt(t *testing.T, root string) *xdgTrasher {
	t.Helper()
	tr, err := newXDGTrasherIn(root)
	if err != nil {
		t.Fatalf("newXDGTrasherIn: %v", err)
	}
	return tr
}

func TestXDGTrasherMovesToFilesAndWritesInfo(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "junk.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	root := filepath.Join(base, "Trash")
	tr := newXDGTrasherAt(t, root)

	res := tr.Trash([]string{src})

	if len(res.Trashed) != 1 || len(res.Failed) != 0 {
		t.Fatalf("Result = %+v, want 1 trashed 0 failed", res)
	}
	dest := filepath.Join(root, "files", "junk.txt")
	if _, err := os.Lstat(dest); err != nil {
		t.Errorf("not moved into Trash/files: %v", err)
	}
	if len(res.Moved) != 1 || res.Moved[0].Dest != dest {
		t.Fatalf("Moved = %+v, want dest %q", res.Moved, dest)
	}

	info := filepath.Join(root, "info", "junk.txt.trashinfo")
	if res.Moved[0].Sidecar != info {
		t.Errorf("Moved[0].Sidecar = %q, want %q", res.Moved[0].Sidecar, info)
	}
	data, err := os.ReadFile(info)
	if err != nil {
		t.Fatalf("read trashinfo: %v", err)
	}
	body := string(data)
	if !strings.HasPrefix(body, "[Trash Info]\n") {
		t.Errorf("trashinfo must start with the [Trash Info] header:\n%s", body)
	}
	if !strings.Contains(body, "Path="+src+"\n") {
		t.Errorf("trashinfo missing Path=%s:\n%s", src, body)
	}
	if !strings.Contains(body, "DeletionDate=") {
		t.Errorf("trashinfo missing DeletionDate:\n%s", body)
	}
}

// 경로에 공백·특수문자가 있으면 Path 는 퍼센트 인코딩된다(디렉토리 구분자 / 는 유지).
func TestXDGTrasherEncodesPathInInfo(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "my project")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := filepath.Join(dir, "junk.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	root := filepath.Join(base, "Trash")
	res := newXDGTrasherAt(t, root).Trash([]string{src})
	if len(res.Moved) != 1 {
		t.Fatalf("Moved = %+v, want 1 entry", res.Moved)
	}

	data, err := os.ReadFile(res.Moved[0].Sidecar)
	if err != nil {
		t.Fatalf("read trashinfo: %v", err)
	}
	if !strings.Contains(string(data), "my%20project/junk.txt") {
		t.Errorf("Path must be percent-encoded:\n%s", data)
	}
}

// 이름이 충돌하면 files 와 info 의 이름이 계속 짝을 이뤄야 한다.
func TestXDGTrasherKeepsInfoInSyncOnCollision(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Trash")
	tr := newXDGTrasherAt(t, root)
	if err := os.WriteFile(filepath.Join(root, "files", "dup"), []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	src := filepath.Join(base, "dup")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	res := tr.Trash([]string{src})

	if len(res.Moved) != 1 {
		t.Fatalf("Moved = %+v, want 1 entry", res.Moved)
	}
	wantDest := filepath.Join(root, "files", "dup 2")
	if res.Moved[0].Dest != wantDest {
		t.Errorf("Dest = %q, want %q", res.Moved[0].Dest, wantDest)
	}
	wantInfo := filepath.Join(root, "info", "dup 2.trashinfo")
	if res.Moved[0].Sidecar != wantInfo {
		t.Errorf("Sidecar = %q, want %q", res.Moved[0].Sidecar, wantInfo)
	}
	if _, err := os.Lstat(wantInfo); err != nil {
		t.Errorf("info file not written for the renamed entry: %v", err)
	}
}

// 옮기지 못하면 이미 쓴 info 파일을 남기지 않는다(짝 없는 info 는 규격 위반).
func TestXDGTrasherCleansInfoWhenMoveFails(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Trash")
	tr := newXDGTrasherAt(t, root)

	res := tr.Trash([]string{filepath.Join(base, "does-not-exist")})

	if len(res.Failed) != 1 || len(res.Trashed) != 0 {
		t.Fatalf("Result = %+v, want 0 trashed 1 failed", res)
	}
	entries, err := os.ReadDir(filepath.Join(root, "info"))
	if err != nil {
		t.Fatalf("ReadDir info: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("info dir = %v, want empty after a failed move", entries)
	}
}

func TestNewTrasherPicksPlatformImplementation(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	tr, err := NewTrasher()
	if err != nil {
		t.Fatalf("NewTrasher: %v", err)
	}
	switch runtime.GOOS {
	case "darwin":
		if _, ok := tr.(*macTrasher); !ok {
			t.Errorf("on darwin NewTrasher = %T, want *macTrasher", tr)
		}
	default:
		if _, ok := tr.(*xdgTrasher); !ok {
			t.Errorf("on %s NewTrasher = %T, want *xdgTrasher", runtime.GOOS, tr)
		}
	}
}

// XDG_DATA_HOME 이 설정돼 있으면 그 아래 Trash 를 쓴다.
func TestNewXDGTrasherHonorsXDGDataHome(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	tr, err := NewXDGTrasher()
	if err != nil {
		t.Fatalf("NewXDGTrasher: %v", err)
	}
	x, ok := tr.(*xdgTrasher)
	if !ok {
		t.Fatalf("NewXDGTrasher = %T, want *xdgTrasher", tr)
	}
	if want := filepath.Join(dataHome, "Trash", "files"); x.filesDir != want {
		t.Errorf("filesDir = %q, want %q", x.filesDir, want)
	}
}
