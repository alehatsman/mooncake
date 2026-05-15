package winutil

// scheduledtask.go renders the Task Scheduler artefacts mooncake needs
// to register survivable autostart for agentd and similar long-lived
// helpers. We use the XML schema (the same one Task Scheduler exports
// via Export-ScheduledTask) rather than the Register-ScheduledTask
// cmdlet's individual flags because:
//
//   - All fields fit naturally into one declarative document — no
//     argument-explosion as we add settings.
//   - The same XML is used for round-trip drift detection: read the
//     remote's current task XML, normalise both sides, diff. The
//     cmdlet's piecewise flag rebuild would be a different shape per
//     field and harder to compare uniformly.
//   - Task XML is the documented stable interchange format — Microsoft
//     has shipped tasks with the same schema since Windows 7.
//
// Identity is the TaskName (passed at registration time, not in the
// XML). The XML is a definition; the name is its address.
//
// Reference for the schema (URI in the doctype):
//   http://schemas.microsoft.com/windows/2004/02/mit/task

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TriggerType is the discriminator on Trigger.
type TriggerType string

const (
	TrigBoot       TriggerType = "boot"
	TrigLogon      TriggerType = "logon"
	TrigRepetition TriggerType = "repetition"
	// Future: daily, weekly, monthly — slot in when needed; the
	// existing dotfiles uses only boot + repetition + logon.
)

// LogonType maps to the principal's <LogonType> XML element. S4U is
// the "survive logout" answer for the personal-fleet use case.
type LogonType string

const (
	LogonInteractive    LogonType = "Interactive"
	LogonS4U            LogonType = "S4U"
	LogonPassword       LogonType = "Password"
	LogonServiceAccount LogonType = "ServiceAccount"
)

// RunLevel maps to <RunLevel> on the principal — Highest = elevated.
type RunLevel string

const (
	RunHighest RunLevel = "HighestAvailable"
	RunLimited RunLevel = "LeastPrivilege"
)

// MultipleInstances controls what happens when the task is triggered
// again while a previous run hasn't finished.
type MultipleInstances string

const (
	MultiIgnoreNew    MultipleInstances = "IgnoreNew"
	MultiParallel     MultipleInstances = "Parallel"
	MultiQueue        MultipleInstances = "Queue"
	MultiStopExisting MultipleInstances = "StopExisting"
)

// Trigger is one entry in the <Triggers> block. Fields not relevant to
// the type are ignored.
type Trigger struct {
	Type    TriggerType
	Enabled *bool

	// LogonUserID is the user the LogOn trigger waits for. Empty =
	// any user.
	LogonUserID string

	// RepetitionInterval is the ISO-8601 duration ("PT5M") between
	// retries when Type == TrigRepetition. Required for that type.
	RepetitionInterval time.Duration

	// RepetitionDuration is the total window the repetition stays
	// active. Zero = indefinite.
	RepetitionDuration time.Duration

	// StartBoundary is the earliest moment the trigger may fire. For
	// repetition triggers we default it to "now" if zero — the
	// scheduler treats a TrigRepetition without a StartBoundary as
	// invalid, so we always emit one.
	StartBoundary time.Time
}

// ExecAction is one entry in the <Actions> block. Today we support
// only Exec actions (Send Email / Show Message are deprecated by
// Microsoft).
type ExecAction struct {
	Command          string
	Arguments        string
	WorkingDirectory string
}

// Principal is the user identity the task runs as.
type Principal struct {
	UserID    string    // e.g. "DESKTOP-X\\aleh" or "SYSTEM"
	LogonType LogonType // S4U / Interactive / Password / ServiceAccount
	RunLevel  RunLevel
}

