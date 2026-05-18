// Package observe_logs implements the observe.logs action: read a
// recent slice of a log source and return per-pattern match counts +
// sample lines (spec-61). Three source modes share one pattern matcher
// and one cap-bound reader: path (file tail), journal_unit (systemd
// journalctl), container (docker/podman logs).
package observe_logs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName         = "observe.logs"
	defaultSince       = 60 * time.Second
	defaultSamples     = 5
	defaultMaxBytes    = int64(1 << 20) // 1 MiB
	defaultMaxLines    = 10000
	matchSampleHardCap = 50 // a single pattern can't keep more than this even if user asks
)

// LogObservation is the typed Value payload for observe.logs.
type LogObservation struct {
	Source     string          `json:"source"`     // "file" / "journal" / "container"
	Identifier string          `json:"identifier"` // path / unit / container name
	Window     string          `json:"window"`     // e.g. "60s"
	LinesRead  int             `json:"lines_read"`
	Truncated  bool            `json:"truncated"` // hit byte or line cap
	Matches    []LogMatchGroup `json:"matches,omitempty"`
}

// LogMatchGroup is one regex pattern's outcome.
type LogMatchGroup struct {
	Pattern     string   `json:"pattern"`
	Count       int      `json:"count"`
	SampleLines []string `json:"sample_lines,omitempty"`
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot read of a log source; returns per-pattern match counts + sample lines",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	o := step.ObserveLogs
	if o == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	srcCount := 0
	if o.Path != "" {
		srcCount++
	}
	if o.JournalUnit != "" {
		srcCount++
	}
	if o.Container != "" {
		srcCount++
	}
	if srcCount == 0 {
		return fmt.Errorf("%s: one of path / journal_unit / container is required", actionName)
	}
	if srcCount > 1 {
		return fmt.Errorf("%s: only one of path / journal_unit / container may be set", actionName)
	}
	if len(o.Patterns) == 0 {
		return fmt.Errorf("%s: patterns is required (non-empty list of regex strings)", actionName)
	}
	for i, p := range o.Patterns {
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("%s: patterns[%d] invalid regex: %w", actionName, i, err)
		}
	}
	if o.Since != "" {
		if _, err := time.ParseDuration(o.Since); err != nil {
			return fmt.Errorf("%s: invalid since %q: %w", actionName, o.Since, err)
		}
	}
	if o.MaxBytes < 0 {
		return fmt.Errorf("%s: max_bytes must be >= 0", actionName)
	}
	if o.MaxLines < 0 {
		return fmt.Errorf("%s: max_lines must be >= 0", actionName)
	}
	if o.SampleLines < 0 {
		return fmt.Errorf("%s: sample_lines must be >= 0", actionName)
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObserveLogs

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	since := defaultSince
	if o.Since != "" {
		since, _ = time.ParseDuration(o.Since)
	}
	maxBytes := o.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	maxLines := o.MaxLines
	if maxLines <= 0 {
		maxLines = defaultMaxLines
	}
	samples := o.SampleLines
	if samples <= 0 {
		samples = defaultSamples
	}
	if samples > matchSampleHardCap {
		samples = matchSampleHardCap
	}

	source, identifier := classifySource(o)

	if ctx.Mode() == actions.ModePlan {
		env := actions.PlanDeferred(LogObservation{
			Source:     source,
			Identifier: identifier,
			Window:     since.String(),
		})
		publish(result, env)
		result.Checkable = true
		result.Reason = fmt.Sprintf("would observe %s %s (deferred to apply)", source, identifier)
		return result, nil
	}

	patterns := make([]*regexp.Regexp, 0, len(o.Patterns))
	for _, p := range o.Patterns {
		patterns = append(patterns, regexp.MustCompile(p))
	}

	rdrCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lines, truncated, err := readSource(rdrCtx, o, since, maxBytes, maxLines)

	obs := LogObservation{
		Source:     source,
		Identifier: identifier,
		Window:     since.String(),
		LinesRead:  len(lines),
		Truncated:  truncated,
	}
	if err == nil {
		obs.Matches = matchLines(patterns, o.Patterns, lines, samples)
	}

	env := actions.ObserveResult{
		Found: err == nil,
		Value: obs,
		AsOf:  time.Now(),
	}
	if err != nil {
		env.Error = err.Error()
	}
	publish(result, env)
	return result, nil
}

func classifySource(o *config.ObserveLogs) (kind, identifier string) {
	switch {
	case o.Path != "":
		return "file", o.Path
	case o.JournalUnit != "":
		return "journal", o.JournalUnit
	case o.Container != "":
		return "container", o.Container
	}
	return "", ""
}

// readSource dispatches to the per-source reader. Returns lines (in
// chronological order, oldest first) + truncated flag + error.
func readSource(ctx context.Context, o *config.ObserveLogs, since time.Duration, maxBytes int64, maxLines int) ([]string, bool, error) {
	switch {
	case o.Path != "":
		return readFile(o.Path, since, maxBytes, maxLines)
	case o.JournalUnit != "":
		return readJournal(ctx, o.JournalUnit, since, maxBytes, maxLines)
	case o.Container != "":
		return readContainer(ctx, o.Container, since, maxBytes, maxLines)
	}
	return nil, false, errors.New("observe.logs: no source")
}

// matchLines walks lines once and records per-pattern match counts +
// sample lines (capped at samples per pattern). preserves the original
// pattern string ordering in the output.
func matchLines(rxs []*regexp.Regexp, patterns []string, lines []string, samples int) []LogMatchGroup {
	out := make([]LogMatchGroup, len(rxs))
	for i, p := range patterns {
		out[i].Pattern = p
	}
	for _, line := range lines {
		for i, rx := range rxs {
			if rx.MatchString(line) {
				out[i].Count++
				if len(out[i].SampleLines) < samples {
					out[i].SampleLines = append(out[i].SampleLines, line)
				}
			}
		}
	}
	return out
}

func publish(r *executor.Result, env actions.ObserveResult) {
	r.SetData(map[string]any{
		"found": env.Found,
		"value": actions.ObserveValueToMap(env.Value),
		"as_of": env.AsOf.Format(time.RFC3339),
		"error": env.Error,
	})
}

// --- Spec-22 ABI no-mutation specialization ---------------------------------

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: true, Risk: 1}, nil
}

func (h *Handler) Permissions(step *config.Step) actions.PermissionSet {
	o := step.ObserveLogs
	bins := []string{}
	if o != nil {
		switch {
		case o.JournalUnit != "":
			bins = append(bins, "journalctl")
		case o.Container != "":
			bins = append(bins, "docker") // podman is best-effort; readContainer tries both
		}
	}
	return actions.PermissionSet{
		RequiredBinaries: bins,
		Notes:            []string{"read-only observation; may need read access to /var/log/*"},
	}
}

func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	o := step.ObserveLogs
	if o == nil {
		return actions.Diff{}, nil
	}
	src, id := classifySource(o)
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: src + ":" + id,
			Attributes: map[string]string{"observe_kind": "logs"},
		},
		Operation: actions.OpNoop,
	}, nil
}

func (h *Handler) Reverse(_ actions.Context, _ *config.Step, _ actions.Result) (*config.Step, error) {
	return nil, nil
}
