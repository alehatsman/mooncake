package facts

import (
	"testing"
)

func TestCollect_Caching(t *testing.T) {
	ClearCache()
	f1 := Collect()
	f2 := Collect()

	// Should return the same instance (same pointer)
	if f1 != f2 {
		t.Error("Expected same instance from cached Collect()")
	}
}

func TestClearCache(t *testing.T) {
	f1 := Collect()
	ClearCache()
	f2 := Collect()

	// After clearing cache, should get a different instance
	if f1 == f2 {
		t.Error("Expected different instances after ClearCache()")
	}
}

func TestCollect_ReturnsValidFacts(t *testing.T) {
	ClearCache()
	f := Collect()

	// Basic facts should always be populated
	if f.OS == "" {
		t.Error("OS should not be empty")
	}
	if f.Arch == "" {
		t.Error("Arch should not be empty")
	}
	if f.CPUCores <= 0 {
		t.Error("CPUCores should be positive")
	}
}