// Settings is the <Settings> block. Sensible defaults are applied in
// withDefaults() so callers can leave most of these zero.
type Settings struct {
	StartWhenAvailable          *bool
	AllowStartIfOnBatteries     *bool
	DontStopIfGoingOnBatteries  *bool
	RunOnlyIfNetworkAvailable   *bool
	MultipleInstancesPolicy     MultipleInstances
	ExecutionTimeLimit          time.Duration // 0 = unbounded
	RestartCount                int
	RestartInterval             time.Duration // ignored when RestartCount==0
	Hidden                      *bool
	DisallowStartIfOnBatteries  *bool // shadow of AllowStartIfOnBatteries for legacy keys
	RestartOnIdle               *bool
}

// Task is the full declarative description of a scheduled task: the
// identity (Name, Description), the triggers, the actions, the
// principal, and the settings.
type Task struct {
	Name        string
	Description string

	Triggers  []Trigger
	Actions   []ExecAction
	Principal Principal
	Settings  Settings
}

// Validate returns the first error in t, or nil. Errors are caught
// here rather than letting Register-ScheduledTask report them, because
// the cmdlet's error messages for malformed XML are notoriously
// opaque.
func (t Task) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("Name is required")
	}
	if strings.ContainsAny(t.Name, `<>:"/\|?*`) {
		// Same charset Task Scheduler refuses; matches NTFS forbidden
		// chars. Catching here gives a clearer error.
		return fmt.Errorf("Name %q contains a forbidden character", t.Name)
	}
	if len(t.Triggers) == 0 {
		return errors.New("at least one Trigger is required")
	}
	for i, tr := range t.Triggers {
		if err := tr.validate(); err != nil {
			return fmt.Errorf("Triggers[%d]: %w", i, err)
		}
	}
	if len(t.Actions) == 0 {
		return errors.New("at least one Action is required")
	}
	for i, a := range t.Actions {
		if strings.TrimSpace(a.Command) == "" {
			return fmt.Errorf("Actions[%d]: Command is required", i)
		}
	}
	if t.Principal.UserID == "" {
		return errors.New("Principal.UserID is required")
	}
	switch t.Principal.LogonType {
	case "", LogonInteractive, LogonS4U, LogonPassword, LogonServiceAccount:
	default:
		return fmt.Errorf("Principal.LogonType %q is invalid", t.Principal.LogonType)
	}
	switch t.Principal.RunLevel {
	case "", RunHighest, RunLimited:
	default:
		return fmt.Errorf("Principal.RunLevel %q is invalid", t.Principal.RunLevel)
	}
	switch t.Settings.MultipleInstancesPolicy {
	case "", MultiIgnoreNew, MultiParallel, MultiQueue, MultiStopExisting:
	default:
		return fmt.Errorf("Settings.MultipleInstancesPolicy %q is invalid", t.Settings.MultipleInstancesPolicy)
	}
	return nil
}

func (tr Trigger) validate() error {
	switch tr.Type {
	case TrigBoot, TrigLogon, TrigRepetition:
	case "":
		return errors.New("Trigger.Type is required")
	default:
		return fmt.Errorf("Trigger.Type %q is invalid", tr.Type)
	}
	if tr.Type == TrigRepetition {
		if tr.RepetitionInterval <= 0 {
			return errors.New("TrigRepetition requires RepetitionInterval > 0")
		}
	}
	return nil
}

