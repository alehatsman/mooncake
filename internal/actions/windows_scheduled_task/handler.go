// Package windows_scheduled_task implements the windows.scheduled_task
// action. Idempotent management of Task Scheduler entries used for
// agentd autostart, WSL keepalive, and similar long-lived helpers.
// spec-57.
//
// Drift detection uses winutil.NormaliseTaskXML on both the rendered
// desired XML and the remote's Export-ScheduledTask output, so element
// reordering / whitespace doesn't trigger spurious updates.
//
//nolint:revive // package name follows mooncake action convention
package windows_scheduled_task

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/winutil"
)

const (
	actionName   = "windows.scheduled_task"
	statePresent = "present"
	stateAbsent  = "absent"
)

// runPS is a package-level hook so tests can swap powershell.exe for
// an in-memory script runner. realPSRun is the production impl.
var runPS = realPSRun

// Handler implements windows.scheduled_task.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage Windows Task Scheduler entries",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"windows"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	t := step.WindowsScheduledTask
	if t == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	state := normalizeState(t.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("%s: state must be present or absent, got %q", actionName, t.State)
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%s: name is required", actionName)
	}
	if state == statePresent {
		// Trigger XOR Triggers must be set.
		if t.Trigger == nil && len(t.Triggers) == 0 {
			return fmt.Errorf("%s: trigger or triggers is required when state=present", actionName)
		}
		if t.Trigger != nil && len(t.Triggers) > 0 {
			return fmt.Errorf("%s: trigger and triggers are mutually exclusive", actionName)
		}
		if len(t.Actions) == 0 {
			return fmt.Errorf("%s: at least one action is required", actionName)
		}
		// Round-trip through Task.Validate for deeper checks.
		task, err := toTask(t)
		if err != nil {
			return fmt.Errorf("%s: %w", actionName, err)
		}
		if err := task.Validate(); err != nil {
			return fmt.Errorf("%s: %w", actionName, err)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	t := step.WindowsScheduledTask
	result := executor.NewResult()
	result.Checkable = true
	result.Target = t.Name

	if runtime.GOOS != "windows" {
		return result, fmt.Errorf("%s: only Windows is supported; got %s", actionName, runtime.GOOS)
	}

	state := normalizeState(t.State)

	currentXML, exists, err := queryTaskXML(t.Name)
	if err != nil {
		return result, fmt.Errorf("%s: query current: %w", actionName, err)
	}

	switch state {
	case stateAbsent:
		if !exists {
			result.Operation = executor.OpNoop
			result.Reason = "task already absent"
			return result, nil
		}
		result.Operation = executor.OpDelete
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = true
			result.Reason = "would remove task " + t.Name
			return result, nil
		}
		if _, err := runPS(winutil.RenderUnregisterCommand(t.Name)); err != nil {
			return result, fmt.Errorf("unregister: %w", err)
		}
		result.Changed = true
		result.Reason = "removed task " + t.Name
		return result, nil

	case statePresent:
		desiredTask, err := toTask(t)
		if err != nil {
			return result, err
		}
		desiredXML, err := winutil.RenderXML(desiredTask)
		if err != nil {
			return result, fmt.Errorf("render xml: %w", err)
		}
		if exists && winutil.NormaliseTaskXML(currentXML) == winutil.NormaliseTaskXML(desiredXML) {
			result.Operation = executor.OpNoop
			result.Reason = "task already at desired state"
			return result, nil
		}
		if !exists {
			result.Operation = executor.OpCreate
		} else {
			result.Operation = executor.OpUpdate
		}
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = true
			if !exists {
				result.Reason = "would create task " + t.Name
			} else {
				result.Reason = "would update task " + t.Name
			}
			return result, nil
		}
		// Stage XML to a temp file, register from there. -Force
		// replaces any existing task with the same name.
		tmp, err := stageXML(desiredXML)
		if err != nil {
			return result, fmt.Errorf("stage xml: %w", err)
		}
		defer func() { _ = os.Remove(tmp) }()

		if _, err := runPS(winutil.RenderRegisterCommand(t.Name, tmp)); err != nil {
			return result, fmt.Errorf("register: %w", err)
		}
		result.Changed = true
		if !exists {
			result.Reason = "created task " + t.Name
		} else {
			result.Reason = "updated task " + t.Name
		}
		return result, nil
	}
	return result, fmt.Errorf("%s: unreachable state %q", actionName, state)
}

