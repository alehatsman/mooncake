// Package windows_firewall_rule implements the windows.firewall_rule
// action. Idempotent management of Windows Firewall inbound/outbound
// rules. spec-57.
//
// The handler renders PowerShell via internal/winutil and invokes it
// via powershell.exe -EncodedCommand (which dodges every quoting
// hazard). Drift detection compares the live rule's PSCustomObject
// summary against the desired Rule struct.
//
//nolint:revive // package name follows mooncake action convention
package windows_firewall_rule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/winutil"
)

const (
	actionName   = "windows.firewall_rule"
	statePresent = "present"
	stateAbsent  = "absent"
)

// runPS is a package-level hook so tests can swap the shell-out for
// an in-memory script. realPSRun is the production implementation.
var runPS = realPSRun

// Handler implements windows.firewall_rule.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata. SupportsBecome is false —
// elevation on Windows is process-wide via UAC, not per-command;
// callers must run mooncake.exe from an elevated shell.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage Windows Firewall inbound/outbound rules",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"windows"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Validate runs schema-level checks before plan/apply. Translates the
// YAML-flavoured config struct to a winutil.Rule and calls Validate
// there for the deeper checks (no duplicating rule logic in two
// places).
func (h *Handler) Validate(step *config.Step) error {
	f := step.WindowsFirewallRule
	if f == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	state := normalizeState(f.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("%s: state must be present or absent, got %q", actionName, f.State)
	}
	if state == statePresent {
		// Round-trip through Rule.Validate for the deep checks (proto+ports
		// nonsense, bad enums, etc.).
		r, err := toRule(f)
		if err != nil {
			return fmt.Errorf("%s: %w", actionName, err)
		}
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%s: %w", actionName, err)
		}
	} else {
		// State=absent only needs Name.
		if strings.TrimSpace(f.Name) == "" {
			return fmt.Errorf("%s: name is required (even for state=absent)", actionName)
		}
	}
	return nil
}

// Run is the canonical apply path. Computes the diff against the
// remote's live rule (if any), then either creates / updates / deletes
// to converge. Plan mode reports what *would* change without making
// changes.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	f := step.WindowsFirewallRule
	result := executor.NewResult()
	result.Checkable = true
	result.Target = f.Name

	if runtime.GOOS != "windows" {
		return result, fmt.Errorf("%s: only Windows is supported; got %s", actionName, runtime.GOOS)
	}

	state := normalizeState(f.State)

	// Query the current rule (if any) so we can decide create/update/delete.
	current, err := queryRule(ctx, f.Name)
	if err != nil {
		return result, fmt.Errorf("%s: query current rule: %w", actionName, err)
	}

	switch state {
	case stateAbsent:
		if current == nil {
			result.Operation = executor.OpNoop
			result.Reason = "rule already absent"
			return result, nil
		}
		result.Operation = executor.OpDelete
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = true
			result.Reason = "would remove rule " + f.Name
			return result, nil
		}
		result.ReverseData = &WindowsFirewallRuleReverseInfo{
			AppliedState: stateAbsent,
			PriorExisted: true,
			PriorRule:    observedRuleToSnapshot(current),
		}
		if err := deleteRule(ctx, f.Name); err != nil {
			return result, err
		}
		result.Changed = true
		result.Reason = "removed rule " + f.Name
		return result, nil

	case statePresent:
		desired, err := toRule(f)
		if err != nil {
			return result, err
		}
		if current != nil && current.Equals(desired) {
			result.Operation = executor.OpNoop
			result.Reason = "rule already at desired state"
			return result, nil
		}
		if current == nil {
			result.Operation = executor.OpCreate
		} else {
			result.Operation = executor.OpUpdate
		}
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = true
			if current == nil {
				result.Reason = "would create rule " + f.Name
			} else {
				result.Reason = "would update rule " + f.Name
			}
			return result, nil
		}
		// Apply. If a rule with this name already exists, we delete +
		// recreate rather than try to Set-NetFirewallRule each
		// drifting field — the latter is field-by-field PowerShell
		// gymnastics that don't pay off given how cheap recreate is.
		result.ReverseData = &WindowsFirewallRuleReverseInfo{
			AppliedState: statePresent,
			PriorExisted: current != nil,
			PriorRule:    observedRuleToSnapshot(current),
		}
		if current != nil {
			if err := deleteRule(ctx, f.Name); err != nil {
				return result, err
			}
		}
		if err := createRule(ctx, desired); err != nil {
			return result, err
		}
		result.Changed = true
		if current == nil {
			result.Reason = "created rule " + f.Name
		} else {
			result.Reason = "updated rule " + f.Name
		}
		return result, nil
	}
	return result, fmt.Errorf("%s: unreachable state %q", actionName, state)
}

