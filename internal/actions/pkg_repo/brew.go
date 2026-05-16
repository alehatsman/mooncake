//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// brewCmdTimeout caps every brew subprocess so a wedged remote can't
// hang the whole apply. brew tap fetches a git remote, brew untap is
// local; 60s covers slow networks without inviting indefinite waits.
const brewCmdTimeout = 60 * time.Second

// runBrew drives state=present / state=absent for a Homebrew tap.
// The key idempotency property — and the proposal-08 motivation —
// is that we LIST first and only mutate when the desired state
// disagrees with the world. Brew exits non-zero when re-tapping an
// already-tapped repo; pre-checking via the list avoids that
// surface entirely, so playbooks don't need
// `failed_when: rc not in [0, 1]` shell wrappers.
func runBrew(ctx actions.Context, r *config.PkgRepo, result *executor.Result) (actions.Result, error) {
	state := normalizeState(r.State)
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

	current, err := brewListTaps()
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

	want := state == statePresent
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

	result.Data = map[string]interface{}{
		"name":      r.Name,
		"tap":       tap,
		"operation": op,
		"driver":    "brew",
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
	// stash a typed envelope the future Reverser can read. Keeping
	// the shape parallel to the apt driver's PkgRepoReverseInfo so
	// downstream consumers don't need driver-specific dispatch.
	result.ReverseData = &PkgRepoBrewReverseInfo{
		Name:      r.Name,
		Tap:       tap,
		WasTapped: have,
	}

	args := []string{op, tap}
	if err := brewExec(args...); err != nil {
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

// realBrewListTaps runs `brew tap` and returns the current set of
// tapped repositories, one per line. Wired into the brewListTaps
// package var by default; tests substitute their own.
func realBrewListTaps() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), brewCmdTimeout)
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

// realBrewExec runs `brew <args...>`. Wired into the brewExec
// package var by default; tests substitute their own. Stderr is
// folded into the error so a misconfigured tap surfaces the
// brew message verbatim instead of the bare exit status.
func realBrewExec(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), brewCmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "brew", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
