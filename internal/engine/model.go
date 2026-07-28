// Package engine 은 파일 트리를 순회하며 용량을 집계하고 junk 규칙을 매칭한다.
// 프론트엔드 비의존적이다: CLI·TUI·향후 GUI 가 모두 이 패키지를 소비한다.
package engine

// Entry 는 규칙이 판단하는 대상 노드 하나다(파일 또는 디렉토리). 순회 중에만 존재하며 트리로
// 쌓이지 않는다 — 규칙은 노드 하나와 그 형제 이름만 보면 되기 때문이다.
type Entry struct {
	Path    string
	Size    int64 // 바이트. 파일: 자기 크기. 디렉토리: 하위 트리 합산 크기.
	IsDir   bool  // 디렉토리면 true. 파일·심볼릭링크는 false.
	ModTime int64 // 마지막 수정 시각(unix 초).
}

// Child 는 스캔 root 바로 아래 항목의 집계다. scan 이 "무엇이 용량을 먹는지" 보여주는 데 쓴다.
// 개수가 root 의 fan-out 만큼이라 트리 크기와 무관하다.
type Child struct {
	Path string
	Size int64
}

// WalkError 는 순회 중 처리하지 못한 경로 하나를 기록한다(예: 권한 거부). 설계상
// 비치명적이다: 순회는 이것들을 모으고 계속 진행하므로, 못 읽는 디렉토리 하나가 전체
// 스캔을 중단시키지 않는다.
type WalkError struct {
	Path string // 실패한 경로
	Err  error  // 원인이 된 os 에러
}

// Report 는 Scan 의 최종 결과다. 노드별 데이터는 남기지 않는다: 총량과 최상위 항목, 매치된
// junk 그룹만 들고 있어 메모리가 트리 크기가 아니라 결과 크기에 비례한다.
type Report struct {
	Root      string          // 실제로 순회한 root 경로(심볼릭링크 resolve 후)
	TotalSize int64           // root 하위 전체 바이트
	FileCount int             // 트리의 파일 노드 수
	DirCount  int             // 트리의 디렉토리 노드 수(root 포함)
	Children  []Child         // root 바로 아래 항목들(순회 순서)
	Groups    []CategoryGroup // 매치된 junk. Size 내림차순.
}
