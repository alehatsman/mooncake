package scaffold

import "time"

// timeNowImpl is split out so scaffold_test.go doesn't need a time import.
func timeNowImpl() time.Duration {
	return time.Duration(time.Now().UnixNano())
}
