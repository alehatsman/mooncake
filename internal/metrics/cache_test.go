package metrics

import (
	"testing"
	"time"
)

// withCollectors swaps the global registry for the duration of the test,
// restoring it after. ClearCache() resets the cache state.
func withCollectors(t *testing.T, cs []Collector, fn func()) {
	t.Helper()
	saved := collectors
	collectors = cs
	ClearCache()
	defer func() {
		collectors = saved
		ClearCache()
	}()
	fn()
}

func TestCacheTTLRespected(t *testing.T) {
	fc := &fakeCollector{name: "fc", outputs: []string{"cpu_usage_pct"}, ttl: 100 * time.Millisecond}
	withCollectors(t, []Collector{fc}, func() {
		// First Collect runs the collector.
		if _, _, err := Collect(nil); err != nil {
			t.Fatalf("first Collect: %v", err)
		}
		if fc.calls != 1 {
			t.Fatalf("expected 1 call after first Collect, got %d", fc.calls)
		}

		// Second Collect within TTL should not re-run.
		if _, _, err := Collect(nil); err != nil {
			t.Fatalf("second Collect: %v", err)
		}
		if fc.calls != 1 {
			t.Errorf("expected 1 call within TTL, got %d", fc.calls)
		}

		// After TTL elapses, Collect should re-run.
		time.Sleep(120 * time.Millisecond)
		if _, _, err := Collect(nil); err != nil {
			t.Fatalf("third Collect: %v", err)
		}
		if fc.calls != 2 {
			t.Errorf("expected 2 calls after TTL, got %d", fc.calls)
		}
	})
}

func TestCacheRefreshInvalidates(t *testing.T) {
	fc := &fakeCollector{name: "fc", outputs: []string{"cpu_usage_pct"}, ttl: 1 * time.Hour}
	withCollectors(t, []Collector{fc}, func() {
		if _, _, err := Collect(nil); err != nil {
			t.Fatal(err)
		}
		if fc.calls != 1 {
			t.Fatalf("expected 1 call, got %d", fc.calls)
		}

		// Without Refresh, the second call is a no-op (TTL is 1h).
		if _, _, err := Collect(nil); err != nil {
			t.Fatal(err)
		}
		if fc.calls != 1 {
			t.Fatalf("expected 1 call still, got %d", fc.calls)
		}

		// Refresh forces the next Collect to re-sample.
		Refresh()
		if _, _, err := Collect(nil); err != nil {
			t.Fatal(err)
		}
		if fc.calls != 2 {
			t.Errorf("expected 2 calls after Refresh, got %d", fc.calls)
		}

		// The forceRefresh flag should clear after one Collect.
		if _, _, err := Collect(nil); err != nil {
			t.Fatal(err)
		}
		if fc.calls != 2 {
			t.Errorf("expected forceRefresh to clear; got %d calls", fc.calls)
		}
	})
}

func TestCacheFieldsFilter(t *testing.T) {
	cpu := &fakeCollector{name: "cpu", outputs: []string{"cpu_usage_pct"}, ttl: 1 * time.Hour}
	load := &fakeCollector{name: "load", outputs: []string{"load_avg_1m"}, ttl: 1 * time.Hour}
	withCollectors(t, []Collector{cpu, load}, func() {
		// Asking only for load should not run cpu.
		if _, _, err := Collect([]string{"load_avg_1m"}); err != nil {
			t.Fatal(err)
		}
		if cpu.calls != 0 {
			t.Errorf("expected cpu to be skipped, got %d calls", cpu.calls)
		}
		if load.calls != 1 {
			t.Errorf("expected load to run, got %d calls", load.calls)
		}
	})
}

func TestCacheCollectedAtPopulated(t *testing.T) {
	fc := &fakeCollector{name: "fc", outputs: []string{"cpu_usage_pct"}, ttl: 1 * time.Hour}
	withCollectors(t, []Collector{fc}, func() {
		_, collectedAt, err := Collect([]string{"cpu_usage_pct"})
		if err != nil {
			t.Fatal(err)
		}
		ts, ok := collectedAt["cpu_usage_pct"]
		if !ok {
			t.Fatalf("expected cpu_usage_pct in collectedAt map, got %v", collectedAt)
		}
		if time.Since(ts) > time.Second {
			t.Errorf("timestamp too old: %v", ts)
		}

		// Unrequested keys must not appear.
		if _, present := collectedAt["load_avg_1m"]; present {
			t.Error("collectedAt contained an unrequested key")
		}
	})
}

func TestCacheCollectorWritesPersist(t *testing.T) {
	fc := &fakeCollector{
		name:    "fc",
		outputs: []string{"cpu_usage_pct"},
		ttl:     1 * time.Hour,
		apply:   func(m *Metrics) { m.CPU.UsagePct = 42 },
	}
	withCollectors(t, []Collector{fc}, func() {
		m, _, err := Collect(nil)
		if err != nil {
			t.Fatal(err)
		}
		if m.CPU.UsagePct != 42 {
			t.Errorf("expected CPU.UsagePct=42, got %v", m.CPU.UsagePct)
		}

		// Subsequent in-TTL call returns the same cached value.
		m2, _, err := Collect(nil)
		if err != nil {
			t.Fatal(err)
		}
		if m2.CPU.UsagePct != 42 {
			t.Errorf("expected cached CPU.UsagePct=42, got %v", m2.CPU.UsagePct)
		}
		if fc.calls != 1 {
			t.Errorf("expected 1 call, got %d", fc.calls)
		}
	})
}
