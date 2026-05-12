package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// SnapshotDiff holds the difference between two SystemSnapshot values.
type SnapshotDiff struct {
	Tools    *MapDiff            `json:"tools,omitempty"`
	HW       map[string]ValDiff  `json:"hw,omitempty"`
	Services *SliceDiff          `json:"services_failed,omitempty"`
	OS       map[string]ValDiff  `json:"os,omitempty"`
}

// MapDiff records string-map changes (e.g., tool versions).
type MapDiff struct {
	Added   map[string]string  `json:"added,omitempty"`
	Changed map[string]ValDiff `json:"changed,omitempty"`
	Removed map[string]string  `json:"removed,omitempty"`
}

// ValDiff holds a before/after pair for a single field.
type ValDiff struct {
	From interface{} `json:"from"`
	To   interface{} `json:"to"`
}

// SliceDiff records slice membership changes.
type SliceDiff struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// Empty returns true when the diff contains no changes.
func (d *SnapshotDiff) Empty() bool {
	if d.Tools != nil && (len(d.Tools.Added) > 0 || len(d.Tools.Changed) > 0 || len(d.Tools.Removed) > 0) {
		return false
	}
	if len(d.HW) > 0 {
		return false
	}
	if d.Services != nil && (len(d.Services.Added) > 0 || len(d.Services.Removed) > 0) {
		return false
	}
	if len(d.OS) > 0 {
		return false
	}
	return true
}

// Diff computes the difference between prev and curr snapshots.
func Diff(prev, curr *SystemSnapshot) *SnapshotDiff {
	d := &SnapshotDiff{}

	// Tools
	d.Tools = diffStringMap(prev.Tools, curr.Tools)

	// HW — only dynamic fields (free memory, free disk)
	hw := make(map[string]ValDiff)
	if prev.HW.RAMFreeMB != curr.HW.RAMFreeMB {
		hw["ram_free_mb"] = ValDiff{From: prev.HW.RAMFreeMB, To: curr.HW.RAMFreeMB}
	}
	if prev.HW.DiskFreeGB != curr.HW.DiskFreeGB {
		hw["disk_free_gb"] = ValDiff{From: prev.HW.DiskFreeGB, To: curr.HW.DiskFreeGB}
	}
	if len(hw) > 0 {
		d.HW = hw
	}

	// Services.Failed
	d.Services = diffStringSlice(prev.Services.Failed, curr.Services.Failed)

	// OS (kernel, distro — static but useful to catch upgrades)
	osMap := make(map[string]ValDiff)
	if prev.OS.Kernel != curr.OS.Kernel {
		osMap["kernel"] = ValDiff{From: prev.OS.Kernel, To: curr.OS.Kernel}
	}
	if prev.OS.Distro != curr.OS.Distro {
		osMap["distro"] = ValDiff{From: prev.OS.Distro, To: curr.OS.Distro}
	}
	if len(osMap) > 0 {
		d.OS = osMap
	}

	return d
}

func diffStringMap(prev, curr map[string]string) *MapDiff {
	md := &MapDiff{
		Added:   make(map[string]string),
		Changed: make(map[string]ValDiff),
		Removed: make(map[string]string),
	}
	for k, v := range curr {
		if old, ok := prev[k]; !ok {
			md.Added[k] = v
		} else if old != v {
			md.Changed[k] = ValDiff{From: old, To: v}
		}
	}
	for k, v := range prev {
		if _, ok := curr[k]; !ok {
			md.Removed[k] = v
		}
	}
	if len(md.Added) == 0 && len(md.Changed) == 0 && len(md.Removed) == 0 {
		return nil
	}
	return md
}

func diffStringSlice(prev, curr []string) *SliceDiff {
	prevSet := make(map[string]bool)
	currSet := make(map[string]bool)
	for _, s := range prev {
		prevSet[s] = true
	}
	for _, s := range curr {
		currSet[s] = true
	}
	sd := &SliceDiff{}
	for s := range currSet {
		if !prevSet[s] {
			sd.Added = append(sd.Added, s)
		}
	}
	for s := range prevSet {
		if !currSet[s] {
			sd.Removed = append(sd.Removed, s)
		}
	}
	sort.Strings(sd.Added)
	sort.Strings(sd.Removed)
	if len(sd.Added) == 0 && len(sd.Removed) == 0 {
		return nil
	}
	return sd
}

// RenderDiffText renders a SnapshotDiff as human-readable text.
func RenderDiffText(d *SnapshotDiff) string {
	if d.Empty() {
		return "no changes"
	}

	var sb strings.Builder

	if d.Tools != nil {
		sb.WriteString("tools:\n")
		for _, k := range sortedKeys(d.Tools.Added) {
			fmt.Fprintf(&sb, "  + %-20s %s\n", k, d.Tools.Added[k])
		}
		for _, k := range sortedKeys2(d.Tools.Changed) {
			v := d.Tools.Changed[k]
			fmt.Fprintf(&sb, "  ~ %-20s %v  (was: %v)\n", k, v.To, v.From)
		}
		for _, k := range sortedKeys(d.Tools.Removed) {
			fmt.Fprintf(&sb, "  - %-20s (was: %s)\n", k, d.Tools.Removed[k])
		}
	}

	if len(d.OS) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("os:\n")
		for _, k := range sortedKeys2(d.OS) {
			v := d.OS[k]
			fmt.Fprintf(&sb, "  ~ %-20s %v  (was: %v)\n", k, v.To, v.From)
		}
	}

	if len(d.HW) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("hw:\n")
		for _, k := range sortedKeys2(d.HW) {
			v := d.HW[k]
			fmt.Fprintf(&sb, "  ~ %-20s %v → %v\n", k, v.From, v.To)
		}
	}

	if d.Services != nil {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("services.failed:\n")
		for _, s := range d.Services.Added {
			fmt.Fprintf(&sb, "  + %s\n", s)
		}
		for _, s := range d.Services.Removed {
			fmt.Fprintf(&sb, "  - %s\n", s)
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// RenderDiffJSON renders a SnapshotDiff as JSON bytes.
func RenderDiffJSON(d *SnapshotDiff) ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

// LoadSnapshot reads a SystemSnapshot from a JSON file.
func LoadSnapshot(path string) (*SystemSnapshot, error) {
	// #nosec G304 -- user-supplied path for snapshot diff is intentional
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file %s: %w", path, err)
	}
	var snap SystemSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot file %s: %w", path, err)
	}
	return &snap, nil
}

// SaveSnapshot writes a SystemSnapshot to a JSON file.
func SaveSnapshot(path string, snap *SystemSnapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	// #nosec G306 -- snapshot files do not contain secrets
	return os.WriteFile(path, data, 0o644)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys2(m map[string]ValDiff) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
