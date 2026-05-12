//go:build linux

package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

// mkHwmon writes a fake hwmon directory under root with the given files.
// File values are strings written as-is.
func mkHwmon(t *testing.T, root, name string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for fname, content := range files {
		if err := os.WriteFile(filepath.Join(dir, fname), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadHwmonSensors(t *testing.T) {
	root := t.TempDir()

	mkHwmon(t, root, "hwmon0", map[string]string{
		"name":        "coretemp",
		"temp1_input": "55000",
		"temp1_label": "Package id 0",
		"temp1_crit":  "105000",
		"temp2_input": "52000",
		"temp2_label": "Core 0",
		"temp3_input": "57000",
		"temp3_label": "Core 1",
	})
	mkHwmon(t, root, "hwmon1", map[string]string{
		"name":        "nvme",
		"temp1_input": "42000",
		"temp1_label": "Composite",
	})
	// hwmon2 has no `name` file — must be skipped, not crashed on.
	if err := os.Mkdir(filepath.Join(root, "hwmon2"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-hwmon directory must be ignored.
	if err := os.Mkdir(filepath.Join(root, "unrelated"), 0o755); err != nil {
		t.Fatal(err)
	}

	sensors, err := readHwmonSensors(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sensors) != 4 {
		t.Fatalf("expected 4 sensors (3 coretemp + 1 nvme), got %d: %+v", len(sensors), sensors)
	}

	// Sorted by chip then label → coretemp first, then nvme.
	if sensors[0].Chip != "coretemp" || sensors[0].Label != "Core 0" {
		t.Errorf("sensors[0] = %+v, want coretemp/Core 0", sensors[0])
	}
	if sensors[0].TempC != 52 {
		t.Errorf("sensors[0].TempC = %v, want 52", sensors[0].TempC)
	}
	if sensors[0].CritC != 0 {
		t.Errorf("sensors[0].CritC = %v, want 0 (no crit file for Core 0)", sensors[0].CritC)
	}

	// Package id 0 should have CritC=105.
	var pkg *Sensor
	for i := range sensors {
		if sensors[i].Label == "Package id 0" {
			pkg = &sensors[i]
			break
		}
	}
	if pkg == nil {
		t.Fatal("missing Package id 0 sensor")
	}
	if pkg.TempC != 55 || pkg.CritC != 105 {
		t.Errorf("Package sensor wrong: %+v", *pkg)
	}

	if sensors[3].Chip != "nvme" || sensors[3].Label != "Composite" {
		t.Errorf("sensors[3] = %+v, want nvme/Composite", sensors[3])
	}
}

func TestPickCPUTempPrefersPackage(t *testing.T) {
	sensors := []Sensor{
		{Chip: "coretemp", Label: "Core 0", TempC: 60},
		{Chip: "coretemp", Label: "Package id 0", TempC: 55},
		{Chip: "coretemp", Label: "Core 1", TempC: 65},
		{Chip: "nvme", Label: "Composite", TempC: 42},
	}
	if got := pickCPUTemp(sensors); got != 55 {
		t.Errorf("expected Package temp 55, got %v", got)
	}
}

func TestPickCPUTempFallsBackToMaxCore(t *testing.T) {
	// No Package label: pick the hottest core.
	sensors := []Sensor{
		{Chip: "coretemp", Label: "Core 0", TempC: 60},
		{Chip: "coretemp", Label: "Core 1", TempC: 75},
		{Chip: "coretemp", Label: "Core 2", TempC: 65},
	}
	if got := pickCPUTemp(sensors); got != 75 {
		t.Errorf("expected max core 75, got %v", got)
	}
}

func TestPickCPUTempAMDTctl(t *testing.T) {
	sensors := []Sensor{
		{Chip: "k10temp", Label: "Tctl", TempC: 48},
		{Chip: "k10temp", Label: "Tdie", TempC: 47},
		{Chip: "nvme", Label: "Composite", TempC: 40},
	}
	if got := pickCPUTemp(sensors); got != 48 {
		t.Errorf("expected Tctl 48, got %v", got)
	}
}

func TestPickCPUTempCPUThermal(t *testing.T) {
	// ARM-style: no Package/Tctl, just one cpu_thermal entry.
	sensors := []Sensor{
		{Chip: "cpu_thermal", Label: "", TempC: 65},
		{Chip: "nvme", Label: "Composite", TempC: 40},
	}
	if got := pickCPUTemp(sensors); got != 65 {
		t.Errorf("expected cpu_thermal 65, got %v", got)
	}
}

func TestPickCPUTempReturnsZeroWhenNoCPUSensor(t *testing.T) {
	sensors := []Sensor{
		{Chip: "nvme", Label: "Composite", TempC: 40},
		{Chip: "acpitz", Label: "", TempC: 50},
	}
	if got := pickCPUTemp(sensors); got != 0 {
		t.Errorf("expected 0, got %v", got)
	}
}

func TestLinuxTempCollectorMissingHwmon(t *testing.T) {
	// hwmon root doesn't exist — collector should not error.
	c := &linuxTempCollector{root: filepath.Join(t.TempDir(), "no_such_dir")}
	m := &Metrics{}
	if err := c.Collect(m); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if m.Temperatures == nil {
		t.Error("expected empty slice, got nil")
	}
	if m.CPUTempC != 0 {
		t.Errorf("expected CPUTempC=0, got %v", m.CPUTempC)
	}
}
