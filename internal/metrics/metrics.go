// Package metrics is the live utilization surface of the host — CPU, GPU,
// memory, load, and network sampled on demand with per-metric TTL caching.
//
// Metrics is intentionally a sibling of internal/facts:
//   - facts describe what the machine *is* (capabilities, configuration).
//   - metrics describe what it's *doing right now*.
//
// One-shot CLI usage samples once and exits. Daemon usage (Spec 18) keeps
// the cache warm and serves repeated queries with TTL-bounded freshness.
//
// See Spec 20 for design rationale.
package metrics

// Metrics is the live utilization snapshot of the host.
type Metrics struct {
	CPU          CPUMetrics
	Memory       MemoryMetrics
	Load         LoadMetrics
	Network      NetworkMetrics
	GPUs         []GPUMetrics
	Temperatures []Sensor
	CPUTempC     float64 // derived: best CPU sensor from Temperatures, or 0 if none
}

// CPUMetrics holds system-wide and per-core CPU utilization.
type CPUMetrics struct {
	UsagePct     float64   // 0..100, aggregate across all cores
	UsagePerCore []float64 // one entry per logical CPU
}

// MemoryMetrics holds used / used-percent memory and swap.
type MemoryMetrics struct {
	UsedMB     int64
	UsedPct    float64 // 0..100
	SwapUsedMB int64
}

// LoadMetrics holds Unix-style load averages.
type LoadMetrics struct {
	Avg1m  float64
	Avg5m  float64
	Avg15m float64
}

// NetworkMetrics holds aggregate throughput across non-loopback interfaces.
type NetworkMetrics struct {
	RxBps int64 // bytes/sec received
	TxBps int64 // bytes/sec transmitted
}

// GPUMetrics is one GPU's live utilization, keyed by Index matching
// facts.Facts.GPUs[i]. v1 populates only NVIDIA entries.
type GPUMetrics struct {
	Index         int
	UsagePct      float64
	MemoryUsedMB  int64
	MemoryUsedPct float64
	TemperatureC  int
}

// Sensor is one hardware temperature sensor reading. Chip identifies the
// driver/device ("coretemp", "k10temp", "nvme", "acpitz", ...); Label is
// the per-input label if available ("Package id 0", "Core 0", "Composite").
// CritC is the manufacturer-set throttle/critical threshold if exposed.
type Sensor struct {
	Chip  string
	Label string
	TempC float64
	CritC float64
}

// ToMap flattens Metrics into the variable namespace used by templates and
// `when:` expressions. Key names are stable and documented in Spec 20.
//
// Keys here must remain disjoint from facts.Facts.ToMap() keys — enforced
// by internal/metrics/disjoint_test.go.
func (m *Metrics) ToMap() map[string]interface{} {
	// Defensively copy the slice-valued fields. m may be the
	// package-global cached pointer (cache.go), which a concurrent
	// Collect() can mutate in place (Spec 18 daemon mode). Embedding
	// the live slices would hand callers an aliased view that races
	// with that mutation; snapshot them instead.
	perCore := append([]float64(nil), m.CPU.UsagePerCore...)
	gpus := append([]GPUMetrics(nil), m.GPUs...)
	temps := append([]Sensor(nil), m.Temperatures...)
	return map[string]interface{}{
		// CPU
		"cpu_usage_pct":      m.CPU.UsagePct,
		"cpu_usage_per_core": perCore,

		// Load
		"load_avg_1m":  m.Load.Avg1m,
		"load_avg_5m":  m.Load.Avg5m,
		"load_avg_15m": m.Load.Avg15m,

		// Memory
		"memory_used_mb":  m.Memory.UsedMB,
		"memory_used_pct": m.Memory.UsedPct,
		"swap_used_mb":    m.Memory.SwapUsedMB,

		// Network
		"net_rx_bps": m.Network.RxBps,
		"net_tx_bps": m.Network.TxBps,

		// GPU (array; per-GPU correlation via Index)
		"gpus_metrics": gpus,

		// Temperatures: full sensor array + derived CPU package temp
		"temperatures": temps,
		"cpu_temp_c":   m.CPUTempC,
	}
}
