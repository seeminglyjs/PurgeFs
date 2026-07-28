package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
	"github.com/seeminglyjs/PurgeFs/internal/history"
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

	err := runPurge(&buf, strings.NewReader("y\n"), root, f, false, false, engine.DefaultRules())
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

	err := runPurge(&buf, strings.NewReader("n\n"), root, f, false, false, engine.DefaultRules())
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
	err := runPurge(&buf, strings.NewReader(""), root, f, false, true, engine.DefaultRules())
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

	err := runPurge(&buf, strings.NewReader(""), root, f, false, true, engine.DefaultRules())
	if err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) != 0 {
		t.Errorf("clean dir should trash nothing: %v", f.got)
	}
}

func TestRunPurgeAbsolutizesRelativeRoot(t *testing.T) {
	root := junkDir(t)
	t.Chdir(root)
	f := &fakeTrasher{}
	var buf bytes.Buffer
	if err := runPurge(&buf, strings.NewReader("y\n"), ".", f, false, false, engine.DefaultRules()); err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	if len(f.got) == 0 {
		t.Fatal("nothing was sent to the trasher")
	}
	for _, p := range f.got {
		if !filepath.IsAbs(p) {
			t.Errorf("trasher got non-absolute path %q; undo would restore to the wrong cwd", p)
		}
	}
}

func TestGuardRootRefusesDangerous(t *testing.T) {
	if err := guardRoot("/"); err == nil {
		t.Error("guardRoot(/) must return an error")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		if err := guardRoot(home); err == nil {
			t.Errorf("guardRoot(home=%q) must return an error", home)
		}
	}
}

func TestGuardRootAllowsSubdir(t *testing.T) {
	if err := guardRoot(t.TempDir()); err != nil {
		t.Errorf("guardRoot(tempdir) should be allowed: %v", err)
	}
}

func TestGuardRootRefusesSymlinkToHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	link := filepath.Join(t.TempDir(), "link-to-home")
	if err := os.Symlink(home, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := guardRoot(link); err == nil {
		t.Error("guardRoot must refuse a symlink that resolves to the home directory")
	}
}

// 홈의 조상(예: /Users)을 지정하면 홈 전체가 대상이 되므로 거부해야 한다.
func TestGuardRootRefusesHomeAncestor(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	parent := filepath.Dir(home)
	if parent == home || parent == string(filepath.Separator) {
		t.Skip("home has no intermediate ancestor to test")
	}
	if err := guardRoot(parent); err == nil {
		t.Errorf("guardRoot(%q) must refuse an ancestor of the home directory", parent)
	}
}

func TestGuardRootRefusesSystemDirs(t *testing.T) {
	for _, p := range []string{"/usr", "/etc", "/var", "/bin"} {
		if _, err := os.Stat(p); err != nil {
			continue // 이 플랫폼에 없는 경로는 건너뛴다
		}
		if err := guardRoot(p); err == nil {
			t.Errorf("guardRoot(%q) must refuse a system directory", p)
		}
	}
}

// recordingTrasher 는 두 번째 호출 시점에 이미 저장돼 있던 매니페스트 항목을 붙잡아 둔다.
// 매니페스트가 전부 끝난 뒤가 아니라 항목마다 기록되는지 확인하기 위한 것이다.
type recordingTrasher struct {
	dir      string
	calls    int
	midItems []history.Item
}

func (r *recordingTrasher) Trash(paths []string) trash.Result {
	r.calls++
	if r.calls == 2 {
		m, _, _ := history.LoadLatest(r.dir)
		r.midItems = m.Items
	}
	var res trash.Result
	for _, p := range paths {
		res.Trashed = append(res.Trashed, p)
		res.Moved = append(res.Moved, trash.Moved{Original: p, Dest: p + ".trashed"})
	}
	return res
}

// 도중에 중단돼도 이미 옮긴 항목은 undo 할 수 있어야 한다: 항목마다 매니페스트를 갱신한다.
func TestTrashAndRecordWritesManifestIncrementally(t *testing.T) {
	dir := t.TempDir()
	tr := &recordingTrasher{dir: dir}

	res, err := trashAndRecord(tr, []string{"/a", "/b"}, dir, 42)
	if err != nil {
		t.Fatalf("trashAndRecord: %v", err)
	}
	if tr.calls != 2 {
		t.Errorf("trasher called %d times, want 1 per path (2)", tr.calls)
	}
	if len(tr.midItems) != 1 || tr.midItems[0].Original != "/a" {
		t.Errorf("manifest during the second move = %+v, want /a already recorded", tr.midItems)
	}
	if len(res.Moved) != 2 || len(res.Trashed) != 2 {
		t.Errorf("res = %+v, want 2 moved and 2 trashed", res)
	}
	m, ok, err := history.LoadLatest(dir)
	if err != nil || !ok || len(m.Items) != 2 {
		t.Errorf("final manifest = (%+v, %v, %v), want 2 items", m, ok, err)
	}
}

// failTrasher 는 모든 경로를 실패로 처리한다.
type failTrasher struct{}

func (failTrasher) Trash(paths []string) trash.Result {
	var f []trash.Failure
	for _, p := range paths {
		f = append(f, trash.Failure{Path: p, Err: errors.New("boom")})
	}
	return trash.Result{Failed: f}
}

func TestRunPurgeAllFailedReturnsError(t *testing.T) {
	root := junkDir(t)
	var buf bytes.Buffer
	err := runPurge(&buf, strings.NewReader(""), root, failTrasher{}, false, true, engine.DefaultRules())
	if err == nil {
		t.Error("expected a non-nil error when every target fails")
	}
	if !strings.Contains(buf.String(), "실패") {
		t.Errorf("output missing failure notice:\n%s", buf.String())
	}
}

func TestGroupsToItems(t *testing.T) {
	groups := []engine.CategoryGroup{
		{Category: "node_modules", Size: 1000, Count: 1, Paths: []string{"/p/node_modules"}},
		{Category: "os-junk", Size: 6, Count: 2, Paths: []string{"/p/.DS_Store", "/p/x/.DS_Store"}},
	}
	items := groupsToItems(groups)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].Label != "node_modules" || items[0].Size != 1000 || len(items[0].Paths) != 1 {
		t.Errorf("item[0] = %+v", items[0])
	}
	if items[1].Label != "os-junk" || len(items[1].Paths) != 2 {
		t.Errorf("item[1] = %+v", items[1])
	}
}

func TestRunPurgeWithPresetSkipsOsJunk(t *testing.T) {
	root := junkDir(t) // node_modules + .DS_Store
	f := &fakeTrasher{}
	var buf bytes.Buffer

	rules, _ := engine.Preset("dev-caches")
	if err := runPurge(&buf, strings.NewReader("y\n"), root, f, false, false, rules); err != nil {
		t.Fatalf("runPurge: %v", err)
	}
	for _, p := range f.got {
		if strings.HasSuffix(p, ".DS_Store") {
			t.Errorf("dev-caches preset must not purge .DS_Store, got %q", p)
		}
	}
}
