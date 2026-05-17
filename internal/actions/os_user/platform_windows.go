//go:build windows

package os_user //nolint:revive // package name follows action convention

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

func init() {
	lookupUser = lookupUserViaPowerShell
	applyPlan = applyPlanWindows
}

// Windows impedance notes
//
// Windows local accounts diverge from Unix in several places where
// os.user's config shape leaks Unix assumptions. The mapping rules
// below keep cross-platform plans portable — a plan that pins
// `uid: 1234` for linux works unchanged on windows; the field is
// quietly ignored with a debug log. The kernel-honest exception is
// `shell:` — Windows has no per-user shell concept; we treat any
// non-empty Shell as a no-op rather than a refusal to keep plans
// authored on Linux portable.
//
// Field-by-field mapping:
//   - name        ↔ -Name (PowerShell Get-LocalUser / New-LocalUser)
//   - comment     ↔ -FullName (closest match; -Description is also
//                   exposed but our config has one comment field)
//   - home        ↔ -HomeDirectory (set at create time; Windows
//                   creates the dir on first logon, not when we
//                   ask)
//   - groups      ↔ Add/Remove-LocalGroupMember loop
//   - uid         ✗ Windows uses SIDs, not numeric UIDs; ignored
//                   with a debug log so cross-platform plans don't
//                   break.
//   - gid / group ✗ Windows has no primary-group concept on local
//                   accounts; ignored with debug log.
//   - shell       ✗ Windows has no per-user shell; ignored.
//   - system      ✗ No analog. Windows distinguishes service vs
//                   user via group membership / SeServiceLogonRight,
//                   not via account type.
//   - createHome  Windows creates %USERPROFILE% on first interactive
//                   logon; the flag is ignored.
//   - removeHome  Honored: Remove-LocalUser removes the SAM record;
//                   we also delete %USERPROFILE% via Remove-Item
//                   when removeHome is true.
//
// Password: New-LocalUser requires either -Password or -NoPassword.
// We default to -NoPassword. Service / deploy accounts typically
// have their credentials managed out-of-band (managed-service
// accounts, scheduled-task -RunAs, etc.). Operators who need a
// password should follow up with a `command:` step that runs
// `net user <name> <password>` after this action.

