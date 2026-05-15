package git_config

import (
	"errors"
	"sort"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// GitConfigSnapshot is the typed Before/After payload for actions.Diff
// when the resource kind is ResourceGit and the underlying action is
// git.config. Each entry mirrors one key → value pair the step
// touches; `op` is "set" or "unset" matching the run-time drift
// classifier.
type GitConfigSnapshot struct {
	// Scope mirrors step.GitConfig.Scope.
	Scope string `json:"scope,omitempty"`

	// Repo mirrors step.GitConfig.Repo (empty for global/system).
	Repo string `json:"repo,omitempty"`

	// Entries is the per-key intent: which keys will be set or
	// unset and to what value. Sorted by Key for stable plan
	// output. Present in After only; Before is summarized via
	// observed values when available (see Diff comments).
	Entries []GitConfigEntry `json:"entries,omitempty"`
}

// GitConfigEntry is one key-level intent or observed value.
type GitConfigEntry struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
	Op    string `json:"op,omitempty"` // "set" | "unset"
}

// Diff implements actions.Differ for git.config (spec-26 phase 5).
//
// Operation: OpUpdate when set/unset has at least one entry,
// OpNoop otherwise (vacuous step). Conservative: Diff does NOT
// shell out to git to read current values (that would couple plan
// time to subprocess execution and depend on whether the daemon
// has read permission on the target config file). The runtime
// drift classifier filters out already-converged keys; Diff
// reports the *declared intent* and lets the apply path produce
// the truthful changed=false on convergence.
//
// Resource.Kind = ResourceGit. Resource.Identifier is
// "<scope>" for global/system and "<scope>:<repo>" for local.
// Identifier shape keeps two-local-repos-on-same-host distinct
// at the consumer level without forcing a per-scope type switch.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.GitConfig == nil {
		return actions.Diff{}, errors.New("git.config Diff: step has no GitConfig payload")
	}
	g := step.GitConfig

	identifier := g.Scope
	if g.Scope == scopeLocal && g.Repo != "" {
		identifier = g.Scope + ":" + g.Repo
	}
	resource := actions.ResourceRef{Kind: actions.ResourceGit, Identifier: identifier}

	entries := make([]GitConfigEntry, 0, len(g.Set)+len(g.Unset))
	keys := make([]string, 0, len(g.Set))
	for k := range g.Set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		entries = append(entries, GitConfigEntry{Key: k, Value: g.Set[k], Op: "set"})
	}
	unsetKeys := append([]string(nil), g.Unset...)
	sort.Strings(unsetKeys)
	for _, k := range unsetKeys {
		entries = append(entries, GitConfigEntry{Key: k, Op: "unset"})
	}

	op := actions.OpUpdate
	if len(entries) == 0 {
		op = actions.OpNoop
	}

	after := &GitConfigSnapshot{
		Scope:   g.Scope,
		Repo:    g.Repo,
		Entries: entries,
	}
	return actions.Diff{Resource: resource, Operation: op, After: after}, nil
}

var _ actions.Differ = (*Handler)(nil)