// ----- diff / apply implementations ----------------------------------------

// queryRule runs winutil.RenderQuery on the remote, parses the
// PSCustomObject summary, and returns it as an *observedRule (or nil
// when no rule exists).
func queryRule(_ actions.Context, name string) (*observedRule, error) {
	script := "ConvertTo-Json -InputObject (" + winutil.RenderQuery(name) + ") -Compress"
	out, err := runPS(script)
	if err != nil {
		return nil, fmt.Errorf("query rule: %w", err)
	}
	out = strings.TrimSpace(out)
	if out == "" || out == "null" {
		return nil, nil
	}
	var raw struct {
		DisplayName string   `json:"DisplayName"`
		Description string   `json:"Description"`
		Direction   string   `json:"Direction"`
		Protocol    string   `json:"Protocol"`
		LocalPort   []string `json:"LocalPort"`
		RemotePort  []string `json:"RemotePort"`
		Action      string   `json:"Action"`
		Profile     string   `json:"Profile"`
		Enabled     string   `json:"Enabled"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("decode rule json: %w (body: %q)", err, out)
	}
	return &observedRule{
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		Direction:   strings.ToLower(raw.Direction),
		Protocol:    strings.ToLower(raw.Protocol),
		LocalPorts:  normaliseObservedPorts(raw.LocalPort),
		RemotePorts: normaliseObservedPorts(raw.RemotePort),
		Action:      strings.ToLower(raw.Action),
		Profiles:    parseProfileMask(raw.Profile),
		Enabled:     strings.EqualFold(raw.Enabled, "True"),
	}, nil
}

func createRule(_ actions.Context, r winutil.Rule) error {
	ps, err := winutil.RenderCreate(r)
	if err != nil {
		return err
	}
	if _, err := runPS(ps); err != nil {
		return fmt.Errorf("create rule: %w", err)
	}
	return nil
}

func deleteRule(_ actions.Context, name string) error {
	if _, err := runPS(winutil.RenderDelete(name)); err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	return nil
}

// observedRule is the parsed PSCustomObject Get-NetFirewallRule
// returns. Same shape as winutil.Rule but with already-lowercased
// strings so equality is straightforward.
type observedRule struct {
	DisplayName string
	Description string
	Direction   string
	Protocol    string
	LocalPorts  []string
	RemotePorts []string
	Action      string
	Profiles    []string
	Enabled     bool
}

// Equals compares an observed rule against a desired winutil.Rule.
// Returns true when every field aligns (after normalisation).
func (o *observedRule) Equals(r winutil.Rule) bool {
	if o.DisplayName != r.DisplayName {
		return false
	}
	if o.Description != r.Description {
		return false
	}
	if o.Direction != string(directionDefault(r.Direction)) {
		return false
	}
	wantProto := protocolForCompare(r.Protocol)
	if o.Protocol != wantProto {
		return false
	}
	if !stringSliceEqualUnsorted(o.LocalPorts, r.LocalPorts) {
		return false
	}
	if !stringSliceEqualUnsorted(o.RemotePorts, r.RemotePorts) {
		return false
	}
	if o.Action != string(actionDefault(r.Action)) {
		return false
	}
	if !profilesEqual(o.Profiles, r.Profiles) {
		return false
	}
	wantEnabled := r.Enabled == nil || *r.Enabled
	if o.Enabled != wantEnabled {
		return false
	}
	return true
}

// ----- helpers --------------------------------------------------------------

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

// toRule maps a config.WindowsFirewallRule (YAML-flavoured) to a
// winutil.Rule (Go-flavoured). Defaults are NOT applied here; that
// happens inside winutil.RenderCreate so the Equals check above can
// see "empty means default" symmetrically.
func toRule(f *config.WindowsFirewallRule) (winutil.Rule, error) {
	r := winutil.Rule{
		DisplayName: f.Name,
		Description: f.Description,
		Direction:   winutil.Direction(strings.ToLower(f.Direction)),
		Protocol:    winutil.Protocol(strings.ToLower(f.Protocol)),
		LocalPorts:  append([]string(nil), f.LocalPort...),
		RemotePorts: append([]string(nil), f.RemotePort...),
		Action:      winutil.Action(strings.ToLower(f.Action)),
		Enabled:     f.Enabled,
	}
	for _, p := range f.Profile {
		r.Profiles = append(r.Profiles, winutil.Profile(strings.ToLower(p)))
	}
	return r, nil
}

func directionDefault(d winutil.Direction) winutil.Direction {
	if d == "" {
		return winutil.DirInbound
	}
	return d
}

func actionDefault(a winutil.Action) winutil.Action {
	if a == "" {
		return winutil.ActionAllow
	}
	return a
}

// protocolForCompare folds the "default protocol" rule into the
// comparison: an empty desired Protocol means TCP (per winutil's
// withDefaults). The cmdlet always reports the literal protocol back.
func protocolForCompare(p winutil.Protocol) string {
	if p == "" {
		return "tcp"
	}
	return strings.ToLower(string(p))
}

// profilesEqual checks if observed profile names match the desired
// ones after applying the default ([Domain, Private]).
func profilesEqual(observed []string, desired []winutil.Profile) bool {
	if len(desired) == 0 {
		desired = []winutil.Profile{winutil.ProfDomain, winutil.ProfPrivate}
	}
	want := make([]string, len(desired))
	for i, p := range desired {
		want[i] = strings.ToLower(string(p))
	}
	// "Any" in either side should expand to all three, but since
	// Get-NetFirewallRule returns the literal "Any" rather than the
	// expanded set, handle it specially.
	if len(observed) == 1 && observed[0] == "any" {
		for _, w := range want {
			if w == "any" {
				return true
			}
		}
		// observed=Any, desired=[Domain,Private] → not equal
		return false
	}
	return stringSliceEqualUnsorted(observed, want)
}

func normaliseObservedPorts(in []string) []string {
	if len(in) == 1 && (in[0] == "Any" || in[0] == "any") {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// parseProfileMask parses the PSCustomObject "Profile" field which is
// typically a comma-joined name list ("Domain, Private") OR the
// literal "Any". Returns lowercase tokens.
func parseProfileMask(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func stringSliceEqualUnsorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
		if seen[x] < 0 {
			return false
		}
	}
	return true
}

// realPSRun invokes powershell.exe with -EncodedCommand. EncodedCommand
// is UTF-16LE-base64 PowerShell script; using it side-steps every
// quoting / escaping hazard at the cmd.exe boundary.
//
// Issue #13: prepend winutil.UTF8OutputPrelude so cmdlets like
// `ConvertTo-Json` emit non-ASCII bytes verbatim instead of getting
// re-encoded to the OEM codepage (which corrupts to 0x1A on round-
// trip and breaks the controller-side JSON decode).
func realPSRun(script string) (string, error) {
	script = winutil.WithUTF8Output(script)
	utf16le := utf16.Encode([]rune(script))
	buf := bytes.Buffer{}
	for _, r := range utf16le {
		buf.WriteByte(byte(r))
		buf.WriteByte(byte(r >> 8))
	}
	encoded := base64StdEncode(buf.Bytes())
	cmd := exec.Command("powershell.exe", "-NoProfile", "-EncodedCommand", encoded)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("powershell exited: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// base64StdEncode wraps encoding/base64 to avoid the import-cycle
// risk of adding it at the top — pure indirection.
func base64StdEncode(b []byte) string {
	return base64Enc.EncodeToString(b)
}
