package git_config //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// GitConfigReverseInfo is the per-step apply-time snapshot that
// git.config stashes on Result.ReverseData. Captures the scope +
// repo plus one entry per key the apply actually mutated — keys
// that were already in the desired state pre-apply are NOT in the
// drift list (computeDrift filters them) and therefore not here.
//
// PriorValue + HadValue let Reverse decide per-key:
//   - HadValue=true  → reverse sets the key back to PriorValue
//   - HadValue=false → reverse unsets the key (it wasn't there
//     before we set it; rollback should remove it)
//
// Only populated when Changed=true. Noop applies leave ReverseData
// nil so Reverse returns (nil, nil).
type GitConfigReverseInfo struct {
	// Scope is the git config scope this step targeted: local |
	// global | system.
	Scope string

	// Repo is the working-tree path when Scope=local. Empty
	// otherwise; preserved verbatim from the original step so
	// Reverse can reconstruct the same scopeArgs.
	Repo string

	// Entries is one per key the apply mutated. PriorValue is the
	// observed value before the change (empty when HadValue=false).
	Entries []GitConfigReverseEntry
}

// GitConfigReverseEntry captures one key's pre-apply state.
type GitConfigReverseEntry struct {
	Key        string
	PriorValue string
	HadValue   bool
}

// buildReverseInfo turns the drift list (computed from
// readKey/scope) into a ReverseData payload. Called from apply()
// BEFORE the shell-out so the captured state isn't influenced by
// the mutation about to happen.
func buildReverseInfo(scope, repo string, drift []driftEntry) *GitConfigReverseInfo {
	entries := make([]GitConfigReverseEntry, 0, len(drift))
	for _, d := range drift {
		entries = append(entries, GitConfigReverseEntry{
			Key:        d.key,
			PriorValue: d.current,
			HadValue:   d.hadValue,
		})
	}
	return &GitConfigReverseInfo{
		Scope:   scope,
		Repo:    repo,
		Entries: entries,
	}
}

// Reverse implements actions.Reverser for git.config (spec-26
// phase 5 / spec-26 reverse-capture follow-up).
//
// Returns a `git.config` step that restores every mutated key to
// its pre-apply observed state:
//   - Keys that had a prior value land in the new step's `set:`
//     map with their old value.
//   - Keys that were unset pre-apply land in the new step's
//     `unset:` list.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - ReverseData wrong type / step nil → defensive error.
//   - ReverseData has zero entries → unusual but valid; return
//     (nil, nil) since there's nothing to undo.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.GitConfig == nil {
		return nil, errors.New("git.config Reverse: step has no GitConfig payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("git.config Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*GitConfigReverseInfo)
	if !ok {
		return nil, fmt.Errorf("git.config Reverse: ReverseData is %T, want *GitConfigReverseInfo", r.ReverseData)
	}
	if len(info.Entries) == 0 {
		return nil, nil
	}

	set := make(map[string]string)
	var unset []string
	for _, e := range info.Entries {
		if e.HadValue {
			set[e.Key] = e.PriorValue
		} else {
			unset = append(unset, e.Key)
		}
	}

	rev := &config.GitConfig{
		Scope: info.Scope,
		Repo:  info.Repo,
	}
	if len(set) > 0 {
		rev.Set = set
	}
	if len(unset) > 0 {
		rev.Unset = unset
	}
	return &config.Step{GitConfig: rev}, nil
}

var _ actions.Reverser = (*Handler)(nil)
