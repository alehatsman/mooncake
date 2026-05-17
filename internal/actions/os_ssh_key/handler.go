// Package os_ssh_key implements the os.ssh_key action: idempotent
// authorized_keys management. Identity is per public-key blob
// (algorithm + base64 material); the trailing comment is descriptive
// and doesn't participate in identity. Supports single-key and
// multi-key forms; the multi-key form can be exclusive (any key not
// in the supplied list is removed).
//
//nolint:revive // Package name matches action name convention (os_ssh_key)
package os_ssh_key

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// eff is the spec-69 sudo-aware Performer used by writeAuthorizedKeys.
// Phase 5b's try-direct-then-sudo semantic means the common case
// (user manages their own ~/.ssh/authorized_keys) succeeds with no
// sudo prompt, while the cross-user case (e.g. provisioning a new
// user's keys from a non-root mooncake) escalates correctly instead
// of the previous "chown EACCES — run with sudo / become: true"
// dead-end. Set by Run() from ctx.Effects() before dispatch.
var eff actions.Performer

// privRunner backs the chown fallback for the cross-user case.
// Performer.Chown supports Become but takes username/groupname
// strings; the existing call site here passes numeric uid/gid via
// the chownFn hook so the test suite can stub it. Keeping the hook
// intact, the sudo fallback below uses /usr/bin/chown directly.
var privRunner actions.PrivilegedRunner = security.PrivilegedRunner{}

const (
	actionName       = "os.ssh_key"
	statePresent     = "present"
	stateAbsent      = "absent"
	authorizedKeys   = "authorized_keys"
	sshDirMode       = 0o700
	authorizedMode   = 0o600
	atomicTempSuffix = ".mooncake-tmp"
)

// Handler implements os.ssh_key.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("OsSSHKeyReverseInfo", func() any { return &OsSSHKeyReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage authorized_keys entries for a user (idempotent per-key)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventFileUpdated)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux", "darwin"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// os.ssh_key writes to authorized_keys for the target user. Two
// cases:
//   - root or system user (e.g. /root/.ssh/authorized_keys, or a
//     user whose home is under a SystemPathPrefixes root): Sudo
//     required.
//   - regular user under /home/<user>/.ssh/authorized_keys: Sudo
//     required iff the daemon isn't already running as that user
//     — which the planner can't determine here, so we conservatively
//     declare Sudo=true.
//
// In practice every os.ssh_key step needs sudo (the file is owned
// by the target user, mode 0600, and the action's apply path
// chowns + chmods the directory + file). No Network, no required
// binaries (the action reads/writes plain files via os.* calls).
//
// When the step pins a custom Path under a system root, surface it
// in FilesystemWrite for the policy layer.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{Sudo: true}
	if step == nil || step.OsSSHKey == nil {
		return ps
	}
	if step.OsSSHKey.Path != "" {
		ps.FilesystemWrite = []string{step.OsSSHKey.Path}
	}
	return ps
}

