package metrics_test

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/metrics"
)

// TestToMap_CopiesSliceFields guards that ToMap snapshots the
// slice-valued fields rather than aliasing the (possibly shared,
// concurrently-mutated) source slices. The cache hands the same
// *Metrics pointer to multiple callers (Spec 18 daemon mode); aliasing
// would let a Collect()-driven in-place mutation race with a ranging
// ToMap consumer.
func TestToMap_CopiesSliceFields(t *testing.T) {
	m := &metrics.Metrics{
		CPU:          metrics.CPUMetrics{UsagePerCore: []float64{1, 2, 3}},
		GPUs:         []metrics.GPUMetrics{{Index: 0, UsagePct: 10}},
		Temperatures: []metrics.Sensor{{Chip: "coretemp", TempC: 40}},
	}

	out := m.ToMap()

	perCore := out["cpu_usage_per_core"].([]float64)
	gpus := out["gpus_metrics"].([]metrics.GPUMetrics)
	temps := out["temperatures"].([]metrics.Sensor)

	// Mutate the source in place; the snapshot must not observe it.
	m.CPU.UsagePerCore[0] = 99
	m.GPUs[0].UsagePct = 99
	m.Temperatures[0].TempC = 99

	if perCore[0] != 1 {
		t.Errorf("cpu_usage_per_core aliases source slice: got %v, want 1", perCore[0])
	}
	if gpus[0].UsagePct != 10 {
		t.Errorf("gpus_metrics aliases source slice: got %v, want 10", gpus[0].UsagePct)
	}
	if temps[0].TempC != 40 {
		t.Errorf("temperatures aliases source slice: got %v, want 40", temps[0].TempC)
	}
}
