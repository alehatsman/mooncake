//go:build linux

package metrics

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
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

// parseNvidiaSmiCSV parses CSV output of:
//   nvidia-smi --query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu --format=csv,noheader,nounits
// Result is sorted by Index ascending so callers can correlate positionally
// with facts.Facts.GPUs (which uses the same nvidia-smi index ordering).
func parseNvidiaSmiCSV(data string) ([]GPUMetrics, error) {
	var out []GPUMetrics
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 5 {
			return nil, fmt.Errorf("nvidia-smi row has %d fields, want 5: %q", len(parts), line)
		}
		idx, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("parse gpu index %q: %w", parts[0], err)
		}
		usage, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse gpu usage %q: %w", parts[1], err)
		}
		memUsed, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse gpu mem used %q: %w", parts[2], err)
		}
		memTotal, err := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse gpu mem total %q: %w", parts[3], err)
		}
		temp, err := strconv.Atoi(strings.TrimSpace(parts[4]))
		if err != nil {
			return nil, fmt.Errorf("parse gpu temp %q: %w", parts[4], err)
		}
		var memPct float64
		if memTotal > 0 {
			memPct = float64(memUsed) / float64(memTotal) * 100
		}
		out = append(out, GPUMetrics{
			Index:         idx,
			UsagePct:      usage,
			MemoryUsedMB:  memUsed,
			MemoryUsedPct: memPct,
			TemperatureC:  temp,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out, nil
}
