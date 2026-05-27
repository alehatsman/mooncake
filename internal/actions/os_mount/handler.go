// Package os_mount implements the os.mount action: declarative
// management of /etc/fstab entries and the corresponding live mount
// state on Linux. Identity is the destination mount point; one fstab
// entry per dest. v1 supports the canonical /etc/fstab + /proc/mounts
// model only (no autofs, no fuse helpers beyond what mount(8) accepts).
//
//nolint:revive // Package name matches action name convention (os_mount)
package os_mount

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

const (
	actionName       = "os.mount"
	stateMounted     = "mounted"
	stateUnmounted   = "unmounted"
	stateFstabOnly   = "fstab_only"
	stateAbsent      = "absent"
	atomicTempSuffix = ".mooncake-tmp"

	// mountCmdTimeout bounds the wallclock budget for the
	// `mount` / `umount` shell-outs. NFS hangs under default
	// `hard,intr` options can otherwise block apply indefinitely
	// because actions.Context doesn't expose a cancellable parent
	// (F051). 30s is generous for slow-but-healthy NFS; failure-
	// mode hangs (server unreachable, locked mount) are caught
	// quickly via context.DeadlineExceeded.
	mountCmdTimeout = 30 * time.Second
)

// mountPaths controls the on-disk locations the handler reads/writes.
// Tests redirect both fstab and the /proc/mounts source.
var mountPaths = struct {
	fstab  string
	mounts string
}{
	fstab:  "/etc/fstab",
	mounts: "/proc/mounts",
}

// Package-level hooks for the runtime side effects (`mount`/`umount`).
// Tests stub these to keep apply-mode hermetic and assert calls.
// Spec-69 phase-5: the hooks take the PrivilegedRunner explicitly so
// the package no longer carries per-Run mutable state.
var (
	mountRun  = realMount  // func(runner, dest) error
	umountRun = realUmount // func(runner, dest) error
	clockNow  = time.Now
)

// Handler implements os.mount.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("OsMountReverseInfo", func() any { return &OsMountReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage an /etc/fstab entry and the matching live mount",
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

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// os.mount writes /etc/fstab and shells to mount/umount for the
// runtime apply. Always Sudo, RequiredBinaries=[mount, umount].
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{"mount", "umount"},
		FilesystemWrite:  []string{"/etc/fstab"},
	}
	if step != nil && step.OsMount != nil && step.OsMount.Dest != "" {
		ps.FilesystemWrite = append(ps.FilesystemWrite, step.OsMount.Dest)
	}
	return ps
}

func (h *Handler) Validate(step *config.Step) error {
	m := step.OsMount
	if m == nil {
		return fmt.Errorf("os.mount requires configuration")
	}
	if strings.TrimSpace(m.Dest) == "" {
		return fmt.Errorf("os.mount: dest is required")
	}
	state := normalizeState(m.State)
	switch state {
	case stateMounted, stateUnmounted, stateFstabOnly, stateAbsent:
	default:
		return fmt.Errorf("os.mount: state must be mounted|unmounted|fstab_only|absent, got %q", m.State)
	}
	if state != stateAbsent {
		if strings.TrimSpace(m.Src) == "" {
			return fmt.Errorf("os.mount: src is required when state=%s", state)
		}
		if strings.TrimSpace(m.FSType) == "" {
			return fmt.Errorf("os.mount: fstype is required when state=%s", state)
		}
	}
	return nil
}

// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` actually retries this idempotent action via the
// centralized executor loop instead of being silently no-op'd.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	m := step.OsMount
	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("os.mount: only Linux is supported; got %s", runtime.GOOS)
	}

	// Spec-69 phase 5: runner + performer are per-Run, threaded into
	// applyPlan instead of riding package-level state.
	runner := ctx.Privileged()
	performer := ctx.Effects()

	rendered, err := renderMount(ctx, m)
	if err != nil {
		return result, err
	}

	plan, err := computePlan(rendered)
	if err != nil {
		return result, err
	}

	result.Data = map[string]interface{}{
		"dest":      rendered.dest,
		"operation": plan.operation,
		"path":      mountPaths.fstab,
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

	// Capture pre-apply state for Reverse() BEFORE applyPlan.
	// computePlan already read the fstab + mount table; re-read
	// here is cheap and keeps the capture isolated from the plan
	// struct internals.
	result.ReverseData = captureReverseInfo(rendered.dest, plan.touchesFstab, plan.doMount, plan.doUmount)

	if err := applyPlan(performer, runner, plan, rendered); err != nil {
		return result, err
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  os.mount: %s (%s)", rendered.dest, plan.operation)

	if plan.touchesFstab {
		if pub := ctx.GetEventPublisher(); pub != nil {
			pub.Publish(events.Event{
				Type: events.EventFileUpdated,
				Data: events.FileOperationData{Path: mountPaths.fstab, Changed: true},
			})
		}
	}
	return result, nil
}

// renderedMount holds the post-template, defaults-applied view used
// throughout the rest of the handler.
type renderedMount struct {
	src     string
	dest    string
	fstype  string
	options []string
	state   string
	dump    int
	pass    int
	backup  bool
}

func renderMount(ctx actions.Context, m *config.OsMount) (renderedMount, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()

	render := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return tmpl.Render(s, vars)
	}

	src, err := render(m.Src)
	if err != nil {
		return renderedMount{}, fmt.Errorf("os.mount: render src: %w", err)
	}
	dest, err := render(m.Dest)
	if err != nil {
		return renderedMount{}, fmt.Errorf("os.mount: render dest: %w", err)
	}
	fstype, err := render(m.FSType)
	if err != nil {
		return renderedMount{}, fmt.Errorf("os.mount: render fstype: %w", err)
	}

	opts := make([]string, 0, len(m.Options))
	for _, o := range m.Options {
		ro, err := render(o)
		if err != nil {
			return renderedMount{}, fmt.Errorf("os.mount: render option: %w", err)
		}
		ro = strings.TrimSpace(ro)
		if ro != "" {
			opts = append(opts, ro)
		}
	}
	if len(opts) == 0 {
		opts = []string{"defaults"}
	}

	dump := 0
	if m.Dump != nil {
		dump = *m.Dump
	}
	pass := 0
	if m.Pass != nil {
		pass = *m.Pass
	}
	backup := true
	if m.Backup != nil {
		backup = *m.Backup
	}

	return renderedMount{
		src:     src,
		dest:    canonicalDest(dest),
		fstype:  fstype,
		options: opts,
		state:   normalizeState(m.State),
		dump:    dump,
		pass:    pass,
		backup:  backup,
	}, nil
}

func normalizeState(s string) string {
	if s == "" {
		return stateMounted
	}
	return strings.ToLower(s)
}

// mountPlan captures the deltas needed to converge on the desired
// state — both fstab mutations and the live mount/umount call.
type mountPlan struct {
	changed      bool
	operation    string // create|update|delete|mount|umount|noop
	reason       string
	wantContent  string // full fstab content after mutation
	touchesFstab bool
	doMount      bool
	doUmount     bool
	backup       bool
	dest         string
}