// withDefaults returns a copy of t with empty fields filled. Defaults
// match what the spec-57 doc promises:
//   - Principal.LogonType defaults to S4U (survivable autostart)
//   - Principal.RunLevel defaults to Highest
//   - MultipleInstancesPolicy defaults to IgnoreNew
//   - Settings booleans default to true where "be permissive" is the
//     personal-fleet answer.
func (t Task) withDefaults() Task {
	if t.Principal.LogonType == "" {
		t.Principal.LogonType = LogonS4U
	}
	if t.Principal.RunLevel == "" {
		t.Principal.RunLevel = RunHighest
	}
	if t.Settings.MultipleInstancesPolicy == "" {
		t.Settings.MultipleInstancesPolicy = MultiIgnoreNew
	}
	t.Settings.StartWhenAvailable = boolPtrDefault(t.Settings.StartWhenAvailable, true)
	t.Settings.AllowStartIfOnBatteries = boolPtrDefault(t.Settings.AllowStartIfOnBatteries, true)
	t.Settings.DontStopIfGoingOnBatteries = boolPtrDefault(t.Settings.DontStopIfGoingOnBatteries, true)
	// DisallowStartIfOnBatteries is the inverse of AllowStartIfOnBatteries;
	// keep them in sync if caller didn't set the legacy field.
	if t.Settings.DisallowStartIfOnBatteries == nil {
		v := !*t.Settings.AllowStartIfOnBatteries
		t.Settings.DisallowStartIfOnBatteries = &v
	}
	// StartBoundary defaults for repetition triggers — see Trigger.
	now := time.Now().UTC()
	for i := range t.Triggers {
		if t.Triggers[i].Type == TrigRepetition && t.Triggers[i].StartBoundary.IsZero() {
			t.Triggers[i].StartBoundary = now
		}
		if t.Triggers[i].Enabled == nil {
			tt := true
			t.Triggers[i].Enabled = &tt
		}
	}
	return t
}

// RenderXML produces the Task Scheduler XML document for t. Output is
// UTF-8-encoded and uses Windows line endings (`\r\n`) — that's what
// Task Scheduler emits when round-tripping a task; matching its style
// keeps the drift-detection compare clean.
func RenderXML(t Task) (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	t = t.withDefaults()

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-16"?>` + "\r\n")
	b.WriteString(`<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">` + "\r\n")

	// <RegistrationInfo>
	b.WriteString("  <RegistrationInfo>\r\n")
	if t.Description != "" {
		b.WriteString("    <Description>")
		b.WriteString(xmlEscape(t.Description))
		b.WriteString("</Description>\r\n")
	}
	b.WriteString("    <URI>\\")
	b.WriteString(xmlEscape(t.Name))
	b.WriteString("</URI>\r\n")
	b.WriteString("  </RegistrationInfo>\r\n")

	// <Triggers>
	b.WriteString("  <Triggers>\r\n")
	for _, tr := range t.Triggers {
		writeTrigger(&b, tr)
	}
	b.WriteString("  </Triggers>\r\n")

	// <Principals>
	b.WriteString("  <Principals>\r\n")
	b.WriteString(`    <Principal id="Author">` + "\r\n")
	b.WriteString("      <UserId>")
	b.WriteString(xmlEscape(t.Principal.UserID))
	b.WriteString("</UserId>\r\n")
	b.WriteString("      <LogonType>")
	b.WriteString(string(t.Principal.LogonType))
	b.WriteString("</LogonType>\r\n")
	b.WriteString("      <RunLevel>")
	b.WriteString(string(t.Principal.RunLevel))
	b.WriteString("</RunLevel>\r\n")
	b.WriteString("    </Principal>\r\n")
	b.WriteString("  </Principals>\r\n")

	// <Settings>
	// Element order matches the Task Scheduler XML schema
	// (https://learn.microsoft.com/en-us/windows/win32/taskschd/tasksettings).
	// Wrong order makes Register-ScheduledTask error out with
	// "The task XML contains an unexpected node".
	//
	// Notable absences: <AllowStartIfOnBatteries> is NOT in the
	// schema — the semantics are expressed via the negative-form
	// <DisallowStartIfOnBatteries> only. The Settings struct still
	// accepts AllowStartIfOnBatteries for ergonomics; withDefaults()
	// folds it into the DisallowStartIfOnBatteries value before we
	// emit.
	b.WriteString("  <Settings>\r\n")
	if t.Settings.RestartCount > 0 {
		b.WriteString("    <RestartOnFailure>\r\n")
		b.WriteString("      <Interval>")
		b.WriteString(durationToISO8601(t.Settings.RestartInterval))
		b.WriteString("</Interval>\r\n")
		b.WriteString(fmt.Sprintf("      <Count>%d</Count>\r\n", t.Settings.RestartCount))
		b.WriteString("    </RestartOnFailure>\r\n")
	}
	writeBool(&b, "MultipleInstancesPolicy", string(t.Settings.MultipleInstancesPolicy), 4)
	writeBoolPtr(&b, "DisallowStartIfOnBatteries", t.Settings.DisallowStartIfOnBatteries, 4)
	writeBoolPtr(&b, "StopIfGoingOnBatteries", boolPtrNot(t.Settings.DontStopIfGoingOnBatteries), 4)
	writeBoolPtr(&b, "StartWhenAvailable", t.Settings.StartWhenAvailable, 4)
	writeBoolPtr(&b, "RunOnlyIfNetworkAvailable", t.Settings.RunOnlyIfNetworkAvailable, 4)
	writeBoolPtr(&b, "Hidden", t.Settings.Hidden, 4)
	if t.Settings.ExecutionTimeLimit > 0 {
		b.WriteString("    <ExecutionTimeLimit>")
		b.WriteString(durationToISO8601(t.Settings.ExecutionTimeLimit))
		b.WriteString("</ExecutionTimeLimit>\r\n")
	} else {
		// 0 = unbounded → PT0S is the canonical wire form.
		b.WriteString("    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>\r\n")
	}
	b.WriteString("  </Settings>\r\n")

	// <Actions>
	b.WriteString(`  <Actions Context="Author">` + "\r\n")
	for _, a := range t.Actions {
		b.WriteString("    <Exec>\r\n")
		b.WriteString("      <Command>")
		b.WriteString(xmlEscape(a.Command))
		b.WriteString("</Command>\r\n")
		if a.Arguments != "" {
			b.WriteString("      <Arguments>")
			b.WriteString(xmlEscape(a.Arguments))
			b.WriteString("</Arguments>\r\n")
		}
		if a.WorkingDirectory != "" {
			b.WriteString("      <WorkingDirectory>")
			b.WriteString(xmlEscape(a.WorkingDirectory))
			b.WriteString("</WorkingDirectory>\r\n")
		}
		b.WriteString("    </Exec>\r\n")
	}
	b.WriteString("  </Actions>\r\n")

	b.WriteString("</Task>\r\n")
	return b.String(), nil
}

