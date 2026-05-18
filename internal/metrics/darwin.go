//go:build darwin

package metrics

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&darwinCPUCollector{})
	Register(&darwinLoadCollector{})
	Register(&darwinMemCollector{})
	Register(&darwinNetCollector{})
	// No GPU collector on darwin in v1 — Apple Silicon needs powermetrics
	// + sudo, which is deferred.
}

// runCmd is overridable in tests.
var runCmd = func(name string, args ...string) ([]byte, error) {
	// #nosec G204 -- command names are fixed string literals at call sites.
	return exec.Command(name, args...).Output()
}

// ----- CPU -------------------------------------------------------------------
//
// macOS does not expose per-core utilization through `top` without cgo +
// host_processor_info. For v1 we emit only the aggregate; per-core is left
// empty. If callers need it later, the collector can be swapped for a cgo
// implementation without changing the Outputs surface.

type darwinCPUCollector struct{}

func (darwinCPUCollector) Name() string       { return "cpu" }
func (darwinCPUCollector) Outputs() []string  { return []string{"cpu_usage_pct", "cpu_usage_per_core"} }
func (darwinCPUCollector) TTL() time.Duration { return 2 * time.Second }
func (darwinCPUCollector) Collect(m *Metrics) error {
	// `top -l 2 -n 0 -s 1`: emit headers twice 1s apart, no process rows.
	// The second "CPU usage:" line is the steady-state delta.
	out, err := runCmd("top", "-l", "2", "-n", "0", "-s", "1")
	if err != nil {
		return fmt.Errorf("top: %w", err)
	}
	usage, err := parseDarwinTopCPU(string(out))
	if err != nil {
		return err
	}
	m.CPU.UsagePct = usage
	m.CPU.UsagePerCore = nil
	return nil
}

var darwinCPUUsageRE = regexp.MustCompile(`CPU usage:\s*([\d.]+)% user,\s*([\d.]+)% sys,\s*([\d.]+)% idle`)

func parseDarwinTopCPU(out string) (float64, error) {
	// Use the LAST match (second sample) for steady-state.
	matches := darwinCPUUsageRE.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("no CPU usage line in top output")
	}
	last := matches[len(matches)-1]
	idle, err := strconv.ParseFloat(last[3], 64)
	if err != nil {
		return 0, fmt.Errorf("parse idle %q: %w", last[3], err)
	}
	usage := 100 - idle
	if usage < 0 {
		usage = 0
	} else if usage > 100 {
		usage = 100
	}
	return usage, nil
}

// ----- Load ------------------------------------------------------------------

type darwinLoadCollector struct{}

func (darwinLoadCollector) Name() string { return "load" }
func (darwinLoadCollector) Outputs() []string {
	return []string{"load_avg_1m", "load_avg_5m", "load_avg_15m"}
}
func (darwinLoadCollector) TTL() time.Duration { return 5 * time.Second }
func (darwinLoadCollector) Collect(m *Metrics) error {
	out, err := runCmd("sysctl", "-n", "vm.loadavg")
	if err != nil {
		return fmt.Errorf("sysctl vm.loadavg: %w", err)
	}
	a, b, c, err := parseDarwinLoadAvg(string(out))
	if err != nil {
		return err
	}
	m.Load.Avg1m, m.Load.Avg5m, m.Load.Avg15m = a, b, c
	return nil
}

func parseDarwinLoadAvg(out string) (float64, float64, float64, error) {
	// Format: "{ 1.23 1.45 1.67 }\n"
	s := strings.Trim(strings.TrimSpace(out), "{}")
	fields := strings.Fields(s)
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("vm.loadavg has %d fields, want 3: %q", len(fields), out)
	}
	a, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse 1m: %w", err)
	}
	b, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse 5m: %w", err)
	}
	c, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse 15m: %w", err)
	}
	return a, b, c, nil
}

// ----- Memory ----------------------------------------------------------------

type darwinMemCollector struct{}

