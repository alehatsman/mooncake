// Package observe_process implements the observe.process action:
// single-shot read of process state by name or pattern (spec-59
// Phase 2). Uses /proc on Linux and shells out to ps elsewhere — both
// paths return the same typed ProcessObservation payload.
package observe_process

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const actionName = "observe.process"

// errNoMatch signals "no process matched the selector" — i.e. the
// resource genuinely doesn't exist, not a lookup failure. MT-61: the
// shared ObserveResult.Error contract reserves Error for actual probe
// failures (DNS, permission, transport); a missing process should
// leave Error empty and surface only via Found=false.
var errNoMatch = errors.New("no matching process")

// ProcessObservation is the typed Value payload for observe.process.
// Pid is set to the first matching pid; Pids carries every match.
type ProcessObservation struct {
	Running   bool     `json:"running"`
	Pid       int      `json:"pid,omitempty"`
	Pids      []int    `json:"pids,omitempty"`
	Args      []string `json:"args,omitempty"`
	User      string   `json:"user,omitempty"`
	StartedAt string   `json:"started_at,omitempty"`
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot read of process state (running? pid? args?)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	o := step.ObserveProcess
	if o == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	if o.Name == "" && o.Pattern == "" {
		return fmt.Errorf("%s: name or pattern is required", actionName)
	}
	if o.Pattern != "" {
		if _, err := regexp.Compile(o.Pattern); err != nil {
			return fmt.Errorf("%s: invalid pattern regex: %w", actionName, err)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObserveProcess

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	target := selectorString(o)

	if ctx.Mode() == actions.ModePlan {
		env := actions.PlanDeferred(ProcessObservation{})
		result.PublishObservation(env, target)
		result.Checkable = true
		result.Reason = fmt.Sprintf("would observe process %s (deferred to apply)", target)
		return result, nil
	}

	obs, err := findProcess(o)
	env := actions.ObserveResult{
		Found: obs.Running,
		Value: obs,
		AsOf:  time.Now(),
	}
	// MT-61: only surface Error for real probe failures (ps fork
	// failure, /proc unreadable, etc.). errNoMatch means "no process
	// matched the selector" — that's the normal Found=false answer,
	// not a failure to observe.
	if err != nil && !obs.Running && !errors.Is(err, errNoMatch) {
		env.Error = err.Error()
	}
	result.PublishObservation(env, target)
	return result, nil
}

func findProcess(o *config.ObserveProcess) (ProcessObservation, error) {
	// Linux fast path: walk /proc. Falls back to ps on macOS/BSD/Solaris.
	if runtime.GOOS == "linux" {
		obs, err := findProcessLinux(o)
		if obs.Running || err == nil {
			return obs, err
		}
		// Linux without /proc (e.g. some containers) — try ps too.
	}
	return findProcessPs(o)
}

func findProcessLinux(o *config.ObserveProcess) (ProcessObservation, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ProcessObservation{}, err
	}
	var re *regexp.Regexp
	if o.Pattern != "" {
		re = regexp.MustCompile(o.Pattern) // Validate guarantees this compiles
	}
	var obs ProcessObservation
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil || len(cmdline) == 0 {
			continue
		}
		// cmdline is NUL-delimited argv.
		args := splitCmdline(cmdline)
		full := strings.Join(args, " ")
		basename := filepath.Base(args[0])
		match := false
		if o.Name != "" && basename == o.Name {
			match = true
		}
		if !match && re != nil && re.MatchString(full) {
			match = true
		}
		if !match {
			continue
		}
		obs.Pids = append(obs.Pids, pid)
		if !obs.Running {
			obs.Running = true
			obs.Pid = pid
			obs.Args = args
		}
	}
	if !obs.Running {
		return obs, errNoMatch
	}
	return obs, nil
}

func findProcessPs(o *config.ObserveProcess) (ProcessObservation, error) {
	// `ps -eo pid,user,args` works on macOS, Linux, BSDs. Solaris's
	// /usr/ucb/ps is out of scope for v1.
	out, err := exec.Command("ps", "-eo", "pid,user,args").Output()
	if err != nil {
		return ProcessObservation{}, fmt.Errorf("ps: %w", err)
	}
	var re *regexp.Regexp
	if o.Pattern != "" {
		re = regexp.MustCompile(o.Pattern)
	}
	var obs ProcessObservation
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PID ") {
			continue
		}
		// Split into PID, USER, then the rest as argv (preserving spaces).
		cols := strings.Fields(line)
		if len(cols) < 3 {
			continue
		}
		pid, err := strconv.Atoi(cols[0])
		if err != nil {
			continue
		}
		user := cols[1]
		// Reconstruct full argv string by finding USER's position and
		// taking everything after it. Two-pass split would be cleaner
		// but Go's strings package doesn't expose a typed SplitN that
		// preserves the remainder; this is the same trick `ps` parsers
		// use throughout the stdlib.
		full := strings.TrimSpace(line[strings.Index(line, user)+len(user):])
		argv := strings.Fields(full)
		basename := ""
		if len(argv) > 0 {
			basename = filepath.Base(argv[0])
		}
		match := false
		if o.Name != "" && basename == o.Name {
			match = true
		}
		if !match && re != nil && re.MatchString(full) {
			match = true
		}
		if !match {
			continue
		}
		obs.Pids = append(obs.Pids, pid)
		if !obs.Running {
			obs.Running = true
			obs.Pid = pid
			obs.Args = argv
			obs.User = user
		}
	}
	if !obs.Running {
		return obs, errNoMatch
	}
	return obs, nil
}

func splitCmdline(b []byte) []string {
	// /proc/PID/cmdline is NUL-separated; trim trailing NUL.
	b = bytes.TrimRight(b, "\x00")
	parts := bytes.Split(b, []byte{0})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) > 0 {
			out = append(out, string(p))
		}
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

func selectorString(o *config.ObserveProcess) string {
	if o.Name != "" {
		return "name=" + o.Name
	}
	return "pattern=" + o.Pattern
}

// --- Spec-22 ABI no-mutation specialization ---------------------------------

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: true, Risk: 1}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	bins := []string{}
	if runtime.GOOS != "linux" {
		bins = append(bins, "ps")
	}
	return actions.PermissionSet{
		RequiredBinaries: bins,
		Notes:            []string{"read-only observation; no mutation"},
	}
}

func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	o := step.ObserveProcess
	if o == nil {
		return actions.Diff{}, nil
	}
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: "process:" + selectorString(o),
			Attributes: map[string]string{"observe_kind": "process"},
		},
		Operation: actions.OpNoop,
	}, nil
}

func (h *Handler) Reverse(_ actions.Context, _ *config.Step, _ actions.Result) (*config.Step, error) {
	return nil, nil
}