func (h *Handler) Validate(step *config.Step) error {
	k := step.OsSSHKey
	if k == nil {
		return fmt.Errorf("os.ssh_key requires configuration")
	}
	if strings.TrimSpace(k.User) == "" {
		return fmt.Errorf("os.ssh_key: user is required")
	}
	if k.Key == "" && len(k.Keys) == 0 {
		return fmt.Errorf("os.ssh_key: key or keys must be provided")
	}
	if k.Key != "" && len(k.Keys) > 0 {
		return fmt.Errorf("os.ssh_key: key and keys are mutually exclusive")
	}
	state := normalizeState(k.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("os.ssh_key: state must be present or absent, got %q", k.State)
	}
	if k.Exclusive && (state != statePresent || len(k.Keys) == 0) {
		return fmt.Errorf("os.ssh_key: exclusive=true requires state=present and keys: [...]")
	}
	for _, raw := range allKeys(k) {
		if _, err := parseKey(raw); err != nil {
			return fmt.Errorf("os.ssh_key: %w", err)
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
	k := step.OsSSHKey
	ec, _ := ctx.(*executor.ExecutionContext)

	result := executor.NewResult()
	result.Checkable = true

	// Wire spec-69 primitives. Common case (user installs their own
	// keys) succeeds without consulting sudo thanks to Performer's
	// phase 5b try-direct-then-fallback; cross-user case escalates.
	eff = ctx.Effects()
	privRunner = ctx.Privileged()

	username, err := ctx.GetTemplate().Render(k.User, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("os.ssh_key: render user: %w", err)
	}
	path, err := resolveAuthorizedKeysPath(ctx, ec, k, username)
	if err != nil {
		return result, err
	}

	desiredKeys, opts, err := renderKeys(ctx, k)
	if err != nil {
		return result, err
	}

	existing, fileExists, err := readAuthorizedKeys(path)
	if err != nil {
		return result, err
	}

	plan := computePlan(plannerInput{
		state:     normalizeState(k.State),
		desired:   desiredKeys,
		options:   opts,
		exclusive: k.Exclusive,
		existing:  existing,
	})

	result.Data = map[string]interface{}{
		"path":    path,
		"user":    username,
		"added":   plan.added,
		"removed": plan.removed,
		"updated": plan.updated,
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

	// Capture pre-apply state for Reverse() BEFORE writeAuthorizedKeys.
	// existing + fileExists are the snapshot; rebuilding the byte-
	// faithful content from the line slice mirrors what readAuthorizedKeys
	// stripped.
	result.ReverseData = &OsSSHKeyReverseInfo{
		Path:         path,
		User:         username,
		PriorExisted: fileExists,
		PriorContent: priorContentBytes(existing, fileExists),
	}

	// F035: fail fast on user.Lookup error BEFORE writing. Pre-fix the
	// lookup error was captured into ownerLookupErr but the write happened
	// anyway with uid=-1/gid=-1, leaving authorized_keys owned by the
	// caller and silently breaking sshd's strict-mode ownership check.
	// Common triggers: LDAP/NSS timeout in CI bootstrapping, username
	// typo, race against a not-yet-created user. Surface remediation in
	// the error so operators see the actual problem instead of a
	// step-ok-but-ssh-broken outcome.
	uid, gid, ownerLookupErr := lookupOwnership(username)
	if ownerLookupErr != nil {
		return result, fmt.Errorf(
			"os.ssh_key: cannot determine uid/gid for user %s: %w "+
				"(create the user first, or set `path:` to a location whose ownership you manage)",
			username, ownerLookupErr,
		)
	}
	if err := writeAuthorizedKeys(path, plan.lines, uid, gid, !fileExists); err != nil {
		return result, fmt.Errorf("os.ssh_key: write: %w", err)
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  os.ssh_key: %s (+%d -%d ~%d)", path, plan.added, plan.removed, plan.updated)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{Path: path, Changed: true},
		})
	}
	return result, nil
}

// resolveAuthorizedKeysPath returns the path to authorized_keys for the
// target user, applying explicit `path:` if set, then falling back to
// `~user/.ssh/authorized_keys` (via os/user lookup).
func resolveAuthorizedKeysPath(ctx actions.Context, ec *executor.ExecutionContext, k *config.OsSSHKey, username string) (string, error) {
	if k.Path != "" {
		if ec != nil {
			return ec.Svc.PathUtil.ExpandPath(k.Path, ec.CurrentDir, ctx.GetVariables())
		}
		return ctx.GetTemplate().Render(k.Path, ctx.GetVariables())
	}
	u, err := user.Lookup(username)
	if err != nil {
		return "", fmt.Errorf("os.ssh_key: lookup user %s: %w", username, err)
	}
	return filepath.Join(u.HomeDir, ".ssh", authorizedKeys), nil
}

func lookupOwnership(username string) (uid, gid int, err error) {
	u, lookupErr := user.Lookup(username)
	if lookupErr != nil {
		return -1, -1, lookupErr
	}
	uid, _ = strconv.Atoi(u.Uid)
	gid, _ = strconv.Atoi(u.Gid)
	return uid, gid, nil
}

func allKeys(k *config.OsSSHKey) []string {
	if k.Key != "" {
		return []string{k.Key}
	}
	return k.Keys
}

func renderKeys(ctx actions.Context, k *config.OsSSHKey) ([]parsedKey, []string, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	keys := make([]parsedKey, 0, len(allKeys(k)))
	for _, raw := range allKeys(k) {
		rendered, err := tmpl.Render(raw, vars)
		if err != nil {
			return nil, nil, fmt.Errorf("os.ssh_key: render key: %w", err)
		}
		pk, err := parseKey(rendered)
		if err != nil {
			return nil, nil, fmt.Errorf("os.ssh_key: %w", err)
		}
		keys = append(keys, pk)
	}
	// Options are applied uniformly across all desired keys for the
	// single-action form. Render them once.
	opts := make([]string, 0, len(k.Options))
	for _, o := range k.Options {
		ro, err := tmpl.Render(o, vars)
		if err != nil {
			return nil, nil, fmt.Errorf("os.ssh_key: render option: %w", err)
		}
		opts = append(opts, ro)
	}
	return keys, opts, nil
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

// parsedKey holds the components of an authorized_keys entry that
// matter for identity and rendering. `options` is preserved on the
// line; identity is (algo, blob).
type parsedKey struct {
	options []string
	algo    string
	blob    string // base64-encoded public material
	comment string
}

func (p parsedKey) identity() string { return p.algo + " " + p.blob }

func (p parsedKey) render() string {
	var sb strings.Builder
	if len(p.options) > 0 {
		sb.WriteString(strings.Join(p.options, ","))
		sb.WriteByte(' ')
	}
	sb.WriteString(p.algo)
	sb.WriteByte(' ')
	sb.WriteString(p.blob)
	if p.comment != "" {
		sb.WriteByte(' ')
		sb.WriteString(p.comment)
	}
	return sb.String()
}

// parseKey breaks a public-key line into options/algo/blob/comment.
// Tolerates lines with leading options; the canonical form is
// `[options] algo blob [comment]`. Lines starting with `#` are
// comments and are not valid keys here (the caller filters them).
func parseKey(s string) (parsedKey, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return parsedKey{}, fmt.Errorf("empty key line")
	}
	// Heuristic: the algo is one of a known set. Split into fields and
	// find the first field that looks like an algo. Anything before it
	// is options (a single comma-joined string per the spec).
	fields := strings.Fields(s)
	algoIdx := -1
	for i, f := range fields {
		if isKnownAlgo(f) {
			algoIdx = i
			break
		}
	}
	if algoIdx == -1 {
		return parsedKey{}, fmt.Errorf("unrecognised key algorithm in %q", s)
	}
	if algoIdx+1 >= len(fields) {
		return parsedKey{}, fmt.Errorf("missing key material after %s", fields[algoIdx])
	}
	pk := parsedKey{
		algo: fields[algoIdx],
		blob: fields[algoIdx+1],
	}
	if algoIdx > 0 {
		// All preceding tokens were one options string (comma-separated).
		// In a strict authorized_keys file the options field is a single
		// token; we join with space for tolerance when users supply
		// space-separated input.
		pk.options = []string{strings.Join(fields[:algoIdx], " ")}
	}
	if algoIdx+2 < len(fields) {
		pk.comment = strings.Join(fields[algoIdx+2:], " ")
	}
	return pk, nil
}

func isKnownAlgo(s string) bool {
	switch s {
	case "ssh-rsa", "ssh-dss", "ssh-ed25519",
		"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521",
		"sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com":
		return true
	}
	return false
}

// readAuthorizedKeys returns the raw lines of the file (including
// blank/comment lines so they're preserved on rewrite) and whether
// the file existed.
func readAuthorizedKeys(path string) ([]string, bool, error) {
	// #nosec G304 -- path from user config.
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	content := string(data)
	if content == "" {
		return nil, true, nil
	}
	// Strip the trailing newline so split produces clean lines; we
	// re-add the trailing newline on write.
	content = strings.TrimSuffix(content, "\n")
	return strings.Split(content, "\n"), true, nil
}

type plannerInput struct {
	state     string
	desired   []parsedKey
	options   []string
	exclusive bool
	existing  []string
}

type planResult struct {
	changed                 bool
	reason                  string
	lines                   []string
	added, removed, updated int
}

func computePlan(in plannerInput) planResult {
	// Index existing lines by parsed-key identity. Preserve non-key
	// lines (comments / blanks) by carrying their original text.
	type entry struct {
		raw      string
		parsed   parsedKey
		isKey    bool
		identity string
		removed  bool // marked during planning
	}
	entries := make([]entry, 0, len(in.existing)+len(in.desired))
	idIndex := map[string]int{}
	for _, raw := range in.existing {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			entries = append(entries, entry{raw: raw})
			continue
		}
		pk, err := parseKey(trimmed)
		if err != nil {
			// Preserve unparseable lines verbatim.
			entries = append(entries, entry{raw: raw})
			continue
		}
		e := entry{raw: raw, parsed: pk, isKey: true, identity: pk.identity()}
		idIndex[pk.identity()] = len(entries)
		entries = append(entries, e)
	}

	result := planResult{}

	// Build the desired-key set, applying the requested options.
	desiredByID := map[string]parsedKey{}
	for _, d := range in.desired {
		// User-supplied options take precedence over whatever was in
		// the key string itself.
		if len(in.options) > 0 {
			d.options = []string{strings.Join(in.options, ",")}
		}
		desiredByID[d.identity()] = d
	}

	switch in.state {
	case statePresent:
		// Add or update each desired key.
		for id, d := range desiredByID {
			if idx, ok := idIndex[id]; ok {
				// Already present; update if options or comment differ.
				cur := entries[idx].parsed
				if !sameRender(cur, d) {
					entries[idx].parsed = d
					entries[idx].raw = d.render()
					result.updated++
				}
			} else {
				entries = append(entries, entry{raw: d.render(), parsed: d, isKey: true, identity: id})
				idIndex[id] = len(entries) - 1
				result.added++
			}
		}
		if in.exclusive {
			for i, e := range entries {
				if !e.isKey {
					continue
				}
				if _, keep := desiredByID[e.identity]; !keep {
					entries[i].removed = true
					result.removed++
				}
			}
		}

	case stateAbsent:
		for id := range desiredByID {
			if idx, ok := idIndex[id]; ok {
				entries[idx].removed = true
				result.removed++
			}
		}
	}

	// Render the new line set.
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.removed {
			continue
		}
		if e.isKey {
			lines = append(lines, e.parsed.render())
		} else {
			lines = append(lines, e.raw)
		}
	}
	result.lines = lines

	result.changed = result.added > 0 || result.removed > 0 || result.updated > 0
	switch {
	case !result.changed && in.state == stateAbsent:
		result.reason = "no matching keys to remove"
	case !result.changed:
		result.reason = "all keys already present in desired state"
	default:
		result.reason = fmt.Sprintf("+%d -%d ~%d", result.added, result.removed, result.updated)
	}
	return result
}