// writeTrigger emits the right XML element for tr's type. Format
// follows what Export-ScheduledTask produces, with consistent
// 4-space indent under <Triggers>.
func writeTrigger(b *strings.Builder, tr Trigger) {
	switch tr.Type {
	case TrigBoot:
		b.WriteString("    <BootTrigger>\r\n")
		writeBoolPtr(b, "Enabled", tr.Enabled, 6)
		b.WriteString("    </BootTrigger>\r\n")
	case TrigLogon:
		b.WriteString("    <LogonTrigger>\r\n")
		writeBoolPtr(b, "Enabled", tr.Enabled, 6)
		if tr.LogonUserID != "" {
			b.WriteString("      <UserId>")
			b.WriteString(xmlEscape(tr.LogonUserID))
			b.WriteString("</UserId>\r\n")
		}
		b.WriteString("    </LogonTrigger>\r\n")
	case TrigRepetition:
		b.WriteString("    <TimeTrigger>\r\n")
		writeBoolPtr(b, "Enabled", tr.Enabled, 6)
		b.WriteString("      <Repetition>\r\n")
		b.WriteString("        <Interval>")
		b.WriteString(durationToISO8601(tr.RepetitionInterval))
		b.WriteString("</Interval>\r\n")
		if tr.RepetitionDuration > 0 {
			b.WriteString("        <Duration>")
			b.WriteString(durationToISO8601(tr.RepetitionDuration))
			b.WriteString("</Duration>\r\n")
		}
		b.WriteString("        <StopAtDurationEnd>false</StopAtDurationEnd>\r\n")
		b.WriteString("      </Repetition>\r\n")
		if !tr.StartBoundary.IsZero() {
			b.WriteString("      <StartBoundary>")
			b.WriteString(tr.StartBoundary.UTC().Format("2006-01-02T15:04:05"))
			b.WriteString("</StartBoundary>\r\n")
		}
		b.WriteString("    </TimeTrigger>\r\n")
	}
}

