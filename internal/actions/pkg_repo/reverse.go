//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Reverse implements actions.Reverser for pkg.repo (spec-24 P6 /
// reverse-capture v4).
//
// Cross-action reverse — returns a `file.write` step that
// reproduces the pre-apply sources file state byte-for-byte (or
// removes it if it didn't exist before). The keyring file is NOT
// reverted (see PkgRepoReverseInfo doc in shared/); operators
// needing full cleanup must run an explicit file.write absent
// against the captured KeyringPath separately.
//
// The PkgRepoReverseInfo type itself lives in pkg_repo/shared and
// is re-exported here as a type alias from handler.go so the
// per-driver sub-packages can populate it without forming an import
// cycle with this parent package.
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
