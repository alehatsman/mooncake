// Package observe_gpu implements the observe.gpu action: single-shot
// read of GPU utilization + memory (spec-62). Wraps the shared
// internal/metrics collector so the underlying nvidia-smi (Linux) /
// powermetrics (macOS) sample is shared with /v1/metrics — no
// duplicate vendor-tool calls.
package observe_gpu

import (
	"fmt"
	"runtime"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/metrics"
)

const actionName = "observe.gpu"

// GPUObservation is the typed Value payload for observe.gpu. Count=0
// is honest: no GPU runtime detected. Each device entry carries
// per-GPU live metrics; Aggregate is summed/maxed across all entries
// for the common "is any GPU busy?" check.
type GPUObservation struct {
	Count     int          `json:"count"`
	Vendor    string       `json:"vendor,omitempty"`
	GPUs      []GPUDevice  `json:"gpus,omitempty"`
	Aggregate GPUAggregate `json:"aggregate"`
}

// GPUDevice carries one GPU's live state.
type GPUDevice struct {
	Index           int     `json:"index"`
	UtilizationPct  float64 `json:"utilization_pct"`
	MemoryUsedBytes int64   `json:"memory_used_bytes"`
	MemoryUsedPct   float64 `json:"memory_used_pct"`
	TemperatureC    int     `json:"temperature_c,omitempty"`
}

// GPUAggregate summarizes across all detected GPUs.
type GPUAggregate struct {
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	MaxUtilizationPct float64 `json:"max_utilization_pct"`
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot read of GPU utilization + memory (NVIDIA/Apple)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	o := step.ObserveGPU
	if o == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	if o.Index != nil && *o.Index < 0 {
		return fmt.Errorf("%s: index must be >= 0", actionName)
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObserveGPU

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	if ctx.Mode() == actions.ModePlan {
		env := actions.PlanDeferred(GPUObservation{})
		result.PublishObservation(env, actions.ObserveTargetHost)
		result.Checkable = true
		result.Reason = "would observe GPU state (deferred to apply)"
		return result, nil
	}

	m, _, err := metrics.Collect([]string{"gpus_metrics"})
	obs := GPUObservation{}
	if m != nil {
		obs = buildObservation(m.GPUs, o.Index)
	}
	env := actions.ObserveResult{
		Found: true, // observation completed; Count=0 is the honest "no GPU" signal
		Value: obs,
		AsOf:  time.Now(),
	}
	if err != nil && obs.Count == 0 {
		// Genuine failure (e.g. nvidia-smi not present where we expected it);
		// surface as Error but keep Found=true so the caller can branch on
		// Count.
		env.Error = err.Error()
	}
	result.PublishObservation(env, actions.ObserveTargetHost)
	return result, nil
}

// buildObservation turns the per-GPU metrics into the spec-62 typed
// shape. When Index is set, the returned slice has at most one entry
// (the matching GPU) and Count=1; otherwise it carries every detected
// GPU.
func buildObservation(in []metrics.GPUMetrics, index *int) GPUObservation {
	out := GPUObservation{}
	if len(in) == 0 {
		return out
	}
	out.Vendor = "nvidia" // metrics package only ships NVIDIA today; document the limitation
	if runtime.GOOS == "darwin" {
		out.Vendor = "apple"
	}

	for _, g := range in {
		if index != nil && g.Index != *index {
			continue
		}
		usedBytes := g.MemoryUsedMB * 1024 * 1024
		out.GPUs = append(out.GPUs, GPUDevice{
			Index:           g.Index,
			UtilizationPct:  g.UsagePct,
			MemoryUsedBytes: usedBytes,
			MemoryUsedPct:   g.MemoryUsedPct,
			TemperatureC:    g.TemperatureC,
		})
		out.Aggregate.MemoryUsedBytes += usedBytes
		if g.UsagePct > out.Aggregate.MaxUtilizationPct {
			out.Aggregate.MaxUtilizationPct = g.UsagePct
		}
	}
	out.Count = len(out.GPUs)
	if out.Count == 0 {
		out.Vendor = ""
	}
	return out
}

// --- Spec-22 ABI no-mutation specialization ---------------------------------

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: true, Risk: 1}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	bins := []string{}
	switch runtime.GOOS {
	case "linux":
		bins = append(bins, "nvidia-smi") // optional; observation gracefully returns Count=0 if absent
	case "darwin":
		bins = append(bins, "powermetrics", "system_profiler")
	}
	return actions.PermissionSet{
		RequiredBinaries: bins,
		Notes:            []string{"read-only observation; absent tools return Count=0 (honest)"},
	}
}

func (h *Handler) Diff(_ actions.Context, _ *config.Step) (actions.Diff, error) {
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "gpu",
			Attributes: map[string]string{"observe_kind": "gpu"},
		},
		Operation: actions.OpNoop,
	}, nil
}

func (h *Handler) Reverse(_ actions.Context, _ *config.Step, _ actions.Result) (*config.Step, error) {
	return nil, nil
}
