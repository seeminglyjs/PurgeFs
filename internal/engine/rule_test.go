package engine

import "testing"

// noSiblings 는 형제를 보지 않는 규칙을 테스트할 때 쓰는 빈 형제 집합이다.
var noSiblings map[string]bool

func TestDirNameRuleMatchesDir(t *testing.T) {
	r := dirNameRule{name: "node_modules", category: "node_modules"}
	m, cat, skip := r.Match(&Entry{Path: "/x/node_modules", IsDir: true}, noSiblings)
	if !m || cat != "node_modules" || !skip {
		t.Fatalf("Match = (%v, %q, %v), want (true, \"node_modules\", true)", m, cat, skip)
	}
}

func TestDirNameRuleIgnoresFileAndOtherNames(t *testing.T) {
	r := dirNameRule{name: "node_modules", category: "node_modules"}
	if m, _, _ := r.Match(&Entry{Path: "/x/node_modules", IsDir: false}, noSiblings); m {
		t.Error("must not match a file named node_modules")
	}
	if m, _, _ := r.Match(&Entry{Path: "/x/src", IsDir: true}, noSiblings); m {
		t.Error("must not match an unrelated directory")
	}
}

func TestFileNameRuleMatchesFile(t *testing.T) {
	r := fileNameRule{name: ".DS_Store", category: "os-junk"}
	m, cat, skip := r.Match(&Entry{Path: "/x/.DS_Store", IsDir: false}, noSiblings)
	if !m || cat != "os-junk" || skip {
		t.Fatalf("Match = (%v, %q, %v), want (true, \"os-junk\", false)", m, cat, skip)
	}
	if m, _, _ := r.Match(&Entry{Path: "/x/.DS_Store", IsDir: true}, noSiblings); m {
		t.Error("must not match a directory named .DS_Store")
	}
}

func TestDirWithSiblingRuleNeedsMarker(t *testing.T) {
	r := dirWithSiblingRule{name: "dist", markers: []string{"package.json"}, category: "build-cache"}
	e := &Entry{Path: "/x/dist", IsDir: true}

	m, cat, skip := r.Match(e, map[string]bool{"dist": true, "package.json": true})
	if !m || cat != "build-cache" || !skip {
		t.Fatalf("Match with marker = (%v, %q, %v), want (true, \"build-cache\", true)", m, cat, skip)
	}
	if m, _, _ := r.Match(e, map[string]bool{"dist": true, "README.md": true}); m {
		t.Error("must not match without a project marker sibling")
	}
	if m, _, _ := r.Match(&Entry{Path: "/x/dist", IsDir: false}, map[string]bool{"package.json": true}); m {
		t.Error("must not match a file named dist")
	}
}

func TestDefaultRulesMatchExpected(t *testing.T) {
	rules := DefaultRules()
	cases := []struct {
		e        *Entry
		siblings map[string]bool
		wantCat  string
	}{
		{&Entry{Path: "/p/node_modules", IsDir: true}, nil, "node_modules"},
		{&Entry{Path: "/p/.gradle", IsDir: true}, nil, "build-cache"},
		{&Entry{Path: "/p/__pycache__", IsDir: true}, nil, "python-cache"},
		{&Entry{Path: "/p/.DS_Store", IsDir: false}, nil, "os-junk"},
		{&Entry{Path: "/p/target", IsDir: true}, map[string]bool{"Cargo.toml": true}, "build-cache"},
		{&Entry{Path: "/p/build", IsDir: true}, map[string]bool{"build.gradle": true}, "build-cache"},
		{&Entry{Path: "/p/dist", IsDir: true}, map[string]bool{"package.json": true}, "build-cache"},
	}
	for _, c := range cases {
		cat, _, ok := matchRules(rules, c.e, c.siblings)
		if !ok || cat != c.wantCat {
			t.Errorf("matchRules(%s) = (%q, ok=%v), want %q", c.e.Path, cat, ok, c.wantCat)
		}
	}
	if _, _, ok := matchRules(rules, &Entry{Path: "/p/src", IsDir: true}, nil); ok {
		t.Error("a normal source dir must not match any default rule")
	}
}

func TestPresetDevCaches(t *testing.T) {
	rules, ok := Preset("dev-caches")
	if !ok {
		t.Fatal("dev-caches preset must exist")
	}
	if cat, _, ok := matchRules(rules, &Entry{Path: "/p/node_modules", IsDir: true}, nil); !ok || cat != "node_modules" {
		t.Errorf("dev-caches should match node_modules, got (%q, %v)", cat, ok)
	}
	// dev-caches 도 빌드 산출물은 마커를 요구한다.
	dist := &Entry{Path: "/p/dist", IsDir: true}
	if _, _, ok := matchRules(rules, dist, map[string]bool{"package.json": true}); !ok {
		t.Error("dev-caches should match dist next to package.json")
	}
	if _, _, ok := matchRules(rules, dist, nil); ok {
		t.Error("dev-caches must not match dist without a project marker")
	}
	// dev-caches 는 OS junk(.DS_Store)를 포함하지 않는다.
	if _, _, ok := matchRules(rules, &Entry{Path: "/p/.DS_Store", IsDir: false}, nil); ok {
		t.Error("dev-caches must not match .DS_Store")
	}
}

func TestPresetUnknown(t *testing.T) {
	if _, ok := Preset("nope"); ok {
		t.Error("unknown preset must return ok=false")
	}
}
