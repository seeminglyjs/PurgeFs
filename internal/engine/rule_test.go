package engine

import "testing"

func TestDirNameRuleMatchesDir(t *testing.T) {
	r := dirNameRule{name: "node_modules", category: "node_modules"}
	m, cat, skip := r.Match(&Entry{Path: "/x/node_modules", IsDir: true})
	if !m || cat != "node_modules" || !skip {
		t.Fatalf("Match = (%v, %q, %v), want (true, \"node_modules\", true)", m, cat, skip)
	}
}

func TestDirNameRuleIgnoresFileAndOtherNames(t *testing.T) {
	r := dirNameRule{name: "node_modules", category: "node_modules"}
	if m, _, _ := r.Match(&Entry{Path: "/x/node_modules", IsDir: false}); m {
		t.Error("must not match a file named node_modules")
	}
	if m, _, _ := r.Match(&Entry{Path: "/x/src", IsDir: true}); m {
		t.Error("must not match an unrelated directory")
	}
}

func TestFileNameRuleMatchesFile(t *testing.T) {
	r := fileNameRule{name: ".DS_Store", category: "os-junk"}
	m, cat, skip := r.Match(&Entry{Path: "/x/.DS_Store", IsDir: false})
	if !m || cat != "os-junk" || skip {
		t.Fatalf("Match = (%v, %q, %v), want (true, \"os-junk\", false)", m, cat, skip)
	}
	if m, _, _ := r.Match(&Entry{Path: "/x/.DS_Store", IsDir: true}); m {
		t.Error("must not match a directory named .DS_Store")
	}
}

func TestDefaultRulesMatchExpected(t *testing.T) {
	rules := DefaultRules()
	cases := []struct {
		e       *Entry
		wantCat string
	}{
		{&Entry{Path: "/p/node_modules", IsDir: true}, "node_modules"},
		{&Entry{Path: "/p/target", IsDir: true}, "build-cache"},
		{&Entry{Path: "/p/build", IsDir: true}, "build-cache"},
		{&Entry{Path: "/p/.gradle", IsDir: true}, "build-cache"},
		{&Entry{Path: "/p/dist", IsDir: true}, "build-cache"},
		{&Entry{Path: "/p/__pycache__", IsDir: true}, "python-cache"},
		{&Entry{Path: "/p/.DS_Store", IsDir: false}, "os-junk"},
	}
	for _, c := range cases {
		cat, _, ok := matchRules(rules, c.e)
		if !ok || cat != c.wantCat {
			t.Errorf("matchRules(%s) = (%q, ok=%v), want %q", c.e.Path, cat, ok, c.wantCat)
		}
	}
	if _, _, ok := matchRules(rules, &Entry{Path: "/p/src", IsDir: true}); ok {
		t.Error("a normal source dir must not match any default rule")
	}
}