func writeBool(b *strings.Builder, name, value string, indent int) {
	b.WriteString(strings.Repeat(" ", indent))
	b.WriteByte('<')
	b.WriteString(name)
	b.WriteByte('>')
	b.WriteString(value)
	b.WriteString("</")
	b.WriteString(name)
	b.WriteString(">\r\n")
}

func writeBoolPtr(b *strings.Builder, name string, v *bool, indent int) {
	if v == nil {
		return
	}
	val := "false"
	if *v {
		val = "true"
	}
	writeBool(b, name, val, indent)
}

// RenderRegisterCommand returns the PowerShell statement that registers
// a task from the rendered XML (passed via -Xml). The XML itself is
// produced separately so callers can stage it as a tempfile + pass
// `Get-Content -Raw` to the cmdlet — that avoids the multi-line
// EncodedCommand bloat for documents containing newlines.
func RenderRegisterCommand(taskName, xmlPath string) string {
	return "Register-ScheduledTask -TaskName " + psStringLiteral(taskName) +
		" -Xml (Get-Content -Raw " + psStringLiteral(xmlPath) + ") -Force | Out-Null"
}

// RenderUnregisterCommand returns the PowerShell statement that removes
// a task. Silently no-ops on missing.
func RenderUnregisterCommand(taskName string) string {
	return "Unregister-ScheduledTask -TaskName " + psStringLiteral(taskName) +
		" -Confirm:$false -ErrorAction SilentlyContinue | Out-Null"
}

// RenderExportCommand returns the PS that emits an existing task's
// XML to stdout. Used for drift detection — capture, normalise,
// compare against RenderXML output.
func RenderExportCommand(taskName string) string {
	return "Export-ScheduledTask -TaskName " + psStringLiteral(taskName)
}

// ----- helpers --------------------------------------------------------------

