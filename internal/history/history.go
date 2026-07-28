// Package history 는 purge 매니페스트를 저장하고, 최신 것을 불러와 복원한다. 매니페스트는
// 원본↔휴지통 경로 매핑을 담아 undo 가 파일을 되돌릴 수 있게 한다.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// Item 은 복원할 항목 하나다(원본 자리 ← 휴지통 경로).
type Item struct {
	Original string `json:"original"`
	Dest     string `json:"dest"`
}

// Manifest 는 한 번의 휴지통 purge 기록이다.
type Manifest struct {
	CreatedAt int64  `json:"created_at"` // 정렬·파일명용 타임스탬프(unix 나노초)
	Items     []Item `json:"items"`
}

// Failure 는 복원하지 못한 항목이다.
type Failure struct {
	Item Item
	Err  error
}

// RestoreResult 는 복원 결과다.
type RestoreResult struct {
	Restored []string // 복원된 원본 경로
	Skipped  []string // 원본이 이미 있거나 휴지통 파일이 없어 건너뜀
	Failed   []Failure
}

// Save 는 매니페스트를 dir/<CreatedAt>.json 으로 쓰고 그 경로를 반환한다.
func Save(dir string, m Manifest) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, strconv.FormatInt(m.CreatedAt, 10)+".json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadLatest 는 dir 안의 매니페스트 중 CreatedAt 이 가장 큰 것을 반환한다. 디렉토리가 없거나
// 매니페스트가 하나도 없으면 ok=false 를 돌려준다(에러 아님).
func LoadLatest(dir string) (Manifest, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, err
	}
	var latest Manifest
	found := false
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return Manifest{}, false, err
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return Manifest{}, false, err
		}
		if !found || m.CreatedAt > latest.CreatedAt {
			latest = m
			found = true
		}
	}
	return latest, found, nil
}

// Restore 는 매니페스트의 각 항목을 휴지통(Dest)에서 원본(Original)으로 되돌린다. 원본 자리에
// 이미 뭔가 있거나 Dest 가 없으면 덮어쓰지 않고 건너뛴다.
//
// 존재 확인과 os.Rename 사이에는 원자성이 없다(표준 라이브러리에 이식 가능한 "없을 때만
// rename" 이 없음). 그 좁은 틈에 다른 프로세스가 Original 을 새로 만들면 rename 이 그것을
// 덮어쓸 수 있다. undo 는 단일 사용자의 로컬 복원 동작이라 이 위험은 감수한다.
func Restore(m Manifest) RestoreResult {
	var r RestoreResult
	for _, it := range m.Items {
		if _, err := os.Lstat(it.Original); err == nil {
			r.Skipped = append(r.Skipped, it.Original) // 원본 자리에 이미 존재
			continue
		}
		if _, err := os.Lstat(it.Dest); err != nil {
			r.Skipped = append(r.Skipped, it.Original) // 휴지통에 없음
			continue
		}
		if err := os.Rename(it.Dest, it.Original); err != nil {
			r.Failed = append(r.Failed, Failure{Item: it, Err: err})
			continue
		}
		r.Restored = append(r.Restored, it.Original)
	}
	return r
}
