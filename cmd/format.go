package cmd

import "fmt"

// humanBytes 는 바이트 수를 1024 기반의 사람이 읽는 문자열로 만든다. 예:
// 0 -> "0 B", 1536 -> "1.5 KB", 1<<30 -> "1.0 GB". 1 KB 미만은 소수점 없이 정확한
// 바이트 수를 출력한다.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	// 단위 사다리를 위로 오른다: 반복마다 1024 로 한 번 더 나눈다. div 는 고른 단위의
	// 나눗수, exp 는 그 단위 문자를 위한 "KMGTPE" 인덱스다.
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// plural 은 n 이 1 이면 one 을, 아니면 many 를 반환한다.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
