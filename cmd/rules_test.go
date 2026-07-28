package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
)

func TestResolveRulesDefault(t *testing.T) {
	rules, err := resolveRules("")
	if err != nil {
		t.Fatalf("resolveRules(\"\"): %v", err)
	}
	if len(rules) != len(engine.DefaultRules()) {
		t.Errorf("resolveRules(\"\") = %d rules, want the default set (%d)", len(rules), len(engine.DefaultRules()))
	}
}

func TestResolveRulesPreset(t *testing.T) {
	rules, err := resolveRules("dev-caches")
	if err != nil {
		t.Fatalf("resolveRules(\"dev-caches\"): %v", err)
	}
	if _, _, ok := matchAny(rules, "/p/.DS_Store"); ok {
		t.Error("dev-caches must not match .DS_Store")
	}
}

func TestResolveRulesUnknownNamesTheAvailableOnes(t *testing.T) {
	_, err := resolveRules("nope")
	if err == nil {
		t.Fatal("unknown preset must be an error")
	}
	if !strings.Contains(err.Error(), "dev-caches") {
		t.Errorf("error should list the available presets, got %q", err)
	}
}

// matchAny 는 파일 경로 하나를 분류해 매치 여부를 본다(엔진 내부 API 를 쓰지 않기 위한 우회).
func matchAny(rules []engine.Rule, path string) (string, int64, bool) {
	root := &engine.Entry{Path: "/p", IsDir: true, Children: []*engine.Entry{
		{Path: path, IsDir: false, Size: 1},
	}}
	groups := engine.Classify(&engine.Report{Root: root}, rules)
	if len(groups) == 0 {
		return "", 0, false
	}
	return groups[0].Category, groups[0].Size, true
}

// scan 도 purge 와 같은 프리셋을 써야 한다. 다르면 미리보기와 실제 삭제 대상이 어긋난다.
func TestRunScanWithPresetSkipsOsJunk(t *testing.T) {
	root := junkDir(t) // node_modules + .DS_Store
	rules, err := resolveRules("dev-caches")
	if err != nil {
		t.Fatalf("resolveRules: %v", err)
	}

	var buf bytes.Buffer
	if err := runScan(&buf, root, rules); err != nil {
		t.Fatalf("runScan: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "os-junk") {
		t.Errorf("dev-caches preset must not report os-junk:\n%s", out)
	}
	if !strings.Contains(out, "node_modules") {
		t.Errorf("output missing node_modules:\n%s", out)
	}
}
