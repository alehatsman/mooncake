// Package pkg_repo implements the pkg.repo action: declarative
// third-party package repository management. The handler is a thin
// dispatcher: at Run time it selects the driver from the populated
// nested block (apt/dnf/brew) and delegates to a per-driver
// sub-package.
//
// Layout:
//
//	pkg_repo/             ← Handler + dispatcher + Validate +
//	                         Permissions + Diff + Cost + Reverse
//	                         (this file + diff.go / cost.go /
//	                         reverse.go)
//	pkg_repo/shared/      ← shared types (PkgRepoReverseInfo,
//	                         state constants, NameRE) +
//	                         shared helpers (HTTPFetchKey,
//	                         VerifyKeyFingerprint, WriteAtomic,
//	                         ReadFile, PathExists, NormalizeState)
//	pkg_repo/apt/         ← apt driver: DEB822 sources, /etc/apt
//	                         keyring, apt-get update
//	pkg_repo/dnf/         ← dnf/yum driver: .repo INI, /etc/pki/rpm-gpg
//	                         keyring, dnf clean expire-cache
//	pkg_repo/brew/        ← Homebrew tap driver: brew tap/untap
//
// Public API unchanged: the parent's Handler / Validate / Run still
// satisfy the actions.Handler interface, and the
// `OsServiceReverseInfo`-shaped re-export (PkgRepoReverseInfo as a
// type alias to shared.PkgRepoReverseInfo) keeps registry +
// schemagen + fleet wire-encoding callers working without changes.
//
//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/apt"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/brew"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/dnf"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/shared"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

const actionName = "pkg.repo"

// State constants re-exported for callers that imported them from
// this package before the split (Validate, tests).
const (
	statePresent = shared.StatePresent
	stateAbsent  = shared.StateAbsent
)

// PkgRepoReverseInfo is re-exported from shared so existing callers
// of `pkg_repo.PkgRepoReverseInfo` (Reverse, runlog, fleet
// wire-encoding) keep working unchanged.
type PkgRepoReverseInfo = shared.PkgRepoReverseInfo

// PkgRepoBrewReverseInfo is re-exported from the brew sub-package
// for the same reason — existing callers (executor's wire-encoding
// test) reference it as `pkg_repo.PkgRepoBrewReverseInfo` and the
// alias keeps them working unchanged.
type PkgRepoBrewReverseInfo = brew.PkgRepoBrewReverseInfo

// Handler implements pkg.repo.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("PkgRepoReverseInfo", func() any { return &PkgRepoReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage a third-party package repository (apt, dnf/yum, brew taps)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventFileUpdated)},
		Version:            "1.1.0",
		SupportedPlatforms: []string{"linux", "darwin"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// Per-driver: apt writes under /etc and uses sudo + apt-get; dnf
// writes under /etc and uses sudo + dnf/yum; brew runs as the
// invoking user against the Homebrew prefix (no sudo) and reaches
// the network on every `brew tap` (it fetches the tap's git remote).
// Defaults to the apt shape when no driver block is set so a
// malformed step still surfaces an apt-flavoured permission preflight.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	if step == nil || step.PkgRepo == nil {
		return actions.PermissionSet{
			Sudo:             true,
			RequiredBinaries: []string{"apt-get"},
		}
	}
	r := step.PkgRepo
	state := r.State
	if state == "" {
		state = statePresent
	}
	if r.Brew != nil {
		// brew tap/untap is user-space; mutating state always reaches
		// the network (the tap is a git remote). Listing taps does
		// not, but the safe declaration is "network=true" — clients
		// shouldn't gate on listing alone.
		return actions.PermissionSet{
			Sudo:             false,
			Network:          true,
			RequiredBinaries: []string{"brew"},
		}
	}
	if r.Dnf != nil {
		ps := actions.PermissionSet{
			Sudo:             true,
			RequiredBinaries: []string{"dnf"},
			FilesystemWrite: []string{
				dnf.RepoPath(r.Name),
				dnf.KeyringPath(r.Name),
			},
		}
		if state == statePresent && r.Dnf.GPGKeyURL != "" {
			ps.Network = true
		}
		return ps
	}
	ps := actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{"apt-get"},
	}
	if state == statePresent && r.Apt != nil && r.Apt.GPGKeyURL != "" {
		ps.Network = true
	}
	ps.FilesystemWrite = []string{
		apt.SourcesPath(r.Name),
		apt.KeyringPath(r.Name),
	}
	return ps
}

