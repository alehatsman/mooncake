// Package os_cron implements the os.cron action: declarative cron-job
// management via /etc/cron.d. Identity is the action `name`, which is
// also the filename. Each invocation manages one file; idempotency is
// byte-identical content. v1 supports the cron.d form only — per-user
// crontab via `crontab -u` is deferred.
//
//nolint:revive // Package name matches action name convention (os_cron)
package os_cron

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName       = "os.cron"
	statePresent     = "present"
	stateAbsent      = "absent"
	atomicTempSuffix = ".mooncake-tmp"
	managedHeader    = "# Managed by mooncake os.cron"
	defaultUser      = "root"
)

// cronPaths controls where the handler writes cron files. Tests
// override this to redirect /etc/cron.d to a tempdir.
var cronPaths = struct {
	dir string
}{
	dir: "/etc/cron.d",
}

// eff is the spec-69 sudo-aware Performer used by applyPlan. Set by
// Run() from ctx.Effects() before dispatch; the Performer's phase 5b
// try-direct-then-sudo semantic makes Become: true work against both
// /etc/cron.d (production, needs sudo) and a t.TempDir (tests).
var eff actions.Performer

// Handler implements os.cron.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("OsCronReverseInfo", func() any { return &OsCronReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage a cron job via /etc/cron.d/<name>",
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

// nameRE validates the cron entry name — same alphabet allowed by
// `run-parts` for files in /etc/cron.d.
var nameRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// os.cron writes /etc/cron.d/<name>. Always Sudo, no Network, no
// required binaries (the action writes a plain file; cron itself
// picks it up via the standard /etc/cron.d watcher).
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{Sudo: true}
	if step == nil || step.OsCron == nil {
		return ps
	}
	if step.OsCron.Name != "" {
		ps.FilesystemWrite = []string{"/etc/cron.d/" + step.OsCron.Name}
	}
	return ps
}

func (h *Handler) Validate(step *config.Step) error {
	c := step.OsCron
	if c == nil {
		return fmt.Errorf("os.cron requires configuration")
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("os.cron: name is required")
	}
	if !nameRE.MatchString(c.Name) {
		return fmt.Errorf("os.cron: name %q must match [A-Za-z0-9._-]+", c.Name)
	}
	state := normalizeState(c.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("os.cron: state must be present or absent, got %q", c.State)
	}
	hasSchedule := strings.TrimSpace(c.Schedule) != ""
	hasFields := c.Minute != "" || c.Hour != "" || c.Day != "" || c.Month != "" || c.Weekday != ""
	if hasSchedule && hasFields {
		return fmt.Errorf("os.cron: schedule and minute/hour/day/month/weekday are mutually exclusive")
	}
	if state == statePresent {
		if strings.TrimSpace(c.Command) == "" {
			return fmt.Errorf("os.cron: command is required when state=present")
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	c := step.OsCron
	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("os.cron: only Linux is supported; got %s", runtime.GOOS)
	}

	// Wire the spec-69 Performer; applyPlan uses it for the mkdir +
	// file write + remove with sudo fallback on EACCES.
	eff = ctx.Effects()

	rendered, err := renderCron(ctx, c)
	if err != nil {
		return result, err
	}

	plan, err := computePlan(rendered)
	if err != nil {
		return result, err
	}

	result.Data = map[string]interface{}{
		"name":      rendered.name,
		"operation": plan.operation,
		"path":      plan.path,
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

	// Capture pre-apply state for Reverse() BEFORE the mutation.
	// computePlan already read the file once; we reuse that read.
	result.ReverseData = &OsCronReverseInfo{
		Path:         plan.path,
		PriorExisted: plan.priorExisted,
		PriorContent: plan.priorContent,
	}

	if err := applyPlan(plan); err != nil {
		return result, err
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  os.cron: %s (%s)", rendered.name, plan.operation)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{Path: plan.path, Changed: true},
		})
	}
	return result, nil
}

// renderedCron holds the post-template, defaults-applied view.
type renderedCron struct {
	name     string
	state    string
	user     string
	schedule string
	command  string
	env      map[string]string
}