// durationToISO8601 renders a Go time.Duration as a Task-Scheduler-
// compatible ISO-8601 duration string. We hand-roll this rather than
// using a third-party library — the rules are simple and the schema
// only accepts the subset we care about.
//
//	PT5M     = 5 minutes
//	PT1H     = 1 hour
//	P1DT2H   = 1 day, 2 hours
//
// Days are intentional; Task Scheduler distinguishes between PT24H
// and P1D even though they represent the same duration.
func durationToISO8601(d time.Duration) string {
	if d == 0 {
		return "PT0S"
	}
	if d < 0 {
		return "PT0S"
	}
	days := int64(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hours := int64(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	mins := int64(d / time.Minute)
	d -= time.Duration(mins) * time.Minute
	secs := int64(d / time.Second)

	var b strings.Builder
	b.WriteByte('P')
	if days > 0 {
		b.WriteString(fmt.Sprintf("%dD", days))
	}
	if hours > 0 || mins > 0 || secs > 0 {
		b.WriteByte('T')
		if hours > 0 {
			b.WriteString(fmt.Sprintf("%dH", hours))
		}
		if mins > 0 {
			b.WriteString(fmt.Sprintf("%dM", mins))
		}
		if secs > 0 {
			b.WriteString(fmt.Sprintf("%dS", secs))
		}
	}
	if b.Len() == 1 {
		// pure-days case: P1D, no T.
	}
	return b.String()
}

// xmlEscape replaces the five XML special characters with their
// entity forms. Used everywhere we substitute user-provided text into
// the document — task names with apostrophes, descriptions with
// ampersands, command paths with quoted args (Arguments often
// contains "C:\Path with spaces\foo.exe" quoted on each side).
func xmlEscape(s string) string {
	if !strings.ContainsAny(s, `<>&"'`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// taskSchedulerInjectedElements lists element local-names that the
// Task Scheduler service ALWAYS rewrites on register, regardless of
// whether we sent them. Comparing the live XML against the rendered
// desired XML would always report drift on these (issue #14):
//
//   - Author/Date/URI inside <RegistrationInfo> are stamped from the
//     registering user + the registration timestamp.
//   - Defaulted Settings the service fills in when we omit them
//     (AllowHardTerminate, ExecutionTimeLimit, IdleSettings,
//     NetworkSettings, RestartOnFailure, RunOnlyIfIdle, …).
//   - <Principal id="Author"> attribute canonicalisation that we
//     handle separately by leaving Principals untouched.
//
// Strip these from BOTH sides of the compare before the string
// equality check. If the operator declared a value explicitly for
// one of these in their YAML, our renderer still emits it; both
// sides drop it during normalisation, which means a user-set
// value also gets ignored for drift purposes — acceptable because:
//
//  1. The service has already applied the explicit value (apply
//     succeeded), so semantic drift is what we'd be looking for, and
//  2. Re-applying a wrong value is harmless and idempotent.
//
// When more trigger types or settings get added to spec-57's
// renderer this list will grow. Track via the explicit Set of
// strip targets so additions are obvious.
var taskSchedulerInjectedElements = []string{
	"Author",
	"Date",
	// URI intentionally NOT stripped — it's the task identity our
	// renderer emits deliberately. Task Scheduler may rewrite the
	// path form (e.g. `\Mooncake` → `\Mooncake\`); a separate
	// canonicalisation step would be needed for that, beyond
	// strip-and-compare.
	"AllowHardTerminate",
	"AllowStartOnDemand",
	"DisallowStartIfOnBatteries",
	"ExecutionTimeLimit",
	"Hidden",
	"IdleSettings",
	"MultipleInstancesPolicy",
	"NetworkSettings",
	"Priority",
	"RestartOnFailure",
	"RunOnlyIfIdle",
	"RunOnlyIfNetworkAvailable",
	"StartWhenAvailable",
	"StopIfGoingOnBatteries",
	"UseUnifiedSchedulingEngine",
	"WakeToRun",
}

// NormaliseTaskXML produces a canonical form of a Task Scheduler XML
// document for drift comparison. The XML the cmdlet writes back may:
//   - Reorder settings elements
//   - Inject defaults we didn't specify
//   - Switch between LF and CRLF line endings
//   - Stamp RegistrationInfo with the registering user + timestamp
//
// We solve that by: (a) splitting on element boundaries, (b) dropping
// whitespace-only lines, (c) trimming each line, (d) stripping any
// element listed in taskSchedulerInjectedElements (handles both the
// inline `<X>v</X>` form and multi-line `<X>...</X>` subtrees), and
// (e) sorting elements within <Settings> alphabetically so
// order-of-elements drift doesn't register as a real diff.
// <Triggers>/<Actions>/<Principals> stay position-sensitive —
// order matters there.
//
// Issue #14: prior to the strip step, post-apply plan always
// reported "would update task X" because the live XML carried
// registration metadata + defaulted Settings that our renderer
// never emitted. The strip is the deferred canonicalisation
// step the spec-57 draft (§"Open questions" #2) promised.
func NormaliseTaskXML(xml string) string {
	xml = stripInjectedElements(xml, taskSchedulerInjectedElements)
	xml = strings.ReplaceAll(xml, "\r\n", "\n")
	lines := strings.Split(xml, "\n")
	out := make([]string, 0, len(lines))
	inSettings := false
	var settingsBuf []string
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "<Settings>") {
			inSettings = true
			out = append(out, trim)
			continue
		}
		if strings.HasPrefix(trim, "</Settings>") {
			sort.Strings(settingsBuf)
			out = append(out, settingsBuf...)
			settingsBuf = settingsBuf[:0]
			inSettings = false
			out = append(out, trim)
			continue
		}
		if inSettings {
			settingsBuf = append(settingsBuf, trim)
			continue
		}
		out = append(out, trim)
	}
	return strings.Join(out, "\n")
}

// stripInjectedElements removes occurrences of every element listed
// in names from xml. Handles three element shapes the Task Scheduler
// emits:
//
//   - `<Name>value</Name>` (single line, scalar leaf)
//   - `<Name attr="…">value</Name>` (single line, scalar with attributes)
//   - `<Name>...</Name>` spanning multiple lines (compound subtree)
//   - `<Name attr="…" />` self-closing variant
//
// Implementation: per name, scan repeatedly for `<Name` and consume
// up to the matching `</Name>` (or end of self-closing tag). Strips
// leading whitespace + the trailing newline of the stripped span so
// the result doesn't carry a blank line where the element was.
func stripInjectedElements(xml string, names []string) string {
	for _, name := range names {
		xml = stripElement(xml, name)
	}
	return xml
}

// stripElement removes every <name...>...</name> (or self-closing
// <name ... />) span from xml. Conservative — bails out cleanly on
// any markup it doesn't recognise rather than chewing past it.
func stripElement(xml, name string) string {
	openTag := "<" + name
	closeTag := "</" + name + ">"
	var b strings.Builder
	b.Grow(len(xml))
	rest := xml
	for {
		i := strings.Index(rest, openTag)
		if i < 0 {
			b.WriteString(rest)
			break
		}
		// Make sure the open tag matches at a word boundary (so
		// "<AuthorEx>" doesn't get treated as "<Author"-prefixed).
		// The next byte after openTag must be one of '>', ' ', '/'
		// (attribute separator or close), or '\t'.
		after := i + len(openTag)
		if after >= len(rest) {
			b.WriteString(rest)
			break
		}
		c := rest[after]
		if c != '>' && c != ' ' && c != '/' && c != '\t' && c != '\n' && c != '\r' {
			// Not a real open tag — emit through and advance past it.
			b.WriteString(rest[:after])
			rest = rest[after:]
			continue
		}

		// Emit everything before the open tag, minus any trailing
		// whitespace on the same line (so we don't leave indentation).
		prefix := rest[:i]
		if j := strings.LastIndexByte(prefix, '\n'); j >= 0 {
			b.WriteString(prefix[:j+1])
		} else {
			b.WriteString(prefix)
		}

		// Find the end of this element. Self-closing first.
		tail := rest[after:]
		closeIdx := strings.Index(tail, ">")
		if closeIdx < 0 {
			// Malformed — emit the rest verbatim and bail.
			b.WriteString(rest[i:])
			return b.String()
		}
		// Self-closing variant: `<Name ... />`.
		if closeIdx > 0 && tail[closeIdx-1] == '/' {
			rest = tail[closeIdx+1:]
			rest = strings.TrimLeft(rest, " \t")
			if strings.HasPrefix(rest, "\n") {
				rest = rest[1:]
			}
			continue
		}
		// Paired: skip past `<Name ...>` then find `</Name>`.
		afterOpen := tail[closeIdx+1:]
		k := strings.Index(afterOpen, closeTag)
		if k < 0 {
			// Open without matching close — bail and emit verbatim.
			b.WriteString(rest[i:])
			return b.String()
		}
		rest = afterOpen[k+len(closeTag):]
		rest = strings.TrimLeft(rest, " \t")
		if strings.HasPrefix(rest, "\n") {
			rest = rest[1:]
		}
	}
	return b.String()
}

// ----- bool-ptr helpers (Go's stdlib lacks a *bool default helper) ----------

func boolPtrDefault(v *bool, def bool) *bool {
	if v != nil {
		return v
	}
	return &def
}

func boolPtrNot(v *bool) *bool {
	if v == nil {
		return nil
	}
	n := !*v
	return &n
}
