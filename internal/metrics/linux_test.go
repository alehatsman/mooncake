//go:build linux

package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestParseProcStat(t *testing.T) {
	s, err := parseProcStat(readFixture(t, "proc_stat_sample1.txt"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.agg.user != 10000 {
		t.Errorf("expected agg.user=10000, got %d", s.agg.user)
	}
	if len(s.perCore) != 4 {
		t.Errorf("expected 4 per-core entries, got %d", len(s.perCore))
	}
	if s.perCore[0].idle != 20000 {
		t.Errorf("expected cpu0 idle=20000, got %d", s.perCore[0].idle)
	}
}

func TestComputeCPUUsage(t *testing.T) {
	first, err := parseProcStat(readFixture(t, "proc_stat_sample1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseProcStat(readFixture(t, "proc_stat_sample2.txt"))
	if err != nil {
		t.Fatal(err)
	}
	agg, per := computeCPUUsage(first, second)
	if agg <= 0 || agg > 100 {
		t.Errorf("aggregate usage out of range: %f", agg)
	}
	if len(per) != 4 {
		t.Errorf("expected 4 per-core values, got %d", len(per))
	}
	// Sanity: with idle delta=800 out of total delta≈1060, usage should be
	// ~24-26% (busy/total). Check it's in a reasonable band.
	if agg < 15 || agg > 40 {
		t.Errorf("aggregate usage outside expected band [15,40]: %f", agg)
	}
}

func TestComputeCPUUsageZeroDelta(t *testing.T) {
	// Identical samples → 0% usage, not NaN, not a panic.
	s, _ := parseProcStat(readFixture(t, "proc_stat_sample1.txt"))
	agg, per := computeCPUUsage(s, s)
	if agg != 0 {
		t.Errorf("expected 0%% on identical samples, got %f", agg)
	}
	for i, p := range per {
		if p != 0 {
			t.Errorf("per[%d]: expected 0, got %f", i, p)
		}
	}
}

func TestParseLoadAvg(t *testing.T) {
	a, b, c, err := parseLoadAvg(readFixture(t, "proc_loadavg.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if a != 0.52 || b != 1.45 || c != 1.67 {
		t.Errorf("unexpected values: %v %v %v", a, b, c)
	}
}

func TestParseLoadAvgMalformed(t *testing.T) {
	if _, _, _, err := parseLoadAvg("only_one_field"); err == nil {
		t.Error("expected error for malformed input")
	}
}

func TestParseMemInfo(t *testing.T) {
	usedMB, usedPct, swapUsedMB, err := parseMemInfo(readFixture(t, "proc_meminfo.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Total = 32_768_000 kB, available = 16_000_000 kB → used = 16_768_000 kB → 16_375 MB
	if usedMB != 16375 {
		t.Errorf("expected usedMB=16375, got %d", usedMB)
	}
	// usedPct ≈ 16_768_000 / 32_768_000 * 100 ≈ 51.17
	if usedPct < 51 || usedPct > 52 {
		t.Errorf("expected usedPct≈51.17, got %f", usedPct)
	}
	// Swap is fully free in the fixture.
	if swapUsedMB != 0 {
		t.Errorf("expected swapUsedMB=0, got %d", swapUsedMB)
	}
}

func TestParseMemInfoFallbackWithoutMemAvailable(t *testing.T) {
	// Old kernels lacked MemAvailable; the parser should fall back to
	// free+buffers+cached.
	data := `MemTotal:       1000000 kB
MemFree:         200000 kB
Buffers:         100000 kB
Cached:          300000 kB
SwapTotal:            0 kB
SwapFree:             0 kB
`
	usedMB, _, _, err := parseMemInfo(data)
	if err != nil {
		t.Fatal(err)
	}
	// total=1_000_000, avail=200+100+300=600_000, used=400_000 kB = 390 MB
	if usedMB != 390 {
		t.Errorf("expected usedMB=390, got %d", usedMB)
	}
}

func TestParseProcNetDev(t *testing.T) {
	s := parseProcNetDev(readFixture(t, "proc_net_dev_sample1.txt"))
	if _, hasLoop := s["lo"]; hasLoop {
		t.Error("loopback should be excluded")
	}
	eth0, ok := s["eth0"]
	if !ok {
		t.Fatal("expected eth0 in sample")
	}
	if eth0.rx != 100000 || eth0.tx != 50000 {
		t.Errorf("eth0 counters wrong: rx=%d tx=%d", eth0.rx, eth0.tx)
	}
}

func TestComputeNetBps(t *testing.T) {
	first := parseProcNetDev(readFixture(t, "proc_net_dev_sample1.txt"))
	second := parseProcNetDev(readFixture(t, "proc_net_dev_sample2.txt"))
	rx, tx := computeNetBps(first, second, time.Second)
	// eth0: rx 10000, tx 5000. wlan0: rx 50000, tx 20000. Total: rx 60000, tx 25000.
	if rx != 60000 {
		t.Errorf("expected rx=60000, got %d", rx)
	}
	if tx != 25000 {
		t.Errorf("expected tx=25000, got %d", tx)
	}
}

func TestComputeNetBpsCounterWrap(t *testing.T) {
	// Second sample has smaller counter (wrap or iface reset) — must not
	// produce a negative delta.
	first := netDevSample{"eth0": {rx: 1_000_000, tx: 1_000_000}}
	second := netDevSample{"eth0": {rx: 100, tx: 100}}
	rx, tx := computeNetBps(first, second, time.Second)
	if rx != 0 || tx != 0 {
		t.Errorf("expected 0 bps on counter wrap, got rx=%d tx=%d", rx, tx)
	}
}

func TestParseNvidiaSmiCSV(t *testing.T) {
	gpus, err := parseNvidiaSmiCSV(readFixture(t, "nvidia_smi.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(gpus) != 2 {
		t.Fatalf("expected 2 GPUs, got %d", len(gpus))
	}
	// First row sorted by index.
	if gpus[0].Index != 0 || gpus[0].UsagePct != 87 {
		t.Errorf("gpus[0] wrong: %+v", gpus[0])
	}
	if gpus[0].MemoryUsedMB != 6400 {
		t.Errorf("expected gpus[0].MemoryUsedMB=6400, got %d", gpus[0].MemoryUsedMB)
	}
	// 6400/8192 ≈ 78.125%
	if gpus[0].MemoryUsedPct < 78 || gpus[0].MemoryUsedPct > 79 {
		t.Errorf("expected gpus[0].MemoryUsedPct≈78.1, got %f", gpus[0].MemoryUsedPct)
	}
	if gpus[1].TemperatureC != 55 {
		t.Errorf("expected gpus[1].TemperatureC=55, got %d", gpus[1].TemperatureC)
	}
}

func TestParseNvidiaSmiCSVEmpty(t *testing.T) {
	// nvidia-smi returns no rows on a system with the driver but no GPUs
	// — parser must return an empty slice without error.
	gpus, err := parseNvidiaSmiCSV("")
	if err != nil {
		t.Fatal(err)
	}
	if len(gpus) != 0 {
		t.Errorf("expected empty result, got %d", len(gpus))
	}
}

func TestNvidiaGPUCollectorSetsEmptySliceOnExecError(t *testing.T) {
	saved := runNvidiaSmi
	defer func() { runNvidiaSmi = saved }()
	runNvidiaSmi = func(string) ([]byte, error) {
		return nil, os.ErrNotExist // simulate exec failure
	}
	c := &nvidiaGPUCollector{path: "fake"}
	m := &Metrics{}
	if err := c.Collect(m); err != nil {
		t.Fatalf("expected nil error on exec failure, got %v", err)
	}
	if m.GPUs == nil {
		t.Error("expected GPUs to be initialized to empty slice, got nil")
	}
	if len(m.GPUs) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(m.GPUs))
	}
}
