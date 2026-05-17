//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// dnfPaths controls where the dnf driver writes files. Tests
// override these to avoid touching /etc.
type dnfPaths struct {
	reposDir   string
	keyringDir string
}

// Package-level hooks. Defaults are wired to real production paths /
// binaries; tests substitute their own for hermetic runs.
var (
	dnf = dnfPaths{
		reposDir:   "/etc/yum.repos.d",
		keyringDir: "/etc/pki/rpm-gpg",
	}
	dnfCleanCache = realDnfCleanCache
)

// runDnf drives state=present / state=absent for a dnf/yum repository.
// Mirrors the apt path: render → plan → (plan-mode? exit) → capture
// pre-state → write atomically → run "dnf clean expire-cache".
func runDnf(ctx actions.Context, r *config.PkgRepo, result *executor.Result) (actions.Result, error) {
	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("pkg.repo.dnf: only Linux is supported; got %s", runtime.GOOS)
	}

	rendered, err := renderDnf(ctx, r)
	if err != nil {
		return result, err
	}

	plan, err := computeDnfPlan(r.Name, normalizeState(r.State), rendered)
	if err != nil {
		return result, err
	}

	result.Data = map[string]interface{}{
		"name":      r.Name,
		"operation": plan.operation,
		"repo":      plan.repoPath,
		"keyring":   plan.keyringPath,
		"driver":    "dnf",
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

	// Capture pre-apply .repo file state for Reverse(). Same shape as
	// the apt driver: read once so both create / update / delete
	// branches reach Reverse with a consistent snapshot.
	priorContent, priorExisted, _ := readFile(plan.repoPath)
	result.ReverseData = &PkgRepoReverseInfo{
		Name:         r.Name,
		SourcesPath:  plan.repoPath,
		KeyringPath:  plan.keyringPath,
		PriorExisted: priorExisted,
		PriorContent: priorContent,
	}

	if err := applyDnf(plan, rendered); err != nil {
		return result, err
	}

	if rendered.updateCache && plan.touchesRepo {
		if err := dnfCleanCache(); err != nil {
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

// renderedDnf holds the post-template, defaults-applied view of the
// dnf block plus enclosing state/name.
type renderedDnf struct {
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

func renderDnf(ctx actions.Context, r *config.PkgRepo) (renderedDnf, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	render := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return tmpl.Render(s, vars)
	}

	out := renderedDnf{
		name:        r.Name,
		state:       normalizeState(r.State),
		gpgCheck:    dnfGPGCheckEnabled(r.Dnf),
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
	if out.description, err = render(r.Dnf.Description); err != nil {
		return out, fmt.Errorf("pkg.repo: render description: %w", err)
	}
	if out.description == "" {
		out.description = r.Name
	}
	if out.baseURL, err = render(r.Dnf.BaseURL); err != nil {
		return out, fmt.Errorf("pkg.repo: render baseurl: %w", err)
	}
	if out.metalink, err = render(r.Dnf.Metalink); err != nil {
		return out, fmt.Errorf("pkg.repo: render metalink: %w", err)
	}
	if out.mirrorlist, err = render(r.Dnf.Mirrorlist); err != nil {
		return out, fmt.Errorf("pkg.repo: render mirrorlist: %w", err)
	}
	if out.gpgKeyURL, err = render(r.Dnf.GPGKeyURL); err != nil {
		return out, fmt.Errorf("pkg.repo: render gpg_key_url: %w", err)
	}
	if out.gpgKeyFingerprint, err = render(r.Dnf.GPGKeyFingerprint); err != nil {
		return out, fmt.Errorf("pkg.repo: render gpg_key_fingerprint: %w", err)
	}
	return out, nil
}

func dnfGPGCheckEnabled(d *config.PkgRepoDnf) bool {
	if d.GPGCheck != nil {
		return *d.GPGCheck
	}
	return true
}

// dnfPlan describes what writes/deletes are needed to reconcile the
// dnf files with the desired state. Same shape as aptPlan.
type dnfPlan struct {
	changed     bool
	operation   string // "create" | "update" | "delete" | "noop"
	reason      string
	repoPath    string
	keyringPath string // empty if no keyring is involved
	wantContent string // desired .repo file content; empty when removing
	touchesRepo bool
}

func computeDnfPlan(name, state string, r renderedDnf) (dnfPlan, error) {
	repoPath := filepath.Join(dnf.reposDir, name+".repo")
	plan := dnfPlan{repoPath: repoPath}

	if r.gpgKeyURL != "" {
		plan.keyringPath = filepath.Join(dnf.keyringDir, "RPM-GPG-KEY-"+name)
	}

	if state == stateAbsent {
		existed, err := pathExists(repoPath)
		if err != nil {
			return plan, err
		}
		if !existed {
			plan.operation = "noop"
			plan.reason = "repo file already absent"
			return plan, nil
		}
		plan.changed = true
		plan.operation = "delete"
		plan.reason = "would remove " + repoPath
		plan.touchesRepo = true
		return plan, nil
	}

	want := renderRepoFile(r, plan.keyringPath)
	plan.wantContent = want

	current, exists, err := readFile(repoPath)
	if err != nil {
		return plan, err
	}
	switch {
	case !exists:
		plan.changed = true
		plan.operation = "create"
		plan.reason = "would create " + repoPath
		plan.touchesRepo = true
	case current != want:
		plan.changed = true
		plan.operation = "update"
		plan.reason = "would update " + repoPath + " (content drift)"
		plan.touchesRepo = true
	default:
		plan.operation = "noop"
		plan.reason = "repo file already at desired state"
	}
	return plan, nil
}

// renderRepoFile emits the byte-identical .repo INI content for the
// desired repository. Stable field order so idempotency checks are
// straightforward (computeDnfPlan compares byte-for-byte).
func renderRepoFile(r renderedDnf, keyringPath string) string {
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

func applyDnf(plan dnfPlan, r renderedDnf) error {
	if plan.operation == "delete" {
		if err := os.Remove(plan.repoPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("pkg.repo.dnf: remove repo file: %w", err)
		}
		// Keyring isn't auto-removed (same convention as apt): it may
		// be shared with another repo and rpm-gpg keys are routinely
		// kept indefinitely. An operator who needs full cleanup can
		// chain a file.write absent step against KeyringPath.
		return nil
	}

	if err := os.MkdirAll(dnf.reposDir, 0o755); err != nil {
		return fmt.Errorf("pkg.repo.dnf: mkdir repos: %w", err)
	}

	if plan.keyringPath != "" {
		if err := os.MkdirAll(dnf.keyringDir, 0o755); err != nil {
			return fmt.Errorf("pkg.repo.dnf: mkdir keyring: %w", err)
		}
		body, err := fetchKey(r.gpgKeyURL)
		if err != nil {
			return fmt.Errorf("pkg.repo.dnf: fetch gpg key: %w", err)
		}
		// Mirror the apt driver's F034 fix: verify fingerprint BEFORE
		// writing the key to the trusted rpm-gpg dir. Without this,
		// the operator's pinned fingerprint is decorative and dnf
		// silently trusts whatever bytes the URL served.
		if r.gpgKeyFingerprint != "" {
			if vErr := verifyKeyFingerprint(body, r.gpgKeyFingerprint); vErr != nil {
				return fmt.Errorf("pkg.repo.dnf: %w (key url: %s)", vErr, r.gpgKeyURL)
			}
		}
		if err := writeAtomic(plan.keyringPath, body, 0o644); err != nil {
			return fmt.Errorf("pkg.repo.dnf: write keyring: %w", err)
		}
	}

	if err := writeAtomic(plan.repoPath, []byte(plan.wantContent), 0o644); err != nil {
		return fmt.Errorf("pkg.repo.dnf: write repo file: %w", err)
	}
	return nil
}

// realDnfCleanCache runs `dnf clean expire-cache`. This is the
// dnf-idiomatic post-change refresh — it invalidates the per-repo
// metadata cache so the next `dnf install` sees the freshly-added
// repo without forcing a full network refresh up front.
//
// Falls back to `yum clean expire-cache` when dnf isn't on PATH so
// the driver works on older RHEL 7 hosts as well as Fedora / RHEL 8+.
func realDnfCleanCache() error {
	bin := "dnf"
	if _, err := exec.LookPath("dnf"); err != nil {
		if _, fallbackErr := exec.LookPath("yum"); fallbackErr == nil {
			bin = "yum"
		}
	}
	// #nosec G204 -- fixed dnf/yum binary, fixed args.
	cmd := exec.Command(bin, "clean", "expire-cache")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
