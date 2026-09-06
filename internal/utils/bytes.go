package utils

import "fmt"

// HumanBytes renders a byte count in the largest binary unit that keeps
// the mantissa under 1024, so an operator reads "9.0 MiB" rather than
// "9467185 bytes".
//
// Shared by `mooncake doctor`'s runs-log check and `mooncake history gc`;
// both grew the same twelve lines independently.
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}
