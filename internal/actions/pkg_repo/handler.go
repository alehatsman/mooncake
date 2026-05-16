// Package pkg_repo implements the pkg.repo action: declarative
// third-party package repository management. v1 ships an apt driver
// (DEB822 source list + keyring) and a brew driver for taps (macOS
// + Homebrew on Linux); the dnf block is accepted in YAML but
// returns a clear "not yet implemented" error.
//
//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/httputil"
)

const (
	actionName       = "pkg.repo"
	statePresent     = "present"
	stateAbsent      = "absent"
	atomicTempSuffix = ".mooncake-tmp"
)

// aptPaths controls where the apt driver writes files. Tests
// override these to avoid touching /etc.
type aptPaths struct {
	sourcesDir  string
	keyringsDir string
}

// Package-level hooks. The defaults are wired to real production
// behavior; tests override them to keep apply-mode hermetic.
var (
	apt = aptPaths{
		sourcesDir:  "/etc/apt/sources.list.d",
		keyringsDir: "/etc/apt/keyrings",
	}
	fetchKey    = httpFetchKey
	updateCache = aptGetUpdate

	// Brew driver hooks (proposal-08). brewListTaps returns the
	// current set of tapped repositories (one per line from
	// `brew tap`, lowercased); brewExec runs `brew <args...>` to
	// mutate state. Pre-checking via the lister is what makes
	// the action tolerant to brew's "already tapped" rc-1 — we
	// only call brew tap/untap when the state actually mismatches,
	// so the shell:`brew tap … || true` workaround the proposal
	// describes is unnecessary.
	brewListTaps = realBrewListTaps
	brewExec     = realBrewExec
)

// Handler implements pkg.repo.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage a third-party package repository (apt + brew taps; dnf deferred)",
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

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// Per-driver: apt writes under /etc and uses sudo + apt-get; brew
// runs as the invoking user against the Homebrew prefix (no sudo)
// and reaches the network on every `brew tap` (it fetches the tap's
// git remote). Defaults to the apt shape when no driver block is
// set so a malformed step still surfaces an apt-flavoured
// permission preflight.
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
	ps := actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{"apt-get"},
	}
	if state == statePresent && r.Apt != nil && r.Apt.GPGKeyURL != "" {
		ps.Network = true
	}
	// FilesystemWrite lists the canonical apt paths; dnf is a
	// reserved driver and we don't pretend to know its layout yet.
	ps.FilesystemWrite = []string{
		apt.sourcesDir + "/" + r.Name + ".sources",
		apt.keyringsDir + "/" + r.Name + ".gpg",
	}
	return ps
}

