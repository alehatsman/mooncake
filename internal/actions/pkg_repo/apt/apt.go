// Package apt implements the apt driver for pkg.repo. Called from
// the parent package's Run dispatcher when step.PkgRepo.Apt is set.
//
// Writes a DEB822 source list to /etc/apt/sources.list.d/<name>.sources
// and, when gpg_key_url is set, a binary keyring to
// /etc/apt/keyrings/<name>.gpg. Atomic write + idempotent compare so
// re-applies skip a no-op write.
package apt

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/shared"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// privRunner is the spec-69 sudo-escalating command runner used by the
// apt-get update hook. Set by Run() from ctx.Privileged() before
// calling apply / updateCache; tests that stub updateCache bypass this
// entirely. See pkg_upgrade for the same pattern.
var privRunner actions.PrivilegedRunner = security.PrivilegedRunner{}

// Paths controls where the apt driver writes files. Tests override
// these via the package-level `paths` var to avoid touching /etc.
type Paths struct {
	SourcesDir  string
	KeyringsDir string
}

// Package-level hooks. Defaults are wired to real production paths
// / binaries; tests substitute their own for hermetic runs.
var (
	paths = Paths{
		SourcesDir:  "/etc/apt/sources.list.d",
		KeyringsDir: "/etc/apt/keyrings",
	}
	updateCache = realAptGetUpdate
)

// SourcesPath returns the absolute path to the DEB822 sources file
// for `name`. Exported so the parent's Permissions() can advertise
// FilesystemWrite without the parent needing to read driver
// internals.
func SourcesPath(name string) string {
	return filepath.Join(paths.SourcesDir, name+".sources")
}

// KeyringPath returns the absolute path to the GPG keyring file for
// `name`. Exported for the same reason as SourcesPath.
func KeyringPath(name string) string {
	return filepath.Join(paths.KeyringsDir, name+".gpg")
}

// Run drives state=present / state=absent for an apt repo. Mirrors
// the parent handler's old runApt path: render → plan → (plan-mode?
// exit) → capture pre-state → write atomically → run apt-get update.
func Run(ctx actions.Context, r *config.PkgRepo, result *executor.Result) (actions.Result, error) {
	if r.Apt == nil {
		return result, fmt.Errorf("pkg.repo: no driver configured")
	}
	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("pkg.repo.apt: only Linux is supported; got %s", runtime.GOOS)
	}

	rendered, err := render(ctx, r)
	if err != nil {
		return result, err
	}

	plan, err := computePlan(r.Name, shared.NormalizeState(r.State), rendered)
	if err != nil {
		return result, err
	}

	result.Data = map[string]interface{}{
		"name":      r.Name,
		"operation": plan.operation,
		"sources":   plan.sourcesPath,
		"keyring":   plan.keyringPath,
	}

	if !plan.changed {
		result.Reason = plan.reason
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = plan.reason
		return result, nil
	}

	// Wire the spec-69 sudo-aware runner so apt-get update escalates
	// to root when mooncake runs as a non-root user. File-op
	// migration is deferred — see the apply() comment.
	privRunner = ctx.Privileged()

	// Capture pre-apply sources file state for Reverse(). The plan
	// path already calls ReadFile on the sources path when
	// state=present, but state=absent only calls PathExists; re-read
	// here so both branches capture content.
	priorContent, priorExisted, _ := shared.ReadFile(plan.sourcesPath)
	result.ReverseData = &shared.PkgRepoReverseInfo{
		Name:         r.Name,
		SourcesPath:  plan.sourcesPath,
		KeyringPath:  plan.keyringPath,
		PriorExisted: priorExisted,
		PriorContent: priorContent,
	}

	if err := apply(plan, rendered); err != nil {
		return result, err
	}

	if rendered.updateCache && plan.touchesSources {
		if err := updateCache(); err != nil {
			return result, fmt.Errorf("pkg.repo.apt: apt-get update: %w", err)
		}
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  pkg.repo: %s (%s)", r.Name, plan.operation)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{Path: plan.sourcesPath, Changed: true},
		})
	}
	return result, nil
}

