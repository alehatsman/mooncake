package tool

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. Plan mode inspects whether the
// tool is already installed (mooncake's standard layout for URL-based
// backends; Backend.Locate for backends that own their layout) and
// reports already-ok or would-install. Execute mode delegates to the
// legacy Execute path.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	t := step.Tool
	if t == nil {
		return nil, fmt.Errorf("tool configuration is nil")
	}

	result := executor.NewResult()
	result.Checkable = true

	// Standard layout installDir is passed through to Locate. URL-based
	// backends use it as their root; mise's Locate ignores it and
	// consults `mise which` directly.
	installDir, err := InstallDir(t.Name, t.Version)
	if err != nil {
		return result, err
	}

	backend, err := Get(t.Backend)
	if err != nil {
		return result, err
	}
	spec := specFromConfig(t)
	binPath, locErr := backend.Locate(context.Background(), spec, installDir)
	if locErr == nil && binPath != "" {
		result.Reason = fmt.Sprintf("%s %s already installed at %s", t.Name, t.Version, filepath.Dir(binPath))
		return result, nil
	}
	result.WouldChange = true
	result.Reason = fmt.Sprintf("would install %s %s via %s", t.Name, t.Version, t.Backend)
	return result, nil
}
