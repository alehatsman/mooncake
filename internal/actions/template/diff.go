package template

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/utils"
)

// Diff implements actions.Differ for file.template (spec-22 phase 4).
//
// file.template's mutation is the same shape as file.write — a single
// Dest file gets written with content. The difference is content
// source: file.write embeds content inline, file.template reads it
// from Src and renders through Pongo2. So Diff() reuses
// filehandler.SnapshotPath / FileResource / FileSnapshot for the
// Before/After payload shape, and adds template-specific logic for
// computing the After Sha256.
//
// Failure modes:
//   - nil step → error (handler doesn't apply)
//   - Src missing or unreadable → After is left with Sha256 empty
//     and Operation forced to OpUpdate; the runtime error path picks
//     up the real Src failure
//   - Template render fails → propagated as error so the caller can
//     surface "your {{ var }} reference is wrong" at plan time
func (Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.FileTemplate == nil {
		return actions.Diff{}, errors.New("file.template Diff: step has no FileTemplate payload")
	}
	tpl := step.FileTemplate

	renderedDest := resolveTemplateDest(ctx, tpl)
	mode := parseTemplateMode(tpl.Mode)
	before := filehandler.SnapshotPath(renderedDest)

	// Read + render the template Src to compute desired After content.
	// Mirror handler.readAndRenderTemplate but kept inline so Diff
	// doesn't need an ExecutionContext.
	var stepVars map[string]interface{}
	if tpl.Vars != nil {
		stepVars = *tpl.Vars
	}
	desired, renderErr := readAndRender(ctx, tpl.Src, stepVars)
	after := &filehandler.FileSnapshot{
		Path:   renderedDest,
		Exists: true,
		Kind:   "file",
		Mode:   fmt.Sprintf("%#o", mode.Perm()),
	}
	if renderErr == nil {
		after.Size = int64(len(desired))
		sum := sha256.Sum256(desired)
		after.Sha256 = hex.EncodeToString(sum[:])
	}

	op := actions.OpUpdate
	switch {
	case !before.Exists:
		op = actions.OpCreate
	case renderErr == nil && before.Kind == "file" && before.Sha256 == after.Sha256 && before.Mode == after.Mode:
		op = actions.OpNoop
	}

	return actions.Diff{
		Resource:  filehandler.FileResource(renderedDest),
		Operation: op,
		Before:    before,
		After:     after,
	}, renderErr
}

func resolveTemplateDest(ctx actions.Context, tpl *config.Template) string {
	if ec, ok := ctx.(*executor.ExecutionContext); ok && ec.Svc != nil && ec.Svc.PathUtil != nil {
		if expanded, err := ec.Svc.PathUtil.ExpandPath(tpl.Dest, ec.CurrentDir, ctx.Variables()); err == nil {
			return expanded
		}
	}
	return tpl.Dest
}

// parseTemplateMode resolves the step's Mode string to an os.FileMode.
// Inlined (rather than calling handler.parseFileMode) so Diff doesn't
// need a Handler receiver — keeping the dispatch in `func (Handler)
// Diff` form symmetric with the other phase-4 handlers.
func parseTemplateMode(s string) os.FileMode {
	if s == "" {
		return 0o644
	}
	var m os.FileMode
	if _, err := fmt.Sscanf(s, "%o", &m); err != nil {
		return 0o644
	}
	return m
}

// readAndRender opens src, reads its bytes, and renders through the
// step's Pongo2 template engine. stepVars (may be nil) are evaluated
// against the current scope first and then merged over it — mirroring
// what handler.Run does — so the SHA reflects the actual rendered output.
func readAndRender(ctx actions.Context, src string, stepVars map[string]interface{}) ([]byte, error) {
	srcBytes, err := os.ReadFile(src) //nolint:gosec // intentional: user-supplied template path
	if err != nil {
		return nil, fmt.Errorf("read template src: %w", err)
	}
	vars := ctx.Variables()
	if len(stepVars) > 0 {
		evaluated, evErr := evalStepVars(stepVars, ctx)
		if evErr != nil {
			return nil, fmt.Errorf("eval step vars: %w", evErr)
		}
		vars = utils.MergeVariables(vars, evaluated)
	}
	rendered, err := ctx.Template().Render(string(srcBytes), vars)
	if err != nil {
		return nil, fmt.Errorf("render template: %w", err)
	}
	return []byte(rendered), nil
}

// Compile-time interface check.
var _ actions.Differ = Handler{}
