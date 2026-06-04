package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Reader defines the interface for reading configuration and variables
type Reader interface {
	ReadConfig(path string) (*ParsedConfig, error)
	ReadVariables(path string) (map[string]interface{}, error)
}

// YAMLConfigReader implements Reader for YAML files
type YAMLConfigReader struct {
	// Can add dependencies here if needed (e.g., FileSystem interface)
}

// NewYAMLConfigReader creates a new YAMLConfigReader
func NewYAMLConfigReader() Reader {
	return &YAMLConfigReader{}
}

// ReadConfig reads configuration steps from a YAML file
// For backward compatibility, this method validates the config and returns
// an error if any validation errors are found
func (r *YAMLConfigReader) ReadConfig(path string) (*ParsedConfig, error) {
	parsedConfig, diagnostics, err := r.ReadConfigWithValidation(path, nil)
	if err != nil {
		return nil, err
	}

	// Convert diagnostics to error for backward compatibility
	if len(diagnostics) > 0 && HasErrors(diagnostics) {
		return parsedConfig, &ValidationError{Diagnostics: diagnostics}
	}

	return parsedConfig, nil
}

// ReadConfigWithValidation reads configuration steps from a YAML file with full validation.
// Returns parsed config (with steps, global vars, version), diagnostics (which may include warnings), and any parsing errors.
//
// isCustom is the action-registry predicate: a typed-key custom action
// (`notify.webhook: {…}`) whose name it recognizes is folded into the generic
// carrier before validation, so it parses, validates, and dispatches exactly
// like a built-in (see normalizeCustomActionSteps). Pass nil for built-ins
// only — every non-builtin key then stays an unknown-field error, preserving
// the strict typo diagnostics.
func (r *YAMLConfigReader) ReadConfigWithValidation(path string, isCustom IsCustomAction) (*ParsedConfig, []Diagnostic, error) {
	// #nosec G304 -- User-specified config file path is intentional and required functionality
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return r.parseConfigData(data, path, isCustom)
}

// ReadConfigBytesWithValidation parses config bytes that are already in
// memory, with the identical validation/normalization pipeline as the
// file-based reader (auto YAML/JSON detection, secret-tag substitution,
// custom-action carrier folding, strict + schema + template validation).
// label is a human-readable source name used only in diagnostics — there
// is no file on disk (#142). Used by the SDK's ApplyBytes inline entry.
func (r *YAMLConfigReader) ReadConfigBytesWithValidation(data []byte, label string, isCustom IsCustomAction) (*ParsedConfig, []Diagnostic, error) {
	return r.parseConfigData(data, label, isCustom)
}

