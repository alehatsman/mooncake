//go:build linux

package metrics

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const cpuSampleSleep = 100 * time.Millisecond

func init() {
	Register(&linuxCPUCollector{})
	Register(&linuxLoadCollector{})
	Register(&linuxMemCollector{})
	Register(&linuxNetCollector{})
}

// ----- CPU -------------------------------------------------------------------

type linuxCPUCollector struct{}

func (linuxCPUCollector) Name() string             { return "cpu" }
func (linuxCPUCollector) Outputs() []string        { return []string{"cpu_usage_pct", "cpu_usage_per_core"} }
func (linuxCPUCollector) TTL() time.Duration       { return 2 * time.Second }
func (linuxCPUCollector) Collect(m *Metrics) error {
	first, err := readProcStat()
	if err != nil {
		return err
	}
	time.Sleep(cpuSampleSleep)
	second, err := readProcStat()
	if err != nil {
		return err
	}
	agg, per := computeCPUUsage(first, second)
	m.CPU.UsagePct = agg
	m.CPU.UsagePerCore = per
	return nil
}

// cpuTimes is the parsed fields after "cpu" / "cpuN" prefix from /proc/stat.
type cpuTimes struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

func (c cpuTimes) total() uint64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

func (c cpuTimes) busy() uint64 {
	return c.total() - c.idle - c.iowait
}

// procStatSample is one read of /proc/stat: aggregate + per-cpu times.
type procStatSample struct {
	agg     cpuTimes
	perCore []cpuTimes
}

func readProcStat() (procStatSample, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return procStatSample{}, err
	}
	return parseProcStat(string(data))
}

func parseProcStat(data string) (procStatSample, error) {
	var s procStatSample
	for _, line := range strings.Split(data, "\n") {
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		t, err := parseCPUTimes(fields[1:])
		if err != nil {
			return procStatSample{}, err
		}
		if fields[0] == "cpu" {
			s.agg = t
		} else {
			s.perCore = append(s.perCore, t)
		}
	}
	return s, nil
}

func parseCPUTimes(fields []string) (cpuTimes, error) {
	vals := make([]uint64, 8)
	for i := 0; i < 8 && i < len(fields); i++ {
		v, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return cpuTimes{}, fmt.Errorf("parse cpu time %q: %w", fields[i], err)
		}
		vals[i] = v
	}
	return cpuTimes{
		user: vals[0], nice: vals[1], system: vals[2], idle: vals[3],
		iowait: vals[4], irq: vals[5], softirq: vals[6], steal: vals[7],
	}, nil
}

func computeCPUUsage(first, second procStatSample) (agg float64, per []float64) {
	agg = cpuUsagePct(first.agg, second.agg)
	n := len(second.perCore)
	if len(first.perCore) < n {
		n = len(first.perCore)
	}
	per = make([]float64, n)
	for i := 0; i < n; i++ {
		per[i] = cpuUsagePct(first.perCore[i], second.perCore[i])
	}
	return
}

func cpuUsagePct(a, b cpuTimes) float64 {
	totalDelta := float64(b.total() - a.total())
	if totalDelta <= 0 {
		return 0
	}
	busyDelta := float64(b.busy() - a.busy())
	pct := (busyDelta / totalDelta) * 100
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	return pct
}

// ----- Load ------------------------------------------------------------------

type linuxLoadCollector struct{}

func (linuxLoadCollector) Name() string             { return "load" }
func (linuxLoadCollector) Outputs() []string        { return []string{"load_avg_1m", "load_avg_5m", "load_avg_15m"} }
func (linuxLoadCollector) TTL() time.Duration       { return 5 * time.Second }
func (linuxLoadCollector) Collect(m *Metrics) error {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return err
	}
	a, b, c, err := parseLoadAvg(string(data))
	if err != nil {
		return err
	}
	m.Load.Avg1m, m.Load.Avg5m, m.Load.Avg15m = a, b, c
	return nil
}

func parseLoadAvg(data string) (float64, float64, float64, error) {
	fields := strings.Fields(data)
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("loadavg has %d fields, want >=3", len(fields))
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

type linuxMemCollector struct{}

func (linuxMemCollector) Name() string       { return "mem" }
func (linuxMemCollector) Outputs() []string  { return []string{"memory_used_mb", "memory_used_pct", "swap_used_mb"} }
func (linuxMemCollector) TTL() time.Duration { return 5 * time.Second }
func (linuxMemCollector) Collect(m *Metrics) error {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return err
	}
	usedMB, usedPct, swapUsedMB, err := parseMemInfo(string(data))
	if err != nil {
		return err
	}
	m.Memory.UsedMB = usedMB
	m.Memory.UsedPct = usedPct
	m.Memory.SwapUsedMB = swapUsedMB
	return nil
}

func parseMemInfo(data string) (usedMB int64, usedPct float64, swapUsedMB int64, err error) {
	fields := map[string]int64{}
	for _, line := range strings.Split(data, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		v, perr := strconv.ParseInt(parts[1], 10, 64)
		if perr != nil {
			continue
		}
		fields[key] = v // kB
	}
	total := fields["MemTotal"]
	avail, hasAvail := fields["MemAvailable"]
	if total == 0 {
		return 0, 0, 0, fmt.Errorf("/proc/meminfo missing MemTotal")
	}
	if !hasAvail {
		// Fallback for very old kernels.
		avail = fields["MemFree"] + fields["Buffers"] + fields["Cached"]
	}
	used := total - avail
	usedMB = used / 1024
	usedPct = float64(used) / float64(total) * 100
	swapTotal := fields["SwapTotal"]
	swapFree := fields["SwapFree"]
	swapUsedMB = (swapTotal - swapFree) / 1024
	if swapUsedMB < 0 {
		swapUsedMB = 0
	}
	return
}

// ----- Network ---------------------------------------------------------------

type linuxNetCollector struct{}

func (linuxNetCollector) Name() string             { return "net" }
func (linuxNetCollector) Outputs() []string        { return []string{"net_rx_bps", "net_tx_bps"} }
func (linuxNetCollector) TTL() time.Duration       { return 2 * time.Second }
func (linuxNetCollector) Collect(m *Metrics) error {
	first, err := readProcNetDev()
	if err != nil {
		return err
	}
	time.Sleep(netSampleSleep)
	second, err := readProcNetDev()
	if err != nil {
		return err
	}
	rxBps, txBps := computeNetBps(first, second, netSampleSleep)
	m.Network.RxBps = rxBps
	m.Network.TxBps = txBps
	return nil
}

func readProcNetDev() (netDevSample, error) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	return parseProcNetDev(string(data)), nil
}

func parseProcNetDev(data string) netDevSample {
	s := netDevSample{}
	for _, line := range strings.Split(data, "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue // header lines
		}
		iface := strings.TrimSpace(line[:idx])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 16 {
			continue
		}
		rx, err1 := strconv.ParseUint(fields[0], 10, 64)
		tx, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		s[iface] = netCounters{rx: rx, tx: tx}
	}
	return s
}

