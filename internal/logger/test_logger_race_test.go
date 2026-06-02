package logger

import (
	"sync"
	"testing"
)

// TestWithPadLevel_SharesMutex verifies that a logger derived via
// WithPadLevel serializes appends against its parent under the SAME
// mutex. Both alias the same Logs backing array, so concurrent logging
// through parent and child must not race. Run under `go test -race` to
// exercise the detector.
func TestWithPadLevel_SharesMutex(t *testing.T) {
	parent := NewTestLogger()
	child := parent.WithPadLevel(2)

	var wg sync.WaitGroup
	const n = 200
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			parent.Infof("parent %d", i)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			child.Infof("child %d", i)
		}
	}()
	wg.Wait()
}
