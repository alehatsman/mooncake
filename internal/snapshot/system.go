package snapshot

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/facts"
)

// SystemSnapshot is a compact view of the machine state.
type SystemSnapshot struct {
	TS       time.Time         `json:"ts"`
	OS       osInfo            `json:"os"`
	Host     hostInfo          `json:"host"`
	HW       hwInfo            `json:"hw"`
	Tools    map[string]string `json:"tools,omitempty"`
	Services serviceInfo       `json:"services,omitempty"`
}

type osInfo struct {
	Name    string `json:"name"`
	Distro  string `json:"distro,omitempty"`
	Kernel  string `json:"kernel,omitempty"`
	Arch    string `json:"arch"`
}

type hostInfo struct {
	Name     string `json:"name"`
	User     string `json:"user"`
	UptimeS  int64  `json:"uptime_s,omitempty"`
}

type hwInfo struct {
	CPUModel    string `json:"cpu_model,omitempty"`
	CPUCores    int    `json:"cpu_cores"`
	RAMTotalMB  int64  `json:"ram_total_mb"`
	RAMFreeMB   int64  `json:"ram_free_mb"`
	DiskTotalGB int64  `json:"disk_total_gb,omitempty"`
	DiskFreeGB  int64  `json:"disk_free_gb,omitempty"`
}

type serviceInfo struct {
	Failed  []string `json:"failed,omitempty"`
	Stopped []string `json:"stopped,omitempty"`
}

// CollectSystem builds a SystemSnapshot from already-collected Facts.
func CollectSystem(f *facts.Facts) *SystemSnapshot {
	snap := &SystemSnapshot{
		TS: time.Now().UTC(),
		OS: osInfo{
			Name:   f.OS,
			Distro: f.Distribution,
			Kernel: f.KernelVersion,
			Arch:   f.Arch,
		},
		Host: hostInfo{
			Name:    f.Hostname,
			User:    f.Username,
			UptimeS: f.UptimeSeconds,
		},
		HW: hwInfo{
			CPUModel:   f.CPUModel,
			CPUCores:   f.CPUCores,
			RAMTotalMB: f.MemoryTotalMB,
			RAMFreeMB:  f.MemoryFreeMB,
		},
		Services: serviceInfo{
			Failed:  f.FailedServices,
			Stopped: f.StoppedServices,
		},
	}

	// Primary disk (largest or first non-loop mount)
	for _, d := range f.Disks {
		if d.SizeGB > snap.HW.DiskTotalGB {
			snap.HW.DiskTotalGB = d.SizeGB
			snap.HW.DiskFreeGB = d.AvailGB
		}
	}

	// Merge all known tool versions: extended tools + legacy fields
	tools := make(map[string]string)
	for k, v := range f.Tools {
		if v != "" {
			tools[k] = v
		}
	}
	if f.GitVersion != "" {
		tools["git"] = f.GitVersion
	}
	if f.GoVersion != "" {
		tools["go"] = f.GoVersion
	}
	if f.DockerVersion != "" {
		tools["docker"] = f.DockerVersion
	}
	if f.PythonVersion != "" {
		tools["python"] = f.PythonVersion
	}
	if f.OllamaVersion != "" {
		tools["ollama"] = f.OllamaVersion
	}
	if len(tools) > 0 {
		snap.Tools = tools
	}

	return snap
}