// rendered holds the post-template, defaults-applied view of the
// apt block plus enclosing state/name.
type rendered_ struct { //nolint:revive // local-only struct; the trailing _ disambiguates from the package's `render` function
	name              string
	state             string
	uri               string
	suites            []string
	components        []string
	architectures     []string
	gpgKeyURL         string
	gpgKeyFingerprint string
	gpgCheck          bool
	updateCache       bool
}

func render(ctx actions.Context, r *config.PkgRepo) (rendered_, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	renderOne := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return tmpl.Render(s, vars)
	}
	renderAll := func(in []string) ([]string, error) {
		out := make([]string, 0, len(in))
		for _, s := range in {
			rs, err := renderOne(s)
			if err != nil {
				return nil, err
			}
			out = append(out, rs)
		}
		return out, nil
	}

	out := rendered_{
		name:        r.Name,
		state:       shared.NormalizeState(r.State),
		gpgCheck:    GPGCheckEnabled(r.Apt),
		updateCache: true,
	}
	if r.Apt.UpdateCache != nil {
		out.updateCache = *r.Apt.UpdateCache
	}

	var err error
	if out.uri, err = renderOne(r.Apt.URI); err != nil {
		return out, fmt.Errorf("pkg.repo: render uri: %w", err)
	}
	if out.suites, err = renderAll(r.Apt.Suites); err != nil {
		return out, fmt.Errorf("pkg.repo: render suites: %w", err)
	}
	comps := r.Apt.Components
	if len(comps) == 0 {
		comps = []string{"main"}
	}
	if out.components, err = renderAll(comps); err != nil {
		return out, fmt.Errorf("pkg.repo: render components: %w", err)
	}
	if out.architectures, err = renderAll(r.Apt.Architectures); err != nil {
		return out, fmt.Errorf("pkg.repo: render architectures: %w", err)
	}
	if out.gpgKeyURL, err = renderOne(r.Apt.GPGKeyURL); err != nil {
		return out, fmt.Errorf("pkg.repo: render gpg_key_url: %w", err)
	}
	if out.gpgKeyFingerprint, err = renderOne(r.Apt.GPGKeyFingerprint); err != nil {
		return out, fmt.Errorf("pkg.repo: render gpg_key_fingerprint: %w", err)
	}
	return out, nil
}

// GPGCheckEnabled reports whether the apt block has gpg_check on
// (the default when unset). Exported because the parent's Validate
// needs to enforce the fingerprint-required rule.
func GPGCheckEnabled(a *config.PkgRepoApt) bool {
	if a.GPGCheck != nil {
		return *a.GPGCheck
	}
	return true
}

// plan_ describes what writes/deletes are needed to reconcile the
// apt files with the desired state. The trailing underscore avoids
// collision with the verb-named computePlan / apply functions.
type plan_ struct { //nolint:revive
	changed        bool
	operation      string // "create" | "update" | "delete" | "noop"
	reason         string
	sourcesPath    string
	keyringPath    string // empty if no keyring is involved
	wantContent    string // desired sources file content; empty when removing
	touchesSources bool
}

func computePlan(name, state string, r rendered_) (plan_, error) {
	sourcesPath := SourcesPath(name)
	p := plan_{sourcesPath: sourcesPath}

	if r.gpgKeyURL != "" {
		p.keyringPath = KeyringPath(name)
	}

	if state == shared.StateAbsent {
		existed, err := shared.PathExists(sourcesPath)
		if err != nil {
			return p, err
		}
		if !existed {
			p.operation = "noop"
			p.reason = "source file already absent"
			return p, nil
		}
		p.changed = true
		p.operation = "delete"
		p.reason = "would remove " + sourcesPath
		p.touchesSources = true
		return p, nil
	}

	want := renderDEB822(r, p.keyringPath)
	p.wantContent = want

	current, exists, err := shared.ReadFile(sourcesPath)
	if err != nil {
		return p, err
	}
	switch {
	case !exists:
		p.changed = true
		p.operation = "create"
		p.reason = "would create " + sourcesPath
		p.touchesSources = true
	case current != want:
		p.changed = true
		p.operation = "update"
		p.reason = "would update " + sourcesPath + " (content drift)"
		p.touchesSources = true
	default:
		p.operation = "noop"
		p.reason = "source file already at desired state"
	}
	return p, nil
}