func (darwinMemCollector) Name() string { return "mem" }
func (darwinMemCollector) Outputs() []string {
	return []string{"memory_used_mb", "memory_used_pct", "swap_used_mb"}
}
func (darwinMemCollector) TTL() time.Duration { return 5 * time.Second }
func (darwinMemCollector) Collect(m *Metrics) error {
	vmOut, err := runCmd("vm_stat")
	if err != nil {
		return fmt.Errorf("vm_stat: %w", err)
	}
	totalOut, err := runCmd("sysctl", "-n", "hw.memsize")
	if err != nil {
		return fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	swapOut, err := runCmd("sysctl", "-n", "vm.swapusage")
	if err != nil {
		return fmt.Errorf("sysctl vm.swapusage: %w", err)
	}
	usedMB, usedPct, err := parseDarwinMem(string(vmOut), string(totalOut))
	if err != nil {
		return err
	}
	swapMB, err := parseDarwinSwap(string(swapOut))
	if err != nil {
		return err
	}
	m.Memory.UsedMB = usedMB
	m.Memory.UsedPct = usedPct
	m.Memory.SwapUsedMB = swapMB
	return nil
}

var (
	darwinPageSizeRE = regexp.MustCompile(`page size of (\d+) bytes`)
	darwinPagesRE    = regexp.MustCompile(`(?m)^Pages ([^:]+):\s+(\d+)`)
)

func parseDarwinMem(vmStatOut, totalSysctlOut string) (usedMB int64, usedPct float64, err error) {
	pageSize := int64(4096)
	if m := darwinPageSizeRE.FindStringSubmatch(vmStatOut); len(m) == 2 {
		if ps, perr := strconv.ParseInt(m[1], 10, 64); perr == nil && ps > 0 {
			pageSize = ps
		}
	}
	pages := map[string]int64{}
	for _, m := range darwinPagesRE.FindAllStringSubmatch(vmStatOut, -1) {
		key := strings.ToLower(strings.TrimSpace(m[1]))
		v, perr := strconv.ParseInt(m[2], 10, 64)
		if perr == nil {
			pages[key] = v
		}
	}
	// "Used" on macOS = wired + active + (compressed). Inactive and
	// speculative are reclaimable; free + purgeable are clearly free.
	used := pages["wired down"] + pages["active"] + pages["occupied by compressor"]
	usedBytes := used * pageSize
	usedMB = usedBytes / (1024 * 1024)

	total, terr := strconv.ParseInt(strings.TrimSpace(totalSysctlOut), 10, 64)
	if terr != nil {
		return 0, 0, fmt.Errorf("parse hw.memsize %q: %w", totalSysctlOut, terr)
	}
	if total > 0 {
		usedPct = float64(usedBytes) / float64(total) * 100
	}
	return usedMB, usedPct, nil
}

var darwinSwapUsedRE = regexp.MustCompile(`used\s*=\s*([\d.]+)([KMG])`)

func parseDarwinSwap(out string) (int64, error) {
	m := darwinSwapUsedRE.FindStringSubmatch(out)
	if len(m) < 3 {
		// Swap may be disabled / format may have changed — not fatal.
		return 0, nil
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("parse swap used %q: %w", m[1], err)
	}
	switch m[2] {
	case "K":
		v /= 1024
	case "M":
		// already MB
	case "G":
		v *= 1024
	}
	return int64(v), nil
}

// ----- Network ---------------------------------------------------------------
//
// `netstat -ibn` emits one row per (interface, address-family). We want
// rolled-up per-interface counters, so take the first row for each iface
// that has bytes-in / bytes-out fields.

type darwinNetCollector struct{}

func (darwinNetCollector) Name() string       { return "net" }
func (darwinNetCollector) Outputs() []string  { return []string{"net_rx_bps", "net_tx_bps"} }
func (darwinNetCollector) TTL() time.Duration { return 2 * time.Second }
func (darwinNetCollector) Collect(m *Metrics) error {
	first, err := readNetstatIBN()
	if err != nil {
		return err
	}
	time.Sleep(netSampleSleep)
	second, err := readNetstatIBN()
	if err != nil {
		return err
	}
	rx, tx := computeNetBps(first, second, netSampleSleep)
	m.Network.RxBps = rx
	m.Network.TxBps = tx
	return nil
}

func readNetstatIBN() (netDevSample, error) {
	out, err := runCmd("netstat", "-ibn")
	if err != nil {
		return nil, fmt.Errorf("netstat -ibn: %w", err)
	}
	return parseNetstatIBN(string(out)), nil
}

// parseNetstatIBN parses BSD-style `netstat -ibn` output. Columns:
//
//	Name  Mtu Network  Address  Ipkts  Ierrs  Ibytes  Opkts  Oerrs  Obytes  Coll
//
// We want Ibytes (col 6, 0-indexed) and Obytes (col 9). Multiple rows can
// share an interface (one per address family); we keep the first.
func parseNetstatIBN(out string) netDevSample {
	s := netDevSample{}
	for i, line := range strings.Split(out, "\n") {
		if i == 0 {
			continue // header
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		iface := fields[0]
		if iface == "lo0" {
			continue
		}
		if _, dup := s[iface]; dup {
			continue
		}
		rx, err1 := strconv.ParseUint(fields[6], 10, 64)
		tx, err2 := strconv.ParseUint(fields[9], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		s[iface] = netCounters{rx: rx, tx: tx}
	}
	return s
}