// ----- helpers -------------------------------------------------------------

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

// queryTaskXML returns the existing task's XML (if any) using
// Export-ScheduledTask, plus an exists flag.
func queryTaskXML(name string) (xml string, exists bool, err error) {
	// Test for existence first — Export-ScheduledTask on a missing
	// task errors out, which we'd otherwise interpret as a real
	// failure. The two-step is cheaper than parsing the error string.
	check := `if (Get-ScheduledTask -TaskName ` + psQuote(name) + ` -ErrorAction SilentlyContinue) { 'yes' } else { 'no' }`
	out, err := runPS(check)
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(out) != "yes" {
		return "", false, nil
	}
	xmlOut, err := runPS(winutil.RenderExportCommand(name))
	if err != nil {
		return "", true, fmt.Errorf("export task: %w", err)
	}
	return xmlOut, true, nil
}

// stageXML writes body to a fresh tempfile and returns its path. The
// caller is responsible for removing the file after Register-
// ScheduledTask consumes it.
func stageXML(body string) (string, error) {
	dir := os.TempDir()
	if dir == "" {
		dir = "."
	}
	f, err := os.CreateTemp(dir, "mooncake-task-*.xml")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(body); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	// Return forward-slashed path through filepath.Clean so the
	// PowerShell renderer's psQuote isn't fighting Windows backslash
	// escapes.
	return filepath.Clean(f.Name()), nil
}

// toTask maps a config.WindowsScheduledTask (YAML-flavoured) to a
// winutil.Task (Go-flavoured). All defaults flow through
// winutil.Task.withDefaults() at render time so this conversion only
// covers identity-preserving translation.
func toTask(t *config.WindowsScheduledTask) (winutil.Task, error) {
	tk := winutil.Task{
		Name:        t.Name,
		Description: t.Description,
	}

	// Triggers: single OR list shape.
	var triggers []config.WindowsScheduledTaskTrigger
	if t.Trigger != nil {
		triggers = []config.WindowsScheduledTaskTrigger{*t.Trigger}
	} else {
		triggers = t.Triggers
	}
	for i, tr := range triggers {
		conv, err := toTrigger(tr)
		if err != nil {
			return winutil.Task{}, fmt.Errorf("trigger[%d]: %w", i, err)
		}
		tk.Triggers = append(tk.Triggers, conv)
	}

	for _, a := range t.Actions {
		tk.Actions = append(tk.Actions, winutil.ExecAction{
			Command:          a.Execute,
			Arguments:        a.Arguments,
			WorkingDirectory: a.WorkingDirectory,
		})
	}

	// When t.Principal is nil, the UserID is filled in by the caller via
	// env vars resolved at apply-time; we can't know it here. Caller must
	// provide it or accept the default empty (which Task.Validate will
	// reject — surface that early).
	if t.Principal != nil {
		tk.Principal = winutil.Principal{
			UserID:    t.Principal.User,
			LogonType: winutil.LogonType(titleCase(t.Principal.LogonType)),
			RunLevel:  winutil.RunLevel(runLevelMap(t.Principal.RunLevel)),
		}
	}

	if t.Settings != nil {
		s := t.Settings
		tk.Settings = winutil.Settings{
			StartWhenAvailable:         s.StartWhenAvailable,
			AllowStartIfOnBatteries:    s.AllowStartIfOnBatteries,
			DontStopIfGoingOnBatteries: s.DontStopIfGoingOnBatteries,
			RunOnlyIfNetworkAvailable:  s.RunOnlyIfNetworkAvailable,
			MultipleInstancesPolicy:    winutil.MultipleInstances(titleCase(s.MultipleInstances)),
			RestartCount:               s.RestartCount,
			Hidden:                     s.Hidden,
		}
		if s.ExecutionTimeLimit != "" {
			d, err := parseDurationLoose(s.ExecutionTimeLimit)
			if err != nil {
				return winutil.Task{}, fmt.Errorf("execution_time_limit: %w", err)
			}
			tk.Settings.ExecutionTimeLimit = d
		}
		if s.RestartInterval != "" {
			d, err := parseDurationLoose(s.RestartInterval)
			if err != nil {
				return winutil.Task{}, fmt.Errorf("restart_interval: %w", err)
			}
			tk.Settings.RestartInterval = d
		}
	}
	return tk, nil
}

