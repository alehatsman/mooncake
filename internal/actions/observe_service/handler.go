// Package observe_service implements the observe.service action:
// single-shot read of init-system service state (spec-59 Phase 2).
// systemd on Linux, launchd on macOS; sysv fallback returns
// Exists=false on platforms without a recognized init system.
package observe_service

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const actionName = "observe.service"

// ServiceObservation is the typed Value payload for observe.service.
// Active=running now; Enabled=starts at boot. Both are independently
// useful and frequently differ (e.g. enabled-but-failed-to-start).
//
// Found mirrors Active, NOT Exists. Found is the single condition the
// `wait:` modifier polls, so it has to be the state a user would wait
// for: `until: running` must not be satisfied by a unit file that is
// installed and stopped, which is the exact state a wait exists to sit
// through. Installation is still observable at `value.exists` — that is
// why the two are separate fields rather than one.
type ServiceObservation struct {
	Exists   bool   `json:"exists"`
	Active   bool   `json:"active"`
	Enabled  bool   `json:"enabled"`
	SubState string `json:"sub_state,omitempty"` // systemd-specific: "running" / "exited" / "dead" / ...
	Manager  string `json:"manager,omitempty"`   // "systemd" | "launchd" | ""
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot read of service state (active? enabled?)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	o := step.ObserveService
	if o == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	if o.Name == "" {
		return fmt.Errorf("%s: name is required", actionName)
	}
	switch o.Manager {
	case "", "auto", "systemd", "launchd":
		// ok
	default:
		return fmt.Errorf("%s: manager must be one of 'systemd', 'launchd', or 'auto'", actionName)
	}
	return actions.ValidateWait(actionName, o.Wait)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObserveService

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	if ctx.Mode() == actions.ModePlan {
		env := actions.PlanDeferred(ServiceObservation{})
		result.PublishObservation(env, o.Name)
		result.Checkable = true
		result.Reason = fmt.Sprintf("would observe service %q (deferred to apply)", o.Name)
		return result, nil
	}

	// Detect once, outside any poll loop: the init system does not appear
	// halfway through a wait.
	mgr := o.Manager
	if mgr == "" || mgr == "auto" {
		mgr = autodetectManager()
	}
	if mgr != "systemd" && mgr != "launchd" {
		// A missing init system is a permanent condition, so short-circuit
		// ahead of `wait:`. Polling it would burn the whole budget and then
		// report a timeout, hiding the actual reason.
		result.PublishObservation(actions.ObserveResult{
			Found: false,
			Value: ServiceObservation{Manager: mgr},
			AsOf:  time.Now(),
			Error: fmt.Sprintf("no supported init system detected on %s", runtime.GOOS),
		}, o.Name)
		return result, nil
	}

	// One attempt, shaped as a closure so the `wait:` modifier can poll it.
	probeOnce := func() actions.ObserveResult {
		var obs ServiceObservation
		if mgr == "systemd" {
			obs = observeSystemd(o.Name)
		} else {
			obs = observeLaunchd(o.Name)
		}
		return actions.ObserveResult{
			// Found tracks Active, not Exists — see ServiceObservation.
			Found: obs.Active,
			Value: obs,
			AsOf:  time.Now(),
		}
	}

	env := probeOnce()
	if o.Wait != nil {
		var err error
		// ObserveWait returns the last observation either way, so a timed-out
		// wait still publishes real data for a downstream `as:` capture.
		env, err = actions.ObserveWait(ctx, actionName, o.Name, o.Wait, probeOnce)
		if err != nil {
			result.PublishObservation(env, o.Name)
			return result, err
		}
	}
	result.PublishObservation(env, o.Name)
	return result, nil
}

func autodetectManager() string {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("systemctl"); err == nil {
			return "systemd"
		}
	case "darwin":
		return "launchd"
	}
	return ""
}

func observeSystemd(name string) ServiceObservation {
	// `systemctl show <name> --no-page --property=...` returns key=value lines.
	// Properties:
	//   LoadState   = loaded / not-found
	//   ActiveState = active / inactive / failed
	//   SubState    = running / exited / dead / ...
	//   UnitFileState = enabled / disabled / static / ...
	out, err := exec.Command(
		"systemctl", "show", name,
		"--no-page",
		"--property=LoadState,ActiveState,SubState,UnitFileState",
	).Output()
	if err != nil {
		return ServiceObservation{Manager: "systemd"}
	}
	return systemdObservation(string(out))
}

// systemdObservation maps `systemctl show` output onto the observation.
// Split from the exec so the mapping — in particular that Exists and Active
// are independent — is testable without a live init system.
func systemdObservation(showOutput string) ServiceObservation {
	obs := ServiceObservation{Manager: "systemd"}
	props := parseProps(showOutput)
	if props["LoadState"] != "" && props["LoadState"] != "not-found" {
		obs.Exists = true
	}
	if props["ActiveState"] == "active" {
		obs.Active = true
	}
	obs.SubState = props["SubState"]
	switch props["UnitFileState"] {
	case "enabled", "enabled-runtime", "static", "alias":
		obs.Enabled = true
	}
	return obs
}

func observeLaunchd(name string) ServiceObservation {
	obs := ServiceObservation{Manager: "launchd"}
	// `launchctl list <name>` exits 0 if loaded; output has the PID
	// (or "-" if not running). Exit non-zero = not loaded.
	out, err := exec.Command("launchctl", "list", name).Output()
	if err != nil {
		// Not loaded; try system domain via list-via-print as a fallback.
		return obs
	}
	obs.Exists = true
	// Output shape: "PID\tStatus\tLabel" — PID "-" means not running.
	if len(out) > 0 {
		first := strings.SplitN(string(out), "\t", 2)
		if len(first) > 0 && first[0] != "-" && first[0] != "" {
			obs.Active = true
		}
	}
	// launchd "enabled" is fuzzier; treat "Disabled" key as the only
	// hard signal. v1 leaves Enabled=false unless we explicitly see
	// otherwise — over-conservative but honest.
	return obs
}

func parseProps(s string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		out[line[:eq]] = line[eq+1:]
	}
	return out
}

// --- Spec-22 ABI no-mutation specialization ---------------------------------

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: false, Risk: 1}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	bins := []string{}
	switch runtime.GOOS {
	case "linux":
		bins = append(bins, "systemctl")
	case "darwin":
		bins = append(bins, "launchctl")
	}
	return actions.PermissionSet{
		RequiredBinaries: bins,
		Notes:            []string{"read-only observation; no mutation"},
	}
}

func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	o := step.ObserveService
	if o == nil {
		return actions.Diff{}, nil
	}
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceService,
			Identifier: o.Name,
			Attributes: map[string]string{"observe_kind": "service"},
		},
		Operation: actions.OpNoop,
	}, nil
}
