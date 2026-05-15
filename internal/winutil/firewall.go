package winutil

// firewall.go renders the PowerShell calls for inbound/outbound TCP/UDP
// Windows Firewall rules: create, query, update, delete. Identity is
// always the rule's DisplayName — the same convention dotfiles and
// every PowerShell-driven installer uses. Numeric IDs (the `Name`
// property on a rule, auto-assigned by Windows) are unstable across
// Windows reinstalls and AD policy refreshes, so we never write them.
//
// Rules are described by a Go struct, validated, then rendered into
// the appropriate command string. We render commands rather than build
// an entire PowerShell module so callers can decide how to ship them:
// in-process via `powershell.exe -EncodedCommand <base64>` (the
// action-handler path) or over SSH to a remote box (the fleet-bootstrap
// path). The renderers themselves are pure, deterministic, and tested
// without touching PowerShell.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Direction is "inbound" or "outbound". inbound is by far the common
// case for the personal-fleet use case spec-57 targets; outbound is
// supported for completeness.
type Direction string

const (
	DirInbound  Direction = "inbound"
	DirOutbound Direction = "outbound"
)

// Protocol covers the protocols the spec promises support for. "any"
// maps to leaving -Protocol off the cmdlet — that's how PowerShell
// expresses "match all protocols" without inventing a sentinel.
type Protocol string

const (
	ProtoTCP    Protocol = "tcp"
	ProtoUDP    Protocol = "udp"
	ProtoICMPv4 Protocol = "icmpv4"
	ProtoICMPv6 Protocol = "icmpv6"
	ProtoAny    Protocol = "any"
)

// Action is the firewall verdict: Allow or Block. We intentionally
// don't expose the "Bypass" / "Block-by-IPSec" variants; they're for
// AD-managed scenarios we're explicitly out-of-scope for.
type Action string

const (
	ActionAllow Action = "allow"
	ActionBlock Action = "block"
)

// Profile is one of the network-location profiles a rule applies to.
// "any" maps to all three of Domain/Private/Public.
type Profile string

const (
	ProfDomain  Profile = "domain"
	ProfPrivate Profile = "private"
	ProfPublic  Profile = "public"
	ProfAny     Profile = "any"
)

// Rule is the declarative spec of a Windows Firewall rule. All fields
// have defaults that match the cmdlet's defaults; only DisplayName is
// strictly required.
type Rule struct {
	// DisplayName is the human-readable identity used for create /
	// query / update / delete. Required and unique within the
	// firewall ruleset (Windows allows duplicates but we forbid them
	// by treating DisplayName as the key).
	DisplayName string

	// Description is optional free-form metadata stored alongside the
	// rule. Useful for "why does this exist" notes.
	Description string

	// Direction defaults to DirInbound when empty.
	Direction Direction

	// Protocol defaults to ProtoTCP when empty. For ICMP variants,
	// port fields are ignored.
	Protocol Protocol

	// LocalPorts is the port(s) the rule applies to on the local
	// side. Empty slice = "any". Each entry is either a decimal port
	// number ("7878") or a range ("7878-7888"). The cmdlet accepts
	// "any" verbatim but we represent that as an empty slice.
	LocalPorts []string

	// RemotePorts is the analogue for remote ports. Same shape as
	// LocalPorts; empty = "any".
	RemotePorts []string

	// Action defaults to ActionAllow.
	Action Action

	// Profiles defaults to [ProfDomain, ProfPrivate] when empty —
	// safer than the cmdlet's default of ProfAny because the
	// personal-fleet use case rarely wants to expose ports on a
	// public-network profile (coffee shop / hotel wifi).
	Profiles []Profile

	// Enabled defaults to true. Set to false to register a rule that
	// stays present-but-inactive — useful for staging a config that
	// gets switched on later.
	Enabled *bool
}

