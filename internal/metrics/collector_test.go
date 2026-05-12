package metrics

import (
	"testing"
	"time"
)

// TestRegisteredCollectorsOutputsDisjoint verifies that no two registered
// collectors claim the same ToMap key — overlap would mean the cache
// can't unambiguously attribute freshness to a single collector.
func TestRegisteredCollectorsOutputsDisjoint(t *testing.T) {
	seen := map[string]string{} // key → owning collector
	for _, c := range Collectors() {
		for _, k := range c.Outputs() {
			if other, dup := seen[k]; dup {
				t.Errorf("output key %q claimed by both %q and %q", k, other, c.Name())
			}
			seen[k] = c.Name()
		}
	}
}

// fakeCollector lets cache_test.go and this file drive the cache with
// deterministic timing and observable invocation counts.
type fakeCollector struct {
	name    string
	outputs []string
	ttl     time.Duration
	calls   int
	apply   func(*Metrics) // optional, defaults to no-op
}

func (f *fakeCollector) Name() string         { return f.name }
func (f *fakeCollector) Outputs() []string    { return f.outputs }
func (f *fakeCollector) TTL() time.Duration   { return f.ttl }
func (f *fakeCollector) Collect(m *Metrics) error {
	f.calls++
	if f.apply != nil {
		f.apply(m)
	}
	return nil
}

func TestFakeCollectorRoundtrip(t *testing.T) {
	// This test exercises Register + Collectors with a fake; no cache yet.
	saved := collectors
	defer func() { collectors = saved }()

	collectors = nil
	fc := &fakeCollector{name: "fake", outputs: []string{"x"}, ttl: time.Second}
	Register(fc)

	if len(Collectors()) != 1 {
		t.Fatalf("expected 1 registered collector, got %d", len(Collectors()))
	}
	if Collectors()[0].Name() != "fake" {
		t.Errorf("expected 'fake', got %q", Collectors()[0].Name())
	}
}
