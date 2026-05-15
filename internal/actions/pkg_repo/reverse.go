//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// PkgRepoReverseInfo is the per-step apply-time snapshot pkg.repo
// stashes on Result.ReverseData. Captures the sources file path,
// keyring path (info-only — see Reverse comment), and the prior
// sources file content + existence.
//
// Scope: pkg.repo touches up to TWO files (sources entry + keyring),
// but the Reverser interface returns ONE step. Reverse restores the
// sources file only; the keyring on disk is left as-is. A keyring
// file with no sources reference is benign (apt simply ignores it);
// operators who need full cleanup can chain a file.write absent
// step against KeyringPath inside a try/catch/finally.
type PkgRepoReverseInfo struct {
	// Name is the repo identifier (used for diagnostic messages).
	Name string

	// SourcesPath is the canonical /etc/apt/sources.list.d/<name>.sources
	// path the apply targeted.
	SourcesPath string

	// KeyringPath is the canonical /etc/apt/keyrings/<name>.gpg
	// path. Captured for visibility; not reverted (see scope note
	// in the struct doc).
	KeyringPath string

	// PriorExisted reports whether the sources file existed
	// pre-apply.
	PriorExisted bool

	// PriorContent is the verbatim file bytes pre-apply. Empty
	// when PriorExisted is false.
	PriorContent string
}

// Reverse implements actions.Reverser for pkg.repo (spec-24 P6 /
// reverse-capture v4).
//
// Cross-action reverse — returns a `file.write` step that
// reproduces the pre-apply sources file state byte-for-byte (or
// removes it if it didn't exist before). The keyring file is NOT
// reverted (see PkgRepoReverseInfo doc); operators needing full
// cleanup must run an explicit file.write absent against the
// captured KeyringPath separately.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.PkgRepo == nil {
		return nil, errors.New("pkg.repo Reverse: step has no PkgRepo payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("pkg.repo Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*PkgRepoReverseInfo)
	if !ok {
		return nil, fmt.Errorf("pkg.repo Reverse: ReverseData is %T, want *PkgRepoReverseInfo", r.ReverseData)
	}
	if info.SourcesPath == "" {
		return nil, errors.New("pkg.repo Reverse: incomplete ReverseData (no SourcesPath)")
	}

	if !info.PriorExisted {
		return &config.Step{
			Name: "reverse: remove pkg.repo " + info.Name + " sources",
			FileWrite: &config.File{
				Path:  info.SourcesPath,
				State: "absent",
			},
		}, nil
	}

	return &config.Step{
		Name: "reverse: restore pkg.repo " + info.Name + " sources",
		FileWrite: &config.File{
			Path:    info.SourcesPath,
			State:   "file",
			Content: info.PriorContent,
			Mode:    "0644",
			Force:   true,
		},
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