// Validate returns the first error in r, or nil. Called by both the
// command renderers (so an invalid rule fails fast before PowerShell
// sees it) and the action handler's Validate hook.
func (r Rule) Validate() error {
	if strings.TrimSpace(r.DisplayName) == "" {
		return errors.New("DisplayName is required")
	}
	if strings.ContainsAny(r.DisplayName, `"'`) {
		// PowerShell's -DisplayName accepts quoted strings, but
		// embedding quote characters in the name itself defeats
		// our own idempotency lookup (-DisplayName "foo\"bar")
		// and most monitoring tools don't handle it gracefully.
		return fmt.Errorf("DisplayName %q must not contain quote characters", r.DisplayName)
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
	for _, p := range r.Profiles {
		switch p {
		case ProfDomain, ProfPrivate, ProfPublic, ProfAny:
		default:
			return fmt.Errorf("invalid Profile %q", p)
		}
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
	// ICMP rules with port specs are nonsense — surface that.
	if (r.Protocol == ProtoICMPv4 || r.Protocol == ProtoICMPv6) &&
		(len(r.LocalPorts) > 0 || len(r.RemotePorts) > 0) {
		return fmt.Errorf("Protocol %s does not take ports", r.Protocol)
	}
	return nil
}

// validatePortSpec checks one entry of LocalPorts/RemotePorts: a port
// number ("7878"), a range ("7878-7888"), or the literal "any". The
// cmdlet would error on bad input but the error message is opaque;
// catching it here makes the failure actionable.
func validatePortSpec(s string) error {
	if s == "any" || s == "Any" {
		return nil
	}
	if dash := strings.IndexByte(s, '-'); dash >= 0 {
		lo, hi := strings.TrimSpace(s[:dash]), strings.TrimSpace(s[dash+1:])
		if err := validateSinglePort(lo); err != nil {
			return fmt.Errorf("range start: %w", err)
		}
		if err := validateSinglePort(hi); err != nil {
			return fmt.Errorf("range end: %w", err)
		}
		return nil
	}
	return validateSinglePort(s)
}

func validateSinglePort(s string) error {
	if s == "" {
		return errors.New("empty port")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return fmt.Errorf("non-digit %q", c)
		}
	}
	if len(s) > 5 {
		return fmt.Errorf("port %s out of range", s)
	}
	return nil
}

// withDefaults returns a copy of r with empty fields filled with the
// package's defaults. Centralises the "what does empty mean" rules so
// the renderers stay one-line-per-arg.
func (r Rule) withDefaults() Rule {
	if r.Direction == "" {
		r.Direction = DirInbound
	}
	if r.Protocol == "" {
		r.Protocol = ProtoTCP
	}
	if r.Action == "" {
		r.Action = ActionAllow
	}
	if len(r.Profiles) == 0 {
		r.Profiles = []Profile{ProfDomain, ProfPrivate}
	}
	if r.Enabled == nil {
		t := true
		r.Enabled = &t
	}
	return r
}

// RenderCreate returns the PowerShell that creates the rule. The
// caller wraps this in a `if -not (Get-NetFirewallRule …) { … }` guard
// if they need idempotency — see RenderEnsure for the all-in-one.
func RenderCreate(r Rule) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	r = r.withDefaults()
	var b strings.Builder
	b.WriteString("New-NetFirewallRule")
	appendStringArg(&b, "DisplayName", r.DisplayName)
	if r.Description != "" {
		appendStringArg(&b, "Description", r.Description)
	}
	appendBareArg(&b, "Direction", titleCase(string(r.Direction)))
	if r.Protocol != ProtoAny {
		appendBareArg(&b, "Protocol", protocolPSValue(r.Protocol))
	}
	if len(r.LocalPorts) > 0 {
		appendPortArg(&b, "LocalPort", r.LocalPorts)
	}
	if len(r.RemotePorts) > 0 {
		appendPortArg(&b, "RemotePort", r.RemotePorts)
	}
	appendBareArg(&b, "Action", titleCase(string(r.Action)))
	appendBareArg(&b, "Profile", profilesPSValue(r.Profiles))
	if !*r.Enabled {
		// -Enabled is a *string* enum on the cmdlet (True / False),
		// not a switch. Only emit when "False" — the default is True.
		appendBareArg(&b, "Enabled", "False")
	}
	b.WriteString(" | Out-Null")
	return b.String(), nil
}

// RenderQuery returns a PowerShell expression that yields a
// PSCustomObject summarising the named rule, or $null if none exists.
// Designed to be eval'd into a variable and then inspected:
//
//	$rule = (<RenderQuery output>)
//	if ($rule -eq $null) { ... }
//
// The summary contains exactly the fields that Rule can describe, so a
// consumer can compare against a desired Rule for drift detection.
func RenderQuery(displayName string) string {
	// PowerShell calls split across pipeline stages get awkward to
	// build inline; this is rendered as a single expression for the
	// caller's convenience.
	return `(Get-NetFirewallRule -DisplayName ` + psStringLiteral(displayName) +
		` -ErrorAction SilentlyContinue | Select-Object -First 1 | ForEach-Object {` +
		` $portFilter = $_ | Get-NetFirewallPortFilter;` +
		` [pscustomobject]@{` +
		` DisplayName  = $_.DisplayName;` +
		` Description  = $_.Description;` +
		` Direction    = "$($_.Direction)";` +
		` Protocol     = "$($portFilter.Protocol)";` +
		` LocalPort    = @($portFilter.LocalPort);` +
		` RemotePort   = @($portFilter.RemotePort);` +
		` Action       = "$($_.Action)";` +
		` Profile      = "$($_.Profile)";` +
		` Enabled      = "$($_.Enabled)"` +
		` }})`
}

