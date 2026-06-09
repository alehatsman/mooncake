//go:build linux

package metrics

import (
	"os/exec"
	"time"
)

func init() {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return // no nvidia-smi: don't register a collector
	}
	Register(&nvidiaGPUCollector{path: path})
}

type nvidiaGPUCollector struct {
	path string
}

func (nvidiaGPUCollector) Name() string       { return "gpu_nvidia" }
func (nvidiaGPUCollector) Outputs() []string  { return []string{"gpus_metrics"} }
func (nvidiaGPUCollector) TTL() time.Duration { return 2 * time.Second }

// nvidiaSmiRunner is overridable in tests via the runNvidiaSmi var.
var runNvidiaSmi = func(path string) ([]byte, error) {
	// #nosec G204 -- path is validated via exec.LookPath in init.
	return exec.Command(path,
		"--query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu",
		"--format=csv,noheader,nounits",
	).Output()
}

func (c *nvidiaGPUCollector) Collect(m *Metrics) error {
	out, err := runNvidiaSmi(c.path)
	if err != nil {
		// nvidia-smi exits non-zero when no NVIDIA devices are visible
		// (driver not loaded, no GPU attached). Treat as "no GPUs" rather
		// than a hard error so the surface stays consistent with hosts
		// where nvidia-smi isn't installed at all.
		m.GPUs = []GPUMetrics{}
		return nil
	}
	gpus, err := parseNvidiaSmiCSV(string(out))
	if err != nil {
		return err
	}
	if gpus == nil {
		gpus = []GPUMetrics{}
	}
	m.GPUs = gpus
	return nil
}
