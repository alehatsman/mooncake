package metrics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// parseNvidiaSmiCSV parses CSV output of:
//
//	nvidia-smi --query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu --format=csv,noheader,nounits
//
// Result is sorted by Index ascending.
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
