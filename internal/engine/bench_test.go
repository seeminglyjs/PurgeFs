package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// bigTree 는 dirs 개의 디렉토리에 각각 filesPerDir 개의 파일을 만든 트리를 반환한다.
// 절반의 디렉토리에는 node_modules 를 두어 분류 규칙도 실제로 매치되게 한다.
func bigTree(tb testing.TB, dirs, filesPerDir int) string {
	tb.Helper()
	root := tb.TempDir()
	for d := range dirs {
		dir := filepath.Join(root, "pkg"+strconv.Itoa(d))
		if d%2 == 0 {
			dir = filepath.Join(dir, "node_modules")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatalf("mkdir: %v", err)
		}
		for f := range filesPerDir {
			p := filepath.Join(dir, "f"+strconv.Itoa(f))
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				tb.Fatalf("write: %v", err)
			}
		}
	}
	return root
}

func heapAlloc() uint64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

// 스캔이 끝난 뒤 Report 가 붙잡고 있는 힙은 트리 크기가 아니라 결과 크기에 비례해야 한다.
// 노드마다 Entry 를 남기면 큰 디렉토리(홈, 프로젝트 모음)에서 수백 MB 를 쓴다.
func TestScanRetainsBoundedMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 20,000 file tree")
	}
	const files = 20000
	root := bigTree(t, 200, 100)

	before := heapAlloc()
	report, _, err := Scan(root, DefaultRules())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	after := heapAlloc()
	runtime.KeepAlive(report)

	if report.FileCount != files {
		t.Fatalf("FileCount = %d, want %d", report.FileCount, files)
	}
	retained := int64(after) - int64(before)
	// 결과는 최상위 항목 200개와 카테고리 1개뿐이다. 1 MB 면 넉넉한 상한이다.
	const limit = 1 << 20
	if retained > limit {
		t.Errorf("Scan retains %d bytes for %d files, want under %d — the whole tree is being kept",
			retained, files, limit)
	}
}

// BenchmarkScan 은 스캔 한 번의 시간과 할당량을 잰다.
func BenchmarkScan(b *testing.B) {
	root := bigTree(b, 200, 100) // 20,000 파일
	b.ResetTimer()
	for b.Loop() {
		if _, _, err := Scan(root, DefaultRules()); err != nil {
			b.Fatalf("Scan: %v", err)
		}
	}
}