func (h *Handler) Validate(step *config.Step) error {
	r := step.PkgRepo
	if r == nil {
		return fmt.Errorf("pkg.repo requires configuration")
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("pkg.repo: name is required")
	}
	if !nameRE.MatchString(r.Name) {
		return fmt.Errorf("pkg.repo: name %q must match [A-Za-z0-9._-]+", r.Name)
	}
	state := normalizeState(r.State)
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
	if r.Apt != nil {
		if state == statePresent {
			if strings.TrimSpace(r.Apt.URI) == "" {
				return fmt.Errorf("pkg.repo.apt: uri is required when state=present")
			}
			if len(r.Apt.Suites) == 0 {
				return fmt.Errorf("pkg.repo.apt: suites is required when state=present")
			}
			if r.Apt.GPGKeyURL != "" && gpgCheckEnabled(r.Apt) && strings.TrimSpace(r.Apt.GPGKeyFingerprint) == "" {
				return fmt.Errorf("pkg.repo.apt: gpg_key_fingerprint is required when gpg_check is true (set gpg_check: false to opt out)")
			}
		}
	}
	if r.Brew != nil && state == statePresent {
		// state=absent is allowed without a tap name only because
		// state=absent without a name doesn't make sense generally —
		// the top-level Name field is already required by the check
		// above and is what brew untap targets, so the tap field is
		// strictly informational on absent. Require it on present so
		// a misconfigured "tap: " can't silently produce a noop.
		if strings.TrimSpace(r.Brew.Tap) == "" {
			return fmt.Errorf("pkg.repo.brew: tap is required when state=present")
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	r := step.PkgRepo
	result := executor.NewResult()
	result.Checkable = true

	if r.Dnf != nil {
		return result, fmt.Errorf("pkg.repo: dnf driver is not yet implemented (apt + brew supported)")
	}
	if r.Brew != nil {
		return runBrew(ctx, r, result)
	}
	if r.Apt == nil {
		return result, fmt.Errorf("pkg.repo: no driver configured")
	}
	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("pkg.repo.apt: only Linux is supported; got %s", runtime.GOOS)
	}

	rendered, err := renderApt(ctx, r)
	if err != nil {
		return result, err
	}

	plan, err := computeAptPlan(r.Name, normalizeState(r.State), rendered)
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

	// Capture pre-apply sources file state for Reverse(). The
	// computeAptPlan path already calls readFile on the sources
	// path when state=present, but state=absent only calls
	// pathExists; re-read here so both branches capture content.
	priorContent, priorExisted, _ := readFile(plan.sourcesPath)
	result.ReverseData = &PkgRepoReverseInfo{
		Name:         r.Name,
		SourcesPath:  plan.sourcesPath,
		KeyringPath:  plan.keyringPath,
		PriorExisted: priorExisted,
		PriorContent: priorContent,
	}

	if err := applyApt(plan, rendered); err != nil {
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

// renderedApt holds the post-template, defaults-applied view of the
// apt block plus enclosing state/name.
type renderedApt struct {
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

func renderApt(ctx actions.Context, r *config.PkgRepo) (renderedApt, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	render := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return tmpl.Render(s, vars)
	}
	renderAll := func(in []string) ([]string, error) {
		out := make([]string, 0, len(in))
		for _, s := range in {
			rs, err := render(s)
			if err != nil {
				return nil, err
			}
			out = append(out, rs)
		}
		return out, nil
	}

	out := renderedApt{
		name:        r.Name,
		state:       normalizeState(r.State),
		gpgCheck:    gpgCheckEnabled(r.Apt),
		updateCache: true,
	}
	if r.Apt.UpdateCache != nil {
		out.updateCache = *r.Apt.UpdateCache
	}

	var err error
	if out.uri, err = render(r.Apt.URI); err != nil {
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
	if out.gpgKeyURL, err = render(r.Apt.GPGKeyURL); err != nil {
		return out, fmt.Errorf("pkg.repo: render gpg_key_url: %w", err)
	}
	if out.gpgKeyFingerprint, err = render(r.Apt.GPGKeyFingerprint); err != nil {
		return out, fmt.Errorf("pkg.repo: render gpg_key_fingerprint: %w", err)
	}
	return out, nil
}

func gpgCheckEnabled(a *config.PkgRepoApt) bool {
	if a.GPGCheck != nil {
		return *a.GPGCheck
	}
	return true
}

// aptPlan describes what writes/deletes are needed to reconcile the
// apt files with the desired state.
type aptPlan struct {
	changed        bool
	operation      string // "create" | "update" | "delete" | "noop"
	reason         string
	sourcesPath    string
	keyringPath    string // empty if no keyring is involved
	wantContent    string // desired sources file content; empty when removing
	touchesSources bool
}

func computeAptPlan(name, state string, r renderedApt) (aptPlan, error) {
	sourcesPath := filepath.Join(apt.sourcesDir, name+".sources")
	plan := aptPlan{sourcesPath: sourcesPath}

	if r.gpgKeyURL != "" {
		plan.keyringPath = filepath.Join(apt.keyringsDir, name+".gpg")
	}

	if state == stateAbsent {
		existed, err := pathExists(sourcesPath)
		if err != nil {
			return plan, err
		}
		if !existed {
			plan.operation = "noop"
			plan.reason = "source file already absent"
			return plan, nil
		}
		plan.changed = true
		plan.operation = "delete"
		plan.reason = "would remove " + sourcesPath
		plan.touchesSources = true
		return plan, nil
	}

	want := renderDEB822(r, plan.keyringPath)
	plan.wantContent = want

	current, exists, err := readFile(sourcesPath)
	if err != nil {
		return plan, err
	}
	switch {
	case !exists:
		plan.changed = true
		plan.operation = "create"
		plan.reason = "would create " + sourcesPath
		plan.touchesSources = true
	case current != want:
		plan.changed = true
		plan.operation = "update"
		plan.reason = "would update " + sourcesPath + " (content drift)"
		plan.touchesSources = true
	default:
		plan.operation = "noop"
		plan.reason = "source file already at desired state"
	}
	return plan, nil
}

// renderDEB822 emits the byte-identical DEB822 content for the
// desired source. Stable field order so idempotency checks are
// straightforward.
func renderDEB822(r renderedApt, keyringPath string) string {
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

func applyApt(plan aptPlan, r renderedApt) error {
	if plan.operation == "delete" {
		if err := os.Remove(plan.sourcesPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("pkg.repo.apt: remove sources: %w", err)
		}
		// Keyring isn't auto-removed: it may be shared with another
		// repo. Future spec-22 reverse hook can do reference counting.
		return nil
	}

	// Ensure parent directories exist.
	if err := os.MkdirAll(apt.sourcesDir, 0o755); err != nil {
		return fmt.Errorf("pkg.repo.apt: mkdir sources: %w", err)
	}

	if plan.keyringPath != "" {
		if err := os.MkdirAll(apt.keyringsDir, 0o755); err != nil {
			return fmt.Errorf("pkg.repo.apt: mkdir keyrings: %w", err)
		}
		body, err := fetchKey(r.gpgKeyURL)
		if err != nil {
			return fmt.Errorf("pkg.repo.apt: fetch gpg key: %w", err)
		}
		// F034: verify the fetched key matches the operator-pinned
		// fingerprint BEFORE writing it to the trusted keyring. Without
		// this check the fingerprint requirement at Validate time is
		// security theater — the operator supplies a fingerprint, the
		// handler fetches whatever bytes the URL serves (over plain
		// HTTP for some configs), and apt then trusts the unverified
		// key forever. gpgCheckEnabled was already enforced at Validate;
		// an empty fingerprint here means the operator opted out via
		// `gpg_check: false` and we skip on purpose.
		if r.gpgKeyFingerprint != "" {
			if vErr := verifyKeyFingerprint(body, r.gpgKeyFingerprint); vErr != nil {
				return fmt.Errorf("pkg.repo.apt: %w (key url: %s)", vErr, r.gpgKeyURL)
			}
		}
		if err := writeAtomic(plan.keyringPath, body, 0o644); err != nil {
			return fmt.Errorf("pkg.repo.apt: write keyring: %w", err)
		}
	}

	if err := writeAtomic(plan.sourcesPath, []byte(plan.wantContent), 0o644); err != nil {
		return fmt.Errorf("pkg.repo.apt: write sources: %w", err)
	}
	return nil
}

// pathExists reports whether a path exists. Errors other than
// fs.ErrNotExist surface to the caller.
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}

func readFile(path string) (string, bool, error) {
	// #nosec G304 -- path is constructed from validated name + fixed dirs.
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), true, nil
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	tmp := path + atomicTempSuffix
	if err := os.WriteFile(tmp, content, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func httpFetchKey(url string) ([]byte, error) {
	// F012: route through httputil for bounded dial / TLS /
	// response-headers timeouts. pkg.repo's caller chain doesn't
	// thread its step ctx down here yet — Background suffices once
	// the transport-level timeouts are in place.
	req, err := httputil.NewRequest(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	// #nosec G107 -- URL comes from user-supplied YAML.
	resp, err := httputil.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("empty key body")
	}
	return body, nil
}

func aptGetUpdate() error {
	// #nosec G204 -- fixed apt-get binary.
	cmd := exec.Command("apt-get", "update")
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

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}
