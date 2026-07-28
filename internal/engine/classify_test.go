package engine

import (
	"path/filepath"
	"testing"
)

// sampleTree 는 다음 트리를 실제로 만든다:
//
//	node_modules/lib/.DS_Store   (안쪽 .DS_Store 는 따로 세면 안 됨)
//	src/__pycache__/x.pyc
//	.DS_Store
func sampleTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "node_modules", "lib"))
	mustWrite(t, filepath.Join(root, "node_modules", "lib", "big.bin"), "0123456789") // 10
	mustWrite(t, filepath.Join(root, "node_modules", "lib", ".DS_Store"), "xxxxxx")   // 6
	mustMkdir(t, filepath.Join(root, "src", "__pycache__"))
	mustWrite(t, filepath.Join(root, "src", "__pycache__", "x.pyc"), "abcd") // 4
	mustWrite(t, filepath.Join(root, ".DS_Store"), "xxxxxx")                 // 6
	return root
}

func TestClassifyGroupsAndSkipsChildren(t *testing.T) {
	r, _, err := Scan(sampleTree(t), DefaultRules())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if g := groupByCategory(r, "node_modules"); g.Size != 16 || g.Count != 1 {
		t.Errorf("node_modules = size %d count %d, want 16/1", g.Size, g.Count)
	}
	if g := groupByCategory(r, "python-cache"); g.Size != 4 || g.Count != 1 {
		t.Errorf("python-cache = size %d count %d, want 4/1", g.Size, g.Count)
	}
	// node_modules 안의 .DS_Store 는 따로 세면 안 됨(하위 skip); 최상위 것만 셈.
	if g := groupByCategory(r, "os-junk"); g.Size != 6 || g.Count != 1 {
		t.Errorf("os-junk = size %d count %d, want 6/1 (inner .DS_Store skipped)", g.Size, g.Count)
	}
}

func TestClassifySortedBySizeDesc(t *testing.T) {
	r, _, err := Scan(sampleTree(t), DefaultRules())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(r.Groups) < 2 {
		t.Fatalf("expected >=2 groups, got %d", len(r.Groups))
	}
	for i := 1; i < len(r.Groups); i++ {
		if r.Groups[i-1].Size < r.Groups[i].Size {
			t.Errorf("groups not sorted desc: %d before %d", r.Groups[i-1].Size, r.Groups[i].Size)
		}
	}
}

func TestClassifyEmptyWhenNoJunk(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "main.go"), "package main")

	r, _, err := Scan(root, DefaultRules())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(r.Groups) != 0 {
		t.Errorf("clean tree should yield no groups, got %d", len(r.Groups))
	}
}

// projectTree 는 root 바로 아래에 marker 파일과 dirName 디렉토리(안에 100바이트)를 만든다.
// marker 가 "" 면 마커 없이 디렉토리만 둔다.
func projectTree(t *testing.T, marker, dirName string) string {
	t.Helper()
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, dirName))
	mustWrite(t, filepath.Join(root, dirName, "blob"), string(make([]byte, 100)))
	if marker != "" {
		mustWrite(t, filepath.Join(root, marker), "{}")
	}
	return root
}

// build/dist/target 은 이름만으로 빌드 산출물인지 알 수 없다. 형제에 프로젝트 마커가 있을 때만
// 매치해야, 소스로 관리되는 build/ 나 커밋된 dist/ 를 지우지 않는다.
func TestClassifyRequiresProjectMarker(t *testing.T) {
	cases := []struct {
		dirName string
		marker  string
	}{
		{"dist", "package.json"},
		{"target", "Cargo.toml"},
		{"target", "pom.xml"},
		{"build", "build.gradle"},
		{"build", "CMakeLists.txt"},
	}
	for _, c := range cases {
		withMarker, _, err := Scan(projectTree(t, c.marker, c.dirName), DefaultRules())
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(withMarker.Groups) != 1 {
			t.Errorf("%s/ next to %s should match, got %d groups", c.dirName, c.marker, len(withMarker.Groups))
		}
		bare, _, err := Scan(projectTree(t, "", c.dirName), DefaultRules())
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(bare.Groups) != 0 {
			t.Errorf("%s/ with no project marker must not match, got %+v", c.dirName, bare.Groups)
		}
	}
}

// 이름만으로 명확한 규칙은 마커를 요구하지 않는다.
func TestClassifyUnambiguousDirsNeedNoMarker(t *testing.T) {
	for _, name := range []string{"node_modules", "__pycache__", ".gradle"} {
		r, _, err := Scan(projectTree(t, "", name), DefaultRules())
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if len(r.Groups) != 1 {
			t.Errorf("%s/ should match without a marker, got %d groups", name, len(r.Groups))
		}
	}
}

// 스캔 root 자체가 규칙에 걸려도 root 는 형제가 없어 마커 규칙은 매치하지 않는다.
func TestClassifyRootItselfIsNotAJunkTarget(t *testing.T) {
	root := t.TempDir()
	nm := filepath.Join(root, "node_modules")
	mustMkdir(t, nm)
	mustWrite(t, filepath.Join(nm, "f"), "x")

	r, _, err := Scan(nm, DefaultRules())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, g := range r.Groups {
		for _, p := range g.Paths {
			if p == nm {
				t.Errorf("the scan root itself must not be listed as a purge target: %q", p)
			}
		}
	}
}
