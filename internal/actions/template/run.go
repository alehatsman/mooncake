package template

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/utils"
)

// Run is the Spec 16 unified entry point. Renders the template source
// against the current variables, then either writes the result (or
// reports what would happen in ModePlan). Drift between preview and
// execute is eliminated because both modes render and compare against
// the same dest content via a single Performer.WriteFile call.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	tmpl := step.Template

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Resolve src path against the preset root if set, otherwise the
	// current config dir. Preserves the existing template-handler
	// contract.
	baseDir := ec.CurrentDir
	if ec.PresetBaseDir != "" {
		baseDir = ec.PresetBaseDir
	}
	src, err := ec.PathUtil.ExpandPath(tmpl.Src, baseDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand src path: %w", err)
	}
	dest, err := ec.PathUtil.ExpandPath(tmpl.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	result := executor.NewResult()
	result.Checkable = true
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Read template source.
	// #nosec G304 -- template source path from user config is intentional
	f, err := os.Open(src)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to read template: %w", err)
	}
	defer func() { _ = f.Close() }()
	templateBytes, err := io.ReadAll(f)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to read template: %w", err)
	}

	// Merge per-step vars before rendering.
	variables := ctx.GetVariables()
	if tmpl.Vars != nil && len(*tmpl.Vars) > 0 {
		variables = utils.MergeVariables(ctx.GetVariables(), *tmpl.Vars)
	}

	output, err := ctx.GetTemplate().Render(string(templateBytes), variables)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to render template: %w", err)
	}
	rendered := []byte(output)
	mode := h.parseFileMode(tmpl.Mode, defaultFileMode)

	// Delegate to Performer.WriteFile — same site decides both
	// "would change?" and "perform write" semantics for both modes.
	eff := ctx.Effects().WriteFile(dest, rendered, mode, actions.PerformerOpts{Become: step.Become})

	if eff.Err != nil {
		result.Failed = true
		return result, eff.Err
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = eff.WouldChange
		result.Reason = eff.Reason
		return result, nil
	}

	result.Changed = eff.Performed

	// Emit template render event — only in execute mode, matching the
	// legacy contract.
	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventTemplateRender,
			Data: events.TemplateRenderData{
				TemplatePath: src,
				DestPath:     dest,
				SizeBytes:    int64(len(rendered)),
				Changed:      result.Changed,
				DryRun:       false,
			},
		})
	}

	return result, nil
}

// bytes is referenced indirectly by the rest of the package; keep the
// import alive across builds that strip unused refs.
var _ = bytes.Equal
