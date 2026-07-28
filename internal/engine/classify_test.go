package engine

import "testing"

// 트리:
// /p
//
//	node_modules (dir, size 1000; 안에 lib 와 .DS_Store 있지만 단일 단위로 셈)
//	src (dir, size 200)
//	  __pycache__ (dir, size 200)
//	.DS_Store (file, size 6)
func sampleTree() *Report {
	root := &Entry{Path: "/p", IsDir: true, Size: 1206, Children: []*Entry{
		{Path: "/p/node_modules", IsDir: true, Size: 1000, Children: []*Entry{
			{Path: "/p/node_modules/lib", IsDir: true, Size: 1000, Children: []*Entry{
				{Path: "/p/node_modules/lib/.DS_Store", IsDir: false, Size: 6},
			}},
		}},
		{Path: "/p/src", IsDir: true, Size: 200, Children: []*Entry{
			{Path: "/p/src/__pycache__", IsDir: true, Size: 200},
		}},
		{Path: "/p/.DS_Store", IsDir: false, Size: 6},
	}}
	return &Report{Root: root, TotalSize: 1206}
}

func TestClassifyGroupsAndSkipsChildren(t *testing.T) {
	groups := Classify(sampleTree(), DefaultRules())

	byCat := map[string]CategoryGroup{}
	for _, g := range groups {
		byCat[g.Category] = g
	}

	if g := byCat["node_modules"]; g.Size != 1000 || g.Count != 1 {
		t.Errorf("node_modules = size %d count %d, want 1000/1", g.Size, g.Count)
	}
	if g := byCat["python-cache"]; g.Size != 200 || g.Count != 1 {
		t.Errorf("python-cache = size %d count %d, want 200/1", g.Size, g.Count)
	}
	// node_modules 안의 .DS_Store 는 따로 세면 안 됨(하위 skip); 최상위 것만 셈.
	if g := byCat["os-junk"]; g.Size != 6 || g.Count != 1 {
		t.Errorf("os-junk = size %d count %d, want 6/1 (inner .DS_Store skipped)", g.Size, g.Count)
	}
}

func TestClassifySortedBySizeDesc(t *testing.T) {
	groups := Classify(sampleTree(), DefaultRules())
	if len(groups) < 2 {
		t.Fatalf("expected >=2 groups, got %d", len(groups))
	}
	for i := 1; i < len(groups); i++ {
		if groups[i-1].Size < groups[i].Size {
			t.Errorf("groups not sorted desc: %d before %d", groups[i-1].Size, groups[i].Size)
		}
	}
}

// projectTree 는 root 바로 아래에 marker 파일과 dirName 디렉토리가 있는 트리를 만든다.
// marker 가 "" 면 마커 없이 디렉토리만 둔다.
func projectTree(marker, dirName string) *Report {
	root := &Entry{Path: "/p", IsDir: true, Children: []*Entry{
		{Path: "/p/" + dirName, IsDir: true, Size: 100},
	}}
	if marker != "" {
		root.Children = append(root.Children, &Entry{Path: "/p/" + marker, IsDir: false, Size: 10})
	}
	root.Size = 100 + int64(len(root.Children)-1)*10
	return &Report{Root: root, TotalSize: root.Size}
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
		if groups := Classify(projectTree(c.marker, c.dirName), DefaultRules()); len(groups) != 1 {
			t.Errorf("%s/ next to %s should match, got %d groups", c.dirName, c.marker, len(groups))
		}
		if groups := Classify(projectTree("", c.dirName), DefaultRules()); len(groups) != 0 {
			t.Errorf("%s/ with no project marker must not match, got %+v", c.dirName, groups)
		}
	}
}

// 이름만으로 명확한 규칙은 마커를 요구하지 않는다.
func TestClassifyUnambiguousDirsNeedNoMarker(t *testing.T) {
	for _, name := range []string{"node_modules", "__pycache__", ".gradle"} {
		if groups := Classify(projectTree("", name), DefaultRules()); len(groups) != 1 {
			t.Errorf("%s/ should match without a marker, got %d groups", name, len(groups))
		}
	}
}

func TestClassifyEmptyWhenNoJunk(t *testing.T) {
	root := &Entry{Path: "/p", IsDir: true, Size: 3, Children: []*Entry{
		{Path: "/p/main.go", IsDir: false, Size: 3},
	}}
	groups := Classify(&Report{Root: root, TotalSize: 3}, DefaultRules())
	if len(groups) != 0 {
		t.Errorf("clean tree should yield no groups, got %d", len(groups))
	}
}