func sameRender(a, b parsedKey) bool {
	return a.render() == b.render()
}

// chownFn is the chown primitive; indirected so unit tests can stub
// an EPERM return without needing a non-root subprocess. Production
// path is os.Chown.
var chownFn = os.Chown

// writeAuthorizedKeys writes the lines to path atomically (temp + rename).
// Ensures the parent .ssh dir is at mode 0700 (tightened even if it
// pre-existed at a wider mode — sshd refuses 0755 .ssh dirs) and the
// file at mode 0600. Sets ownership when uid >= 0; chown on the file
// itself is load-bearing (sshd rejects authorized_keys not owned by
// the user) so failures surface as errors. Parent-dir chown stays
// best-effort because non-root callers managing keys via an explicit
// `path:` should still be able to write.
//
// F035: pre-fix this function swallowed every chown error silently to
// "not fail unit tests on user-owned dirs." That covered the test
// case but left a real production failure path — a non-root operator
// installing keys for another user would land the file with operator
// ownership, sshd would reject it, and the step still reported
// success. Now the file-itself chown returns its error; tests that
// don't run as root must use a `path:` whose target uid matches their
// own (existing tests already do via currentUsername).
func writeAuthorizedKeys(path string, lines []string, uid, gid int, createParentMode bool) error {
	parent := filepath.Dir(path)
	pOpts := actions.PerformerOpts{Become: true}
	if e := eff.Mkdir(parent, sshDirMode, pOpts); e.Err != nil {
		return fmt.Errorf("mkdir %s: %w", parent, e.Err)
	}
	// F035: tighten parent perms unconditionally — Mkdir's idempotency
	// is no-op-on-mode for an existing directory, so pre-existing 0755
	// .ssh stays at 0755 without this. sshd rejects authorized_keys
	// under a too-permissive parent.
	if e := eff.Chmod(parent, sshDirMode, pOpts); e.Err != nil && !errors.Is(e.Err, fs.ErrPermission) {
		return fmt.Errorf("chmod %s: %w", parent, e.Err)
	}
	_ = createParentMode // kept for ABI compat; the chmod above subsumes it.

	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}

	if e := eff.WriteFile(path, []byte(content), authorizedMode, actions.PerformerOpts{Become: true, ExplicitMode: true}); e.Err != nil {
		return fmt.Errorf("write %s: %w", path, e.Err)
	}
	if uid >= 0 && gid >= 0 {
		if err := chownFn(path, uid, gid); err != nil {
			if !errors.Is(err, fs.ErrPermission) {
				return fmt.Errorf("chown %s: %w", path, err)
			}
			// Cross-user case: direct chown denied. Retry via
			// ctx.Privileged() so installing keys for another user
			// from a non-root mooncake works when sudo IS configured.
			// When sudo isn't configured, fall back to the pre-spec-69
			// fs.ErrPermission + "run with sudo" hint so existing
			// downstream code (and the spec-22-era F035 test that
			// asserts both behaviors) keeps working unchanged.
			spec := strconv.Itoa(uid) + ":" + strconv.Itoa(gid)
			out, sErr := privRunner.Run(context.TODO(), "chown", spec, path)
			if sErr == nil {
				// Sudo path succeeded — chown completed.
			} else if errors.Is(sErr, security.ErrBecomeNoSudoPass) || errors.Is(sErr, security.ErrBecomeUnsupported) {
				return fmt.Errorf(
					"chown %s to uid=%d gid=%d: %w "+
						"(run with sudo / become: true to install keys for another user)",
					path, uid, gid, err,
				)
			} else {
				msg := strings.TrimSpace(string(out))
				if msg != "" {
					return fmt.Errorf("chown %s: %w: %s", path, sErr, msg)
				}
				return fmt.Errorf("chown %s: %w", path, sErr)
			}
		}
		// Parent-dir chown is best-effort: non-fatal on any error so
		// a non-root operator managing their own keys can still write
		// (chownFn returns nil success when the dir is already user-
		// owned; only the cross-user path is interesting and the file
		// chown above is the canonical authoritative target).
		_ = chownFn(parent, uid, gid)
	}
	return nil
}
