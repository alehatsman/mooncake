// Package dnf implements the dnf/yum driver for pkg.repo. Called
// from the parent package's Run dispatcher when step.PkgRepo.Dnf is
// set.
//
// Writes an INI .repo file to /etc/yum.repos.d/<name>.repo and,
// when gpg_key_url is set, a binary keyring to
// /etc/pki/rpm-gpg/RPM-GPG-KEY-<name>. Atomic write + idempotent
// compare so re-applies skip a no-op write. Falls back to `yum`
// when dnf isn't on PATH so the driver works on RHEL 7 hosts as
// well as Fedora / RHEL 8+.
package dnf

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
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

// Paths controls where the dnf driver writes files. Tests override
// via the package-level `paths` var to avoid touching /etc.
type Paths struct {
	ReposDir   string
	KeyringDir string
}

// Package-level hooks. Defaults are wired to real production paths
// / binaries; tests substitute their own for hermetic runs. Spec-69
// phase-5: cleanCache takes a runner so the package no longer carries
// per-Run mutable state.
var (
	paths = Paths{
		ReposDir:   "/etc/yum.repos.d",
		KeyringDir: "/etc/pki/rpm-gpg",
	}
	cleanCache = realCleanCache // func(runner) error
)

// RepoPath returns the absolute path to the .repo INI file for
// `name`. Exported so the parent's Permissions() can advertise
// FilesystemWrite without reading driver internals.
func RepoPath(name string) string {
	return filepath.Join(paths.ReposDir, name+".repo")
}

// KeyringPath returns the absolute path to the rpm-gpg keyring for
// `name`. Exported for the same reason as RepoPath.
func KeyringPath(name string) string {
	return filepath.Join(paths.KeyringDir, "RPM-GPG-KEY-"+name)
}

// Run drives state=present / state=absent for a dnf/yum repository.
// Mirrors the apt path: render → plan → (plan-mode? exit) → capture
// pre-state → write atomically → run "dnf clean expire-cache".
func Run(ctx actions.Context, r *config.PkgRepo, result *executor.Result) (actions.Result, error) {
	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("pkg.repo.dnf: only Linux is supported; got %s", runtime.GOOS)
	}

	rendered, err := render(ctx, r)
	if err != nil {
		return result, err
	}

	plan, err := computePlan(r.Name, shared.NormalizeState(r.State), rendered)
	if err != nil {
		return result, err
	}

	result.Operation = executor.Operation(plan.operation)
	result.Target = r.Name
	result.Data = map[string]interface{}{
		"name":    r.Name,
		"repo":    plan.repoPath,
		"keyring": plan.keyringPath,
		"driver":  "dnf",
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
	// apply / cleanCache. Performer's try-direct-then-fallback (phase
	// 5b) means file writes work under sudo against /etc and under
	// the test user against t.TempDir.
	runner := ctx.Privileged()
	performer := ctx.Effects()

	priorContent, priorExisted, _ := shared.ReadFile(plan.repoPath)
	result.ReverseData = &shared.PkgRepoReverseInfo{
		Name:         r.Name,
		SourcesPath:  plan.repoPath,
		KeyringPath:  plan.keyringPath,
		PriorExisted: priorExisted,
		PriorContent: priorContent,
	}

	if err := apply(performer, plan, rendered); err != nil {
		return result, err
	}

	if rendered.updateCache && plan.touchesRepo {
		if err := cleanCache(runner); err != nil {
			return result, fmt.Errorf("pkg.repo.dnf: dnf clean expire-cache: %w", err)
		}
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  pkg.repo: %s (%s)", r.Name, plan.operation)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{Path: plan.repoPath, Changed: true},
		})
	}
	return result, nil
}

// rendered_ holds the post-template, defaults-applied view of the
// dnf block plus enclosing state/name.
type rendered_ struct { //nolint:revive
	name              string
	state             string
	description       string
	baseURL           string
	metalink          string
	mirrorlist        string
	enabled           bool
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

	out := rendered_{
		name:        r.Name,
		state:       shared.NormalizeState(r.State),
		gpgCheck:    GPGCheckEnabled(r.Dnf),
		enabled:     true,
		updateCache: true,
	}
	if r.Dnf.Enabled != nil {
		out.enabled = *r.Dnf.Enabled
	}
	if r.Dnf.UpdateCache != nil {
		out.updateCache = *r.Dnf.UpdateCache
	}

	var err error
	if out.description, err = renderOne(r.Dnf.Description); err != nil {
		return out, fmt.Errorf("pkg.repo: render description: %w", err)
	}
	if out.description == "" {
		out.description = r.Name
	}
	if out.baseURL, err = renderOne(r.Dnf.BaseURL); err != nil {
		return out, fmt.Errorf("pkg.repo: render baseurl: %w", err)
	}
	if out.metalink, err = renderOne(r.Dnf.Metalink); err != nil {
		return out, fmt.Errorf("pkg.repo: render metalink: %w", err)
	}
	if out.mirrorlist, err = renderOne(r.Dnf.Mirrorlist); err != nil {
		return out, fmt.Errorf("pkg.repo: render mirrorlist: %w", err)
	}
	if out.gpgKeyURL, err = renderOne(r.Dnf.GPGKeyURL); err != nil {
		return out, fmt.Errorf("pkg.repo: render gpg_key_url: %w", err)
	}
	if out.gpgKeyFingerprint, err = renderOne(r.Dnf.GPGKeyFingerprint); err != nil {
		return out, fmt.Errorf("pkg.repo: render gpg_key_fingerprint: %w", err)
	}
	return out, nil
}

