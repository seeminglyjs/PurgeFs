package engine

import "path/filepath"

// Rule 은 하나의 junk 판별 규칙이다. Match 는 e 가 이 규칙에 해당하면
// (true, 카테고리, skipChildren) 을 반환한다. skipChildren 이 true 면 매치된
// 디렉토리를 단일 회수 단위로 보고 그 하위는 더 분류하지 않는다(node_modules 처럼).
type Rule interface {
	Match(e *Entry) (matched bool, category string, skipChildren bool)
}

// dirNameRule 은 특정 이름의 디렉토리를 매치한다. 디렉토리 통째가 하나의 회수 단위라
// skipChildren 은 true 다.
type dirNameRule struct {
	name     string
	category string
}

func (r dirNameRule) Match(e *Entry) (bool, string, bool) {
	if e.IsDir && filepath.Base(e.Path) == r.name {
		return true, r.category, true
	}
	return false, "", false
}

// fileNameRule 은 특정 이름의 파일을 매치한다. 파일이므로 하위가 없어 skipChildren 은
// 무의미하며 false 다.
type fileNameRule struct {
	name     string
	category string
}

func (r fileNameRule) Match(e *Entry) (bool, string, bool) {
	if !e.IsDir && filepath.Base(e.Path) == r.name {
		return true, r.category, false
	}
	return false, "", false
}

// DefaultRules 는 P2 의 내장 규칙 집합을 순서대로 반환한다. 앞의 규칙이 먼저 매치된다.
func DefaultRules() []Rule {
	return []Rule{
		dirNameRule{name: "node_modules", category: "node_modules"},
		dirNameRule{name: "target", category: "build-cache"},
		dirNameRule{name: "build", category: "build-cache"},
		dirNameRule{name: ".gradle", category: "build-cache"},
		dirNameRule{name: "dist", category: "build-cache"},
		dirNameRule{name: "__pycache__", category: "python-cache"},
		fileNameRule{name: ".DS_Store", category: "os-junk"},
	}
}

// Preset 은 이름에 해당하는 규칙 집합을 반환한다. 없으면 ok=false. dev-caches 는 빌드·의존성
// 캐시 디렉토리만 대상으로 하고 OS junk(.DS_Store)는 제외한다.
func Preset(name string) ([]Rule, bool) {
	switch name {
	case "dev-caches":
		return []Rule{
			dirNameRule{name: "node_modules", category: "node_modules"},
			dirNameRule{name: "target", category: "build-cache"},
			dirNameRule{name: "build", category: "build-cache"},
			dirNameRule{name: ".gradle", category: "build-cache"},
			dirNameRule{name: "dist", category: "build-cache"},
			dirNameRule{name: "__pycache__", category: "python-cache"},
		}, true
	default:
		return nil, false
	}
}

// matchRules 는 규칙들을 순서대로 시도해 첫 매치의 (카테고리, skipChildren, true) 를
// 반환한다. 아무 것도 매치하지 않으면 ("", false, false).
func matchRules(rules []Rule, e *Entry) (string, bool, bool) {
	for _, r := range rules {
		if m, cat, skip := r.Match(e); m {
			return cat, skip, true
		}
	}
	return "", false, false
}
