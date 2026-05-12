package metrics

import "time"

// Collector samples one or more live metrics and writes them into m.
//
// Implementations are typically one-per-metric-family (cpu, load, mem, gpu,
// net) so the underlying tool (e.g. nvidia-smi) is invoked once per sample,
// not once per output field.
//
// The cache (see cache.go) calls Collect only when at least one of
// Outputs() is requested AND elapsed time since the last invocation
// exceeds TTL().
type Collector interface {
	// Name is a stable identifier used as a key in the cache's per-collector
	// timestamp map. Must be unique across all registered collectors.
	Name() string

	// Outputs are the ToMap() keys this collector populates. Used by the
	// cache to decide whether the collector needs to run when a fields=
	// filter is supplied. Output sets across collectors must not overlap —
	// enforced by collector_test.go.
	Outputs() []string

	// TTL is how long a sample remains fresh. Within the TTL window the
	// cache serves prior values without re-invoking Collect.
	TTL() time.Duration

	// Collect samples the underlying source and writes results into m.
	// Errors are propagated; the cache does not retry failed collectors
	// within the TTL window.
	Collect(m *Metrics) error
}

var collectors []Collector

// Register adds a collector to the global registry. Called from
// platform-specific init() functions (linux.go, linux_gpu.go, darwin.go).
func Register(c Collector) {
	collectors = append(collectors, c)
}

// Collectors returns the registered collectors. Exposed for tests and for
// the cache to iterate.
func Collectors() []Collector {
	return collectors
}
