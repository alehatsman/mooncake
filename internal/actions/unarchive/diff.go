package unarchive

import (
	"errors"
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"

	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
)

// Diff implements actions.Differ for file.unarchive (spec-22 phase 4e).
//
// file.unarchive extracts an archive into a destination directory.
// Predicting the exact post-state would mean parsing the archive (Src)
// and walking every entry — workable but heavy. Spec-22 keeps Diff
// cheap, so unarchive uses a coarse shape:
//
//   - Resource: ResourceFile (no ResourceDirectory kind in spec-22;
//     Snapshot.Kind="directory" carries the precise type info).
//   - After: a FileSnapshot with Kind="directory" at Dest.
//   - Operation depends on:
//     1. step.Creates marker present + exists on disk → OpNoop
//        (mirrors the runtime idempotency check — if the marker
//        file/dir is already there, the archive was already
//        extracted).
//     2. Dest exists (any kind) → OpUpdate. Even if Dest is a
//        non-directory, the apply path will fail; Diff stays honest
//        about the user's intent ("there will be an extraction here")
//        and the runtime error path remains the backstop.
//     3. Dest missing → OpCreate.
//
// Src is NOT included in the Diff's Resource ref — it's a read-only
// input on the controller's FS, matching how text.patch handles
// PatchFile (and how the spec-22 PermissionSet for file.unarchive
// already excludes Src from FilesystemWrite).
func (Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.FileUnarchive == nil {
		return actions.Diff{}, errors.New("file.unarchive Diff: step has no FileUnarchive payload")
	}
	ua := step.FileUnarchive

	renderedDest := resolveUnarchivePath(ctx, ua.Dest)
	mode := parseUnarchiveMode(ua.Mode)
	before := filehandler.SnapshotPath(renderedDest)

	after := &filehandler.FileSnapshot{
		Path:   renderedDest,
		Exists: true,
		Kind:   "directory",
		Mode:   fmt.Sprintf("%#o", mode.Perm()),
	}

	// Idempotency-marker noop: if step.Creates is set and that path
	// exists, the handler will refuse to extract — match that here.
	if ua.Creates != "" {
		renderedCreates := resolveUnarchivePath(ctx, ua.Creates)
		if _, err := os.Lstat(renderedCreates); err == nil {
			return actions.Diff{
				Resource:  filehandler.FileResource(renderedDest),
				Operation: actions.OpNoop,
				Before:    before,
				After:     after,
			}, nil
		}
	}

	op := actions.OpUpdate
	if !before.Exists {
		op = actions.OpCreate
	}

	return actions.Diff{
		Resource:  filehandler.FileResource(renderedDest),
		Operation: op,
		Before:    before,
		After:     after,
	}, nil
}

func resolveUnarchivePath(ctx actions.Context, raw string) string {
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

func parseUnarchiveMode(s string) os.FileMode {
	if s == "" {
		return 0o755
	}
	var m os.FileMode
	if _, err := fmt.Sscanf(s, "%o", &m); err != nil {
		return 0o755
	}
	return m
}

// Compile-time interface check.
var _ actions.Differ = Handler{}
