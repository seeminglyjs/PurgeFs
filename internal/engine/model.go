// Package engine 은 파일 트리를 순회하며 용량을 집계하고, (이후 단계에서) junk 규칙을
// 매칭한다. 프론트엔드 비의존적이다: CLI·TUI·향후 GUI 가 모두 이 패키지를 소비한다.
package engine

// Entry 는 스캔한 트리의 한 노드(파일 또는 디렉토리)다. 디렉토리는 자식을 Children 에
// 담고 Size 는 하위 트리 전체의 합이다. 파일은 Children 이 nil 이고 Size 는 자기 자신의
// 바이트 수다. 모든 프론트엔드가 이 한 가지 모양을 렌더한다 — 용량 트리 뷰와 카테고리
// 뷰 둘 다 같은 Entry 그래프에서 만들어진다.
type Entry struct {
	Path     string
	Size     int64    // 바이트. 파일: 자기 크기. 디렉토리: 하위 트리 합산 크기.
	IsDir    bool     // 디렉토리면 true. 파일·심볼릭링크는 false.
	ModTime  int64    // 마지막 수정 시각(unix 초). 이후 단계의 staleness 스코어링에 사용.
	Children []*Entry // 디렉토리의 자식 목록. 파일·심볼릭링크는 nil(순회 안 함).
}

// WalkError 는 순회 중 처리하지 못한 경로 하나를 기록한다(예: 권한 거부). 설계상
// 비치명적이다: 순회는 이것들을 모으고 계속 진행하므로, 못 읽는 디렉토리 하나가 전체
// 스캔을 중단시키지 않는다.
type WalkError struct {
	Path string // 실패한 경로
	Err  error  // 원인이 된 os 에러
}

// Report 는 Scan 의 최종 결과다: 트리의 root 와, 프론트엔드가 다시 순회하지 않고 바로
// 출력할 수 있는 집계 총량.
type Report struct {
	Root      *Entry // 스캔한 트리의 root. Root.Size == TotalSize
	TotalSize int64  // root 하위 전체 바이트
	FileCount int    // 트리의 파일 노드 수
	DirCount  int    // 트리의 디렉토리 노드 수(root 포함)
}
