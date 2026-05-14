package copy

import (
	"errors"
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Diff implements actions.Differ for file.copy (spec-22 phase 4).
//
// file.copy's mutation: read Src (a path on the local FS), write it
// to Dest. Diff predicts the resulting state by hashing Src once and
// comparing against the Before snapshot of Dest. Reuses
// filehandler.FileSnapshot for the Before/After payload shape because
// the end result is the same kind of resource — a regular file at
// Dest.
//
// Failure modes:
//   - nil step / no FileCopy payload → error
//   - Src missing or unreadable → After.Sha256 is left empty, Operation
//     forced to OpUpdate so the runtime EACCES/ENOENT is the backstop
//     (Diff is read-only; we don't error out because plan output is
//     still useful even if Src happens to be missing at plan time)
func (Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.FileCopy == nil {
		return actions.Diff{}, errors.New("file.copy Diff: step has no FileCopy payload")
	}
	cp := step.FileCopy

	renderedDest := resolveCopyPath(ctx, cp.Dest)
	mode := parseCopyMode(cp.Mode)
	before := filehandler.SnapshotPath(renderedDest)

	after := &filehandler.FileSnapshot{
		Path:   renderedDest,
		Exists: true,
		Kind:   "file",
		Mode:   fmt.Sprintf("%#o", mode.Perm()),
	}

	// Compute After.Sha256 by hashing Src directly. Resolve Src
	// templates the same way Run does so Diff and Apply see the same
	// source file.
	renderedSrc := resolveCopyPath(ctx, cp.Src)
	srcInfo, statErr := os.Stat(renderedSrc)
	if statErr == nil && srcInfo.Mode().IsRegular() {
		if sum, err := filehandler.HashFile(renderedSrc); err == nil {
			after.Sha256 = sum
			after.Size = srcInfo.Size()
		}
	}

	op := actions.OpUpdate
	switch {
	case !before.Exists:
		op = actions.OpCreate
	case after.Sha256 != "" &&
		before.Kind == "file" &&
		before.Sha256 == after.Sha256 &&
		before.Mode == after.Mode:
		op = actions.OpNoop
	}

	return actions.Diff{
		Resource:  filehandler.FileResource(renderedDest),
		Operation: op,
		Before:    before,
		After:     after,
	}, nil
}

func resolveCopyPath(ctx actions.Context, raw string) string {
	if raw == "" {
		return ""
	}
	if ec, ok := ctx.(*executor.ExecutionContext); ok && ec.Svc != nil && ec.Svc.PathUtil != nil {
		if expanded, err := ec.Svc.PathUtil.ExpandPath(raw, ec.CurrentDir, ctx.GetVariables()); err == nil {
			return expanded
		}
	}
	return raw
}

func parseCopyMode(s string) os.FileMode {
	if s == "" {
		return 0o644
	}
	var m os.FileMode
	if _, err := fmt.Sscanf(s, "%o", &m); err != nil {
		return 0o644
	}
	return m
}

// Compile-time interface check.
var _ actions.Differ = Handler{}