func computePlan(r renderedMount) (mountPlan, error) {
	plan := mountPlan{dest: r.dest, backup: r.backup}

	fstabContent, err := readFstab()
	if err != nil {
		return plan, err
	}
	lines := parseFstab(fstabContent)
	idx := findByDest(lines, r.dest)
	hasEntry := idx >= 0
	currentMounts, err := readMounts()
	if err != nil {
		return plan, err
	}
	_, isMounted := currentMounts[r.dest]

	want := fstabEntry{
		src:     r.src,
		dest:    r.dest,
		fstype:  r.fstype,
		options: append([]string(nil), r.options...),
		dump:    r.dump,
		pass:    r.pass,
	}

	switch r.state {
	case stateAbsent:
		switch {
		case hasEntry && isMounted:
			plan.changed = true
			plan.touchesFstab = true
			plan.doUmount = true
			plan.operation = "delete"
			plan.reason = "would unmount " + r.dest + " and remove fstab entry"
		case hasEntry:
			plan.changed = true
			plan.touchesFstab = true
			plan.operation = "delete"
			plan.reason = "would remove fstab entry for " + r.dest
		case isMounted:
			plan.changed = true
			plan.doUmount = true
			plan.operation = "umount"
			plan.reason = "would unmount " + r.dest + " (no fstab entry)"
		default:
			plan.operation = "noop"
			plan.reason = "fstab entry already absent and not mounted"
		}
		if plan.touchesFstab {
			next := removeEntry(lines, r.dest)
			plan.wantContent = renderFstab(next)
		}
		return plan, nil

	case stateFstabOnly:
		switch {
		case !hasEntry:
			plan.changed = true
			plan.touchesFstab = true
			plan.operation = "create"
			plan.reason = "would add fstab entry for " + r.dest
		case !entryMatches(*lines[idx].entry, want):
			plan.changed = true
			plan.touchesFstab = true
			plan.operation = "update"
			plan.reason = "would update fstab entry for " + r.dest + " (drift)"
		default:
			plan.operation = "noop"
			plan.reason = "fstab entry already at desired state"
		}
		if plan.touchesFstab {
			next := upsertEntry(lines, want)
			plan.wantContent = renderFstab(next)
		}
		return plan, nil

	case stateUnmounted:
		// Keep / write fstab entry, but ensure unmounted now.
		switch {
		case !hasEntry:
			plan.changed = true
			plan.touchesFstab = true
			plan.operation = "create"
			plan.reason = "would add fstab entry for " + r.dest
		case !entryMatches(*lines[idx].entry, want):
			plan.changed = true
			plan.touchesFstab = true
			plan.operation = "update"
			plan.reason = "would update fstab entry for " + r.dest + " (drift)"
		}
		if isMounted {
			plan.changed = true
			plan.doUmount = true
			if plan.operation == "" {
				plan.operation = "umount"
				plan.reason = "would unmount " + r.dest
			}
		}
		if !plan.changed {
			plan.operation = "noop"
			plan.reason = "fstab entry already at desired state and not mounted"
			return plan, nil
		}
		if plan.touchesFstab {
			next := upsertEntry(lines, want)
			plan.wantContent = renderFstab(next)
		}
		return plan, nil

	case stateMounted:
		// fstab entry must exist & match; live mount must be present.
		if hasEntry {
			cur := *lines[idx].entry
			if !entryMatches(cur, want) {
				plan.changed = true
				plan.touchesFstab = true
				plan.operation = "update"
				plan.reason = "would update fstab entry for " + r.dest + " (drift)"
			}
		} else {
			plan.changed = true
			plan.touchesFstab = true
			plan.operation = "create"
			plan.reason = "would add fstab entry for " + r.dest
		}
		if !isMounted {
			plan.changed = true
			plan.doMount = true
			if plan.operation == "" {
				plan.operation = "mount"
				plan.reason = "would mount " + r.dest
			}
		} else {
			// Already mounted; refuse to silently remount with different
			// options/fs. Remount is risky and outside this action's
			// remit — the user must `unmount` first.
			liveEntry := currentMounts[r.dest]
			if liveEntry.fstype != "" && liveEntry.fstype != want.fstype {
				return plan, fmt.Errorf("os.mount: %s already mounted as %s; refusing to remount as %s (unmount first)", r.dest, liveEntry.fstype, want.fstype)
			}
		}
		if !plan.changed {
			plan.operation = "noop"
			plan.reason = "fstab entry and live mount already at desired state"
			return plan, nil
		}
		if plan.touchesFstab {
			next := upsertEntry(lines, want)
			plan.wantContent = renderFstab(next)
		}
		return plan, nil
	}
	return plan, fmt.Errorf("unreachable: state=%q", r.state)
}

// removeEntry returns the line slice with the entry at dest stripped.
// Unrelated lines (blanks, comments, other entries) are preserved
// byte-identical.
func removeEntry(lines []fstabLine, dest string) []fstabLine {
	canon := canonicalDest(dest)
	out := make([]fstabLine, 0, len(lines))
	for _, ln := range lines {
		if ln.entry != nil && canonicalDest(ln.entry.dest) == canon {
			continue
		}
		out = append(out, ln)
	}
	return out
}

