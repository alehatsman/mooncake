package install

import (
	"strings"
	"testing"
)

// Spec-72 Phase 4 — pin the systemd-unit-template directives that
// affect escalation. The 2026-05-18 F051 incident (commit 600b619d)
// was rooted in NoNewPrivileges=true accidentally inheriting from
// the system unit into the user-mode template, which silently broke
// every sudo call from the user-mode agentd. These tests are the
// regression guard: a future "hardening tweak" that re-adds NNP to
// the user unit (or drops `Environment=PATH=` containing
// `%h/.local/bin:`, or User=root from the system unit) fails CI
// immediately.
//
// The matrix is the spec-72 §4 table. Each rendered template (system
// + user) is asserted against the directives it must contain and the
// directives it must not. "Allowed" directives — no escalation impact
// either way — aren't asserted; the goal is to pin the escalation-
// load-bearing directives, not freeze every byte of the template.

// renderSystemUnit returns the system-mode unit body with {{PORT}}
// substituted. Mirrors what Installer{OS:"linux", AsUser:false}.Render
// produces in production.
func renderSystemUnit(t *testing.T) string {
	t.Helper()
	body, err := (Installer{OS: "linux", Port: 8765}).Render()
	if err != nil {
		t.Fatalf("Render system unit: %v", err)
	}
	return string(body)
}

// renderUserUnit returns the user-mode unit body, same shape as
// Installer{OS:"linux", AsUser:true}.Render in production.
func renderUserUnit(t *testing.T) string {
	t.Helper()
	body, err := (Installer{OS: "linux", Port: 8765, AsUser: true}).Render()
	if err != nil {
		t.Fatalf("Render user unit: %v", err)
	}
	return string(body)
}

// containsDirective reports whether body contains the given directive
// substring outside of a comment context. Comments (lines starting
// with #) are stripped before the match so commentary about a
// directive (e.g. "# No NoNewPrivileges=") doesn't false-positive
// against a require-forbidden assertion.
func containsDirective(body, directive string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, directive) {
			return true
		}
	}
	return false
}

// TestUnitSecurity_SystemUnit_RequiredDirectives — every directive
// in this set MUST appear in the system unit. Removing any one breaks
// the spec-72 §4 escalation invariants.
func TestUnitSecurity_SystemUnit_RequiredDirectives(t *testing.T) {
	body := renderSystemUnit(t)
	required := []struct {
		directive string
		why       string
	}{
		{
			directive: "User=root",
			why:       "system-scope agentd runs as root; spec-72 §4 invariant",
		},
		{
			directive: "Group=root",
			why:       "system-scope agentd group; spec-72 §4 invariant",
		},
		{
			directive: "ReadWritePaths=/usr/local/bin",
			why:       "issue #19: punches the ProtectSystem hole `fleet upgrade` needs for the binary swap",
		},
		{
			directive: "WantedBy=multi-user.target",
			why:       "system-scope target; required so 'systemctl enable mooncake-agentd' wires the unit to boot",
		},
		{
			directive: "--system",
			why:       "system-scope ExecStart must pass --system so agentd picks the system paths (token at /etc/mooncake/agentd.token, etc.)",
		},
	}
	for _, r := range required {
		if !containsDirective(body, r.directive) {
			t.Errorf("system unit missing required directive %q\n  why: %s\n  rendered body:\n%s",
				r.directive, r.why, body)
		}
	}
}

// TestUnitSecurity_UserUnit_RequiredDirectives — the user unit MUST
// contain these. PATH= with %h/.local/bin is the F051-d regression
// guard; WantedBy=default.target is the user-scope target; the
// ExecStart shape pins the binary location.
func TestUnitSecurity_UserUnit_RequiredDirectives(t *testing.T) {
	body := renderUserUnit(t)
	required := []struct {
		directive string
		why       string
	}{
		{
			directive: "Environment=PATH=",
			why:       "F051-d: user-mode shell-action steps need %h/.local/bin on PATH so `claude`, `mcsearch`, etc. resolve (bash -c non-interactive doesn't source ~/.bashrc)",
		},
		{
			directive: "%h/.local/bin:",
			why:       "F051-d: PATH must specifically prepend the user's local bin; otherwise user-installed tools (the whole point of user-mode agentd) don't resolve",
		},
		{
			directive: "WantedBy=default.target",
			why:       "user-scope target; required so 'systemctl --user enable mooncake-agentd' wires the unit to login",
		},
		{
			directive: "ExecStart=%h/.local/bin/mooncake",
			why:       "binary lives in the user's local bin under user-mode; baking absolute /home/<user>/... would break cross-machine reuse",
		},
	}
	for _, r := range required {
		if !containsDirective(body, r.directive) {
			t.Errorf("user unit missing required directive %q\n  why: %s\n  rendered body:\n%s",
				r.directive, r.why, body)
		}
	}
}

