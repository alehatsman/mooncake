// Package observe_cpu implements the observe.cpu action: single-shot
// read of CPU utilization + load averages (spec-60). Wraps the shared
// internal/metrics collector so the underlying sampler runs once for
// both /v1/metrics and observe.cpu — no duplicate OS calls.
package observe_cpu

import (
	"fmt"
	"runtime"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/metrics"
)

const actionName = "observe.cpu"

// CPUObservation is the typed Value payload for observe.cpu.
type CPUObservation struct {
	Cores      int     `json:"cores"`
	UsagePct   float64 `json:"usage_pct"`
	IdlePct    float64 `json:"idle_pct"`
	LoadAvg1m  float64 `json:"load_1m"`
	LoadAvg5m  float64 `json:"load_5m"`
	LoadAvg15m float64 `json:"load_15m"`
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot read of CPU utilization + load averages",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.ObserveCPU == nil {
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
		env := actions.PlanDeferred(CPUObservation{Cores: runtime.NumCPU()})
		result.PublishObservation(env, actions.ObserveTargetHost)
		result.Checkable = true
		result.Reason = "would observe CPU usage (deferred to apply)"
		return result, nil
	}

	m, _, err := metrics.Collect([]string{
		"cpu_usage_pct",
		"load_avg_1m",
		"load_avg_5m",
		"load_avg_15m",
	})
	obs := CPUObservation{
		Cores: runtime.NumCPU(),
	}
	if m != nil {
		obs.UsagePct = m.CPU.UsagePct
		obs.IdlePct = clampPct(100 - m.CPU.UsagePct)
		obs.LoadAvg1m = m.Load.Avg1m
		obs.LoadAvg5m = m.Load.Avg5m
		obs.LoadAvg15m = m.Load.Avg15m
	}
	env := actions.ObserveResult{
		Found: err == nil && m != nil,
		Value: obs,
		AsOf:  time.Now(),
	}
	if err != nil {
		env.Error = err.Error()
	}
	result.PublishObservation(env, actions.ObserveTargetHost)
	return result, nil
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// --- Spec-22 ABI no-mutation specialization ---------------------------------

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: true, Risk: 1}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Notes: []string{"read-only observation; reads /proc/stat (Linux) or host_processor_info (macOS)"},
	}
}

func (h *Handler) Diff(_ actions.Context, _ *config.Step) (actions.Diff, error) {
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "cpu",
			Attributes: map[string]string{"observe_kind": "cpu"},
		},
		Operation: actions.OpNoop,
	}, nil
}

func (h *Handler) Reverse(_ actions.Context, _ *config.Step, _ actions.Result) (*config.Step, error) {
	return nil, nil
}
