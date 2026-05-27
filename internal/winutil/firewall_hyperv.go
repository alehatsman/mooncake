package winutil

// firewall_hyperv.go is the parallel-track renderer for Windows
// Hyper-V Firewall rules, which sit on a separate stack from the
// regular Windows Firewall and gate WSL2's mirrored-mode networking.
// Same shape as firewall.go but with two key differences:
//
//   - Rules are scoped by VMCreatorID (a GUID identifying the Hyper-V
//     VM "subscriber") instead of by Profile. There's no notion of
//     Domain/Private/Public here — the Hyper-V firewall is per-VM.
//   - The cmdlets are New-NetFirewallHyperVRule /
//     Get-NetFirewallHyperVRule / Remove-NetFirewallHyperVRule, which
//     don't accept -Profile and require -VMCreatorID on every call.
//
// Proposal-19 (WSL2 mirrored-mode) explains the motivation; this file
// implements the rendering half. Auto-discovery of WSL's VMCreatorID
// lives in the action handler so the resolver can shell out before
// command construction.

import (
	"errors"
	"fmt"
	"strings"
)

// WSLDefaultVMCreatorID is the well-known fallback GUID for WSL's
// Hyper-V subscription, baked in for fresh boxes that haven't booted
// a WSL VM yet (in which case Get-NetFirewallHyperVVMCreator returns
// nothing). When `vm_creator_id: auto` resolves to empty, callers
// substitute this constant.
const WSLDefaultVMCreatorID = "{40E0AC32-46A5-438A-A0B2-2B479E8F2E90}"

// HyperVRule is the declarative spec of a Windows Hyper-V Firewall
// rule. Mirrors Rule (regular firewall) minus Profile, plus the
// required VMCreatorID.
type HyperVRule struct {
	// DisplayName is the human-readable identity used for create /
	// query / update / delete. Required and unique within the scope
	// of (VMCreatorID, DisplayName) — Windows allows duplicates but
	// we forbid them by treating the pair as the key.
	DisplayName string

	// Description is YAML-only metadata for the human reader.
	// New-NetFirewallHyperVRule (unlike New-NetFirewallRule) does NOT
	// expose a -Description parameter, so this field is intentionally
	// not rendered into the cmdlet call and not compared against the
	// observed rule. It exists purely so the YAML stays self-
	// documenting alongside the regular windows.firewall_rule block.
	Description string

	// VMCreatorID is the GUID of the Hyper-V VM subscriber the rule
	// scopes to. Required at render time; the handler resolves
	// `vm_creator_id: auto` before calling in, so this field always
	// holds a concrete GUID by the time RenderHyperV* sees it.
	VMCreatorID string

	// Direction defaults to DirInbound when empty.
	Direction Direction

	// Protocol defaults to ProtoTCP when empty. For ICMP variants,
	// port fields are ignored.
	Protocol Protocol

	// LocalPorts is the port(s) the rule applies to on the local
	// side. Empty slice = "any". Same shape as Rule.LocalPorts.
	LocalPorts []string

	// RemotePorts is the analogue for remote ports. Empty = "any".
	RemotePorts []string

	// Action defaults to ActionAllow.
	Action Action

	// Enabled defaults to true. Set to false to register a rule that
	// stays present-but-inactive.
	Enabled *bool
}

// Validate returns the first error in r, or nil. Same shape as
// Rule.Validate but with the VMCreatorID requirement and no Profile
// check.
func (r HyperVRule) Validate() error {
	if strings.TrimSpace(r.DisplayName) == "" {
		return errors.New("DisplayName is required")
	}
	if strings.ContainsAny(r.DisplayName, `"'`) {
		return fmt.Errorf("DisplayName %q must not contain quote characters", r.DisplayName)
	}
	if strings.TrimSpace(r.VMCreatorID) == "" {
		return errors.New("VMCreatorID is required")
	}
	if strings.ContainsAny(r.VMCreatorID, `"'`) {
		return fmt.Errorf("VMCreatorID %q must not contain quote characters", r.VMCreatorID)
	}
	switch r.Direction {
	case "", DirInbound, DirOutbound:
	default:
		return fmt.Errorf("invalid Direction %q", r.Direction)
	}
	switch r.Protocol {
	case "", ProtoTCP, ProtoUDP, ProtoICMPv4, ProtoICMPv6, ProtoAny:
	default:
		return fmt.Errorf("invalid Protocol %q", r.Protocol)
	}
	switch r.Action {
	case "", ActionAllow, ActionBlock:
	default:
		return fmt.Errorf("invalid Action %q", r.Action)
	}
	for _, port := range r.LocalPorts {
		if err := validatePortSpec(port); err != nil {
			return fmt.Errorf("LocalPorts[%q]: %w", port, err)
		}
	}
	for _, port := range r.RemotePorts {
		if err := validatePortSpec(port); err != nil {
			return fmt.Errorf("RemotePorts[%q]: %w", port, err)
		}
	}
	if (r.Protocol == ProtoICMPv4 || r.Protocol == ProtoICMPv6) &&
		(len(r.LocalPorts) > 0 || len(r.RemotePorts) > 0) {
		return fmt.Errorf("Protocol %s does not take ports", r.Protocol)
	}
	return nil
}