func toTrigger(tr config.WindowsScheduledTaskTrigger) (winutil.Trigger, error) {
	out := winutil.Trigger{
		Type:        winutil.TriggerType(strings.ToLower(tr.Type)),
		LogonUserID: tr.UserID,
	}
	if tr.Interval != "" {
		d, err := parseDurationLoose(tr.Interval)
		if err != nil {
			return winutil.Trigger{}, fmt.Errorf("interval: %w", err)
		}
		out.RepetitionInterval = d
	}
	if tr.Duration != "" {
		d, err := parseDurationLoose(tr.Duration)
		if err != nil {
			return winutil.Trigger{}, fmt.Errorf("duration: %w", err)
		}
		out.RepetitionDuration = d
	}
	if tr.StartBoundary != "" {
		t, err := time.Parse(time.RFC3339, tr.StartBoundary)
		if err != nil {
			// Try the simpler Task Scheduler form too.
			t, err = time.Parse("2006-01-02T15:04:05", tr.StartBoundary)
			if err != nil {
				return winutil.Trigger{}, fmt.Errorf("start_boundary: %w", err)
			}
		}
		out.StartBoundary = t
	}
	return out, nil
}

// parseDurationLoose accepts Go-style ("5m") or ISO-8601 ("PT5M")
// durations. ISO-8601 is a small subset; we parse just enough to
// cover what the spec promises (DTHMS).
func parseDurationLoose(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if strings.HasPrefix(s, "P") {
		return parseISO8601(s)
	}
	return 0, fmt.Errorf("not a Go duration or ISO-8601: %q", s)
}

func parseISO8601(s string) (time.Duration, error) {
	if !strings.HasPrefix(s, "P") {
		return 0, fmt.Errorf("ISO-8601 duration must start with P")
	}
	s = s[1:]
	var total time.Duration
	timePart := false
	var num strings.Builder
	for _, r := range s {
		switch {
		case r == 'T':
			timePart = true
		case r >= '0' && r <= '9':
			num.WriteRune(r)
		case r == 'D':
			n, err := atoi(num.String())
			if err != nil {
				return 0, err
			}
			total += time.Duration(n) * 24 * time.Hour
			num.Reset()
		case r == 'H' && timePart:
			n, err := atoi(num.String())
			if err != nil {
				return 0, err
			}
			total += time.Duration(n) * time.Hour
			num.Reset()
		case r == 'M' && timePart:
			n, err := atoi(num.String())
			if err != nil {
				return 0, err
			}
			total += time.Duration(n) * time.Minute
			num.Reset()
		case r == 'S' && timePart:
			n, err := atoi(num.String())
			if err != nil {
				return 0, err
			}
			total += time.Duration(n) * time.Second
			num.Reset()
		default:
			return 0, fmt.Errorf("unrecognised char %q", r)
		}
	}
	return total, nil
}

func atoi(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit %q", c)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// titleCase capitalises the first letter — the winutil enums use
// PascalCase strings (S4U, Interactive, ...).
func titleCase(s string) string {
	if s == "" {
		return s
	}
	// S4U / IgnoreNew / Parallel / Queue / StopExisting / HighestAvailable
	// — all PascalCase. Just upper-the-first and pass through, since
	// the YAML values were lowercase by convention.
	switch strings.ToLower(s) {
	case "s4u":
		return "S4U"
	case "interactive":
		return "Interactive"
	case "password":
		return "Password"
	case "service_account":
		return "ServiceAccount"
	case "ignore_new":
		return "IgnoreNew"
	case "parallel":
		return "Parallel"
	case "queue":
		return "Queue"
	case "stop_existing":
		return "StopExisting"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func runLevelMap(s string) string {
	switch strings.ToLower(s) {
	case "highest", "highestavailable", "":
		return "HighestAvailable"
	case "limited", "leastprivilege":
		return "LeastPrivilege"
	}
	return s
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func realPSRun(script string) (string, error) {
	// Issue #13: prepend the UTF-8 output prelude so Export-ScheduledTask
	// and any other cmdlet that emits non-ASCII data doesn't get
	// re-encoded to the OEM codepage on the way out (which would corrupt
	// to 0x1A and break the controller-side decode).
	script = winutil.WithUTF8Output(script)
	utf16le := utf16.Encode([]rune(script))
	buf := bytes.Buffer{}
	for _, r := range utf16le {
		buf.WriteByte(byte(r))
		buf.WriteByte(byte(r >> 8))
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	cmd := exec.Command("powershell.exe", "-NoProfile", "-EncodedCommand", encoded)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("powershell exited: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
