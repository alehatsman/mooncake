//go:build darwin

package metrics

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func init() {
	// powermetrics is part of the macOS base install; LookPath should succeed
	// on every Mac. We still gate registration on it so future stripped-down
	// images don't break the collector contract.
	if _, err := exec.LookPath("powermetrics"); err != nil {
		return
	}
	Register(&darwinTempCollector{})
}

type darwinTempCollector struct{}

func (darwinTempCollector) Name() string       { return "temp" }
func (darwinTempCollector) Outputs() []string  { return []string{"temperatures", "cpu_temp_c"} }
func (darwinTempCollector) TTL() time.Duration { return 5 * time.Second }

// runPowermetrics is overridable in tests.
var runPowermetrics = func() ([]byte, error) {
	// #nosec G204 -- arguments are fixed string literals.
	return exec.Command("powermetrics",
		"--samplers", "smc",
		"-n", "1",
		"-i", "100",
	).Output()
}

func (darwinTempCollector) Collect(m *Metrics) error {
	out, err := runPowermetrics()
	if err != nil {
		// powermetrics requires root. When the CLI is invoked by a regular
		// user it exits with "Process must be run as root" — surface this as
		// "no sensors" rather than a hard error so the metrics surface stays
		// consistent. The agent daemon (Spec 18) runs as root and gets real
		// data; user-shell invocations get empty.
		m.Temperatures = []Sensor{}
		m.CPUTempC = 0
		return nil
	}
	sensors := parsePowermetricsSMC(string(out))
	m.Temperatures = sensors
	m.CPUTempC = pickDarwinCPUTemp(sensors)
	return nil
}

// powermetricsTempRE matches lines like:
//
//	CPU die temperature: 51.50 C
//	GPU die temperature: 47.25 C
//	CPU heat sink temperature: 48.00 C
//	Battery temperature: 31.00 C
//
// Some sensor names contain "Plimit" or "prochots" — those are not
// temperatures and must not match. Anchoring on "temperature:" filters them.
var powermetricsTempRE = regexp.MustCompile(`(?m)^(.+?) temperature:\s*([\-\d.]+)\s*C\b`)

func parsePowermetricsSMC(out string) []Sensor {
	var sensors []Sensor
	for _, m := range powermetricsTempRE.FindAllStringSubmatch(out, -1) {
		label := strings.TrimSpace(m[1])
		tempC, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		// Ignore obviously bogus readings (some sensors return ridiculous
		// values when unpopulated — e.g. -127 C for an unconnected probe).
		if tempC < -50 || tempC > 150 {
			continue
		}
		sensors = append(sensors, Sensor{
			Chip:  "smc",
			Label: label,
			TempC: tempC,
		})
	}
	if sensors == nil {
		return []Sensor{}
	}
	return sensors
}

// pickDarwinCPUTemp returns the CPU temperature from SMC output. Preference:
//
//	"CPU die temperature"        (Intel — direct silicon reading)
//	"CPU heat sink temperature"  (fallback)
//	0 otherwise (Apple Silicon — powermetrics doesn't expose die temps)
func pickDarwinCPUTemp(sensors []Sensor) float64 {
	for _, want := range []string{"CPU die", "CPU heat sink", "CPU"} {
		for _, s := range sensors {
			if s.Chip != "smc" {
				continue
			}
			if strings.HasPrefix(s.Label, want) {
				return s.TempC
			}
		}
	}
	return 0
}
