// Package process implements the `process` action: supervised long-running
// processes scoped to either the current plan run (scope: plan) or the user
// session (scope: session, default). It fills the gap between shell: (fire-
// and-forget) and os.service: (OS-installed daemon).
//
// State vocabulary:
//
//	running   — start if not running; wait for health gate.
//	stopped   — kill if running, remove pid file.
//	restarted — kill (if running), then start; wait for health gate.
//	absent    — stop + delete log files + delete pid file.
//
// Scope:
//
//	session (default) — process survives apply; managed declaratively
//	                    across runs. A subsequent plan with state: absent reaps it.
//	plan              — process is killed when the plan context is cancelled
//	                    (apply finishes, ctrl-c, etc.).
//
// PID files and default log paths live in ~/.mooncake/processes/.
package process

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	StateRunning   = "running"
	StateStopped   = "stopped"
	StateRestarted = "restarted"
	StateAbsent    = "absent"

	ScopeSession = "session"
	ScopePlan    = "plan"

	defaultHealthTimeout = 30 * time.Second
	dialTimeout          = 2 * time.Second
	healthPollInterval   = 500 * time.Millisecond
)

// Handler implements the process action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "process",
		Description:        "Supervise a long-running process scoped to the plan or session",
		Category:           actions.CategoryCommand,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		ImplementsCheck:    true,
		Examples: []string{
			`# Start ollama for the duration of this plan
- name: Start ollama
  process:
    name: ollama
    command: [ollama, serve]
    scope: plan
    health:
      port_open: 11434
      timeout: 30s`,
			`# Ensure a session-scoped helper is running
- name: Ensure helper running
  process:
    name: my-helper
    command: [/usr/local/bin/my-helper, --port, "9000"]
    state: running
    health:
      port_open: 9000`,
		},
	}
}

// Validate checks the step configuration.
func (h *Handler) Validate(step *config.Step) error {
	p := step.Process
	if p == nil {
		return fmt.Errorf("process configuration is nil")
	}
	if p.Name == "" {
		return fmt.Errorf("process.name is required")
	}
	if len(p.Command) == 0 {
		return fmt.Errorf("process.command is required and must have at least one element")
	}
	switch p.State {
	case "", StateRunning, StateStopped, StateRestarted, StateAbsent:
	default:
		return fmt.Errorf("process.state must be one of: running, stopped, restarted, absent (got %q)", p.State)
	}
	switch p.Scope {
	case "", ScopeSession, ScopePlan:
	default:
		return fmt.Errorf("process.scope must be session or plan (got %q)", p.Scope)
	}
	if p.Health != nil && p.Health.Timeout != "" {
		if _, err := time.ParseDuration(p.Health.Timeout); err != nil {
			return fmt.Errorf("process.health.timeout: %w", err)
		}
	}
	return nil
}

// Run executes the process action.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	p := step.Process
	state := p.State
	if state == "" {
		state = StateRunning
	}

	if ctx.Mode() == actions.ModePlan {
		return h.planMode(ctx, p, state)
	}

	stateDir, err := processStateDir()
	if err != nil {
		return nil, err
	}

	switch state {
	case StateRunning:
		return h.ensureRunning(ctx, p, stateDir)
	case StateStopped:
		return h.ensureStopped(ctx, p, stateDir)
	case StateRestarted:
		return h.ensureRestarted(ctx, p, stateDir)
	case StateAbsent:
		return h.ensureAbsent(ctx, p, stateDir)
	default:
		return nil, fmt.Errorf("unknown state %q", state)
	}
}

func (h *Handler) planMode(_ actions.Context, p *config.ProcessAction, state string) (actions.Result, error) {
	r := executor.NewResult()
	r.Checkable = true
	r.WouldChange = true
	switch state {
	case StateRunning:
		r.Reason = fmt.Sprintf("would ensure process %q is running", p.Name)
	case StateStopped:
		r.Reason = fmt.Sprintf("would stop process %q", p.Name)
	case StateRestarted:
		r.Reason = fmt.Sprintf("would restart process %q", p.Name)
	case StateAbsent:
		r.Reason = fmt.Sprintf("would remove process %q", p.Name)
	}
	return r, nil
}