// parseConfigData is the shared body behind the file-based and
// bytes-based readers: everything after the source bytes are obtained.
// path is the source label (a real file path, or a "<...>" marker for
// in-memory input) used in diagnostics and the empty-input message.
func (r *YAMLConfigReader) parseConfigData(data []byte, path string, isCustom IsCustomAction) (*ParsedConfig, []Diagnostic, error) {
	// MT-73: empty or whitespace-only files would otherwise surface as
	// io.EOF (YAML) or "unexpected end of JSON input" (JSON). Replace with
	// a clear "what to put here" message before format detection runs.
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil, fmt.Errorf("config file is empty: %s — expected a list of steps (e.g. `- shell: echo hello`) or a `steps:` block", path)
	}

	// Parse to yaml.Node, auto-detecting YAML vs JSON input format.
	// JSON input is round-tripped through yaml.Marshal so the resulting
	// node tree is indistinguishable from a YAML-sourced one — downstream
	// code (location map, secret-tag substitution, strict revalidation)
	// works unchanged.
	rootNodePtr, err := DecodeAutoNode(data)
	if err != nil {
		return nil, nil, err
	}
	rootNode := *rootNodePtr

	// spec-23 §3: rewrite `!secret <ref>` tagged scalars to sentinel-
	// marker strings BEFORE downstream decode. Action-struct fields are
	// typed `string`; the marker flows through naturally and the executor
	// later swaps it for the resolved value (or the plan-output redactor
	// rewrites it as `!secret <ref>` for JSON serialization).
	substituteSecretTags(&rootNode)

	// Fold typed-key custom actions (`notify.webhook: {…}`) into the generic
	// #111 carrier BEFORE the location map, decode, and both validators run —
	// so every downstream stage sees only carrier form and is untouched.
	// No-op when isCustom is nil (no registry → built-ins only).
	normalizeCustomActionSteps(&rootNode, isCustom)

	// Build location map from yaml.Node tree
	locationMap := buildLocationMap(&rootNode)

	// MT-72: catch the common "forgot the leading dash" mistake before
	// the schema validator does — its error blames the action name
	// ("unknown field `log`") and sends users to docs for typos instead
	// of pointing at the actual problem (top-level dict instead of a
	// list of step objects).
	if hint := detectTopLevelDictShape(&rootNode); hint != "" {
		return nil, nil, fmt.Errorf("%s: %s", path, hint)
	}

	// Parse config - supports both old format (array) and new format (object with steps)
	var parsedConfig *ParsedConfig
	if isArrayFormat(&rootNode) {
		// Old format: plain array of steps
		var steps []Step
		err = rootNode.Decode(&steps)
		if err != nil {
			return nil, nil, err
		}

		// Attach source locations from locationMap
		attachSourceLocations(steps, locationMap, "")

		parsedConfig = &ParsedConfig{
			Steps:      steps,
			GlobalVars: make(map[string]interface{}),
			Version:    "",
		}
	} else {
		// New format: RunConfig structure with version, vars, and steps
		var runConfig RunConfig
		err = rootNode.Decode(&runConfig)
		if err != nil {
			return nil, nil, err
		}
		// Initialize GlobalVars to empty map if nil
		globalVars := runConfig.Vars
		if globalVars == nil {
			globalVars = make(map[string]interface{})
		}

		// Attach source locations from locationMap
		attachSourceLocations(runConfig.Steps, locationMap, "/steps")

		// Attach source locations for each named task's steps so plan /
		// apply diagnostics anchor at the right line when a task is
		// invoked. Task names land in the YAML mapping in declaration
		// order; the JSON-pointer key is /tasks/<name>/steps.
		for name, task := range runConfig.Tasks {
			attachSourceLocations(task.Steps, locationMap, "/tasks/"+name+"/steps")
		}

		parsedConfig = &ParsedConfig{
			Steps:      runConfig.Steps,
			GlobalVars: globalVars,
			Modules:    runConfig.Modules,
			Version:    runConfig.Version,
			Tasks:      runConfig.Tasks,
		}
	}

	// Strict-mode pass: re-decode the post-substitution YAML tree with
	// KnownFields(true) so unknown step / action fields surface as
	// diagnostics instead of being silently dropped. Without this, a
	// typo like `register:` (post-spec-21 rename, now `as:`) or
	// `sha256:` inside `file.download` (correct field is `checksum:`)
	// disappears at parse time and the user sees a "successful" apply
	// that didn't capture or didn't verify. Closes finding #44 and
	// guards every future field rename. The permissive decode above
	// stays unchanged — strict errors are diagnostics, not parse-aborts.
	strictDiagnostics := validateKnownFields(&rootNode, locationMap, path)

	// Run schema validation
	validator, err := NewSchemaValidator()
	if err != nil {
		// If schema validation setup fails, fall back to basic validation
		// This ensures the system still works even if schema is broken
		return parsedConfig, append(strictDiagnostics, Diagnostic{
			FilePath: path,
			Line:     1,
			Column:   1,
			Message:  "schema validator initialization failed: " + err.Error(),
			Severity: "warning",
		}), nil
	}

	diagnostics := validator.Validate(parsedConfig, locationMap, path)
	diagnostics = append(strictDiagnostics, diagnostics...)

	// MT-4: when validateKnownFields already pinpoints a typo'd
	// step-level or action-level field (e.g. `file.template.content`),
	// the schema validator's generic "Step must have exactly one
	// action (artifact.capture, …, file.template, …)" diagnostic
	// fires alongside it for the same line — misleading because the
	// step DOES name a valid action; the action just has an unknown
	// sub-property. Drop the second message when a sharper unknown-
	// field one is already attached at the same location.
	diagnostics = suppressRedundantOneOfWhenUnknownField(diagnostics)

	// Validate template syntax in all templatable fields
	templateValidator := NewTemplateValidator()
	templateDiagnostics := templateValidator.ValidateSteps(parsedConfig.Steps, locationMap, path)
	diagnostics = append(diagnostics, templateDiagnostics...)

	return parsedConfig, diagnostics, nil
}

