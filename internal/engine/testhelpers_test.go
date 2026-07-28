package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// childByPath 는 Report 의 최상위 항목 중 경로가 맞는 것을 찾는다.
func childByPath(r *Report, path string) *Child {
	for i, c := range r.Children {
		if c.Path == path {
			return &r.Children[i]
		}
	}
	return nil
}

// groupByCategory 는 분류 결과에서 카테고리 하나를 꺼낸다. 없으면 빈 그룹.
func groupByCategory(r *Report, category string) CategoryGroup {
	for _, g := range r.Groups {
		if g.Category == category {
			return g
		}
	}
	return CategoryGroup{}
}

// resolved 는 심볼릭링크를 푼 경로다. Scan 이 root 를 resolve 하므로 macOS 의 t.TempDir()
// (/var → /private/var)과 비교하려면 기대값도 같이 풀어야 한다.
func resolved(t *testing.T, path string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks %s: %v", path, err)
	}
	return r
}
