//go:build windows

package metrics

import (
	"os/exec"
	"sort"
	"time"
)

func init() {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return // nvidia-smi not in PATH; no GPU collector
	}
	Register(&windowsNvidiaGPUCollector{path: path})
}

type windowsNvidiaGPUCollector struct{ path string }

func (windowsNvidiaGPUCollector) Name() string       { return "gpu_nvidia" }
func (windowsNvidiaGPUCollector) Outputs() []string  { return []string{"gpus_metrics"} }
func (windowsNvidiaGPUCollector) TTL() time.Duration { return 2 * time.Second }

func (c *windowsNvidiaGPUCollector) Collect(m *Metrics) error {
	// #nosec G204 -- path is validated via exec.LookPath in init.
	out, err := exec.Command(c.path,
		"--query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
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
	sort.Slice(gpus, func(i, j int) bool { return gpus[i].Index < gpus[j].Index })
	m.GPUs = gpus
	return nil
}
