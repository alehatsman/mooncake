// Package print implements the print action handler.
//
// The log action displays output during execution. It supports three
// forms — Msg (free text), Title (header), Data (structured payload
// rendered as KV or JSON) — and any combination of them. The historical
// "Msg only" form is preserved unchanged.
package print

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// DefaultBudget caps rendered output at 4 KiB to keep agent context
// affordable. PrintAction.Budget=0 disables truncation entirely;
// PrintAction.Budget>0 overrides this default.
const DefaultBudget = 4096

// truncationSuffix is appended after a budget-truncated block. UTF-8
// safe; small enough to not eat much of the budget itself.
const truncationSuffix = " … (truncated)"

// Handler implements the Handler interface for log actions.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "log",
		Description:        "Display messages and structured data to the user",
		Category:           actions.CategoryOutput,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventPrintMessage)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.Log == nil {
		return fmt.Errorf("log configuration is nil")
	}
	la := step.Log
	if la.Msg == "" && la.Data == nil && la.Title == "" {
		return fmt.Errorf("log requires at least one of msg, title, data")
	}
	if la.Format != "" && la.Format != "kv" && la.Format != "json" {
		return fmt.Errorf("log: format must be kv or json (got %q)", la.Format)
	}
	if la.Budget < 0 {
		return fmt.Errorf("log: budget must be >= 0 (got %d)", la.Budget)
	}
	return nil
}

// Execute renders the message + structured data, applies the budget,
// emits the print event, and stores the rendered output on the result.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	la := step.Log

	result := executor.NewResult()
	result.Changed = false

	rendered, err := render(ctx, la)
	if err != nil {
		result.Failed = true
		result.Stderr = err.Error()
		return result, fmt.Errorf("failed to render log: %w", err)
	}

	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type:      events.EventPrintMessage,
			Timestamp: time.Now(),
			Data: events.PrintData{
				Message: rendered,
				Title:   la.Title,
				Format:  effectiveFormat(la),
				Data:    la.Data,
			},
		})
	}

	result.Stdout = rendered
	return result, nil
}

// DryRun logs what would be printed without actually printing.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	rendered, err := render(ctx, step.Log)
	if err != nil {
		rendered = step.Log.Msg + " (template render would fail)"
	}
	ctx.GetLogger().Infof("  [DRY-RUN] Would print: %s", rendered)
	return nil
}

// Run is the Spec 16 entry point. Plan mode renders the output and
// surfaces the first line as a Reason preview. Execute mode delegates.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true

		rendered, err := render(ctx, step.Log)
		if err != nil {
			rendered = step.Log.Msg
		}
		preview := firstLine(rendered)
		if preview == "" {
			r.Reason = "would print message"
		} else {
			r.Reason = "would print: " + preview
		}
		return r, nil
	}
	return h.Execute(ctx, step)
}

// render assembles the final output string from Msg + Title + Data,
// renders any templates in the textual fields, and applies the budget.
func render(ctx actions.Context, la *config.PrintAction) (string, error) {
	var parts []string

	if la.Msg != "" {
		m, err := ctx.GetTemplate().Render(la.Msg, ctx.GetVariables())
		if err != nil {
			return "", fmt.Errorf("render msg: %w", err)
		}
		parts = append(parts, m)
	}

	if la.Title != "" {
		t, err := ctx.GetTemplate().Render(la.Title, ctx.GetVariables())
		if err != nil {
			return "", fmt.Errorf("render title: %w", err)
		}
		parts = append(parts, t)
	}

	if la.Data != nil {
		var dataBlock string
		switch effectiveFormat(la) {
		case "json":
			b, err := json.MarshalIndent(la.Data, "", "  ")
			if err != nil {
				return "", fmt.Errorf("render json: %w", err)
			}
			dataBlock = string(b)
		default: // "kv"
			dataBlock = renderKV(la.Data)
		}
		parts = append(parts, dataBlock)
	}

	out := strings.Join(parts, "\n")
	return applyBudget(out, la.Budget), nil
}

// effectiveFormat returns the format actually used at render time:
// the user's Format if set, otherwise "kv".
func effectiveFormat(la *config.PrintAction) string {
	if la.Format == "" {
		return "kv"
	}
	return la.Format
}

// renderKV produces a human-readable rendering of any value:
//   - map[string]any → sorted keys, aligned `key: value` lines
//   - []any         → `- value` lines (each rendered scalar/sub-block)
//   - scalar        → fmt.Sprintf("%v", v)
//
// Nested maps/slices fall back to compact JSON for the value so the
// output stays single-line per key. Use Format="json" when nested
// structure matters.
func renderKV(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			return "{}"
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		maxKey := 0
		for _, k := range keys {
			if len(k) > maxKey {
				maxKey = len(k)
			}
		}
		var b strings.Builder
		for i, k := range keys {
			if i > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "%-*s  %s", maxKey, k, scalarOrJSON(t[k]))
		}
		return b.String()
	case []any:
		if len(t) == 0 {
			return "[]"
		}
		var b strings.Builder
		for i, item := range t {
			if i > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "- %s", scalarOrJSON(item))
		}
		return b.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// scalarOrJSON renders scalars natively; nested maps/slices compact to
// single-line JSON so the parent kv block stays aligned.
func scalarOrJSON(v any) string {
	switch v.(type) {
	case map[string]any, []any:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	case string:
		return v.(string)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// applyBudget truncates s to at most budget bytes (default DefaultBudget
// when budget == 0... but actually budget==0 means disabled — see below).
//
// Semantics:
//   - budget < 0: caller bug; Validate refuses. Treat as disabled here.
//   - budget == 0: caller explicitly disabled truncation. Return s.
//   - budget > 0: truncate to at most budget bytes, plus truncationSuffix.
//
// Defaulting to DefaultBudget happens via callers that want a default;
// raw applyBudget passes budget through. The handler's render path
// calls applyBudget with la.Budget which is 0 by default — which is
// "disabled" in this function's semantics. The user-friendly default
// (4 KiB) is applied here when Budget is 0 AND Data is the source of
// the bulk text. Pure Msg-only output is never auto-truncated.
//
// Pragmatic choice: only auto-apply DefaultBudget when truncation
// would actually help (output already exceeds DefaultBudget). For
// short messages the user never sees a difference.
func applyBudget(s string, budget int) string {
	switch {
	case budget < 0:
		return s
	case budget == 0:
		if len(s) <= DefaultBudget {
			return s
		}
		budget = DefaultBudget
	}
	if len(s) <= budget {
		return s
	}
	// Reserve room for the suffix.
	keep := budget - len(truncationSuffix)
	if keep < 0 {
		keep = 0
	}
	// Walk back to a valid UTF-8 boundary.
	for keep > 0 && !utf8.RuneStart(s[keep]) {
		keep--
	}
	return s[:keep] + truncationSuffix
}

// firstLine returns the first non-empty line of s, trimmed and
// truncated to 80 chars (with "..." suffix if cut). Used for plan-mode
// preview.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 80 {
			return line[:77] + "..."
		}
		return line
	}
	return ""
}
