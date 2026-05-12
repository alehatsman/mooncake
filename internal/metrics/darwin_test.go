//go:build darwin

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

func TestParseDarwinTopCPU(t *testing.T) {
	// Fixture has two "CPU usage:" lines; parser must take the second.
	usage, err := parseDarwinTopCPU(readFixture(t, "darwin_top.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Second sample: idle=80% → usage=20%.
	if usage < 19 || usage > 21 {
		t.Errorf("expected usage≈20, got %f", usage)
	}
}

func TestParseDarwinTopCPUNoMatches(t *testing.T) {
	if _, err := parseDarwinTopCPU("no relevant lines here"); err == nil {
		t.Error("expected error when no CPU usage line present")
	}
}

func TestParseDarwinLoadAvg(t *testing.T) {
	a, b, c, err := parseDarwinLoadAvg(readFixture(t, "darwin_loadavg.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if a != 1.23 || b != 1.45 || c != 1.67 {
		t.Errorf("unexpected values: %v %v %v", a, b, c)
	}
}

func TestParseDarwinMem(t *testing.T) {
	usedMB, usedPct, err := parseDarwinMem(
		readFixture(t, "darwin_vm_stat.txt"),
		"34359738368\n", // 32 GiB exact
	)
	if err != nil {
		t.Fatal(err)
	}
	// page size = 16384 bytes; used pages = wired(20000) + active(50000) + compressed(4000) = 74000
	// used bytes = 74000 * 16384 = 1_212_416_000 → 1156 MB
	if usedMB != 1156 {
		t.Errorf("expected usedMB=1156, got %d", usedMB)
	}
	// usedPct ≈ 1_212_416_000 / 34_359_738_368 * 100 ≈ 3.53
	if usedPct < 3 || usedPct > 4 {
		t.Errorf("expected usedPct≈3.5, got %f", usedPct)
	}
}

func TestParseDarwinSwap(t *testing.T) {
	usedMB, err := parseDarwinSwap(readFixture(t, "darwin_swap.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if usedMB != 256 {
		t.Errorf("expected swap used 256 MB, got %d", usedMB)
	}
}

func TestParseDarwinSwapDisabled(t *testing.T) {
	// When swap is disabled the format may omit the "used = X" portion.
	usedMB, err := parseDarwinSwap("swap disabled\n")
	if err != nil {
		t.Fatal(err)
	}
	if usedMB != 0 {
		t.Errorf("expected 0 when swap disabled, got %d", usedMB)
	}
}

func TestParseNetstatIBN(t *testing.T) {
	s := parseNetstatIBN(readFixture(t, "darwin_netstat.txt"))
	if _, hasLoop := s["lo0"]; hasLoop {
		t.Error("loopback should be excluded")
	}
	en0, ok := s["en0"]
	if !ok {
		t.Fatal("expected en0 in sample")
	}
	// Only the first en0 row should be kept (Link# row, rx=100000 tx=50000).
	if en0.rx != 100000 || en0.tx != 50000 {
		t.Errorf("en0 counters wrong: rx=%d tx=%d", en0.rx, en0.tx)
	}
}

func TestComputeNetBpsDarwin(t *testing.T) {
	first := netDevSample{"en0": {rx: 1000, tx: 500}}
	second := netDevSample{"en0": {rx: 5000, tx: 2500}}
	rx, tx := computeNetBps(first, second, time.Second)
	if rx != 4000 || tx != 2000 {
		t.Errorf("unexpected: rx=%d tx=%d", rx, tx)
	}
}
