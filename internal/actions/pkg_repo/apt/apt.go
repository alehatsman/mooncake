// Package apt implements the apt driver for pkg.repo. Called from
// the parent package's Run dispatcher when step.PkgRepo.Apt is set.
//
// Writes a DEB822 source list to /etc/apt/sources.list.d/<name>.sources
// and, when gpg_key_url is set, a binary keyring to
// /etc/apt/keyrings/<name>.gpg. Atomic write + idempotent compare so
// re-applies skip a no-op write.
package apt

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

// Paths controls where the apt driver writes files. Tests override
// these via the package-level `paths` var to avoid touching /etc.
type Paths struct {
	SourcesDir  string
	KeyringsDir string
}

// Package-level hooks. Defaults are wired to real production paths
// / binaries; tests substitute their own for hermetic runs. Spec-69
// phase-5: updateCache takes a runner so the package no longer carries
// per-Run mutable state.
var (
	paths = Paths{
		SourcesDir:  "/etc/apt/sources.list.d",
		KeyringsDir: "/etc/apt/keyrings",
	}
	updateCache = realAptGetUpdate // func(runner) error
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

	// Proposal-22: expand `ppa:` shorthand into uri/suites/components +
	// (when state=present and the operator didn't pre-pin) the
	// launchpad-discovered fingerprint and keyserver URL. The expansion
	// is deferred until apply time because fingerprint discovery is a
	// network call.
	if r.Apt.PPA != "" {
		state := shared.NormalizeState(r.State)
		if err := expandPPA(ctx, &rendered, r.Apt, state); err != nil {
			return result, err
		}
	}

	plan, err := computePlan(r.Name, shared.NormalizeState(r.State), rendered)
	if err != nil {
		return result, err
	}

	result.Operation = executor.Operation(plan.operation)
	result.Target = r.Name
	result.Data = map[string]interface{}{
		"name":    r.Name,
		"sources": plan.sourcesPath,
		"keyring": plan.keyringPath,
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

	// Spec-69 phase 5: runner + performer are per-Run, threaded into
	// apply / updateCache. Tests that override paths.SourcesDir to a
	// t.TempDir continue to work because the Performer tries direct first.
	runner := ctx.Privileged()
	performer := ctx.Effects()

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

	if err := apply(ctx.Ctx(), performer, plan, rendered); err != nil {
		return result, err
	}

	if rendered.updateCache && plan.touchesSources {
		if err := updateCache(runner); err != nil {
			return result, fmt.Errorf("pkg.repo.apt: apt-get update: %w", err)
		}
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.Logger().Infof("  pkg.repo: %s (%s)", r.Name, plan.operation)

	if pub := ctx.EventPublisher(); pub != nil {
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
	tmpl := ctx.Template()
	vars := ctx.Variables()
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

func apply(ctx context.Context, performer actions.Performer, p plan_, r rendered_) error {
	// All file ops go through the supplied Performer with Become: true.
	// The Performer (spec-69 phase 5b) tries the direct os.* first and
	// only sudos on EACCES, so this works equally well under sudo
	// against /etc and against a user-owned tempdir in tests.
	pOpts := actions.PerformerOpts{}
	pOptsWithMode := actions.PerformerOpts{ExplicitMode: true}

	if p.operation == "delete" {
		if e := performer.Remove(p.sourcesPath, false, pOpts); e.Err != nil && !errors.Is(e.Err, fs.ErrNotExist) {
			return fmt.Errorf("pkg.repo.apt: remove sources: %w", e.Err)
		}
		// Keyring isn't auto-removed: it may be shared with another
		// repo. Future spec-22 reverse hook can do reference counting.
		return nil
	}

	if e := performer.Mkdir(paths.SourcesDir, 0o755, pOpts); e.Err != nil {
		return fmt.Errorf("pkg.repo.apt: mkdir sources: %w", e.Err)
	}

	if p.keyringPath != "" {
		if e := performer.Mkdir(paths.KeyringsDir, 0o755, pOpts); e.Err != nil {
			return fmt.Errorf("pkg.repo.apt: mkdir keyrings: %w", e.Err)
		}
		body, err := shared.HTTPFetchKey(ctx, r.gpgKeyURL)
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
		// apt's DEB822 Signed-By expects a *binary* keyring. Keys from
		// keyserver.ubuntu.com (the ppa shorthand path) and many third-
		// party mirrors return ASCII-armored .asc bodies, so normalise
		// here. shared.MaybeDearmor is a no-op for binary input.
		keyBytes, err := shared.MaybeDearmor(body)
		if err != nil {
			return fmt.Errorf("pkg.repo.apt: normalise gpg key: %w", err)
		}
		if e := performer.WriteFile(p.keyringPath, keyBytes, 0o644, pOptsWithMode); e.Err != nil {
			return fmt.Errorf("pkg.repo.apt: write keyring: %w", e.Err)
		}
	}

	if e := performer.WriteFile(p.sourcesPath, []byte(p.wantContent), 0o644, pOptsWithMode); e.Err != nil {
		return fmt.Errorf("pkg.repo.apt: write sources: %w", e.Err)
	}
	return nil
}

func realAptGetUpdate(runner *security.Privileged) error {
	out, err := runner.Run(context.TODO(), "apt-get", "update")
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
