// Package brew implements the brew driver for pkg.repo. Called from
// the parent package's Run dispatcher when step.PkgRepo.Brew is set.
//
// Manages Homebrew taps. Brew taps are macOS's analogue of an APT
// sources.list entry — they pull a third-party formula repository
// onto the host. The driver LIST-first / mutate-only-on-drift
// pattern makes the action tolerant of brew's "already tapped" rc-1
// without forcing operators to write `failed_when: rc not in [0, 1]`
// shell wrappers (proposal-08).
package brew

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/pkg_repo/shared"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// PkgRepoBrewReverseInfo is the pre-apply snapshot for a brew-driver
// pkg.repo step. Stashed on Result.ReverseData so a future Reverser
// can build an inverse step (tap → untap, untap → tap) without
// re-running the live brew tap list.
type PkgRepoBrewReverseInfo struct {
	// Name is the step's canonical repo identifier.
	Name string `json:"name"`
	// Tap is the rendered, lowercased tap name actually targeted
	// (e.g. "homebrew/cask-fonts").
	Tap string `json:"tap"`
	// WasTapped reports whether the tap was present pre-apply. A
	// Reverser uses this directly: WasTapped=false means the apply
	// added it (reverse = untap); WasTapped=true means the apply
	// removed it (reverse = tap).
	WasTapped bool `json:"was_tapped"`
}

// R2.1c phase 2: register the brew-driver ReverseData type alongside
// the apt/dnf-driver one in the parent's init. Brew taps are a
// sub-driver of pkg.repo, not a separate handler — but they stash
// their own typed payload here, so the discriminator-tagged wire
// encoding needs the name known to decode correctly.
func init() {
	executor.RegisterReverseDataType("PkgRepoBrewReverseInfo", func() any { return &PkgRepoBrewReverseInfo{} })
}

// cmdTimeout caps every brew subprocess so a wedged remote can't
// hang the whole apply. brew tap fetches a git remote, brew untap is
// local; 60s covers slow networks without inviting indefinite waits.
const cmdTimeout = 60 * time.Second

// Package-level hooks for tests to override. F2: both accept ctx so
// SIGINT / fleet kill / MCP shutdown cancels in-flight brew subprocesses.
var (
	// ListTaps returns the current set of tapped repositories (one
	// per line from `brew tap`, lowercased).
	ListTaps = realListTaps
	// Exec runs `brew <args...>` to mutate state.
	Exec = realExec
)

// Run drives state=present / state=absent for a Homebrew tap. LIST
// first; only call brew tap / untap when the desired state actually
// differs from the world.
func Run(ctx actions.Context, r *config.PkgRepo, result *executor.Result) (actions.Result, error) {
	state := shared.NormalizeState(r.State)
	tap := strings.TrimSpace(r.Brew.Tap)

	// Render the tap through the template engine so {{ var }} works
	// the same way it does for r.Apt.URI / r.Name etc.
	if tap != "" {
		rendered, err := ctx.GetTemplate().Render(tap, ctx.GetVariables())
		if err != nil {
			return result, fmt.Errorf("pkg.repo.brew: render tap: %w", err)
		}
		tap = strings.TrimSpace(rendered)
	}
	// state=absent with no tap name targets r.Name (the canonical
	// repo identifier). Validate already enforces tap != "" when
	// state=present, so by here we have either an explicit tap or
	// an empty string + state=absent.
	if tap == "" {
		tap = r.Name
	}
	tap = strings.ToLower(tap)

	current, err := ListTaps(ctx.Ctx())
	if err != nil {
		return result, fmt.Errorf("pkg.repo.brew: list taps: %w", err)
	}
	have := false
	for _, t := range current {
		if strings.ToLower(strings.TrimSpace(t)) == tap {
			have = true
			break
		}
	}

	want := state == shared.StatePresent
	op := "noop"
	switch {
	case want && have:
		op = "already-tapped"
	case !want && !have:
		op = "already-untapped"
	case want && !have:
		op = "tap"
	case !want && have:
		op = "untap"
	}

	result.Target = tap
	result.Data = map[string]interface{}{
		"name":   r.Name,
		"tap":    tap,
		"phase":  op, // brew-specific verb (tap / untap / already-tapped / already-untapped)
		"driver": "brew",
	}

	switch op {
	case "tap":
		result.Operation = executor.OpCreate
	case "untap":
		result.Operation = executor.OpDelete
	default:
		result.Operation = executor.OpNoop
	}

	if op == "already-tapped" || op == "already-untapped" {
		result.Reason = fmt.Sprintf("%s is %s", tap, op)
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would %s %s", op, tap)
		return result, nil
	}

	// Capture pre-state for Reverse: the inverse of tap is untap and
	// vice versa. Both are recoverable single-step actions, so we
	// stash a typed envelope the future Reverser can read.
	result.ReverseData = &PkgRepoBrewReverseInfo{
		Name:      r.Name,
		Tap:       tap,
		WasTapped: have,
	}

	args := []string{op, tap}
	if err := Exec(ctx.Ctx(), args...); err != nil {
		return result, fmt.Errorf("pkg.repo.brew: %s %s: %w", op, tap, err)
	}

	result.Changed = true
	result.Reason = fmt.Sprintf("%sed %s", op, tap)
	ctx.GetLogger().Infof("  pkg.repo: %s (%s %s)", r.Name, op, tap)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{Path: "brew:" + tap, Changed: true},
		})
	}
	return result, nil
}

// realListTaps runs `brew tap` and returns the current set of
// tapped repositories, one per line. Wired into the ListTaps
// package var by default; tests substitute their own.
//
// F2: the parent ctx is the run-wide cancellation surface; cmdTimeout
// chains on top so a wedged remote still can't hang the apply.
func realListTaps(parent context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(parent, cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "brew", "tap")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("brew tap: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// realExec runs `brew <args...>`. Wired into the Exec package var
// by default; tests substitute their own. Stderr is folded into the
// error so a misconfigured tap surfaces the brew message verbatim
// instead of the bare exit status.
//
// F2: the parent ctx is the run-wide cancellation surface; cmdTimeout
// chains on top.
func realExec(parent context.Context, args ...string) error {
	ctx, cancel := context.WithTimeout(parent, cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "brew", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