// upsertEntry inserts or replaces the entry matching `want.dest`. When
// replacing, the new line replaces the old in-place; when inserting,
// the line is appended at the end. The raw content is recomputed so
// it reflects the canonical render.
func upsertEntry(lines []fstabLine, want fstabEntry) []fstabLine {
	want.raw = renderEntry(want)
	idx := findByDest(lines, want.dest)
	if idx >= 0 {
		out := make([]fstabLine, len(lines))
		copy(out, lines)
		e := want
		out[idx] = fstabLine{raw: want.raw, entry: &e}
		return out
	}
	out := make([]fstabLine, 0, len(lines)+1)
	out = append(out, lines...)
	e := want
	out = append(out, fstabLine{raw: want.raw, entry: &e})
	return out
}

func applyPlan(performer actions.Performer, runner *security.Privileged, plan mountPlan, r renderedMount) error {
	if plan.touchesFstab {
		if err := writeFstab(performer, plan.wantContent, plan.backup); err != nil {
			return err
		}
	}
	if plan.doUmount {
		if err := umountRun(runner, r.dest); err != nil {
			return fmt.Errorf("os.mount: umount %s: %w", r.dest, err)
		}
	}
	if plan.doMount {
		if err := mountRun(runner, r.dest); err != nil {
			return fmt.Errorf("os.mount: mount %s: %w", r.dest, err)
		}
	}
	return nil
}

// readFstab returns the file content and any stat error other than
// ENOENT (which is treated as "empty fstab"). The original signature
// also returned a "did exist" bool but no caller used it — fstab
// missing on Linux is so rare we treat both cases (missing / empty)
// the same way at the consumer level.
func readFstab() (string, error) {
	data, err := os.ReadFile(mountPaths.fstab)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", mountPaths.fstab, err)
	}
	return string(data), nil
}

// readMounts parses /proc/mounts (or the stubbed equivalent). On
// systems without /proc/mounts (or when stubbed to a missing path),
// returns an empty map rather than erroring — the caller treats that
// as "nothing mounted".
func readMounts() (map[string]fstabEntry, error) {
	data, err := os.ReadFile(mountPaths.mounts)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]fstabEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", mountPaths.mounts, err)
	}
	return parseMounts(string(data)), nil
}

// writeFstab snapshots the existing file (when backup is enabled and
// the file exists) and writes the new content atomically. Atomic
// write is critical: a half-written fstab can prevent boot.
func writeFstab(performer actions.Performer, content string, backup bool) error {
	if backup {
		if err := snapshotFstab(performer); err != nil {
			return err
		}
	}
	pOpts := actions.PerformerOpts{}
	if e := performer.Mkdir(filepath.Dir(mountPaths.fstab), 0o755, pOpts); e.Err != nil {
		return fmt.Errorf("os.mount: mkdir %s: %w", filepath.Dir(mountPaths.fstab), e.Err)
	}
	if e := performer.WriteFile(mountPaths.fstab, []byte(content), 0o644, actions.PerformerOpts{ExplicitMode: true}); e.Err != nil {
		return fmt.Errorf("os.mount: write %s: %w", mountPaths.fstab, e.Err)
	}
	return nil
}

// snapshotFstab copies the current fstab to fstab.bak.<unix-ts> when
// the file exists. Missing file is not an error — there is nothing to
// snapshot.
func snapshotFstab(performer actions.Performer) error {
	data, err := os.ReadFile(mountPaths.fstab)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("os.mount: snapshot read %s: %w", mountPaths.fstab, err)
	}
	ts := clockNow().UTC().Format("20060102T150405Z")
	dest := mountPaths.fstab + ".bak." + ts
	if e := performer.WriteFile(dest, data, 0o644, actions.PerformerOpts{ExplicitMode: true}); e.Err != nil {
		return fmt.Errorf("os.mount: snapshot write %s: %w", dest, e.Err)
	}
	return nil
}

// realMount shells out to `mount <dest>` via the supplied runner,
// bounded by mountCmdTimeout (F051).
func realMount(runner *security.Privileged, dest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), mountCmdTimeout)
	defer cancel()
	out, err := runner.Run(ctx, "mount", dest)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// realUmount shells out to `umount <dest>` via the supplied runner,
// bounded by mountCmdTimeout (F051).
func realUmount(runner *security.Privileged, dest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), mountCmdTimeout)
	defer cancel()
	out, err := runner.Run(ctx, "umount", dest)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
