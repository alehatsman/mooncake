// Package windows_registry implements the windows.registry action.
// Idempotent management of Windows registry keys and values.
//
// Two modes:
//
//	key-only  (name omitted): ensure a registry key is present/absent.
//	value     (name set):     ensure a value inside a key is present/absent
//	                          with the desired data and type.
//
//nolint:revive // package name follows mooncake action convention
package windows_registry

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
	actionName   = "windows.registry"
	statePresent = "present"
	stateAbsent  = "absent"
)

// runPS is a package-level hook so tests can replace the shell-out.
var runPS = realPSRun

// Handler implements windows.registry.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Manage Windows registry keys and values",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		Version:            "1.0.0",
		SupportedPlatforms: []string{"windows"},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	r := step.WindowsRegistry
	if r == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	if strings.TrimSpace(r.Path) == "" {
		return fmt.Errorf("%s: path is required", actionName)
	}
	state := normalizeState(r.State)
	if state != statePresent && state != stateAbsent {
		return fmt.Errorf("%s: state must be present or absent, got %q", actionName, r.State)
	}
	if r.Name != "" {
		if err := validateType(r.Type); err != nil {
			return fmt.Errorf("%s: %w", actionName, err)
		}
		if state == statePresent && r.Value == "" {
			return fmt.Errorf("%s: value is required when name is set and state=present", actionName)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	r := step.WindowsRegistry
	result := executor.NewResult()
	result.Checkable = true
	result.Target = r.Path

	if runtime.GOOS != "windows" {
		return result, fmt.Errorf("%s: only Windows is supported; got %s", actionName, runtime.GOOS)
	}

	state := normalizeState(r.State)

	if r.Name == "" {
		return h.runKeyMode(ctx, r, result, state)
	}
	return h.runValueMode(ctx, r, result, state)
}

// ----- key-only mode --------------------------------------------------------

func (h *Handler) runKeyMode(ctx actions.Context, r *config.WindowsRegistry, result *executor.Result, state string) (actions.Result, error) {
	exists, err := queryKeyExists(r.Path)
	if err != nil {
		return result, fmt.Errorf("%s: query key: %w", actionName, err)
	}

	switch state {
	case stateAbsent:
		if !exists {
			result.Operation = executor.OpNoop
			result.Reason = "key already absent"
			return result, nil
		}
		result.Operation = executor.OpDelete
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = true
			result.Reason = "would remove key " + r.Path
			return result, nil
		}
		if _, err := runPS(renderDeleteKey(r.Path)); err != nil {
			return result, fmt.Errorf("delete key: %w", err)
		}
		result.Changed = true
		result.Reason = "removed key " + r.Path
		return result, nil

	case statePresent:
		if exists {
			result.Operation = executor.OpNoop
			result.Reason = "key already present"
			return result, nil
		}
		result.Operation = executor.OpCreate
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = true
			result.Reason = "would create key " + r.Path
			return result, nil
		}
		if _, err := runPS(renderCreateKey(r.Path)); err != nil {
			return result, fmt.Errorf("create key: %w", err)
		}
		result.Changed = true
		result.Reason = "created key " + r.Path
		return result, nil
	}
	return result, fmt.Errorf("%s: unreachable state %q", actionName, state)
}

// ----- value mode -----------------------------------------------------------

