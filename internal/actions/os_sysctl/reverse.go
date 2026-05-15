package os_sysctl //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsSysctlReverseInfo is the per-step apply-time snapshot os.sysctl
// stashes on Result.ReverseData. Captures the prior runtime value
// (from `sysctl -n <key>`) and the prior persist-file line (from
// /etc/sysctl.d/99-mooncake.conf), plus flags recording which of
// those the apply actually changed.
type OsSysctlReverseInfo struct {
	// Name is the sysctl key (e.g. "net.ipv4.ip_forward").
	Name string

	// AppliedState is the step's State at apply time:
	// "present" or "absent".
	AppliedState string

	// PriorRuntimeValue is the runtime sysctl value pre-apply
	// (sysctl -n). Empty when the read failed; HadPriorRuntime
	// distinguishes that case.
	PriorRuntimeValue string
	HadPriorRuntime   bool

	// PriorPersistValue is the value found on the line for Name
	// in the persist file pre-apply. Empty when HadPriorPersist
	// is false.
	PriorPersistValue string
	HadPriorPersist   bool

	// TouchedPersistFile / TouchedRuntime mirror the plan flags
	// — they record whether the apply actually rewrote the persist
	// file and/or invoked `sysctl -w`. Used by Reverse to decide
	// which inverse to emit.
	TouchedPersistFile bool
	TouchedRuntime     bool
}

// Reverse implements actions.Reverser for os.sysctl (spec-28 P6 /
// reverse-capture v3).
//
// Strategy:
//   - If the apply touched the persist file: emit an os.sysctl
//     step that re-asserts the prior state. When HadPriorPersist
//     was true → state=present with the prior value (and the
//     prior runtime value if HadPriorRuntime). When
//     HadPriorPersist was false → state=absent (removes the line
//     we added).
//   - If the apply only touched the runtime value: emit an
//     os.sysctl step with the prior runtime value + persist=false
//     so we don't accidentally write a persist line that didn't
//     exist before.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Neither TouchedPersistFile nor TouchedRuntime → noop, same.
//   - Step / result missing / wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsSysctl == nil {
		return nil, errors.New("os.sysctl Reverse: step has no OsSysctl payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.sysctl Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsSysctlReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.sysctl Reverse: ReverseData is %T, want *OsSysctlReverseInfo", r.ReverseData)
	}
	if !info.TouchedPersistFile && !info.TouchedRuntime {
		return nil, nil
	}

	// Case 1: persist file was touched. Emit a step that restores
	// the persist-file state (and optionally the runtime value if
	// we captured it).
	if info.TouchedPersistFile {
		if !info.HadPriorPersist {
			// We added the line; reverse removes it.
			absent := "absent"
			noPersist := false
			return &config.Step{
				Name: "reverse: remove " + info.Name + " from persist file",
				OsSysctl: &config.OsSysctl{
					Name:    info.Name,
					State:   absent,
					Persist: &noPersist, // ensure no re-write
				},
			}, nil
		}
		// We changed a line that existed. Restore prior value;
		// also restore runtime if we changed it.
		yesPersist := true
		yesReload := info.TouchedRuntime && info.HadPriorRuntime
		return &config.Step{
			Name: "reverse: restore " + info.Name,
			OsSysctl: &config.OsSysctl{
				Name:    info.Name,
				Value:   priorValueFor(info),
				Persist: &yesPersist,
				Reload:  boolPtrIf(yesReload),
			},
		}, nil
	}

	// Case 2: only runtime was touched (no persist write). Restore
	// the prior runtime value without writing a persist line.
	if !info.HadPriorRuntime {
		// We can't safely restore runtime without a captured value
		// — the kernel had no readable prior. Refuse this slice.
		return nil, fmt.Errorf("os.sysctl Reverse: prior runtime value for %q was not captured; cannot reverse runtime-only mutation", info.Name)
	}
	noPersist := false
	yesReload := true
	return &config.Step{
		Name: "reverse: restore runtime " + info.Name,
		OsSysctl: &config.OsSysctl{
			Name:    info.Name,
			Value:   info.PriorRuntimeValue,
			Persist: &noPersist,
			Reload:  &yesReload,
		},
	}, nil
}

// priorValueFor picks the best available prior value: runtime
// reading first (most authoritative — actual kernel state), falling
// back to persist-file value if runtime wasn't readable.
func priorValueFor(info *OsSysctlReverseInfo) string {
	if info.HadPriorRuntime {
		return info.PriorRuntimeValue
	}
	return info.PriorPersistValue
}

func boolPtrIf(b bool) *bool {
	if !b {
		return nil
	}
	t := true
	return &t
}

var _ actions.Reverser = (*Handler)(nil)