// suppressRedundantOneOfWhenUnknownField removes "Step must have
// exactly one action (…)" diagnostics from a file that ALREADY has
// at least one unknown-field diagnostic. Rationale: the schema
// validator emits the generic oneOf message on the step header line
// whenever an action sub-property is unknown, but validateKnownFields
// has already pinpointed the actual culprit by name. The two messages
// fire on slightly different lines (header vs. inner property) so a
// line-level join doesn't catch them; a file-level suppression is
// strictly better because keeping the oneOf message is what misled
// users in the first place (they see "Step must have exactly one
// action: file.template, …" alongside their `file.template:` step
// and assume the action keyword itself is wrong). Genuinely
// action-less steps in the same file are still flagged on the next
// retry once the unknown field is removed.
//
// Also suppresses the noise-level "Doesn't validate with …schema…"
// parent diagnostic that the jsonschema library emits at the step
// header — it never adds information beyond what's already in the
// child diagnostic.
func suppressRedundantOneOfWhenUnknownField(diags []Diagnostic) []Diagnostic {
	hasUnknown := make(map[string]bool, len(diags))
	for _, d := range diags {
		if strings.Contains(d.Message, "unknown field") || strings.Contains(d.Message, "Unknown field") {
			hasUnknown[d.FilePath] = true
		}
	}
	if len(hasUnknown) == 0 {
		return diags
	}
	out := diags[:0]
	for _, d := range diags {
		if hasUnknown[d.FilePath] {
			if strings.Contains(d.Message, "Step must have exactly one action") {
				continue
			}
			if strings.HasPrefix(d.Message, "Doesn't validate with") {
				continue
			}
		}
		out = append(out, d)
	}
	return out
}

// validateKnownFields runs a strict YAML decode pass over the
// already-parsed root node and returns a diagnostic for each unknown
// field encountered. Implementation: marshal the (secret-tag-
// substituted) rootNode back to bytes, then decode WITH
// KnownFields(true) so the yaml.v3 library does the unknown-key
// detection for us — including nested action structs like
// file.download (the strict mode applies recursively through every
// typed struct in the tree).
//
// Returns an empty slice on success or schema-shape ambiguity. Any
// error is surfaced as one error-severity Diagnostic; the YAML
// library reports the first unknown field per decode pass, so users
// fix one, retry, see the next.
func validateKnownFields(rootNode *yaml.Node, _ *LocationMap, path string) []Diagnostic {
	buf, err := yaml.Marshal(rootNode)
	if err != nil {
		// Should be unreachable — rootNode came from yaml.Decode and
		// is well-formed. Surface as a warning so it doesn't block.
		return []Diagnostic{{
			FilePath: path,
			Line:     1,
			Column:   1,
			Message:  "strict-mode validation skipped: marshal root: " + err.Error(),
			Severity: "warning",
		}}
	}
	dec := yaml.NewDecoder(bytes.NewReader(buf))
	dec.KnownFields(true)

	var decodeErr error
	if isArrayFormat(rootNode) {
		var steps []Step
		decodeErr = dec.Decode(&steps)
	} else {
		var rc RunConfig
		decodeErr = dec.Decode(&rc)
	}
	if decodeErr == nil {
		return nil
	}
	// yaml.v3's error message looks like:
	//   "yaml: unmarshal errors:\n  line 4: field register not found in type config.Step"
	// Try to parse the line number to anchor the diagnostic.
	msg := decodeErr.Error()
	line, col := 1, 1
	if li := extractYAMLLine(msg); li > 0 {
		line = li
	}
	return []Diagnostic{{
		FilePath: path,
		Line:     line,
		Column:   col,
		Message:  formatStrictFieldError(msg),
		Severity: "error",
	}}
}

