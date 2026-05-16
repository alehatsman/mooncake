package pilot

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/config"
)

// ResponseKind is the parsed shape of one operator input line at the
// plan-confirm gate (spec-67 §10).
type ResponseKind int

const (
	RespInvalid ResponseKind = iota
	RespApply
	RespReject
	RespEdit
	RespExplain
	RespAbort
)

// Response is the parsed user input from the confirm-gate prompt.
type Response struct {
	Kind ResponseKind
	// N is the 1-based step index for `explain N`. Zero otherwise.
	N int
}

// ConfirmOutcome is the final disposition of one confirm-gate session.
type ConfirmOutcome int

const (
	OutcomeApply ConfirmOutcome = iota
	OutcomeReject
	OutcomeAbort
)

// ConfirmResult is what ConfirmPlan returns to the caller. PlanBytes is
// the plan the caller should now act on — possibly edited from what the
// caller passed in. Only meaningful when Outcome == OutcomeApply.
type ConfirmResult struct {
	Outcome   ConfirmOutcome
	PlanBytes []byte
}

// ParseResponse parses one line of operator input at the confirm-gate
// prompt. Empty input maps to `N` (reject) — that is the documented
// default in spec-67 §10. Whitespace and case are tolerated.
//
// Returns RespInvalid for unrecognized input so the caller can re-prompt
// without dispatching. Never returns an error.
func ParseResponse(line string) Response {
	s := strings.ToLower(strings.TrimSpace(line))
	if s == "" {
		return Response{Kind: RespReject}
	}
	switch s {
	case "y", "yes", "apply":
		return Response{Kind: RespApply}
	case "n", "no", "reject":
		return Response{Kind: RespReject}
	case "edit", "e":
		return Response{Kind: RespEdit}
	case "abort", "quit", "q":
		return Response{Kind: RespAbort}
	}

	if rest, ok := stripPrefix(s, "explain "); ok {
		n, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil || n < 1 {
			return Response{Kind: RespInvalid}
		}
		return Response{Kind: RespExplain, N: n}
	}
	if rest, ok := stripPrefix(s, "explain"); ok && rest == "" {
		// Bare `explain` without a step number is a typo, not a valid form.
		return Response{Kind: RespInvalid}
	}

	return Response{Kind: RespInvalid}
}

func stripPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return "", false
}

// ConfirmPlan presents the plan to the operator and loops until they
// choose a terminal outcome (apply / reject / abort). Editor and
// explain selections re-prompt; invalid input re-prompts.
//
// planBytes is the UNWRAPPED plan (the LLM's output, fences stripped).
// On OutcomeApply, the returned PlanBytes is the form the caller should
// wrap and execute — equal to the input unless the operator picked
// `edit` and saved a modified plan.
//
// Editor flow: $VISUAL, then $EDITOR, then `vi`. Re-validates the
// edited plan; on validation error, prints the message and re-prompts
// (operator can edit again or pick another response).
//
// Caller-supplied in/out are typically os.Stdin / os.Stderr. The
// caller is responsible for ensuring `in` is a terminal — see
// EnsureInteractive.
func ConfirmPlan(in io.Reader, out io.Writer, planBytes []byte) (ConfirmResult, error) {
	current := planBytes
	reader := bufio.NewReader(in)

	for {
		if err := renderPlan(out, current); err != nil {
			return ConfirmResult{}, err
		}
		fmt.Fprint(out, "Apply? [y/N/edit/explain N/abort]: ")

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && strings.TrimSpace(line) == "" {
				// No input + EOF (closed stdin, piped no-input) — treat as
				// reject so we never silently apply.
				return ConfirmResult{Outcome: OutcomeReject}, nil
			}
			if !errors.Is(err, io.EOF) {
				return ConfirmResult{}, fmt.Errorf("read confirm response: %w", err)
			}
		}

		resp := ParseResponse(line)
		switch resp.Kind {
		case RespApply:
			return ConfirmResult{Outcome: OutcomeApply, PlanBytes: current}, nil
		case RespReject:
			return ConfirmResult{Outcome: OutcomeReject}, nil
		case RespAbort:
			return ConfirmResult{Outcome: OutcomeAbort}, nil
		case RespExplain:
			explainStep(out, current, resp.N)
		case RespEdit:
			edited, editErr := editPlan(out, current)
			if editErr != nil {
				fmt.Fprintf(out, "edit failed: %v\n", editErr)
				continue
			}
			if err := validateForGate(edited); err != nil {
				fmt.Fprintf(out, "edited plan failed validation: %v\n", err)
				continue
			}
			current = edited
		case RespInvalid:
			fmt.Fprintln(out, "did not understand response. Try one of: y, N, edit, explain N, abort.")
		}
	}
}

// EnsureInteractive returns nil if `in` is a terminal capable of
// answering the confirm-gate prompt. If not, returns a typed error
// the caller can surface as "use --auto-apply for unattended runs".
//
// Treats nil and non-*os.File readers as "not a TTY" — those callers
// (tests, piped input) need scripted input via direct ConfirmPlan
// calls, not the interactive runtime path.
func EnsureInteractive(in io.Reader) error {
	f, ok := in.(*os.File)
	if !ok {
		return errors.New("stdin is not a terminal — pass --auto-apply for unattended runs")
	}
	if !term.IsTerminal(int(f.Fd())) {
		return errors.New("stdin is not a terminal — pass --auto-apply for unattended runs")
	}
	return nil
}

