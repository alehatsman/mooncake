package package_handler

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. It consults ctx.Mode() and
// either inspects which packages would change (ModePlan) or actually
// installs / removes / upgrades them (ModeApply).
//
// The shared preamble (manager detection, package-list building,
// template rendering, state normalization) runs once for both modes,
// eliminating any chance of the plan preview disagreeing with what
// execute would actually do.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	pkg := step.Pkg

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Shared preamble — same for both modes.
	manager, err := h.determinePackageManager(pkg.Manager, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to determine package manager: %w", err)
	}

	state := pkg.State
	if state == "" {
		state = statePresent
	}

	packages := h.buildPackageList(pkg)
	for i, name := range packages {
		if rendered, rerr := ctx.GetTemplate().Render(name, ctx.GetVariables()); rerr == nil {
			packages[i] = rendered
		}
	}

	if ctx.Mode() == actions.ModePlan {
		return h.runPlan(ec, manager, state, packages, pkg)
	}

	// ModeApply — preserve the legacy Execute behavior. Upgrade and
	// cache update both fall through to their existing helpers.
	result := executor.NewResult()
	result.SetChanged(false)

	if pkg.Upgrade {
		return h.executeUpgrade(ec, manager, pkg, step.ShouldBecome())
	}

	if pkg.UpdateCache {
		if err := h.updateCache(ec, manager, step.ShouldBecome()); err != nil {
			return nil, fmt.Errorf("failed to update package cache: %w", err)
		}
	}

	switch state {
	case statePresent, stateLatest:
		return h.installPackages(ec, manager, packages, state == stateLatest, pkg.Extra, step.ShouldBecome())
	case stateAbsent:
		return h.removePackages(ec, manager, packages, pkg.Extra, step.ShouldBecome())
	default:
		return nil, fmt.Errorf("unsupported state: %s", state)
	}
}

// runPlan inspects each package's installed status and aggregates a
// per-step prediction. Returns Checkable: true so the plan output
// shows accurate would-install / would-remove / already-matches.
func (h *Handler) runPlan(ec *executor.ExecutionContext, manager, state string, packages []string, pkg *config.Package) (actions.Result, error) {
	result := executor.NewResult()
	result.Checkable = true

	// Upgrade is not idempotent at the package-list level — we can't
	// know without running whether anything would update. Treat as
	// would-change.
	if pkg.Upgrade {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would upgrade packages via %s", manager)
		return result, nil
	}

	// First package that would change wins the reason string; the
	// inspection short-circuits as soon as a change is detected, which
	// matches the legacy Check behavior.
	for _, name := range packages {
		installed, err := h.isPackageInstalled(ec, manager, name)
		if err != nil {
			result.Reason = fmt.Sprintf("check error: %v", err)
			return result, nil
		}
		switch state {
		case statePresent, stateLatest:
			if !installed {
				result.WouldChange = true
				result.Reason = fmt.Sprintf("would install %s", name)
				return result, nil
			}
		case stateAbsent:
			if installed {
				result.WouldChange = true
				result.Reason = fmt.Sprintf("would remove %s", name)
				return result, nil
			}
		}
	}

	result.Reason = "already in desired state"
	return result, nil
}
