// Package os_firewall implements the os.firewall action. v1 ships a
// single backend driver (ufw); `backend: auto` resolves to ufw when
// present and errors otherwise. Idempotency is computed by parsing
// the live ruleset (`ufw status numbered`), diffing against the
// desired set, and applying only the deltas — adding missing rules
// and removing rules present-but-unwanted (`state: absent`).
//
//nolint:revive // package name follows action convention
package os_firewall

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

const (
	actionName   = "os.firewall"
	statePresent = "present"
	stateAbsent  = "absent"

	backendUFW  = "ufw"
	backendAuto = "auto"

	actionAllow  = "allow"
	actionDeny   = "deny"
	actionReject = "reject"

	protoTCP = "tcp"
	protoUDP = "udp"
)

// ufwRun / ufwStatus are package-level hooks so tests can swap the
// shell-out for an in-memory ruleset. ufwLookPath gates "is ufw
// installed" so backend=auto can fail cleanly in tests.
var (
	ufwRun      = realUFWRun
	ufwStatus   = realUFWStatus
	ufwLookPath = realUFWLookPath
)

// privRunner is the spec-69 sudo-aware runner. ufw needs root for
// both rule-mutation and rule-listing (the status output includes
// PRIVATE per-IP rules) so we wrap both realUFWRun and realUFWStatus.
// Set by Run() from ctx.Privileged() before dispatch.
var privRunner actions.PrivilegedRunner = security.PrivilegedRunner{}

// Handler implements os.firewall.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("OsFirewallReverseInfo", func() any { return &OsFirewallReverseInfo{} })
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage host firewall rules (ufw backend)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"linux"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22 phase 3).
//
// os.firewall always Sudo: the action shells to `ufw` which writes
// to /etc/ufw/* and (re)loads netfilter rules. RequiredBinaries=[ufw]
// (v1 supports ufw only; non-ufw drivers are deferred per spec-28).
//
// No Network at apply time even though the action affects network
// reachability — Network in PermissionSet means "this step makes
// outbound network calls", which firewall config doesn't.
func (Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Sudo:             true,
		RequiredBinaries: []string{"ufw"},
	}
}

func (h *Handler) Validate(step *config.Step) error {
	f := step.OsFirewall
	if f == nil {
		return fmt.Errorf("os.firewall requires configuration")
	}
	state := normalizeState(f.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("os.firewall: state must be present or absent, got %q", f.State)
	}
	backend := normalizeBackend(f.Backend)
	switch backend {
	case backendUFW, backendAuto:
	default:
		return fmt.Errorf("os.firewall: backend %q not supported (valid: ufw, auto)", f.Backend)
	}
	if f.Rule != nil && len(f.Rules) > 0 {
		return fmt.Errorf("os.firewall: rule and rules are mutually exclusive")
	}
	if f.Rule == nil && len(f.Rules) == 0 {
		return fmt.Errorf("os.firewall: at least one of rule or rules is required")
	}
	rules := allRules(f)
	for i, r := range rules {
		if err := validateRule(r); err != nil {
			return fmt.Errorf("os.firewall: rule[%d]: %w", i, err)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	f := step.OsFirewall
	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		return result, fmt.Errorf("os.firewall: only Linux is supported; got %s", runtime.GOOS)
	}

	// Wire spec-69 runner for the ufw shell-outs.
	privRunner = ctx.Privileged()

	backend := normalizeBackend(f.Backend)
	if backend == backendAuto {
		if _, err := ufwLookPath(); err != nil {
			return result, fmt.Errorf("os.firewall: backend=auto and no supported backend found (ufw is the only v1 backend; install ufw or set backend explicitly)")
		}
		backend = backendUFW
	}

	desired, err := renderDesired(ctx, f)
	if err != nil {
		return result, err
	}

	current, err := readCurrent()
	if err != nil {
		return result, fmt.Errorf("os.firewall: read current rules: %w", err)
	}

	plan := computePlan(current, desired, normalizeState(f.State))
	result.Data = map[string]interface{}{
		"backend":   backend,
		"to_add":    len(plan.toAdd),
		"to_remove": len(plan.toRemove),
	}

	if !plan.changed() {
		result.Reason = "firewall rules already at desired state"
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = plan.reason()
		return result, nil
	}

	// Capture pre-apply mutation list for Reverse() BEFORE applyPlan.
	// Per computePlan's design, only one of toAdd / toRemove is
	// populated per apply (state=present only adds, state=absent only
	// removes), so the reverse is always single-direction.
	result.ReverseData = &OsFirewallReverseInfo{
		Backend:      backend,
		AppliedState: normalizeState(f.State),
		AddedRules:   ruleSliceToSnapshot(plan.toAdd),
		RemovedRules: ruleSliceToSnapshot(plan.toRemove),
	}

	if err := applyPlan(plan); err != nil {
		return result, err
	}

	result.Changed = true
	result.Reason = plan.reason()
	ctx.GetLogger().Infof("  os.firewall: +%d -%d", len(plan.toAdd), len(plan.toRemove))
	return result, nil
}

// rule is the normalized internal form: every field has its default
// resolved, and templating has happened. The triple (Port, Protocol,
// From, Action) determines identity; Comment is descriptive.
type rule struct {
	Port     int
	Protocol string
	Action   string
	From     string
	Comment  string
}

func renderDesired(ctx actions.Context, f *config.OsFirewall) ([]rule, error) {
	tmpl := ctx.GetTemplate()
	vars := ctx.GetVariables()
	render := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		return tmpl.Render(s, vars)
	}

	raw := allRules(f)
	out := make([]rule, 0, len(raw))
	for _, r := range raw {
		from, err := render(r.From)
		if err != nil {
			return nil, fmt.Errorf("render from: %w", err)
		}
		comment, err := render(r.Comment)
		if err != nil {
			return nil, fmt.Errorf("render comment: %w", err)
		}
		out = append(out, rule{
			Port:     r.Port,
			Protocol: defaultProto(r.Protocol),
			Action:   defaultAction(r.Action),
			From:     defaultFrom(from),
			Comment:  comment,
		})
	}
	return out, nil
}