// AutoApplyWarning is the banner emitted once per RunLoop (at the
// start) and once per single-shot Run when --auto-apply is set. The
// confirm-gate is the default; --auto-apply is the dangerous mode.
const AutoApplyWarning = "WARNING: --auto-apply set. The plan-confirm gate is skipped; every generated plan will run against the system without operator review."

// renderPlan writes the recap line + step table for the plan to out.
// On a malformed plan it falls back to a single-line message so the
// gate still works (the operator can `edit` to fix it).
func renderPlan(out io.Writer, planBytes []byte) error {
	steps, _, _, decodeErr := decodePlan(planBytes)
	if decodeErr != nil {
		fmt.Fprintf(out, "plan: <unparseable: %v>\n\n", decodeErr)
		return nil
	}

	irreversible, _ := CountIrreversibleSteps(planBytes)
	fmt.Fprintf(out, "\nPlan: %d step%s", len(steps), plural(len(steps)))
	if irreversible > 0 {
		fmt.Fprintf(out, ", %d irreversible", irreversible)
	}
	fmt.Fprintln(out, ". Wrapped in transaction: failure auto-reverts.")

	for i, s := range steps {
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		action := s.DetermineActionType()
		if action == "" {
			action = "?"
		}
		fmt.Fprintf(out, "  [%d] %-30s  action=%s\n", i+1, truncate(name, 30), action)
	}
	fmt.Fprintln(out)
	return nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

// explainStep prints the full YAML body of step N (1-based) to out.
// Uses raw yaml.Node decoding rather than config.Step so action fields
// the typed schema doesn't know about (LLM hallucinations, schema
// drift between Mooncake versions) still render — the operator needs
// to see exactly what they're about to apply.
//
// Out-of-range N is reported but is not an error — the operator can
// re-prompt.
func explainStep(out io.Writer, planBytes []byte, n int) {
	nodes, err := decodeStepNodes(planBytes)
	if err != nil {
		fmt.Fprintf(out, "cannot explain: plan failed to parse (%v)\n", err)
		return
	}
	if n < 1 || n > len(nodes) {
		fmt.Fprintf(out, "step %d out of range (plan has %d steps)\n", n, len(nodes))
		return
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(nodes[n-1]); err != nil {
		fmt.Fprintf(out, "encode step %d: %v\n", n, err)
		return
	}
	_ = enc.Close()
	fmt.Fprintf(out, "\n--- step %d ---\n%s\n", n, buf.String())
}

// decodeStepNodes returns the top-level steps as raw yaml.Nodes. Handles
// both supported input shapes (bare list of steps, or RunConfig with
// `steps:`). Preserves all keys present in source, regardless of
// whether config.Step recognizes them.
func decodeStepNodes(planBytes []byte) ([]*yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(planBytes, &root); err != nil {
		return nil, err
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = *root.Content[0]
	}
	switch root.Kind {
	case yaml.SequenceNode:
		return root.Content, nil
	case yaml.MappingNode:
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == "steps" {
				stepsNode := root.Content[i+1]
				if stepsNode.Kind == yaml.SequenceNode {
					return stepsNode.Content, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("plan root is neither a list of steps nor a RunConfig with a steps key")
}

// editPlan writes planBytes to a tempfile, opens it in $VISUAL /
// $EDITOR / vi, and returns the saved contents. Caller validates.
//
// editorRunner is a package-level var so tests can swap in a stub that
// rewrites the file deterministically instead of spawning vi.
var editorRunner = runEditor

func editPlan(out io.Writer, planBytes []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "mooncake-pilot-edit-*.yml")
	if err != nil {
		return nil, fmt.Errorf("tempfile: %w", err)
	}
	path := tmp.Name()
	defer func() { _ = os.Remove(path) }()

	if _, err := tmp.Write(planBytes); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close tempfile: %w", err)
	}

	editor := pickEditor()
	fmt.Fprintf(out, "opening plan in %s (%s)...\n", editor, path)
	if err := editorRunner(editor, path); err != nil {
		return nil, fmt.Errorf("%s exited non-zero: %w", editor, err)
	}

	edited, err := os.ReadFile(path) // #nosec G304 -- tempfile path created by this function
	if err != nil {
		return nil, fmt.Errorf("read edited plan: %w", err)
	}
	return edited, nil
}

func pickEditor() string {
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

func runEditor(editor, path string) error {
	// #nosec G204 -- editor is from $VISUAL/$EDITOR (operator's own env)
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// validateForGate validates that the (possibly edited) plan parses and
// passes JSON Schema. It does NOT wrap the plan in a transaction — the
// wrap happens after the gate accepts. Catches plan-shape errors that
// would otherwise blow up later in the loop.
func validateForGate(planBytes []byte) error {
	wrapped, err := WrapInTransaction(planBytes)
	if err != nil {
		return fmt.Errorf("wrap: %w", err)
	}
	tmp, err := os.CreateTemp("", "mooncake-pilot-validate-*.yml")
	if err != nil {
		return fmt.Errorf("tempfile: %w", err)
	}
	path := tmp.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := tmp.Write(wrapped); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tempfile: %w", err)
	}
	_, diags, err := config.ReadConfigWithValidation(path)
	if err != nil {
		return err
	}
	if config.HasErrors(diags) {
		return fmt.Errorf("%s", config.FormatDiagnostics(diags))
	}
	return nil
}