// renderDEB822 emits the byte-identical DEB822 content for the
// desired source. Stable field order so idempotency checks are
// straightforward.
func renderDEB822(r rendered_, keyringPath string) string {
	var sb strings.Builder
	sb.WriteString("# Managed by mooncake pkg.repo. Do not edit by hand.\n")
	sb.WriteString("Types: deb\n")
	sb.WriteString("URIs: ")
	sb.WriteString(r.uri)
	sb.WriteByte('\n')
	sb.WriteString("Suites: ")
	sb.WriteString(strings.Join(r.suites, " "))
	sb.WriteByte('\n')
	sb.WriteString("Components: ")
	sb.WriteString(strings.Join(r.components, " "))
	sb.WriteByte('\n')
	if len(r.architectures) > 0 {
		sb.WriteString("Architectures: ")
		sb.WriteString(strings.Join(r.architectures, " "))
		sb.WriteByte('\n')
	}
	if keyringPath != "" {
		sb.WriteString("Signed-By: ")
		sb.WriteString(keyringPath)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func apply(p plan_, r rendered_) error {
	// Note (spec-69 phase 5b): the file ops below still call os.* /
	// shared.WriteAtomic directly. Migrating them to ctx.Effects()
	// with PerformerOpts{Become: true} regresses tests that point
	// paths.SourcesDir at a user-owned tempdir (Performer always
	// sudos when Become is set, even when the target is writable by
	// the current user). Two-step fix lives in spec-69: (1) teach
	// the Performer to fall back to direct ops when the user has
	// write access (matches service/handler.go:writeFileWithPrivileges),
	// (2) migrate this apply path. For now, the privRunner-driven
	// apt-get update below is the only spec-69 site here — the file
	// writes inherit pre-spec-69 behavior, which is the same
	// "running as root works, non-root fails with EACCES" semantic
	// the rest of pkg.repo had before today.
	if p.operation == "delete" {
		if err := os.Remove(p.sourcesPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("pkg.repo.apt: remove sources: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(paths.SourcesDir, 0o755); err != nil {
		return fmt.Errorf("pkg.repo.apt: mkdir sources: %w", err)
	}

	if p.keyringPath != "" {
		if err := os.MkdirAll(paths.KeyringsDir, 0o755); err != nil {
			return fmt.Errorf("pkg.repo.apt: mkdir keyrings: %w", err)
		}
		body, err := shared.HTTPFetchKey(r.gpgKeyURL)
		if err != nil {
			return fmt.Errorf("pkg.repo.apt: fetch gpg key: %w", err)
		}
		// F034: verify the fetched key matches the operator-pinned
		// fingerprint BEFORE writing it to the trusted keyring.
		if r.gpgKeyFingerprint != "" {
			if vErr := shared.VerifyKeyFingerprint(body, r.gpgKeyFingerprint); vErr != nil {
				return fmt.Errorf("pkg.repo.apt: %w (key url: %s)", vErr, r.gpgKeyURL)
			}
		}
		if err := shared.WriteAtomic(p.keyringPath, body, 0o644); err != nil {
			return fmt.Errorf("pkg.repo.apt: write keyring: %w", err)
		}
	}

	if err := shared.WriteAtomic(p.sourcesPath, []byte(p.wantContent), 0o644); err != nil {
		return fmt.Errorf("pkg.repo.apt: write sources: %w", err)
	}
	return nil
}

func realAptGetUpdate() error {
	out, err := privRunner.Run(nil, "apt-get", "update")
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