// validateApt enforces apt-driver field rules for state=present. Split
// out of Validate so the cross-cutting checks there stay below the
// project-wide cyclomatic cap. Proposal-22 added the ppa branch.
func validateApt(a *config.PkgRepoApt) error {
	hasPPA := strings.TrimSpace(a.PPA) != ""
	hasURI := strings.TrimSpace(a.URI) != ""
	switch {
	case hasPPA && hasURI:
		return fmt.Errorf("pkg.repo.apt: ppa and uri are mutually exclusive (ppa derives uri)")
	case hasPPA && a.GPGKeyURL != "":
		return fmt.Errorf("pkg.repo.apt: ppa and gpg_key_url are mutually exclusive (ppa discovers the key via launchpad)")
	case !hasPPA && !hasURI:
		return fmt.Errorf("pkg.repo.apt: one of uri or ppa is required when state=present")
	}
	if hasPPA {
		if !apt.PPARE.MatchString(a.PPA) {
			return fmt.Errorf("pkg.repo.apt: ppa %q must match <owner>/<name> (e.g. neovim-ppa/unstable)", a.PPA)
		}
	} else if len(a.Suites) == 0 {
		return fmt.Errorf("pkg.repo.apt: suites is required when state=present")
	}
	// gpg_key_fingerprint pinning rule still applies when the user
	// supplies their own gpg_key_url. With `ppa:` the driver discovers
	// the fingerprint from launchpad before fetching the key, so the
	// operator doesn't need to set it by hand.
	if a.GPGKeyURL != "" && apt.GPGCheckEnabled(a) && strings.TrimSpace(a.GPGKeyFingerprint) == "" {
		return fmt.Errorf("pkg.repo.apt: gpg_key_fingerprint is required when gpg_check is true (set gpg_check: false to opt out)")
	}
	return nil
}

func (h *Handler) Validate(step *config.Step) error {
	r := step.PkgRepo
	if r == nil {
		return fmt.Errorf("pkg.repo requires configuration")
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("pkg.repo: name is required")
	}
	if !shared.NameRE.MatchString(r.Name) {
		return fmt.Errorf("pkg.repo: name %q must match [A-Za-z0-9._-]+", r.Name)
	}
	state := shared.NormalizeState(r.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("pkg.repo: state must be present or absent, got %q", r.State)
	}
	blocks := 0
	if r.Apt != nil {
		blocks++
	}
	if r.Dnf != nil {
		blocks++
	}
	if r.Brew != nil {
		blocks++
	}
	if blocks == 0 {
		return fmt.Errorf("pkg.repo: at least one of apt/dnf/brew is required")
	}
	if blocks > 1 {
		return fmt.Errorf("pkg.repo: exactly one of apt/dnf/brew may be supplied per step")
	}
	if r.Apt != nil && state == statePresent {
		if err := validateApt(r.Apt); err != nil {
			return err
		}
	}
	if r.Dnf != nil && state == statePresent {
		sources := 0
		if strings.TrimSpace(r.Dnf.BaseURL) != "" {
			sources++
		}
		if strings.TrimSpace(r.Dnf.Metalink) != "" {
			sources++
		}
		if strings.TrimSpace(r.Dnf.Mirrorlist) != "" {
			sources++
		}
		if sources == 0 {
			return fmt.Errorf("pkg.repo.dnf: one of baseurl/metalink/mirrorlist is required when state=present")
		}
		if sources > 1 {
			return fmt.Errorf("pkg.repo.dnf: only one of baseurl/metalink/mirrorlist may be set per step")
		}
		if r.Dnf.GPGKeyURL != "" && dnf.GPGCheckEnabled(r.Dnf) && strings.TrimSpace(r.Dnf.GPGKeyFingerprint) == "" {
			return fmt.Errorf("pkg.repo.dnf: gpg_key_fingerprint is required when gpg_check is true (set gpg_check: false to opt out)")
		}
	}
	if r.Brew != nil && state == statePresent {
		// state=absent without a tap name is allowed because Name is
		// already required at the top and is what brew untap targets.
		// Require it on present so a misconfigured "tap: " can't
		// silently produce a noop.
		if strings.TrimSpace(r.Brew.Tap) == "" {
			return fmt.Errorf("pkg.repo.brew: tap is required when state=present")
		}
	}
	return nil
}

// Run dispatches to the per-driver sub-package based on which nested
// block the step populated. Validate has already enforced
// exactly-one-of-{apt,dnf,brew} by the time we reach here.
// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` actually retries this idempotent action via the
// centralized executor loop instead of being silently no-op'd.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	r := step.PkgRepo
	result := executor.NewResult()
	result.Checkable = true

	switch {
	case r.Dnf != nil:
		return dnf.Run(ctx, r, result)
	case r.Brew != nil:
		return brew.Run(ctx, r, result)
	case r.Apt != nil:
		return apt.Run(ctx, r, result)
	}
	return result, fmt.Errorf("pkg.repo: no driver configured")
}