func (h *Handler) runValueMode(ctx actions.Context, r *config.WindowsRegistry, result *executor.Result, state string) (actions.Result, error) {
	result.Target = r.Path + `\` + r.Name

	obs, err := queryValue(r.Path, r.Name)
	if err != nil {
		return result, fmt.Errorf("%s: query value: %w", actionName, err)
	}

	switch state {
	case stateAbsent:
		if !obs.ValueExists {
			result.Operation = executor.OpNoop
			result.Reason = "value already absent"
			return result, nil
		}
		result.Operation = executor.OpDelete
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = true
			result.Reason = "would remove value " + r.Name
			return result, nil
		}
		if _, err := runPS(renderDeleteValue(r.Path, r.Name)); err != nil {
			return result, fmt.Errorf("delete value: %w", err)
		}
		result.Changed = true
		result.Reason = "removed value " + r.Name
		return result, nil

	case statePresent:
		desiredKind := normalizeType(r.Type)
		if obs.ValueExists && obs.Kind == desiredKind && obs.Value == normalizeValueForCompare(r.Value, r.Type) {
			result.Operation = executor.OpNoop
			result.Reason = "value already at desired state"
			return result, nil
		}
		if !obs.ValueExists {
			result.Operation = executor.OpCreate
		} else {
			result.Operation = executor.OpUpdate
		}
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = true
			if !obs.ValueExists {
				result.Reason = "would create value " + r.Name
			} else {
				result.Reason = "would update value " + r.Name
			}
			return result, nil
		}
		if _, err := runPS(renderSetValue(r.Path, r.Name, r.Value, r.Type)); err != nil {
			return result, fmt.Errorf("set value: %w", err)
		}
		result.Changed = true
		if !obs.ValueExists {
			result.Reason = "created value " + r.Name
		} else {
			result.Reason = "updated value " + r.Name
		}
		return result, nil
	}
	return result, fmt.Errorf("%s: unreachable state %q", actionName, state)
}

// ----- query helpers --------------------------------------------------------

func queryKeyExists(path string) (bool, error) {
	script := "if (Test-Path -LiteralPath " + psQuote(path) + ") { 'yes' } else { 'no' }"
	out, err := runPS(script)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "yes", nil
}

type observedValue struct {
	KeyExists   bool
	ValueExists bool
	Value       string // normalised string for comparison
	Kind        string // DWord, QWord, String, ExpandString, MultiString, Binary
}

func queryValue(path, name string) (observedValue, error) {
	// Use [char]10 instead of `n to avoid Go raw-string/backtick conflicts.
	script := "$key = Get-Item -LiteralPath " + psQuote(path) + " -ErrorAction SilentlyContinue\n" +
		"if ($null -eq $key) {\n" +
		"  ConvertTo-Json @{KeyExists=$false;ValueExists=$false} -Compress\n" +
		"} elseif ($key.GetValueNames() -notcontains " + psQuote(name) + ") {\n" +
		"  ConvertTo-Json @{KeyExists=$true;ValueExists=$false} -Compress\n" +
		"} else {\n" +
		"  $v = $key.GetValue(" + psQuote(name) + ")\n" +
		"  $k = $key.GetValueKind(" + psQuote(name) + ").ToString()\n" +
		"  $s = if ($k -eq 'MultiString') { $v -join ([char]10) }" +
		" elseif ($k -eq 'Binary') { [System.BitConverter]::ToString($v).Replace('-','').ToLower() }" +
		" else { [string]$v }\n" +
		"  ConvertTo-Json @{KeyExists=$true;ValueExists=$true;Value=$s;Kind=$k} -Compress\n" +
		"}\n"
	out, err := runPS(script)
	if err != nil {
		return observedValue{}, err
	}
	out = strings.TrimSpace(out)
	var raw struct {
		KeyExists   bool   `json:"KeyExists"`
		ValueExists bool   `json:"ValueExists"`
		Value       string `json:"Value"`
		Kind        string `json:"Kind"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return observedValue{}, fmt.Errorf("decode value json: %w (body: %q)", err, out)
	}
	return observedValue{
		KeyExists:   raw.KeyExists,
		ValueExists: raw.ValueExists,
		Value:       raw.Value,
		Kind:        raw.Kind,
	}, nil
}

// ----- render helpers -------------------------------------------------------

func renderCreateKey(path string) string {
	return "New-Item -Path " + psQuote(path) + " -Force | Out-Null"
}

func renderDeleteKey(path string) string {
	return "Remove-Item -LiteralPath " + psQuote(path) + " -Recurse -ErrorAction SilentlyContinue"
}

func renderDeleteValue(path, name string) string {
	return "Remove-ItemProperty -LiteralPath " + psQuote(path) + " -Name " + psQuote(name) + " -ErrorAction SilentlyContinue"
}

func renderSetValue(path, name, value, typ string) string {
	psType := normalizeType(typ)
	psValue := renderPSValue(value, typ)
	return "if (!(Test-Path -LiteralPath " + psQuote(path) + ")) { New-Item -Path " + psQuote(path) + " -Force | Out-Null }; " +
		"Set-ItemProperty -LiteralPath " + psQuote(path) + " -Name " + psQuote(name) + " -Value " + psValue + " -Type " + psType
}

// renderPSValue produces the PowerShell expression for the value field.
func renderPSValue(value, typ string) string {
	switch strings.ToLower(typ) {
	case "dword":
		return "[int]" + psQuote(value)
	case "qword":
		return "[long]" + psQuote(value)
	case "binary":
		// Hex string "0a1b2c" → byte array via inline PS loop (PS 5.1 compat).
		return "([byte[]]@(for($i=0;$i -lt " + psQuote(value) + ".Length;$i+=2){[Convert]::ToByte(" + psQuote(value) + ".Substring($i,2),16)}))"
	case "multi_string":
		// Newline-separated string → PS string array.
		parts := strings.Split(value, "\n")
		quoted := make([]string, len(parts))
		for i, p := range parts {
			quoted[i] = psQuote(strings.TrimRight(p, "\r"))
		}
		return "@(" + strings.Join(quoted, ",") + ")"
	default:
		return psQuote(value)
	}
}

// normalizeType maps the YAML type field to the PowerShell registry value
// kind name returned by GetValueKind().ToString().
func normalizeType(t string) string {
	switch strings.ToLower(t) {
	case "dword":
		return "DWord"
	case "qword":
		return "QWord"
	case "expand_string":
		return "ExpandString"
	case "multi_string":
		return "MultiString"
	case "binary":
		return "Binary"
	default: // "", "string"
		return "String"
	}
}

// normalizeValueForCompare normalises the user-supplied value string to
// match the format queryValue returns for the given type.
func normalizeValueForCompare(value, typ string) string {
	switch strings.ToLower(typ) {
	case "binary":
		return strings.ToLower(strings.ReplaceAll(value, "-", ""))
	case "multi_string":
		return strings.ReplaceAll(value, "\r\n", "\n")
	default:
		return value
	}
}

func validateType(t string) error {
	switch strings.ToLower(t) {
	case "", "string", "dword", "qword", "expand_string", "multi_string", "binary":
		return nil
	}
	return fmt.Errorf("type must be one of string, dword, qword, expand_string, multi_string, binary; got %q", t)
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

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