// RenderDelete returns the PowerShell to remove the named rule.
// Non-existent name is silently ignored (use RenderQuery if you need
// to assert presence first).
func RenderDelete(displayName string) string {
	return "Remove-NetFirewallRule -DisplayName " + psStringLiteral(displayName) +
		" -ErrorAction SilentlyContinue | Out-Null"
}

// RenderEnsure is the idempotent all-in-one: if no rule with this
// DisplayName exists, create it; otherwise no-op. The compare-and-
// update path (drift correction) is handled by the action's Run()
// using RenderQuery output, not this function — keeping ensure simple
// makes the bootstrap-time call (where we don't care about drift
// because nothing existed) trivial.
//
// Returns a self-contained PowerShell statement suitable for
// `powershell.exe -EncodedCommand`.
func RenderEnsure(r Rule) (string, error) {
	create, err := RenderCreate(r)
	if err != nil {
		return "", err
	}
	return "if (-not (Get-NetFirewallRule -DisplayName " + psStringLiteral(r.DisplayName) +
		" -ErrorAction SilentlyContinue)) { " + create + " }", nil
}

// ----- low-level renderers ---------------------------------------------------

// appendStringArg writes ` -Name "value"` (with PowerShell escaping).
func appendStringArg(b *strings.Builder, name, value string) {
	b.WriteString(" -")
	b.WriteString(name)
	b.WriteByte(' ')
	b.WriteString(psStringLiteral(value))
}

// appendBareArg writes ` -Name Value` for enum-style PS arguments
// (Direction, Action, Protocol, ...). Value is assumed to be a
// PowerShell-safe identifier — only used for our own enumerated
// constants, never user-supplied text.
func appendBareArg(b *strings.Builder, name, value string) {
	b.WriteString(" -")
	b.WriteString(name)
	b.WriteByte(' ')
	b.WriteString(value)
}

// appendPortArg writes ` -LocalPort 7878,8000-8010,...` — comma-joined
// list of validated port specs. PowerShell auto-coerces these to int
// or string based on context.
func appendPortArg(b *strings.Builder, name string, ports []string) {
	b.WriteString(" -")
	b.WriteString(name)
	b.WriteByte(' ')
	// Sort for deterministic output (test stability + diff stability).
	sorted := append([]string(nil), ports...)
	sort.Strings(sorted)
	for i, p := range sorted {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(p)
	}
}

// psStringLiteral wraps a Go string in PowerShell single quotes,
// doubling any embedded single quotes per PowerShell's literal-string
// rules. Single quotes prevent variable expansion ($foo stays literal)
// which is what we want for rule names + descriptions.
func psStringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// titleCase capitalises the first letter — the cmdlets use PascalCase
// for their enum arguments (Inbound, Outbound, Allow, Block, Domain).
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// protocolPSValue maps our lowercase enum to the cmdlet's case-
// sensitive value. Most PS cmdlets are case-insensitive but
// New-NetFirewallRule has historically been strict here.
func protocolPSValue(p Protocol) string {
	switch p {
	case ProtoTCP:
		return "TCP"
	case ProtoUDP:
		return "UDP"
	case ProtoICMPv4:
		return "ICMPv4"
	case ProtoICMPv6:
		return "ICMPv6"
	}
	return string(p)
}

// profilesPSValue renders a profile list as a comma-joined value
// (which PowerShell accepts for -Profile). Any in the list short-
// circuits to "Any" since that subsumes the others.
func profilesPSValue(profs []Profile) string {
	for _, p := range profs {
		if p == ProfAny {
			return "Any"
		}
	}
	// Sort for deterministic output. The cmdlet doesn't care, but
	// our tests do, and so do diff readers.
	names := make([]string, 0, len(profs))
	for _, p := range profs {
		names = append(names, titleCase(string(p)))
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}