// RenderText formats the snapshot as compact human/agent-readable text.
// The output is structured so each line is parseable but also readable.
// budget is the approximate token budget (1 token ≈ 4 chars); 0 = unlimited.
func (s *SystemSnapshot) RenderText(budget int) string {
	var b strings.Builder

	// --- line 1: os/kernel/host/user ---
	distro := s.OS.Name
	if s.OS.Distro != "" && s.OS.Distro != s.OS.Name {
		distro = s.OS.Name + "/" + s.OS.Distro
	}
	kernel := ""
	if s.OS.Kernel != "" {
		kernel = "  kernel: " + s.OS.Kernel
	}
	fmt.Fprintf(&b, "os: %s %s%s  host: %s  user: %s\n",
		distro, s.OS.Arch, kernel, s.Host.Name, s.Host.User)

	// --- line 2: hw ---
	cpu := s.HW.CPUModel
	if cpu == "" {
		cpu = fmt.Sprintf("%d cores", s.HW.CPUCores)
	} else {
		cpu = fmt.Sprintf("%d cores (%s)", s.HW.CPUCores, trimCPUModel(cpu))
	}
	ramFree := fmtMB(s.HW.RAMFreeMB)
	ramTotal := fmtMB(s.HW.RAMTotalMB)
	hw := fmt.Sprintf("hw: %s  ram: %s free / %s", cpu, ramFree, ramTotal)
	if s.HW.DiskTotalGB > 0 {
		hw += fmt.Sprintf("  disk: %dGB free / %dGB", s.HW.DiskFreeGB, s.HW.DiskTotalGB)
	}
	fmt.Fprintln(&b, hw)

	// --- line 3: uptime (drop if budget tight) ---
	uptime := ""
	if s.Host.UptimeS > 0 {
		uptime = fmt.Sprintf("uptime: %s", fmtUptime(s.Host.UptimeS))
	}

	if budget == 0 || estimateTokens(b.String()) < budget {
		if uptime != "" {
			fmt.Fprintln(&b, uptime)
		}
	}

	// --- tools section ---
	if len(s.Tools) > 0 && (budget == 0 || estimateTokens(b.String()) < budget) {
		fmt.Fprintln(&b, "\ntools:")
		// Sort tools for stable output
		names := make([]string, 0, len(s.Tools))
		for n := range s.Tools {
			names = append(names, n)
		}
		sort.Strings(names)

		// Lay out in rows of ~5 per line
		const cols = 5
		row := make([]string, 0, cols)
		for _, name := range names {
			if budget > 0 && estimateTokens(b.String()) >= budget {
				break
			}
			row = append(row, fmt.Sprintf("  %-8s %s", name+":", s.Tools[name]))
			if len(row) == cols {
				fmt.Fprintln(&b, strings.Join(row, "  "))
				row = row[:0]
			}
		}
		if len(row) > 0 {
			fmt.Fprintln(&b, strings.Join(row, "  "))
		}
	}

	// --- services section ---
	if len(s.Services.Failed) > 0 && (budget == 0 || estimateTokens(b.String()) < budget) {
		fmt.Fprintf(&b, "\nservices (failed): %s\n", strings.Join(s.Services.Failed, ", "))
	}
	if len(s.Services.Stopped) > 0 && (budget == 0 || estimateTokens(b.String()) < budget) {
		fmt.Fprintf(&b, "services (stopped): %s\n", strings.Join(s.Services.Stopped, ", "))
	}

	return strings.TrimRight(b.String(), "\n")
}

// RenderJSON returns the snapshot as pretty-printed JSON.
func (s *SystemSnapshot) RenderJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// estimateTokens estimates token count as chars/4.
func estimateTokens(s string) int {
	return len(s) / 4
}

// fmtMB formats MB as "X.XGB" or "XXXXXMB".
func fmtMB(mb int64) string {
	if mb >= 1024 {
		gb := float64(mb) / 1024
		return fmt.Sprintf("%.1fGB", gb)
	}
	return fmt.Sprintf("%dMB", mb)
}

// fmtUptime converts seconds to "Xd Xh" or "Xh Xm" or "Xm".
func fmtUptime(s int64) string {
	d := s / 86400
	h := (s % 86400) / 3600
	m := (s % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh", d, h)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// trimCPUModel shortens verbose CPU model strings.
func trimCPUModel(model string) string {
	// Strip "(R)", "(TM)", "CPU @", excess whitespace
	r := strings.NewReplacer("(R)", "", "(TM)", "", " CPU", "", "  ", " ")
	model = r.Replace(model)
	// Truncate at 30 chars
	if len(model) > 30 {
		model = model[:30]
	}
	return strings.TrimSpace(model)
}
