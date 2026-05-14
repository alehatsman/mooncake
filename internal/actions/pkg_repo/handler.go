// Package pkg_repo implements the pkg.repo action: declarative
// third-party package repository management. v1 ships an apt driver
// (DEB822 source list + keyring); dnf and brew blocks are accepted
// in YAML but return a clear "not yet implemented" error.
//
//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"bytes"
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
	fetchKey = httpFetchKey
	updateCache = aptGetUpdate
)

// Handler implements pkg.repo.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage a third-party package repository (apt; dnf/brew deferred)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventFileUpdated)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
	}
}

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

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
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	r := step.PkgRepo
	result := executor.NewResult()
	result.Checkable = true

	if r.Dnf != nil {
		return result, fmt.Errorf("pkg.repo: dnf driver is not yet implemented (apt only in v1)")
	}
	if r.Brew != nil {
		return result, fmt.Errorf("pkg.repo: brew driver is not yet implemented (apt only in v1)")
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
	// #nosec G107 -- URL comes from user-supplied YAML.
	resp, err := http.Get(url)
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