// extractYAMLLine pulls the first "line N:" number out of a yaml.v3
// unmarshal-errors message. Returns 0 if not found.
func extractYAMLLine(msg string) int {
	const marker = "line "
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return 0
	}
	rest := msg[idx+len(marker):]
	end := strings.Index(rest, ":")
	if end <= 0 {
		return 0
	}
	n := 0
	for i := 0; i < end; i++ {
		c := rest[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// formatStrictFieldError turns the yaml.v3 raw message into a single
// user-facing line. The library's wording is fine but the prefix
// "yaml: unmarshal errors: line N:" is noisy when we already attach
// the line via Diagnostic.Line.
func formatStrictFieldError(msg string) string {
	// Strip the "yaml: unmarshal errors:\n  line N: " prefix.
	if i := strings.Index(msg, "field "); i > 0 {
		core := strings.TrimSpace(msg[i:])
		// Drop the leading "field " from the library wording so the
		// composed sentence reads cleanly.
		core = strings.TrimPrefix(core, "field ")
		return "unknown field `" + truncBeforeSpace(core, "not found in type ") +
			"` (likely a typo or a renamed field — see docs-next/guide/config/actions.md)"
	}
	return "strict-mode validation: " + msg
}

// truncBeforeSpace returns the prefix of s up to the first occurrence
// of marker. Used to pull "register" out of "register not found in
// type config.Step".
func truncBeforeSpace(s, marker string) string {
	if i := strings.Index(s, marker); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// isArrayFormat checks if the YAML root node represents an array (old format)
// or an object (new RunConfig format)
func isArrayFormat(node *yaml.Node) bool {
	// The root node is a document node, so we need to check its content
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		firstNode := node.Content[0]
		return firstNode.Kind == yaml.SequenceNode
	}
	return node.Kind == yaml.SequenceNode
}

// detectTopLevelDictShape returns a human-readable hint when the YAML
// root is a mapping that LOOKS like the user forgot the leading `- ` —
// i.e. it's an object instead of a list of step objects, and none of
// its top-level keys are recognized RunConfig fields (`version`,
// `vars`, `steps`). Returns an empty string when the shape is fine.
// MT-72.
func detectTopLevelDictShape(node *yaml.Node) string {
	root := node
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return ""
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return ""
	}
	// Mapping content is alternating key/value scalar nodes.
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "version", "vars", "steps":
			// Looks like a RunConfig; let the regular path handle it.
			return ""
		}
	}
	// Top-level mapping with no RunConfig keys — almost certainly a
	// stray step where the leading `- ` was forgotten.
	var firstKey string
	if len(root.Content) > 0 && root.Content[0].Kind == yaml.ScalarNode {
		firstKey = root.Content[0].Value
	}
	indent := "      "
	example := "  - <action>:\n" + indent + "<args>"
	if firstKey != "" {
		example = "  - " + firstKey + ":\n" + indent + "..."
	}
	return "expected a list of steps; got a top-level object. Add a leading `- ` to each step:\n" + example
}

// attachSourceLocations populates SourceLocation for each step from the locationMap.
// This captures the exact line number where each step is defined in the YAML source.
// The basePath parameter is the JSON pointer prefix:
//   - "" for old format (plain array)
//   - "/steps" for new format (RunConfig with steps field)
func attachSourceLocations(steps []Step, locationMap *LocationMap, basePath string) {
	for i := range steps {
		// Build JSON pointer path for this step
		// e.g., "/0", "/1" for old format or "/steps/0", "/steps/1" for new format
		stepPath := formatArrayPath(basePath, i)

		// Get position from locationMap
		pos := locationMap.Get(stepPath)
		if pos.Line > 0 {
			steps[i].SourceLocation = &Position{
				Line:   pos.Line,
				Column: pos.Column,
			}
		}
	}
}

// ReadVariables reads variables from a YAML file
func (r *YAMLConfigReader) ReadVariables(path string) (map[string]interface{}, error) {
	if path == "" {
		return make(map[string]interface{}), nil
	}

	// #nosec G304 -- User-specified variables file path is intentional and required functionality
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	variables := make(map[string]interface{})
	if len(bytes.TrimSpace(data)) == 0 {
		return variables, nil
	}

	if err := DecodeAuto(data, &variables); err != nil {
		return nil, err
	}

	return variables, nil
}

// Package-level functions for backward compatibility
var defaultReader = NewYAMLConfigReader()

// ReadConfig is a convenience function using the default YAML reader
func ReadConfig(path string) (*ParsedConfig, error) {
	return defaultReader.ReadConfig(path)
}

// ReadConfigWithValidation is a convenience function using the default YAML reader.
// Returns parsed config (with steps, global vars, version), diagnostics, and any parsing errors.
// See (*YAMLConfigReader).ReadConfigWithValidation for isCustom.
func ReadConfigWithValidation(path string, isCustom IsCustomAction) (*ParsedConfig, []Diagnostic, error) {
	reader, ok := defaultReader.(*YAMLConfigReader)
	if !ok {
		return nil, nil, fmt.Errorf("internal error: defaultReader is not a YAMLConfigReader")
	}
	return reader.ReadConfigWithValidation(path, isCustom)
}

// ReadConfigBytesWithValidation is a convenience wrapper over the default
// YAML reader for in-memory config bytes (#142). label names the source
// in diagnostics; there is no file on disk. See
// (*YAMLConfigReader).ReadConfigBytesWithValidation.
func ReadConfigBytesWithValidation(data []byte, label string, isCustom IsCustomAction) (*ParsedConfig, []Diagnostic, error) {
	reader, ok := defaultReader.(*YAMLConfigReader)
	if !ok {
		return nil, nil, fmt.Errorf("internal error: defaultReader is not a YAMLConfigReader")
	}
	return reader.ReadConfigBytesWithValidation(data, label, isCustom)
}

// ReadVariables is a convenience function using the default YAML reader
func ReadVariables(path string) (map[string]interface{}, error) {
	return defaultReader.ReadVariables(path)
}
