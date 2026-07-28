package engine

import "path/filepath"

// Rule 은 하나의 junk 판별 규칙이다. Match 는 e 가 이 규칙에 해당하면
// (true, 카테고리, skipChildren) 을 반환한다. skipChildren 이 true 면 매치된
// 디렉토리를 단일 회수 단위로 보고 그 하위는 더 분류하지 않는다(node_modules 처럼).
//
// siblings 는 e 와 같은 부모에 있는 항목 이름들이다. 이름만으로 판단할 수 없는 규칙이
// 옆의 프로젝트 마커 파일을 볼 수 있게 넘긴다. root 엔트리는 부모가 없어 비어 있다.
type Rule interface {
	Match(e *Entry, siblings map[string]bool) (matched bool, category string, skipChildren bool)
}

// dirNameRule 은 특정 이름의 디렉토리를 매치한다. 디렉토리 통째가 하나의 회수 단위라
// skipChildren 은 true 다.
type dirNameRule struct {
	name     string
	category string
}

func (r dirNameRule) Match(e *Entry, _ map[string]bool) (bool, string, bool) {
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

func (r fileNameRule) Match(e *Entry, _ map[string]bool) (bool, string, bool) {
	if !e.IsDir && filepath.Base(e.Path) == r.name {
		return true, r.category, false
	}
	return false, "", false
}

// dirWithSiblingRule 은 name 디렉토리를, 같은 부모에 markers 중 하나가 있을 때만 매치한다.
// build/dist/target 은 이름만으로 빌드 산출물인지 알 수 없다 — 소스로 관리되는 build/, 커밋된
// dist/, Rust 아닌 프로젝트의 target/ 이 흔하다. 옆에 있는 프로젝트 마커 파일을 근거로 삼아
// 실제 빌드 산출물일 때만 지운다.
type dirWithSiblingRule struct {
	name     string
	markers  []string
	category string
}

func (r dirWithSiblingRule) Match(e *Entry, siblings map[string]bool) (bool, string, bool) {
	if !e.IsDir || filepath.Base(e.Path) != r.name {
		return false, "", false
	}
	for _, m := range r.markers {
		if siblings[m] {
			return true, r.category, true
		}
	}
	return false, "", false
}

// buildOutputRules 는 마커가 필요한 빌드 산출물 규칙들이다. DefaultRules 와 dev-caches 프리셋이
// 같은 목록을 쓴다.
func buildOutputRules() []Rule {
	return []Rule{
		dirWithSiblingRule{name: "target", markers: []string{"Cargo.toml", "pom.xml"}, category: "build-cache"},
		dirWithSiblingRule{name: "dist", markers: []string{"package.json"}, category: "build-cache"},
		dirWithSiblingRule{
			name:     "build",
			markers:  []string{"build.gradle", "build.gradle.kts", "pom.xml", "CMakeLists.txt"},
			category: "build-cache",
		},
	}
}

// DefaultRules 는 P2 의 내장 규칙 집합을 순서대로 반환한다. 앞의 규칙이 먼저 매치된다.
func DefaultRules() []Rule {
	rules := []Rule{
		dirNameRule{name: "node_modules", category: "node_modules"},
		dirNameRule{name: ".gradle", category: "build-cache"},
		dirNameRule{name: "__pycache__", category: "python-cache"},
		fileNameRule{name: ".DS_Store", category: "os-junk"},
	}
	return append(rules, buildOutputRules()...)
}

// Preset 은 이름에 해당하는 규칙 집합을 반환한다. 없으면 ok=false. dev-caches 는 빌드·의존성
// 캐시 디렉토리만 대상으로 하고 OS junk(.DS_Store)는 제외한다.
func Preset(name string) ([]Rule, bool) {
	switch name {
	case "dev-caches":
		rules := []Rule{
			dirNameRule{name: "node_modules", category: "node_modules"},
			dirNameRule{name: ".gradle", category: "build-cache"},
			dirNameRule{name: "__pycache__", category: "python-cache"},
		}
		return append(rules, buildOutputRules()...), true
	default:
		return nil, false
	}
}

// matchRules 는 규칙들을 순서대로 시도해 첫 매치의 (카테고리, skipChildren, true) 를
// 반환한다. 아무 것도 매치하지 않으면 ("", false, false).
func matchRules(rules []Rule, e *Entry, siblings map[string]bool) (string, bool, bool) {
	for _, r := range rules {
		if m, cat, skip := r.Match(e, siblings); m {
			return cat, skip, true
		}
	}
	return "", false, false
}