// GPGCheckEnabled reports whether the dnf block has gpg_check on
// (the default when unset). Exported because the parent's Validate
// needs to enforce the fingerprint-required rule.
func GPGCheckEnabled(d *config.PkgRepoDnf) bool {
	if d.GPGCheck != nil {
		return *d.GPGCheck
	}
	return true
}

// plan_ describes what writes/deletes are needed to reconcile the
// dnf files with the desired state.
type plan_ struct { //nolint:revive
	changed     bool
	operation   string
	reason      string
	repoPath    string
	keyringPath string
	wantContent string
	touchesRepo bool
}

func computePlan(name, state string, r rendered_) (plan_, error) {
	repoPath := RepoPath(name)
	p := plan_{repoPath: repoPath}

	if r.gpgKeyURL != "" {
		p.keyringPath = KeyringPath(name)
	}

	if state == shared.StateAbsent {
		existed, err := shared.PathExists(repoPath)
		if err != nil {
			return p, err
		}
		if !existed {
			p.operation = "noop"
			p.reason = "repo file already absent"
			return p, nil
		}
		p.changed = true
		p.operation = "delete"
		p.reason = "would remove " + repoPath
		p.touchesRepo = true
		return p, nil
	}

	want := renderRepoFile(r, p.keyringPath)
	p.wantContent = want

	current, exists, err := shared.ReadFile(repoPath)
	if err != nil {
		return p, err
	}
	switch {
	case !exists:
		p.changed = true
		p.operation = "create"
		p.reason = "would create " + repoPath
		p.touchesRepo = true
	case current != want:
		p.changed = true
		p.operation = "update"
		p.reason = "would update " + repoPath + " (content drift)"
		p.touchesRepo = true
	default:
		p.operation = "noop"
		p.reason = "repo file already at desired state"
	}
	return p, nil
}

// renderRepoFile emits the byte-identical .repo INI content for the
// desired repository.
func renderRepoFile(r rendered_, keyringPath string) string {
	var sb strings.Builder
	sb.WriteString("# Managed by mooncake pkg.repo. Do not edit by hand.\n")
	sb.WriteString("[")
	sb.WriteString(r.name)
	sb.WriteString("]\n")
	sb.WriteString("name=")
	sb.WriteString(r.description)
	sb.WriteByte('\n')
	if r.baseURL != "" {
		sb.WriteString("baseurl=")
		sb.WriteString(r.baseURL)
		sb.WriteByte('\n')
	}
	if r.metalink != "" {
		sb.WriteString("metalink=")
		sb.WriteString(r.metalink)
		sb.WriteByte('\n')
	}
	if r.mirrorlist != "" {
		sb.WriteString("mirrorlist=")
		sb.WriteString(r.mirrorlist)
		sb.WriteByte('\n')
	}
	sb.WriteString("enabled=")
	sb.WriteString(boolOneZero(r.enabled))
	sb.WriteByte('\n')
	sb.WriteString("gpgcheck=")
	sb.WriteString(boolOneZero(r.gpgCheck))
	sb.WriteByte('\n')
	if keyringPath != "" {
		sb.WriteString("gpgkey=file://")
		sb.WriteString(keyringPath)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func boolOneZero(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func apply(performer actions.Performer, p plan_, r rendered_) error {
	// All file ops go through the supplied Performer — its spec-69
	// phase 5b try-direct-then-fallback makes Become: true work
	// equally for /etc/yum.repos.d and tempdir-overridden test paths.
	pOpts := actions.PerformerOpts{}
	pOptsWithMode := actions.PerformerOpts{ExplicitMode: true}

	if p.operation == "delete" {
		if e := performer.Remove(p.repoPath, false, pOpts); e.Err != nil && !errors.Is(e.Err, fs.ErrNotExist) {
			return fmt.Errorf("pkg.repo.dnf: remove repo file: %w", e.Err)
		}
		return nil
	}

	if e := performer.Mkdir(paths.ReposDir, 0o755, pOpts); e.Err != nil {
		return fmt.Errorf("pkg.repo.dnf: mkdir repos: %w", e.Err)
	}

	if p.keyringPath != "" {
		if e := performer.Mkdir(paths.KeyringDir, 0o755, pOpts); e.Err != nil {
			return fmt.Errorf("pkg.repo.dnf: mkdir keyring: %w", e.Err)
		}
		body, err := shared.HTTPFetchKey(r.gpgKeyURL)
		if err != nil {
			return fmt.Errorf("pkg.repo.dnf: fetch gpg key: %w", err)
		}
		if r.gpgKeyFingerprint != "" {
			if vErr := shared.VerifyKeyFingerprint(body, r.gpgKeyFingerprint); vErr != nil {
				return fmt.Errorf("pkg.repo.dnf: %w (key url: %s)", vErr, r.gpgKeyURL)
			}
		}
		if e := performer.WriteFile(p.keyringPath, body, 0o644, pOptsWithMode); e.Err != nil {
			return fmt.Errorf("pkg.repo.dnf: write keyring: %w", e.Err)
		}
	}

	if e := performer.WriteFile(p.repoPath, []byte(p.wantContent), 0o644, pOptsWithMode); e.Err != nil {
		return fmt.Errorf("pkg.repo.dnf: write repo file: %w", e.Err)
	}
	return nil
}

// realCleanCache runs `dnf clean expire-cache` (with `yum` fallback
// on RHEL 7). Invalidates the per-repo metadata cache so the next
// `dnf install` sees the freshly-added repo without forcing a full
// network refresh up front.
func realCleanCache(runner *security.Privileged) error {
	bin := "dnf"
	if _, err := exec.LookPath("dnf"); err != nil {
		if _, fallbackErr := exec.LookPath("yum"); fallbackErr == nil {
			bin = "yum"
		}
	}
	out, err := runner.Run(context.TODO(), bin, "clean", "expire-cache")
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
