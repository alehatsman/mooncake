package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/metrics"
	"github.com/urfave/cli/v2"
)

func metricsCommand(c *cli.Context) error {
	if c.Bool("refresh") {
		metrics.Refresh()
	}

	fields := parseMetricsFields(c.StringSlice("fields"))

	m, collectedAt, err := metrics.Collect(fields)
	if err != nil {
		// Collection errors are non-fatal: partial data is still useful for
		// `--query`/`--fields` paths. Surface to stderr and continue.
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	mm := m.ToMap()

	// --query mode mirrors `mooncake facts --query`. Honor --format json
	// here so `metrics --format json -q cpu_usage_pct` emits an object
	// rather than the default key=value text — matches what users expect
	// from a flag pair that works on `--fields`.
	if queries := c.StringSlice("query"); len(queries) > 0 {
		if c.String("format") == outputFormatJSON {
			return queryMapJSON(mm, queries)
		}
		return queryMap(mm, queries)
	}

	// Build the output payload (full map, or fields-filtered).
	payload := mm
	if len(fields) > 0 {
		payload = map[string]interface{}{}
		for _, k := range fields {
			payload[k] = mm[k]
		}
		payload["_collected_at"] = formatCollectedAt(collectedAt)
	}

	format := c.String("format")
	if format != outputFormatText && format != outputFormatJSON {
		return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
	}

	if out := c.String("output"); out != "" {
		return writeMetricsJSONFile(payload, out)
	}

	switch format {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	case outputFormatText:
		// When fields are filtered we honor the filter even in text mode.
		if len(fields) > 0 {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}
		displayMetricsText(m)
		return nil
	}
	return nil
}

// parseMetricsFields accepts both repeated --fields flags and comma-separated
// values within one flag (e.g. `--fields cpu_usage_pct,load_avg_1m`).
func parseMetricsFields(raw []string) []string {
	var out []string
	for _, r := range raw {
		for _, f := range strings.Split(r, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				out = append(out, f)
			}
		}
	}
	return out
}

func formatCollectedAt(ts map[string]time.Time) map[string]string {
	out := make(map[string]string, len(ts))
	for k, v := range ts {
		out[k] = v.UTC().Format(time.RFC3339)
	}
	return out
}

func writeMetricsJSONFile(payload map[string]interface{}, path string) error {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func displayMetricsText(m *metrics.Metrics) {
	fmt.Printf("CPU:    %.1f%%\n", m.CPU.UsagePct)
	if len(m.CPU.UsagePerCore) > 0 {
		parts := make([]string, len(m.CPU.UsagePerCore))
		for i, p := range m.CPU.UsagePerCore {
			parts[i] = fmt.Sprintf("%.0f", p)
		}
		fmt.Printf("  per-core: [%s]%%\n", strings.Join(parts, " "))
	}
	fmt.Printf("Memory: %d MB used (%.1f%%)\n", m.Memory.UsedMB, m.Memory.UsedPct)
	if m.Memory.SwapUsedMB > 0 {
		fmt.Printf("  swap: %d MB\n", m.Memory.SwapUsedMB)
	}
	fmt.Printf("Load:   %.2f / %.2f / %.2f\n", m.Load.Avg1m, m.Load.Avg5m, m.Load.Avg15m)
	fmt.Printf("Net:    %s/s in, %s/s out\n", humanBytes(m.Network.RxBps), humanBytes(m.Network.TxBps))
	if m.CPUTempC > 0 {
		fmt.Printf("Temps:  CPU %.0f°C", m.CPUTempC)
		// Show NVMe / disk temps inline when present.
		for _, s := range m.Temperatures {
			if s.Chip == "nvme" {
				fmt.Printf(", %s %.0f°C", s.Label, s.TempC)
			}
		}
		fmt.Println()
	}
	if len(m.GPUs) > 0 {
		fmt.Println("GPUs:")
		for _, g := range m.GPUs {
			fmt.Printf("  [%d] %.0f%% util, %d MB used (%.0f%%), %d°C\n",
				g.Index, g.UsagePct, g.MemoryUsedMB, g.MemoryUsedPct, g.TemperatureC)
		}
	}
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
