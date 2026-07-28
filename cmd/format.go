package cmd

import "fmt"

// humanBytes formats a byte count as a base-1024 human-readable string.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// plural returns one when n == 1, otherwise many.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
