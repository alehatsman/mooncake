//go:build linux

package metrics

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const sysHwmonRoot = "/sys/class/hwmon"

func init() {
	Register(&linuxTempCollector{root: sysHwmonRoot})
}

type linuxTempCollector struct {
	root string
}

func (linuxTempCollector) Name() string       { return "temp" }
func (linuxTempCollector) Outputs() []string  { return []string{"temperatures", "cpu_temp_c"} }
func (linuxTempCollector) TTL() time.Duration { return 2 * time.Second }

func (c *linuxTempCollector) Collect(m *Metrics) error {
	sensors, err := readHwmonSensors(c.root)
	if err != nil {
		// hwmon may be entirely absent in some containers; treat as "no
		// sensors" rather than a hard error so the collector stays quiet.
		m.Temperatures = []Sensor{}
		m.CPUTempC = 0
		return nil
	}
	m.Temperatures = sensors
	m.CPUTempC = pickCPUTemp(sensors)
	return nil
}

// readHwmonSensors walks each /sys/class/hwmon/hwmon* directory and emits
// one Sensor per tempN_input file found. The result is sorted by chip name
// then label for deterministic ordering.
func readHwmonSensors(root string) ([]Sensor, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Sensor
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "hwmon") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		chip := readTrim(filepath.Join(dir, "name"))
		if chip == "" {
			continue
		}
		out = append(out, readHwmonDir(dir, chip)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Chip != out[j].Chip {
			return out[i].Chip < out[j].Chip
		}
		return out[i].Label < out[j].Label
	})
	return out, nil
}

// readHwmonDir scans one hwmon directory for tempN_input files and reads
// their input / label / crit siblings.
func readHwmonDir(dir, chip string) []Sensor {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Sensor
	for _, f := range files {
		name := f.Name()
		if !strings.HasPrefix(name, "temp") || !strings.HasSuffix(name, "_input") {
			continue
		}
		idx := strings.TrimSuffix(strings.TrimPrefix(name, "temp"), "_input")
		tempMC, ok := readMilliDeg(filepath.Join(dir, name))
		if !ok {
			continue
		}
		label := readTrim(filepath.Join(dir, "temp"+idx+"_label"))
		crit, _ := readMilliDeg(filepath.Join(dir, "temp"+idx+"_crit"))
		out = append(out, Sensor{
			Chip:  chip,
			Label: label,
			TempC: tempMC,
			CritC: crit,
		})
	}
	return out
}

func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readMilliDeg(path string) (float64, bool) {
	s := readTrim(path)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return float64(v) / 1000.0, true
}

// pickCPUTemp returns the best "CPU temperature" from a sensor list, or 0
// if no obvious CPU sensor is present. Ordering of attempts:
//  1. Intel coretemp / AMD k10temp / AMD zenpower: prefer the "Package"
//     or "Tctl" label; fall back to the highest-temp core.
//  2. ARM `cpu_thermal`: first reading.
//  3. Generic `acpitz` with an obviously CPU-related label.
func pickCPUTemp(sensors []Sensor) float64 {
	for _, want := range []string{"coretemp", "k10temp", "zenpower"} {
		if t, ok := bestCPUFromChip(sensors, want); ok {
			return t
		}
	}
	for _, s := range sensors {
		if s.Chip == "cpu_thermal" {
			return s.TempC
		}
	}
	return 0
}

// bestCPUFromChip picks the most authoritative CPU temperature from a single
// chip's sensor entries. Priority order:
//
//	Tctl (AMD throttle-control temp)        ← 3, strongest match
//	Package id 0 (Intel CPU package)        ← 2
//	Tdie (AMD die temp, fan-curve fallback) ← 1
//	max(Core *)                             ← used only if no package-level reading
//
// Multiple matches at the same priority keep the first; higher priority always
// overrides lower.
func bestCPUFromChip(sensors []Sensor, chip string) (float64, bool) {
	pkgPriority := 0
	var pkg float64
	var maxCore float64
	coreFound := false
	for _, s := range sensors {
		if s.Chip != chip {
			continue
		}
		lbl := strings.ToLower(s.Label)
		switch {
		case lbl == "tctl" && pkgPriority < 3:
			pkg, pkgPriority = s.TempC, 3
		case strings.HasPrefix(lbl, "package") && pkgPriority < 2:
			pkg, pkgPriority = s.TempC, 2
		case lbl == "tdie" && pkgPriority < 1:
			pkg, pkgPriority = s.TempC, 1
		case strings.HasPrefix(lbl, "core"):
			if !coreFound || s.TempC > maxCore {
				maxCore, coreFound = s.TempC, true
			}
		}
	}
	if pkgPriority > 0 {
		return pkg, true
	}
	if coreFound {
		return maxCore, true
	}
	return 0, false
}
