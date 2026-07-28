package cmd

import (
	"fmt"

	"github.com/seeminglyjs/PurgeFs/internal/engine"
)

// resolveRules 는 --preset 값에 해당하는 규칙 집합을 돌려준다. 빈 문자열이면 내장 기본 규칙이다.
// scan 과 purge 가 같은 함수를 써야 미리보기와 실제 삭제 대상이 어긋나지 않는다.
func resolveRules(preset string) ([]engine.Rule, error) {
	if preset == "" {
		return engine.DefaultRules(), nil
	}
	rules, ok := engine.Preset(preset)
	if !ok {
		return nil, fmt.Errorf("unknown preset %q (available: dev-caches)", preset)
	}
	return rules, nil
}
