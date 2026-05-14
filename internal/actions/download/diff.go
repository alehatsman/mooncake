package download

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"

	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
)

// Diff implements actions.Differ for file.download (spec-22 phase 4e).
//
// file.download is a Network action — its terminal effect is "write a
// remote resource to Dest". Spec-22 explicitly says NOT to fetch the
// URL at plan time: that would couple Diff to network connectivity,
// add latency to every plan run, and surprise users who plan without
// internet.
//
// Instead, Diff uses the user-declared Checksum field (when present)
// as the After.Sha256 prediction. Checksum format is bare hex per the
// existing handler — 64 chars = SHA256, 32 chars = MD5. SHA256 is the
// only one that maps cleanly into FileSnapshot.Sha256 (which is
// sha256-typed by name); MD5 checksums can't predict noop here and
// degrade to "unknown After.Sha256".
//
// Operation matrix:
//
//	Dest missing                                    → OpCreate
//	Dest exists, checksum (SHA256) matches          → OpNoop
//	Dest exists, checksum set but doesn't match     → OpUpdate
//	Dest exists, no checksum or MD5-only            → OpUpdate
//	                                                  (conservative — can't predict noop)
//
// Force flag is intentionally NOT respected in Diff: it changes
// runtime behaviour (forces re-download even when checksum matches)
// but doesn't change the predicted post-state. Surfacing it in the
// returned After would be misleading.
func (Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.FileDownload == nil {
		return actions.Diff{}, errors.New("file.download Diff: step has no FileDownload payload")
	}
	dl := step.FileDownload

	renderedDest := resolveDownloadPath(ctx, dl.Dest)
	mode := parseDownloadMode(dl.Mode)
	before := filehandler.SnapshotPath(renderedDest)

	after := &filehandler.FileSnapshot{
		Path:   renderedDest,
		Exists: true,
		Kind:   "file",
		Mode:   fmt.Sprintf("%#o", mode.Perm()),
	}

	// Predict After.Sha256 from the declared checksum when it's a
	// SHA256 hex string. MD5 checksums skip — we can't represent them
	// in FileSnapshot.Sha256, and trying would surprise consumers.
	predictedSha := sha256FromChecksum(dl.Checksum)
	if predictedSha != "" {
		after.Sha256 = predictedSha
	}

	op := actions.OpUpdate
	switch {
	case !before.Exists:
		op = actions.OpCreate
	case predictedSha != "" && before.Kind == "file" && before.Sha256 == predictedSha:
		op = actions.OpNoop
	}

	return actions.Diff{
		Resource:  filehandler.FileResource(renderedDest),
		Operation: op,
		Before:    before,
		After:     after,
	}, nil
}

// sha256FromChecksum returns the checksum string when it's a SHA256
// hex (64 lowercase-hex chars). Returns "" for MD5 (32 chars) or any
// other shape so consumers don't misclassify.
func sha256FromChecksum(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) != 64 {
		return ""
	}
	for _, c := range s {
		if !isHexRune(c) {
			return ""
		}
	}
	return s
}

func isHexRune(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}

func resolveDownloadPath(ctx actions.Context, raw string) string {
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

func parseDownloadMode(s string) os.FileMode {
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