func allRules(f *config.OsFirewall) []config.FirewallRule {
	if f.Rule != nil {
		return []config.FirewallRule{*f.Rule}
	}
	return f.Rules
}

func validateRule(r config.FirewallRule) error {
	if r.Port <= 0 || r.Port > 65535 {
		return fmt.Errorf("port %d out of range (1-65535)", r.Port)
	}
	proto := defaultProto(r.Protocol)
	if proto != protoTCP && proto != protoUDP {
		return fmt.Errorf("protocol %q invalid (valid: tcp, udp)", r.Protocol)
	}
	act := defaultAction(r.Action)
	if act != actionAllow && act != actionDeny && act != actionReject {
		return fmt.Errorf("action %q invalid (valid: allow, deny, reject)", r.Action)
	}
	return nil
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

func normalizeBackend(b string) string {
	if b == "" {
		return backendAuto
	}
	return strings.ToLower(b)
}

func defaultProto(p string) string {
	if p == "" {
		return protoTCP
	}
	return strings.ToLower(p)
}

func defaultAction(a string) string {
	if a == "" {
		return actionAllow
	}
	return strings.ToLower(a)
}

func defaultFrom(f string) string {
	if f == "" {
		return "any"
	}
	return strings.ToLower(f)
}

// firewallPlan describes the deltas that converge current → desired.
// Rules in toAdd are not yet on the system; rules in toRemove are
// present but unwanted.
type firewallPlan struct {
	toAdd    []rule
	toRemove []rule
}

func (p *firewallPlan) changed() bool { return len(p.toAdd) > 0 || len(p.toRemove) > 0 }

func (p *firewallPlan) reason() string {
	parts := []string{}
	if len(p.toAdd) > 0 {
		parts = append(parts, fmt.Sprintf("would add %d rule(s)", len(p.toAdd)))
	}
	if len(p.toRemove) > 0 {
		parts = append(parts, fmt.Sprintf("would remove %d rule(s)", len(p.toRemove)))
	}
	if len(parts) == 0 {
		return "no firewall changes"
	}
	return strings.Join(parts, "; ")
}

// computePlan diffs current vs desired by rule key. For state=present:
// missing rules go in toAdd; extras stay (we only manage what the user
// declared). For state=absent: any matching rule goes in toRemove;
// missing rules are noops.
func computePlan(current, desired []rule, state string) firewallPlan {
	plan := firewallPlan{}
	currentSet := map[string]rule{}
	for _, r := range current {
		currentSet[ruleKey(r)] = r
	}

	if state == stateAbsent {
		for _, r := range desired {
			if _, ok := currentSet[ruleKey(r)]; ok {
				plan.toRemove = append(plan.toRemove, r)
			}
		}
		return plan
	}

	for _, r := range desired {
		if _, ok := currentSet[ruleKey(r)]; !ok {
			plan.toAdd = append(plan.toAdd, r)
		}
	}
	return plan
}

// ruleKey is the identity used for idempotency. Comments are intentionally
// excluded — a comment change shouldn't force adding a duplicate rule.
func ruleKey(r rule) string {
	return fmt.Sprintf("%d/%s/%s/%s", r.Port, r.Protocol, r.Action, r.From)
}

func applyPlan(plan firewallPlan) error {
	for _, r := range plan.toRemove {
		if err := ufwRun(deleteArgs(r)...); err != nil {
			return fmt.Errorf("ufw delete: %w", err)
		}
	}
	for _, r := range plan.toAdd {
		if err := ufwRun(addArgs(r)...); err != nil {
			return fmt.Errorf("ufw add: %w", err)
		}
	}
	return nil
}

// addArgs builds the ufw command line for inserting a rule. Form:
//
//	ufw allow|deny|reject [from <cidr>] [to any] port <port> proto tcp|udp [comment "<note>"]
func addArgs(r rule) []string {
	args := []string{r.Action}
	if r.From != "" && r.From != "any" {
		args = append(args, "from", r.From, "to", "any")
	}
	args = append(args, "port", strconv.Itoa(r.Port), "proto", r.Protocol)
	if r.Comment != "" {
		args = append(args, "comment", r.Comment)
	}
	return args
}

// deleteArgs builds the inverse — `ufw delete allow port 22 proto tcp`.
// Source isn't always preserved by ufw on simple form rules; we pass
// it when non-default to match the way `addArgs` framed it.
func deleteArgs(r rule) []string {
	args := []string{"delete", r.Action}
	if r.From != "" && r.From != "any" {
		args = append(args, "from", r.From, "to", "any")
	}
	args = append(args, "port", strconv.Itoa(r.Port), "proto", r.Protocol)
	return args
}

// statusLineRE matches a numbered status line:
//
//	[ 1] 22/tcp                     ALLOW IN    Anywhere                   # ssh
//
// Capture order: port, protocol, action, source. Trailing comment after
// `#` is stripped; ufw's "Anywhere" is normalized to "any".
var statusLineRE = regexp.MustCompile(`^\s*\[\s*\d+\]\s+(\d+)/(\w+)\s+\(?(ALLOW|DENY|REJECT)(?:\s+IN)?\)?\s+(\S+(?:\s+\S+)*?)(?:\s+#\s*(.*))?$`)

// readCurrent parses `ufw status numbered` into the normalized rule
// shape. Ignores non-rule lines (headers, comments, IPv6 mirrors of
// IPv4 entries with "(v6)" markers — collapsed via key).
func readCurrent() ([]rule, error) {
	out, err := ufwStatus()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	rules := []rule{}
	for _, ln := range strings.Split(out, "\n") {
		r, ok := parseStatusLine(ln)
		if !ok {
			continue
		}
		key := ruleKey(r)
		if seen[key] {
			continue // collapse IPv4/IPv6 dup
		}
		seen[key] = true
		rules = append(rules, r)
	}
	return rules, nil
}

func parseStatusLine(line string) (rule, bool) {
	m := statusLineRE.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return rule{}, false
	}
	port, err := strconv.Atoi(m[1])
	if err != nil {
		return rule{}, false
	}
	r := rule{
		Port:     port,
		Protocol: strings.ToLower(m[2]),
		Action:   strings.ToLower(m[3]),
		From:     normalizeSource(m[4]),
		Comment:  strings.TrimSpace(m[5]),
	}
	return r, true
}

// normalizeSource flattens ufw's source-column variants — "Anywhere",
// "Anywhere (v6)", explicit CIDRs — into the same vocabulary we render
// to: "any" or a literal CIDR.
func normalizeSource(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "any"
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "anywhere") {
		return "any"
	}
	return low
}

// realUFWRun shells out to `ufw <args>` via ctx.Privileged(). The
// action is registered as RequiresSudo: true; the wrapper handles
// escalation when mooncake runs as a non-root user.
func realUFWRun(args ...string) error {
	out, err := privRunner.Run(nil, "ufw", args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func realUFWStatus() (string, error) {
	out, err := privRunner.Run(nil, "ufw", "status", "numbered")
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return string(out), nil
}

func realUFWLookPath() (string, error) {
	return exec.LookPath("ufw")
}