func renderCron(ctx actions.Context, c *config.OsCron) (renderedCron, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()

	render := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return tmpl.Render(s, vars)
	}

	user, err := render(c.User)
	if err != nil {
		return renderedCron{}, fmt.Errorf("os.cron: render user: %w", err)
	}
	if user == "" {
		user = defaultUser
	}

	schedule, err := render(c.Schedule)
	if err != nil {
		return renderedCron{}, fmt.Errorf("os.cron: render schedule: %w", err)
	}
	if schedule == "" {
		schedule, err = renderFields(render, c)
		if err != nil {
			return renderedCron{}, err
		}
	}

	command, err := render(c.Command)
	if err != nil {
		return renderedCron{}, fmt.Errorf("os.cron: render command: %w", err)
	}

	env := map[string]string{}
	for k, v := range c.Env {
		rv, err := render(v)
		if err != nil {
			return renderedCron{}, fmt.Errorf("os.cron: render env %s: %w", k, err)
		}
		env[k] = rv
	}

	return renderedCron{
		name:     c.Name,
		state:    normalizeState(c.State),
		user:     user,
		schedule: schedule,
		command:  command,
		env:      env,
	}, nil
}

// renderFields combines the individual cron-field knobs (minute, hour,
// ...), substituting "*" where unset, into the canonical 5-field schedule.
func renderFields(render func(string) (string, error), c *config.OsCron) (string, error) {
	parts := make([]string, 0, 5)
	for _, raw := range []string{c.Minute, c.Hour, c.Day, c.Month, c.Weekday} {
		out, err := render(raw)
		if err != nil {
			return "", fmt.Errorf("os.cron: render field: %w", err)
		}
		if out == "" {
			out = "*"
		}
		parts = append(parts, out)
	}
	return strings.Join(parts, " "), nil
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

// cronPlan describes the write/delete needed to converge on the
// desired state. priorExisted / priorContent capture the pre-apply
// file state so Reverse can restore it; these are populated by
// computePlan regardless of whether the apply ends up running.
type cronPlan struct {
	changed     bool
	operation   string // create|update|delete|noop
	reason      string
	path        string
	wantContent string // empty for delete

	priorExisted bool
	priorContent string
}

func computePlan(r renderedCron) (cronPlan, error) {
	path := filepath.Join(cronPaths.dir, r.name)
	plan := cronPlan{path: path}

	if r.state == stateAbsent {
		current, exists, err := readFile(path)
		if err != nil {
			return plan, err
		}
		plan.priorExisted = exists
		if exists {
			plan.priorContent = current
		}
		if !exists {
			plan.operation = "noop"
			plan.reason = "cron file already absent"
			return plan, nil
		}
		plan.changed = true
		plan.operation = "delete"
		plan.reason = "would remove " + path
		return plan, nil
	}

	want := renderCronFile(r)
	plan.wantContent = want

	current, exists, err := readFile(path)
	if err != nil {
		return plan, err
	}
	plan.priorExisted = exists
	if exists {
		plan.priorContent = current
	}
	switch {
	case !exists:
		plan.changed = true
		plan.operation = "create"
		plan.reason = "would create " + path
	case current != want:
		plan.changed = true
		plan.operation = "update"
		plan.reason = "would update " + path + " (content drift)"
	default:
		plan.operation = "noop"
		plan.reason = "cron file already at desired state"
	}
	return plan, nil
}

// renderCronFile emits the byte-stable content for the cron.d file.
// Env keys are sorted so two runs produce identical bytes.
func renderCronFile(r renderedCron) string {
	var sb strings.Builder
	sb.WriteString(managedHeader)
	sb.WriteByte('\n')
	if len(r.env) > 0 {
		keys := make([]string, 0, len(r.env))
		for k := range r.env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(k)
			sb.WriteByte('=')
			sb.WriteString(r.env[k])
			sb.WriteByte('\n')
		}
	}
	sb.WriteString(r.schedule)
	sb.WriteByte(' ')
	sb.WriteString(r.user)
	sb.WriteByte(' ')
	sb.WriteString(r.command)
	sb.WriteByte('\n')
	return sb.String()
}

func applyPlan(plan cronPlan) error {
	pOpts := actions.PerformerOpts{Become: true}
	if plan.operation == "delete" {
		if e := eff.Remove(plan.path, false, pOpts); e.Err != nil && !errors.Is(e.Err, fs.ErrNotExist) {
			return fmt.Errorf("os.cron: remove %s: %w", plan.path, e.Err)
		}
		return nil
	}
	if e := eff.Mkdir(cronPaths.dir, 0o755, pOpts); e.Err != nil {
		return fmt.Errorf("os.cron: mkdir %s: %w", cronPaths.dir, e.Err)
	}
	if e := eff.WriteFile(plan.path, []byte(plan.wantContent), 0o644, actions.PerformerOpts{Become: true, ExplicitMode: true}); e.Err != nil {
		return fmt.Errorf("os.cron: write %s: %w", plan.path, e.Err)
	}
	return nil
}

func readFile(path string) (string, bool, error) {
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