// ensureRunning starts the process if it is not already running.
func (h *Handler) ensureRunning(ctx actions.Context, p *config.ProcessAction, stateDir string) (actions.Result, error) {
	pidFile := pidFilePath(stateDir, p.Name)
	if alive, _ := isAlive(pidFile); alive {
		r := executor.NewResult()
		r.Operation = executor.OpQuery
		r.Target = p.Name
		r.Changed = false
		r.Data = map[string]any{"name": p.Name, "state": "running", "pid_file": pidFile}
		ctx.Logger().Infof("process %q already running (pid file: %s)", p.Name, pidFile)
		return r, nil
	}
	return h.startProcess(ctx, p, stateDir)
}

// ensureStopped kills the process if running.
func (h *Handler) ensureStopped(_ actions.Context, p *config.ProcessAction, stateDir string) (actions.Result, error) {
	pidFile := pidFilePath(stateDir, p.Name)
	killed, err := killByPidFile(pidFile)
	if err != nil {
		return nil, fmt.Errorf("process.stop %q: %w", p.Name, err)
	}
	r := executor.NewResult()
	r.Operation = executor.OpDelete
	r.Target = p.Name
	r.Changed = killed
	r.Data = map[string]any{"name": p.Name, "state": "stopped"}
	return r, nil
}

// ensureRestarted kills then restarts.
func (h *Handler) ensureRestarted(ctx actions.Context, p *config.ProcessAction, stateDir string) (actions.Result, error) {
	pidFile := pidFilePath(stateDir, p.Name)
	if _, err := killByPidFile(pidFile); err != nil {
		return nil, fmt.Errorf("process.restart %q (kill phase): %w", p.Name, err)
	}
	return h.startProcess(ctx, p, stateDir)
}

// ensureAbsent stops and removes all process artifacts.
func (h *Handler) ensureAbsent(_ actions.Context, p *config.ProcessAction, stateDir string) (actions.Result, error) {
	pidFile := pidFilePath(stateDir, p.Name)
	killed, err := killByPidFile(pidFile)
	if err != nil {
		return nil, fmt.Errorf("process.absent %q: %w", p.Name, err)
	}

	_ = os.Remove(pidFile)

	// Remove log files if they exist under the state dir (default paths only).
	stdoutLog := filepath.Join(stateDir, p.Name+".out")
	stderrLog := filepath.Join(stateDir, p.Name+".err")
	_ = os.Remove(stdoutLog)
	_ = os.Remove(stderrLog)

	r := executor.NewResult()
	r.Operation = executor.OpDelete
	r.Target = p.Name
	r.Changed = killed
	r.Data = map[string]any{"name": p.Name, "state": "absent"}
	return r, nil
}