// withDefaults returns a copy of r with empty fields filled. Centralises
// the "empty means default" rules so the renderers stay one-line-per-arg.
func (r HyperVRule) withDefaults() HyperVRule {
	if r.Direction == "" {
		r.Direction = DirInbound
	}
	if r.Protocol == "" {
		r.Protocol = ProtoTCP
	}
	if r.Action == "" {
		r.Action = ActionAllow
	}
	if r.Enabled == nil {
		t := true
		r.Enabled = &t
	}
	return r
}

// RenderHyperVCreate returns the PowerShell that creates the rule.
// Caller is responsible for the existence guard — see
// RenderHyperVEnsure for the all-in-one.
func RenderHyperVCreate(r HyperVRule) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	r = r.withDefaults()
	var b strings.Builder
	b.WriteString("New-NetFirewallHyperVRule")
	appendStringArg(&b, "DisplayName", r.DisplayName)
	// Description intentionally omitted — see HyperVRule.Description.
	appendStringArg(&b, "VMCreatorID", r.VMCreatorID)
	appendBareArg(&b, "Direction", titleCase(string(r.Direction)))
	if r.Protocol != ProtoAny {
		appendBareArg(&b, "Protocol", protocolPSValue(r.Protocol))
	}
	if len(r.LocalPorts) > 0 {
		appendPortArg(&b, "LocalPorts", r.LocalPorts)
	}
	if len(r.RemotePorts) > 0 {
		appendPortArg(&b, "RemotePorts", r.RemotePorts)
	}
	appendBareArg(&b, "Action", titleCase(string(r.Action)))
	if !*r.Enabled {
		appendBareArg(&b, "Enabled", "False")
	}
	b.WriteString(" | Out-Null")
	return b.String(), nil
}

// RenderHyperVQuery returns a PowerShell expression yielding a
// PSCustomObject summarising the named rule under the given
// VMCreatorID, or $null if none exists. Same shape as RenderQuery.
//
// Get-NetFirewallHyperVRule requires -VMCreatorID; we filter by
// DisplayName after pulling the VM's full ruleset because the cmdlet
// doesn't take -DisplayName directly the way Get-NetFirewallRule does.
func RenderHyperVQuery(displayName, vmCreatorID string) string {
	return `(Get-NetFirewallHyperVRule -VMCreatorID ` + psStringLiteral(vmCreatorID) +
		` -ErrorAction SilentlyContinue |` +
		` Where-Object { $_.DisplayName -eq ` + psStringLiteral(displayName) + ` } |` +
		` Select-Object -First 1 | ForEach-Object {` +
		` [pscustomobject]@{` +
		` DisplayName  = $_.DisplayName;` +
		` Description  = $_.Description;` +
		` VMCreatorID  = "$($_.VMCreatorID)";` +
		` Direction    = "$($_.Direction)";` +
		` Protocol     = "$($_.Protocol)";` +
		` LocalPort    = @($_.LocalPorts);` +
		` RemotePort   = @($_.RemotePorts);` +
		` Action       = "$($_.Action)";` +
		` Enabled      = "$($_.Enabled)"` +
		` }})`
}

// RenderHyperVDelete returns the PowerShell to remove the named rule
// scoped to vmCreatorID. Non-existent name is silently ignored.
func RenderHyperVDelete(displayName, vmCreatorID string) string {
	return "Get-NetFirewallHyperVRule -VMCreatorID " + psStringLiteral(vmCreatorID) +
		" -ErrorAction SilentlyContinue |" +
		" Where-Object { $_.DisplayName -eq " + psStringLiteral(displayName) + " } |" +
		" Remove-NetFirewallHyperVRule -ErrorAction SilentlyContinue | Out-Null"
}

// RenderHyperVEnsure is the idempotent all-in-one: if no rule with
// this (VMCreatorID, DisplayName) exists, create it; otherwise no-op.
// Mirrors RenderEnsure for the regular firewall.
func RenderHyperVEnsure(r HyperVRule) (string, error) {
	create, err := RenderHyperVCreate(r)
	if err != nil {
		return "", err
	}
	exists := "(Get-NetFirewallHyperVRule -VMCreatorID " + psStringLiteral(r.VMCreatorID) +
		" -ErrorAction SilentlyContinue |" +
		" Where-Object { $_.DisplayName -eq " + psStringLiteral(r.DisplayName) + " })"
	return "if (-not " + exists + ") { " + create + " }", nil
}

// RenderHyperVResolveVMCreator returns a PowerShell expression
// yielding the WSL Hyper-V VMCreatorID GUID, with the documented
// fallback constant when the live discovery returns nothing.
//
// The script emits exactly one line: the GUID (no quoting, no
// surrounding whitespace), so the caller can trim and use it as-is
// in subsequent commands.
//
// match is the substring matched against Get-NetFirewallHyperVVMCreator's
// Name field — typically "WSL". fallback is the GUID returned when
// no match is found.
func RenderHyperVResolveVMCreator(match, fallback string) string {
	return `$wslId = (Get-NetFirewallHyperVVMCreator -ErrorAction SilentlyContinue |` +
		` Where-Object { $_.Name -match ` + psStringLiteral(match) + ` } |` +
		` Select-Object -First 1 -ExpandProperty VMCreatorID);` +
		` if (-not $wslId) { $wslId = ` + psStringLiteral(fallback) + ` };` +
		` $wslId`
}