// TestUnitSecurity_UserUnit_ForbiddenDirectives — the user unit MUST
// NOT contain these. NoNewPrivileges=true and RestrictSUIDSGID=true
// both block sudo (setuid execution), which the user-mode agentd
// relies on for every `as_user: root` step. Re-adding either silently
// breaks the entire user-mode escalation story — exactly the F051-b
// regression we shipped 600b619d to fix.
func TestUnitSecurity_UserUnit_ForbiddenDirectives(t *testing.T) {
	body := renderUserUnit(t)
	forbidden := []struct {
		directive string
		why       string
	}{
		{
			directive: "NoNewPrivileges=true",
			why:       "F051-b: NNP blocks setuid execution → sudo refuses with 'The no new privileges flag is set'. Every as_user:root step from user-mode agentd would break.",
		},
		{
			directive: "RestrictSUIDSGID=true",
			why:       "Same blast radius as NNP: blocks sudo (a setuid binary) regardless of NOPASSWD / SudoPass configuration",
		},
		{
			directive: "CapabilityBoundingSet=",
			why:       "An overly-restrictive bounding set (one not including CAP_SETUID + CAP_SETGID) also breaks sudo. We don't allow ANY bounding set on user-mode rather than parsing the value, since the user-mode agentd has no operational reason to constrain capabilities.",
		},
		{
			directive: "WantedBy=multi-user.target",
			why:       "system-scope target; user-mode units must use default.target instead. Mixing them prevents 'systemctl --user enable' from wiring correctly.",
		},
		{
			directive: "User=root",
			why:       "user units run as the owning user automatically — setting User=root inside a user-scope unit is either ignored or rejected by systemd, and signals confusion about scope",
		},
		{
			directive: "Group=root",
			why:       "same as User=root — user-scope units inherit the owning user/group",
		},
		{
			directive: "ReadWritePaths=/usr/local/bin",
			why:       "user-mode agentd doesn't write to /usr/local/bin; the binary lives in ~/.local/bin and the user already owns it. Carrying the system unit's hole here is confused-scope.",
		},
		{
			directive: "--system",
			why:       "user-mode ExecStart must NOT pass --system; that flag pins agentd to /etc/mooncake/agentd.token + 0.0.0.0 bind defaults that the user has no write permission for",
		},
	}
	for _, f := range forbidden {
		if containsDirective(body, f.directive) {
			t.Errorf("user unit contains forbidden directive %q\n  why: %s\n  rendered body:\n%s",
				f.directive, f.why, body)
		}
	}
}

// TestUnitSecurity_SystemUnit_NNPAllowed pins the converse: the
// system unit can keep NoNewPrivileges=true because root never needs
// to escalate (already root). This isn't a load-bearing assertion
// per se, but it documents the spec-72 §4 asymmetry — if someone
// proposes removing NNP from the system unit "for consistency", this
// test surfaces the rationale.
func TestUnitSecurity_SystemUnit_NNPAllowed(t *testing.T) {
	body := renderSystemUnit(t)
	if !containsDirective(body, "NoNewPrivileges=true") {
		t.Skip("NNP not currently set on the system unit; spec-72 §4 allows but doesn't require it. Skipping informational guard.")
	}
}

// TestUnitSecurity_PortSubstitution sanity-checks that the {{PORT}}
// placeholder is gone after rendering. Tangential to the escalation
// matrix but every other test in this file depends on the renderer
// having run; locking in this contract here means a renderer
// regression surfaces before any escalation-directive assertion fires.
func TestUnitSecurity_PortSubstitution(t *testing.T) {
	for name, body := range map[string]string{
		"system": renderSystemUnit(t),
		"user":   renderUserUnit(t),
	} {
		if strings.Contains(body, "{{PORT}}") {
			t.Errorf("%s unit contains unsubstituted {{PORT}} placeholder", name)
		}
		if !strings.Contains(body, "8765") {
			t.Errorf("%s unit missing the test port 8765 (renderer didn't substitute)", name)
		}
	}
}
