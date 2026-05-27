// Package windows_hyperv_firewall_rule implements the
// windows.hyperv_firewall_rule action. Idempotent management of
// Windows Hyper-V Firewall rules — the separate firewall stack that
// gates WSL2 mirrored-mode networking. proposal-19.
//
// Shape mirrors windows_firewall_rule: render PowerShell via
// internal/winutil, invoke via powershell.exe -EncodedCommand, compare
// the live PSCustomObject against the desired Rule for drift.
// Differences: every rule is scoped by VMCreatorID, `auto` triggers
// runtime discovery of WSL's VMCreatorID, and there's no profile field
// (Hyper-V firewall has no Domain/Private/Public concept).
//
//nolint:revive // package name follows mooncake action convention
package windows_hyperv_firewall_rule

import (
	"bytes"
	"encoding/base64"
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
	actionName   = "windows.hyperv_firewall_rule"
	statePresent = "present"
	stateAbsent  = "absent"

	// vmCreatorAuto is the sentinel that triggers WSL VMCreatorID
	// discovery. Anything else is treated as a literal GUID.
	vmCreatorAuto = "auto"

	// wslMatchName is the substring matched against
	// Get-NetFirewallHyperVVMCreator's Name field. WSL registers as
	// "WSL" on every supported Windows build we care about.
	wslMatchName = "WSL"
)

// runPS is a package-level hook so tests can swap the shell-out for
// an in-memory script. realPSRun is the production implementation.
var runPS = realPSRun

// Handler implements windows.hyperv_firewall_rule.
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
		Description:        "Manage Windows Hyper-V Firewall rules (WSL2 mirrored networking)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"windows"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Validate runs schema-level checks. VMCreatorID may be "auto" here
// (deferred to Run); the deeper checks happen after auto-resolution.
func (h *Handler) Validate(step *config.Step) error {
	f := step.WindowsHyperVFirewallRule
	if f == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	state := normalizeState(f.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("%s: state must be present or absent, got %q", actionName, f.State)
	}
	if strings.TrimSpace(f.Name) == "" {
		return fmt.Errorf("%s: name is required", actionName)
	}
	if state == statePresent {
		// Validate the would-be HyperVRule with a placeholder
		// VMCreatorID so "auto" doesn't trip the required-field
		// check at plan-validation time.
		r := toRule(f)
		if r.VMCreatorID == "" || strings.EqualFold(r.VMCreatorID, vmCreatorAuto) {
			r.VMCreatorID = winutil.WSLDefaultVMCreatorID
		}
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%s: %w", actionName, err)
		}
	}
	return nil
}

// Run is the canonical apply path. Resolves vm_creator_id (live
// discovery for "auto", literal GUID otherwise), then diffs and
// converges. Plan mode reports what *would* change without writing.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	f := step.WindowsHyperVFirewallRule
	result := executor.NewResult()
	result.Checkable = true
	result.Target = f.Name

	if runtime.GOOS != "windows" {
		return result, fmt.Errorf("%s: only Windows is supported; got %s", actionName, runtime.GOOS)
	}

	state := normalizeState(f.State)

	vmID, err := resolveVMCreatorID(f.VMCreatorID)
	if err != nil {
		return result, fmt.Errorf("%s: resolve vm_creator_id: %w", actionName, err)
	}

	current, err := queryRule(f.Name, vmID)
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
		if err := deleteRule(f.Name, vmID); err != nil {
			return result, err
		}
		result.Changed = true
		result.Reason = "removed rule " + f.Name
		return result, nil

	case statePresent:
		desired := toRule(f)
		desired.VMCreatorID = vmID
		if err := desired.Validate(); err != nil {
			return result, fmt.Errorf("%s: %w", actionName, err)
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
		// Apply. Like windows.firewall_rule, we delete+recreate on
		// drift rather than field-by-field Set-* gymnastics.
		if current != nil {
			if err := deleteRule(f.Name, vmID); err != nil {
				return result, err
			}
		}
		if err := createRule(desired); err != nil {
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

// resolveVMCreatorID returns a concrete GUID for the rule. Empty or
// "auto" triggers the resolver script; anything else is returned
// verbatim (after trimming).
func resolveVMCreatorID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, vmCreatorAuto) {
		script := winutil.RenderHyperVResolveVMCreator(wslMatchName, winutil.WSLDefaultVMCreatorID)
		out, err := runPS(script)
		if err != nil {
			return "", err
		}
		out = strings.TrimSpace(out)
		if out == "" {
			// Resolver guarantees the fallback fires when discovery
			// returns nothing, so an empty result means PowerShell
			// itself produced no output — treat as a hard error
			// rather than silently substituting.
			return "", fmt.Errorf("resolver returned empty output")
		}
		return out, nil
	}
	return raw, nil
}

