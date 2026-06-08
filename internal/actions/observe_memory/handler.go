// Package observe_memory implements the observe.memory action: single-
// shot read of RAM + swap state (spec-60). Reads /proc/meminfo on
// Linux, sysctl on macOS, GlobalMemoryStatusEx on Windows.
//
// Distinct from the metrics package because internal/metrics tracks
// only Used; this handler returns Total / Used / Free / Available /
// SwapTotal / SwapUsed in bytes so callers can branch on absolute
// thresholds ("> 1 GiB free") without doing the percentage math.
package observe_memory

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const actionName = "observe.memory"

// MemoryObservation is the typed Value payload for observe.memory.
// All byte counts are absolute (not MB) so threshold comparisons
// against typed values work without unit math.
type MemoryObservation struct {
	TotalBytes     int64 `json:"total_bytes"`
	UsedBytes      int64 `json:"used_bytes"`
	FreeBytes      int64 `json:"free_bytes"`
	AvailableBytes int64 `json:"available_bytes"`
	SwapTotalBytes int64 `json:"swap_total_bytes,omitempty"`
	SwapUsedBytes  int64 `json:"swap_used_bytes,omitempty"`
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot read of memory + swap state (total/used/free/available)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.ObserveMemory == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, _ *config.Step) (actions.Result, error) {
	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	if ctx.Mode() == actions.ModePlan {
		env := actions.PlanDeferred(MemoryObservation{})
		result.PublishObservation(env, actions.ObserveTargetHost)
		result.Checkable = true
		result.Reason = "would observe memory state (deferred to apply)"
		return result, nil
	}

	obs, err := readMemory()
	env := actions.ObserveResult{
		Found: err == nil,
		Value: obs,
		AsOf:  time.Now(),
	}
	if err != nil {
		env.Error = err.Error()
	}
	result.PublishObservation(env, actions.ObserveTargetHost)
	return result, nil
}

// readMemory dispatches per OS. Each path returns absolute byte
// counts to keep the typed payload uniform; the underlying source
// (kB on Linux, pages on macOS, bytes on Windows) is normalized here.
func readMemory() (MemoryObservation, error) {
	switch runtime.GOOS {
	case "linux":
		return readMemoryLinux()
	case "darwin":
		return readMemoryDarwin()
	}
	return MemoryObservation{}, fmt.Errorf("observe.memory not implemented on %s", runtime.GOOS)
}

// readMemoryLinux parses /proc/meminfo. Keys are in kB; we multiply
// by 1024 to yield bytes.
func readMemoryLinux() (MemoryObservation, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemoryObservation{}, err
	}
	defer f.Close() //nolint:errcheck

	want := map[string]int64{
		"MemTotal":     0,
		"MemFree":      0,
		"MemAvailable": 0,
		"SwapTotal":    0,
		"SwapFree":     0,
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		colon := strings.Index(line, ":")
		if colon <= 0 {
			continue
		}
		key := line[:colon]
		if _, ok := want[key]; !ok {
			continue
		}
		rest := strings.TrimSpace(line[colon+1:])
		// Format: "12345 kB" — drop unit suffix.
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		want[key] = n * 1024
	}
	if err := scanner.Err(); err != nil {
		return MemoryObservation{}, err
	}

	obs := MemoryObservation{
		TotalBytes:     want["MemTotal"],
		FreeBytes:      want["MemFree"],
		AvailableBytes: want["MemAvailable"],
		SwapTotalBytes: want["SwapTotal"],
		SwapUsedBytes:  want["SwapTotal"] - want["SwapFree"],
	}
	obs.UsedBytes = obs.TotalBytes - obs.AvailableBytes
	return obs, nil
}

// readMemoryDarwin uses sysctl + vm_stat-style derivation. macOS
// doesn't expose a single "available" metric the same way Linux does;
// we approximate via inactive + free + speculative. v1 lives with
// the approximation.
func readMemoryDarwin() (MemoryObservation, error) {
	total, err := sysctlInt64("hw.memsize")
	if err != nil {
		return MemoryObservation{}, err
	}
	pageSize, err := sysctlInt64("hw.pagesize")
	if err != nil {
		pageSize = 4096
	}
	// `vm_stat` output: "Pages free: 12345." per-page numbers.
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return MemoryObservation{}, fmt.Errorf("vm_stat: %w", err)
	}
	var freePages, inactivePages, speculativePages int64
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Pages free:"):
			freePages = parseVMStatPages(line)
		case strings.HasPrefix(line, "Pages inactive:"):
			inactivePages = parseVMStatPages(line)
		case strings.HasPrefix(line, "Pages speculative:"):
			speculativePages = parseVMStatPages(line)
		}
	}
	free := freePages * pageSize
	avail := (freePages + inactivePages + speculativePages) * pageSize
	used := total - avail

	return MemoryObservation{
		TotalBytes:     total,
		UsedBytes:      used,
		FreeBytes:      free,
		AvailableBytes: avail,
		// Swap: macOS sysctl vm.swapusage gives a string; skipping in
		// v1 to keep the handler small.
	}, nil
}

func parseVMStatPages(line string) int64 {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return 0
	}
	val := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line[colon+1:]), "."))
	n, _ := strconv.ParseInt(val, 10, 64)
	return n
}

func sysctlInt64(name string) (int64, error) {
	out, err := exec.Command("sysctl", "-n", name).Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl %s: %w", name, err)
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

// --- Spec-22 ABI no-mutation specialization ---------------------------------

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: true, Risk: 1}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	bins := []string{}
	if runtime.GOOS == "darwin" {
		bins = []string{"sysctl", "vm_stat"}
	}
	return actions.PermissionSet{
		RequiredBinaries: bins,
		Notes:            []string{"read-only observation"},
	}
}

func (h *Handler) Diff(_ actions.Context, _ *config.Step) (actions.Diff, error) {
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "memory",
			Attributes: map[string]string{"observe_kind": "memory"},
		},
		Operation: actions.OpNoop,
	}, nil
}

func (h *Handler) Reverse(_ actions.Context, _ *config.Step, _ actions.Result) (*config.Step, error) {
	return nil, nil
}