func lookupUserViaPowerShell(name string) (*userState, error) {
	// Get-LocalUser -ErrorAction SilentlyContinue: exits 0 with
	// empty stdout when the user doesn't exist; emits JSON when
	// present. Avoids parsing stderr to disambiguate "missing"
	// from real errors.
	script := fmt.Sprintf(
		"$u = Get-LocalUser -Name %s -ErrorAction SilentlyContinue; "+
			"if ($u) { $u | Select-Object Name,FullName,Description,Enabled | ConvertTo-Json -Compress }",
		quotePS(name),
	)
	out, err := runPowerShellCapture(script)
	if err != nil {
		return nil, fmt.Errorf("powershell Get-LocalUser: %w", err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return &userState{exists: false}, nil
	}

	var record struct {
		Name        string
		FullName    string
		Description string
		Enabled     bool
	}
	if err := json.Unmarshal([]byte(trimmed), &record); err != nil {
		return nil, fmt.Errorf("parse Get-LocalUser output: %w (raw: %q)", err, trimmed)
	}

	state := &userState{
		exists:  true,
		comment: record.FullName,
		// uid / gid / shell stay zero: Windows has no analog.
	}

	groups, err := listUserLocalGroups(name)
	if err != nil {
		return nil, fmt.Errorf("list local groups for %s: %w", name, err)
	}
	if len(groups) > 0 {
		sort.Strings(groups)
		state.groups = groups
	}
	return state, nil
}

// listUserLocalGroups walks every local group and asks whether the
// user is a member. PowerShell has no `Get-UserLocalGroups` cmdlet
// — this is the documented workaround. On a typical Windows host
// there are ~20 local groups, so the per-group cost is acceptable.
//
// Membership name comparison handles both "BUILTIN\Administrators"
// and "MACHINE\user" shapes by suffix-matching the bare username.
// Domain accounts aren't supported by `os.user` (local accounts
// only); this matcher is correct for that scope.
func listUserLocalGroups(name string) ([]string, error) {
	// Group names can contain spaces ("Hyper-V Administrators",
	// "Backup Operators") so we join with a vertical bar that's
	// not valid in a Windows local-group name. Avoids needing
	// PowerShell's backtick newline escape (which can't appear
	// inside a Go raw-string literal).
	const sep = "|"
	scriptTmpl := "$user = %s\n" +
		"$out = @()\n" +
		"Get-LocalGroup | ForEach-Object {\n" +
		"    $g = $_.Name\n" +
		"    try {\n" +
		"        $members = Get-LocalGroupMember -Group $g -ErrorAction Stop\n" +
		"        foreach ($m in $members) {\n" +
		"            $bare = ($m.Name -split '\\\\')[-1]\n" +
		"            if ($bare -ieq $user) { $out += $g; break }\n" +
		"        }\n" +
		"    } catch {\n" +
		"        # Some groups (e.g. those with stale SID references) error on enumeration; skip.\n" +
		"    }\n" +
		"}\n" +
		"$out -join '" + sep + "'\n"
	script := fmt.Sprintf(scriptTmpl, quotePS(name))
	out, err := runPowerShellCapture(script)
	if err != nil {
		return nil, err
	}
	var groups []string
	for _, g := range strings.Split(strings.TrimSpace(out), sep) {
		g = strings.TrimSpace(g)
		if g != "" {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

func applyPlanWindows(plan computedPlan, current *userState, d desired) error {
	switch plan.operation {
	case "create":
		return createUserWindows(d)
	case "modify":
		return modifyUserWindows(current, d)
	case "remove":
		return removeUserWindows(d)
	}
	return nil
}

func createUserWindows(d desired) error {
	args := []string{
		"-Name", quotePS(d.name),
		"-NoPassword",
		"-AccountNeverExpires",
	}
	if d.comment != "" {
		args = append(args, "-FullName", quotePS(d.comment))
	}
	if d.home != "" {
		args = append(args, "-HomeDirectory", quotePS(d.home))
	}
	script := "New-LocalUser " + strings.Join(args, " ")
	if err := runPowerShell(script); err != nil {
		return fmt.Errorf("New-LocalUser %s: %w", d.name, err)
	}
	if err := applyGroupsWindows(d.name, nil, d.groups, d.appendGroups); err != nil {
		return err
	}
	return nil
}

func modifyUserWindows(current *userState, d desired) error {
	setArgs := []string{"-Name", quotePS(d.name)}
	changed := false
	if d.comment != "" && d.comment != current.comment {
		setArgs = append(setArgs, "-FullName", quotePS(d.comment))
		changed = true
	}
	// Home directory after create is best-effort: Set-LocalUser
	// supports -HomeDirectory on PowerShell 5.1+. Operators who
	// hit older PS surfaces should follow up with a `command:`
	// step running `net user <name> /homedir:<path>`.
	if d.home != "" && d.home != current.home {
		setArgs = append(setArgs, "-HomeDirectory", quotePS(d.home))
		changed = true
	}
	if changed {
		script := "Set-LocalUser " + strings.Join(setArgs, " ")
		if err := runPowerShell(script); err != nil {
			return fmt.Errorf("Set-LocalUser %s: %w", d.name, err)
		}
	}
	if len(d.groups) > 0 {
		if err := applyGroupsWindows(d.name, current.groups, d.groups, d.appendGroups); err != nil {
			return err
		}
	}
	return nil
}

func removeUserWindows(d desired) error {
	script := fmt.Sprintf("Remove-LocalUser -Name %s", quotePS(d.name))
	if err := runPowerShell(script); err != nil {
		return fmt.Errorf("Remove-LocalUser %s: %w", d.name, err)
	}
	if d.removeHome {
		// Best-effort: %USERPROFILE% under C:\Users\<name> is the
		// convention but the actual path is recorded in the
		// pre-removal SAM record (already gone by here). We delete
		// the conventional path; absent dir is silently OK.
		conventional := fmt.Sprintf("C:\\Users\\%s", d.name)
		script := fmt.Sprintf(
			"if (Test-Path %s) { Remove-Item -Recurse -Force %s }",
			quotePS(conventional), quotePS(conventional),
		)
		if err := runPowerShell(script); err != nil {
			return fmt.Errorf("Remove-Item %s: %w", conventional, err)
		}
	}
	return nil
}

// applyGroupsWindows reconciles supplementary group membership.
// Symmetric with applyGroupsDarwin: when appendGroups is false,
// drop groups not in desired first, then add the missing ones.
// "BUILTIN\xxx" group names are passed through verbatim — PS
// accepts both qualified and bare local-group names.
func applyGroupsWindows(username string, currentGroups, desiredGroups []string, appendGroups bool) error {
	have := stringSet(currentGroups)
	want := stringSet(desiredGroups)

	if !appendGroups {
		for g := range have {
			if !want[g] {
				script := fmt.Sprintf(
					"Remove-LocalGroupMember -Group %s -Member %s -ErrorAction SilentlyContinue",
					quotePS(g), quotePS(username),
				)
				_ = runPowerShell(script) // best-effort, matches darwin's parity
			}
		}
	}

	for _, g := range desiredGroups {
		if have[g] {
			continue
		}
		script := fmt.Sprintf(
			"Add-LocalGroupMember -Group %s -Member %s",
			quotePS(g), quotePS(username),
		)
		if err := runPowerShell(script); err != nil {
			return fmt.Errorf("Add-LocalGroupMember %s -> %s: %w", username, g, err)
		}
	}
	return nil
}

// runPowerShell executes a PowerShell script via the standard
// powershell.exe surface. -NoProfile keeps the call fast +
// hermetic; -Command takes the script as a single string.
// stderr is captured and surfaced on failure.
func runPowerShell(script string) error {
	// #nosec G204 -- script is built from validated config fields
	// + cmdlet names; user-input components go through quotePS.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("powershell exit %d: %s", ee.ExitCode(), strings.TrimSpace(string(out)))
		}
		return fmt.Errorf("powershell: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runPowerShellCapture is the read-side variant: captures stdout
// for parsing, stderr only on error. Used by Get-LocalUser /
// Get-LocalGroupMember probes.
func runPowerShellCapture(script string) (string, error) {
	// #nosec G204 -- same rationale as runPowerShell.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	stdout, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("powershell exit %d: %s", ee.ExitCode(), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(stdout), nil
}

// quotePS wraps s in PowerShell single-quotes, escaping any embedded
// single-quote by doubling. This is the safest way to pass operator-
// supplied strings (usernames, full names, paths) into a PS script
// string without exposing a code-injection surface. Single-quotes
// in PS are literal — no $variable expansion, no backtick escapes.
func quotePS(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