// queryRule runs winutil.RenderHyperVQuery, parses the PSCustomObject
// summary, and returns it as an *observedRule (or nil when no rule
// exists for the (name, vmID) pair).
func queryRule(name, vmID string) (*observedRule, error) {
	script := "ConvertTo-Json -InputObject (" + winutil.RenderHyperVQuery(name, vmID) + ") -Compress"
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
		VMCreatorID string   `json:"VMCreatorID"`
		Direction   string   `json:"Direction"`
		Protocol    string   `json:"Protocol"`
		LocalPort   []string `json:"LocalPort"`
		RemotePort  []string `json:"RemotePort"`
		Action      string   `json:"Action"`
		Enabled     string   `json:"Enabled"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("decode rule json: %w (body: %q)", err, out)
	}
	return &observedRule{
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		VMCreatorID: raw.VMCreatorID,
		Direction:   strings.ToLower(raw.Direction),
		Protocol:    strings.ToLower(raw.Protocol),
		LocalPorts:  normaliseObservedPorts(raw.LocalPort),
		RemotePorts: normaliseObservedPorts(raw.RemotePort),
		Action:      strings.ToLower(raw.Action),
		Enabled:     strings.EqualFold(raw.Enabled, "True"),
	}, nil
}

func createRule(r winutil.HyperVRule) error {
	ps, err := winutil.RenderHyperVCreate(r)
	if err != nil {
		return err
	}
	if _, err := runPS(ps); err != nil {
		return fmt.Errorf("create rule: %w", err)
	}
	return nil
}

func deleteRule(name, vmID string) error {
	if _, err := runPS(winutil.RenderHyperVDelete(name, vmID)); err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	return nil
}

// observedRule is the parsed PSCustomObject Get-NetFirewallHyperVRule
// returns. Same shape as winutil.HyperVRule with already-lowercased
// strings for easy comparison.
type observedRule struct {
	DisplayName string
	Description string
	VMCreatorID string
	Direction   string
	Protocol    string
	LocalPorts  []string
	RemotePorts []string
	Action      string
	Enabled     bool
}

// Equals compares an observed rule against a desired winutil.HyperVRule.
// Returns true when every field aligns (after normalisation).
//
// VMCreatorID is compared case-insensitively because PowerShell's
// GUID-to-string round-trip can swap case on the curly braces.
func (o *observedRule) Equals(r winutil.HyperVRule) bool {
	if o.DisplayName != r.DisplayName {
		return false
	}
	// Description deliberately not compared — the Hyper-V cmdlet
	// doesn't accept -Description, so the rule never stores one;
	// comparing would force a perpetual "drift" loop.
	if !strings.EqualFold(o.VMCreatorID, r.VMCreatorID) {
		return false
	}
	if o.Direction != string(directionDefault(r.Direction)) {
		return false
	}
	if o.Protocol != protocolForCompare(r.Protocol) {
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

// toRule maps a config.WindowsHyperVFirewallRule (YAML-flavoured) to a
// winutil.HyperVRule. Defaults are NOT applied here so the Equals
// check can see "empty means default" symmetrically.
func toRule(f *config.WindowsHyperVFirewallRule) winutil.HyperVRule {
	return winutil.HyperVRule{
		DisplayName: f.Name,
		Description: f.Description,
		VMCreatorID: strings.TrimSpace(f.VMCreatorID),
		Direction:   winutil.Direction(strings.ToLower(f.Direction)),
		Protocol:    winutil.Protocol(strings.ToLower(f.Protocol)),
		LocalPorts:  append([]string(nil), f.LocalPort...),
		RemotePorts: append([]string(nil), f.RemotePort...),
		Action:      winutil.Action(strings.ToLower(f.Action)),
		Enabled:     f.Enabled,
	}
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

func protocolForCompare(p winutil.Protocol) string {
	if p == "" {
		return "tcp"
	}
	return strings.ToLower(string(p))
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

// realPSRun invokes powershell.exe with -EncodedCommand. Same shape
// as windows_firewall_rule.realPSRun: UTF-16LE base64 dodges every
// quoting hazard at the cmd.exe boundary, and WithUTF8Output prevents
// the OEM-codepage round-trip from corrupting non-ASCII output
// (issue #13).
func realPSRun(script string) (string, error) {
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
