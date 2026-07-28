package cmd

import "fmt"

// humanBytes formats a byte count as a base-1024 human-readable string, e.g.
// 0 -> "0 B", 1536 -> "1.5 KB", 1<<30 -> "1.0 GB". Below 1 KB it prints the
// exact byte count with no decimal.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	// Walk up the unit ladder: each loop divides by 1024 once more. div is the
	// divisor for the chosen unit; exp indexes into "KMGTPE" for its letter.
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
