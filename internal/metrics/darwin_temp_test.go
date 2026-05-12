//go:build darwin

package metrics

import (
	"errors"
	"testing"
)

func TestParsePowermetricsIntel(t *testing.T) {
	sensors := parsePowermetricsSMC(readFixture(t, "darwin_powermetrics_intel.txt"))
	if len(sensors) != 4 {
		t.Fatalf("expected 4 sensors (CPU die, GPU die, CPU heat sink, Battery), got %d: %+v", len(sensors), sensors)
	}
	wantLabels := map[string]float64{
		"CPU die":       51.5,
		"GPU die":       47.25,
		"CPU heat sink": 48.0,
		"Battery":       31.0,
	}
	for _, s := range sensors {
		if s.Chip != "smc" {
			t.Errorf("expected Chip=smc, got %q", s.Chip)
		}
		want, ok := wantLabels[s.Label]
		if !ok {
			t.Errorf("unexpected sensor label %q", s.Label)
			continue
		}
		if s.TempC != want {
			t.Errorf("%s: got %v, want %v", s.Label, s.TempC, want)
		}
	}
}

func TestParsePowermetricsArmEmpty(t *testing.T) {
	// Apple Silicon: powermetrics doesn't expose SMC die temps. Parser must
	// return an empty (non-nil) slice without misidentifying "Plimit" or
	// "Fan: not present" as temperatures.
	sensors := parsePowermetricsSMC(readFixture(t, "darwin_powermetrics_arm.txt"))
	if sensors == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(sensors) != 0 {
		t.Errorf("expected 0 sensors on Apple Silicon, got %d: %+v", len(sensors), sensors)
	}
}

func TestParsePowermetricsIgnoresNonTempLines(t *testing.T) {
	// "Plimit" and "prochots" contain colons but no "temperature:" — must
	// not match.
	input := `CPU die temperature: 50.0 C
CPU Plimit: 0.00
Number of prochots: 0
Some other field: 12345`
	sensors := parsePowermetricsSMC(input)
	if len(sensors) != 1 {
		t.Fatalf("expected 1 sensor, got %d: %+v", len(sensors), sensors)
	}
	if sensors[0].Label != "CPU die" {
		t.Errorf("unexpected label: %q", sensors[0].Label)
	}
}

func TestParsePowermetricsRejectsBogusValues(t *testing.T) {
	// Unconnected SMC probes can return wild values; the parser drops them.
	input := `Phantom sensor temperature: -127.00 C
CPU die temperature: 50.0 C
Way too hot temperature: 999.99 C`
	sensors := parsePowermetricsSMC(input)
	if len(sensors) != 1 {
		t.Fatalf("expected 1 valid sensor, got %d: %+v", len(sensors), sensors)
	}
}

func TestPickDarwinCPUTempPrefersDie(t *testing.T) {
	sensors := []Sensor{
		{Chip: "smc", Label: "CPU heat sink", TempC: 48},
		{Chip: "smc", Label: "CPU die", TempC: 55},
		{Chip: "smc", Label: "GPU die", TempC: 47},
	}
	if got := pickDarwinCPUTemp(sensors); got != 55 {
		t.Errorf("expected CPU die 55, got %v", got)
	}
}

func TestPickDarwinCPUTempFallsBackToHeatSink(t *testing.T) {
	sensors := []Sensor{
		{Chip: "smc", Label: "CPU heat sink", TempC: 48},
		{Chip: "smc", Label: "GPU die", TempC: 47},
	}
	if got := pickDarwinCPUTemp(sensors); got != 48 {
		t.Errorf("expected CPU heat sink 48, got %v", got)
	}
}

func TestPickDarwinCPUTempZeroOnAppleSilicon(t *testing.T) {
	// Apple Silicon: no CPU temp sensors in powermetrics output.
	if got := pickDarwinCPUTemp(nil); got != 0 {
		t.Errorf("expected 0 on nil input, got %v", got)
	}
	if got := pickDarwinCPUTemp([]Sensor{}); got != 0 {
		t.Errorf("expected 0 on empty input, got %v", got)
	}
}

func TestDarwinTempCollectorHandlesPermissionDenied(t *testing.T) {
	// Simulate the non-root case: powermetrics exits non-zero. Collector
	// must not propagate the error.
	saved := runPowermetrics
	defer func() { runPowermetrics = saved }()
	runPowermetrics = func() ([]byte, error) {
		return nil, errors.New("Process must be run as root")
	}
	c := darwinTempCollector{}
	m := &Metrics{}
	if err := c.Collect(m); err != nil {
		t.Fatalf("expected nil error on perm denied, got %v", err)
	}
	if m.Temperatures == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(m.Temperatures) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(m.Temperatures))
	}
	if m.CPUTempC != 0 {
		t.Errorf("expected CPUTempC=0, got %v", m.CPUTempC)
	}
}