// startProcess launches the process, writes the pid file, and waits for the
// health gate. For scope=plan it attaches a goroutine that kills the process
// when ctx is cancelled.
func (h *Handler) startProcess(ctx actions.Context, p *config.ProcessAction, stateDir string) (actions.Result, error) {
	// Resolve command (template-render each argument).
	args := make([]string, len(p.Command))
	for i, arg := range p.Command {
		rendered, err := ctx.Template().Render(arg, ctx.Variables())
		if err != nil {
			return nil, fmt.Errorf("process.command[%d]: %w", i, err)
		}
		args[i] = rendered
	}

	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec

	// Working directory.
	if p.Cwd != "" {
		rendered, err := ctx.Template().Render(p.Cwd, ctx.Variables())
		if err != nil {
			return nil, fmt.Errorf("process.cwd: %w", err)
		}
		cmd.Dir = rendered
	}

	// Environment.
	cmd.Env = os.Environ()
	for k, v := range p.Env {
		rendered, err := ctx.Template().Render(v, ctx.Variables())
		if err != nil {
			return nil, fmt.Errorf("process.env[%s]: %w", k, err)
		}
		cmd.Env = append(cmd.Env, k+"="+rendered)
	}

	// Log files.
	stdoutPath, stderrPath := resolveLogPaths(p, stateDir)
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		return nil, fmt.Errorf("process: create state dir: %w", err)
	}
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("process: open stdout log %s: %w", stdoutPath, err)
	}
	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		_ = stdoutFile.Close()
		return nil, fmt.Errorf("process: open stderr log %s: %w", stderrPath, err)
	}
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	setSysProcAttr(cmd) // platform-specific: new process group so kill(-pgid) works

	if err := cmd.Start(); err != nil {
		_ = stdoutFile.Close()
		_ = stderrFile.Close()
		return nil, fmt.Errorf("process %q: start: %w", p.Name, err)
	}

	pid := cmd.Process.Pid

	// Close log file handles — the child inherited them; we don't need them open.
	_ = stdoutFile.Close()
	_ = stderrFile.Close()

	// Write pid file.
	pidFile := pidFilePath(stateDir, p.Name)
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)+"\n"), 0o640); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("process %q: write pid file: %w", p.Name, err)
	}

	ctx.Logger().Infof("process %q started (pid %d)", p.Name, pid)

	// Health gate.
	if p.Health != nil {
		if err := waitHealth(ctx.Ctx(), p.Health); err != nil {
			// Kill the process if health check fails; don't leave an orphan.
			_ = cmd.Process.Kill()
			_ = os.Remove(pidFile)
			return nil, fmt.Errorf("process %q: health gate: %w", p.Name, err)
		}
		ctx.Logger().Infof("process %q health gate passed", p.Name)
	}

	// For scope=plan, kill on context cancellation (plan exit / ctrl-c).
	scope := p.Scope
	if scope == "" {
		scope = ScopeSession
	}
	if scope == ScopePlan {
		go func() {
			<-ctx.Ctx().Done()
			_ = cmd.Process.Kill()
			_ = os.Remove(pidFile)
		}()
	}

	r := executor.NewResult()
	r.Operation = executor.OpUpdate
	r.Target = p.Name
	r.Changed = true
	r.Data = map[string]any{
		"name":     p.Name,
		"pid":      pid,
		"state":    "running",
		"scope":    scope,
		"stdout":   stdoutPath,
		"stderr":   stderrPath,
		"pid_file": pidFile,
	}
	return r, nil
}

// waitHealth polls health predicates until they pass or timeout expires.
func waitHealth(ctx context.Context, h *config.ProcessHealth) error {
	timeout := defaultHealthTimeout
	if h.Timeout != "" {
		d, err := time.ParseDuration(h.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = d
	}

	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	check := func() bool {
		if h.PortOpen > 0 {
			addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(h.PortOpen))
			conn, err := net.DialTimeout("tcp", addr, dialTimeout)
			if err != nil {
				return false
			}
			_ = conn.Close()
		}
		if h.HTTPOk != "" {
			resp, err := httpGet(pollCtx, h.HTTPOk)
			if err != nil || resp < 200 || resp >= 300 {
				return false
			}
		}
		if h.FileExists != "" {
			if _, err := os.Stat(h.FileExists); err != nil {
				return false
			}
		}
		return true
	}

	ticker := time.NewTicker(healthPollInterval)
	defer ticker.Stop()

	if check() {
		return nil
	}
	for {
		select {
		case <-pollCtx.Done():
			return fmt.Errorf("health gate timed out after %s", timeout)
		case <-ticker.C:
			if check() {
				return nil
			}
		}
	}
}

func httpGet(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// processStateDir returns ~/.mooncake/processes, creating it if needed.
func processStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("process: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".mooncake", "processes"), nil
}

func pidFilePath(stateDir, name string) string {
	return filepath.Join(stateDir, name+".pid")
}

func resolveLogPaths(p *config.ProcessAction, stateDir string) (stdout, stderr string) {
	stdout = filepath.Join(stateDir, p.Name+".out")
	stderr = filepath.Join(stateDir, p.Name+".err")
	if p.Log != nil {
		if p.Log.Stdout != "" {
			stdout = p.Log.Stdout
		}
		if p.Log.Stderr != "" {
			stderr = p.Log.Stderr
		}
	}
	return
}

// isAlive reads the pid file and checks whether the process is still running.
func isAlive(pidFile string) (bool, int) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid
	}
	return isProcessAlive(proc), pid
}

// killByPidFile reads the pid file and sends SIGTERM (Unix) / Kill (Windows).
// Returns true if a process was actually killed.
func killByPidFile(pidFile string) (bool, error) {
	alive, pid := isAlive(pidFile)
	if !alive {
		_ = os.Remove(pidFile)
		return false, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	if err := killProcess(proc); err != nil {
		return false, err
	}
	_ = os.Remove(pidFile)
	return true, nil
}
