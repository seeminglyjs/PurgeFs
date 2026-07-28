package cmd

import (
	"bytes"
	"os"
	"path/filepath"
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
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	report, _, err := engine.Scan(root, rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Groups) != 0 {
		t.Errorf("dev-caches must not match .DS_Store, got %+v", report.Groups)
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
