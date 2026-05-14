package fleet

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// PeerEventKind classifies a PeerEvent for the multiplexer's renderer.
type PeerEventKind int

const (
	// KindEvent carries a real SSE event off the wire; render with formatEvent.
	KindEvent PeerEventKind = iota
	// KindSync is a control event emitted after a successful sync pass.
	KindSync
	// KindSubmit is emitted once after the run is accepted by the daemon.
	KindSubmit
	// KindDisconnect is emitted when an SSE stream ends mid-run without a
	// terminal status, or any other peer-side connectivity failure that
	// doesn't already surface as an Apply error.
	KindDisconnect
	// KindError is emitted when Apply fails before/around stream start
	// (sync/version/submit). The Message holds the redacted reason.
	KindError
)

// PeerEvent tags one observation about a peer with the source name. Apply
// emits PeerEvents into a shared channel; the Multiplexer drains the channel
// and writes prefixed lines to a single Writer.
type PeerEvent struct {
	Peer    string
	Kind    PeerEventKind
	Event   transport.Event // populated when Kind == KindEvent
	Message string          // populated for control events (sync/submit/disconnect/error)
}

// Multiplexer renders PeerEvents to a single Writer. One instance per
// `fleet apply` invocation. Writes are serialized through an internal mutex
// so concurrent calls from any goroutine still produce line-atomic output.
type Multiplexer struct {
	out      io.Writer
	labels   map[string]string // peer name → fully rendered "[padded-name]" with ANSI if enabled
	useColor bool

	mu sync.Mutex
}

// NewMultiplexer pre-computes the per-peer prefix label (padding to the
// widest peer name; ANSI color when useColor is true). useColor should be
// the result of ShouldColor on the same writer.
func NewMultiplexer(out io.Writer, peerNames []string, useColor bool) *Multiplexer {
	width := 0
	for _, n := range peerNames {
		if len(n) > width {
			width = len(n)
		}
	}
	labels := make(map[string]string, len(peerNames))
	for _, n := range peerNames {
		padded := n + strings.Repeat(" ", width-len(n))
		if useColor {
			labels[n] = peerColor(n) + "[" + padded + "]" + ansiReset
		} else {
			labels[n] = "[" + padded + "]"
		}
	}
	return &Multiplexer{out: out, labels: labels, useColor: useColor}
}

// Drain consumes events from ch until ch is closed, rendering each event to
// the writer. Returns when the channel is closed and emptied.
func (m *Multiplexer) Drain(ch <-chan PeerEvent) {
	for ev := range ch {
		m.write(ev)
	}
}

// Banner writes a free-form line to the underlying writer without a peer
// prefix. Used for the ^C "remote runs continue" notice and the final
// summary line. Concurrency-safe.
func (m *Multiplexer) Banner(s string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fmt.Fprintln(m.out, s)
}

func (m *Multiplexer) write(ev PeerEvent) {
	line := m.render(ev)
	if line == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	fmt.Fprintln(m.out, line)
}

func (m *Multiplexer) render(ev PeerEvent) string {
	label, ok := m.labels[ev.Peer]
	if !ok {
		// Defensive fallback: a peer we didn't pre-register (shouldn't
		// happen). Render unpadded, no color.
		label = "[" + ev.Peer + "]"
	}
	switch ev.Kind {
	case KindEvent:
		return label + " " + formatEvent(ev.Event)
	case KindSync:
		return label + " " + ev.Message
	case KindSubmit:
		return label + " " + ev.Message
	case KindDisconnect:
		return label + " *** disconnected ***"
	case KindError:
		return label + " ✗ " + ev.Message
	default:
		return ""
	}
}

// ShouldColor returns true when ANSI color codes are appropriate for w:
// w is an *os.File pointing at a TTY, NO_COLOR is unset, and disable is
// false. Callers pass the user's --no-color flag as disable.
func ShouldColor(w io.Writer, disable bool) bool {
	if disable {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// peerColor maps a peer name to a stable ANSI foreground color from an
// 8-color palette via FNV-32 hashing. The palette skips bright-white and
// regular-white (poor contrast on light terminals).
func peerColor(name string) string {
	palette := []string{
		"\x1b[36m", // cyan
		"\x1b[33m", // yellow
		"\x1b[32m", // green
		"\x1b[35m", // magenta
		"\x1b[34m", // blue
		"\x1b[31m", // red
		"\x1b[96m", // bright cyan
		"\x1b[93m", // bright yellow
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return palette[int(h.Sum32())%len(palette)]
}

const ansiReset = "\x1b[0m"

// formatEvent renders one SSE event as a single human-friendly line, sans
// any peer prefix (the Multiplexer adds that). Kept here, not on Apply, so
// the formatter and the prefix logic are co-located.
//
// The formatter intentionally covers run.* / step.* events with concise
// summaries and falls back to a compact-JSON debug line for everything
// else. Visual polish iteration lives in spec-46.
func formatEvent(ev transport.Event) string {
	switch ev.Type {
	case "run.started":
		return "▶ run started"
	case "run.completed":
		return "✔ run complete " + summarizeRunCompleted(ev.Data)
	case "plan.loaded":
		return "plan loaded " + extractField(ev.Data, "step_count", "0") + " steps"
	case "step.started":
		return "  ▸ " + extractField(ev.Data, "name", "(unnamed step)")
	case "step.completed":
		return "    ✔ " + extractField(ev.Data, "name", "")
	case "step.skipped":
		return "    – " + extractField(ev.Data, "name", "") + " (skipped)"
	case "step.failed":
		return "    ✗ " + extractField(ev.Data, "name", "") + ": " +
			extractField(ev.Data, "error", "(no error message)")
	case "step.stdout", "step.stderr":
		txt := extractField(ev.Data, "line", "")
		if txt == "" {
			return ev.Type
		}
		return "      " + txt
	default:
		if len(ev.Data) > 0 {
			return ev.Type + " " + string(ev.Data)
		}
		return ev.Type
	}
}

func extractField(data json.RawMessage, key, fallback string) string {
	if len(data) == 0 {
		return fallback
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return fallback
	}
	raw, ok := m[key]
	if !ok {
		return fallback
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func summarizeRunCompleted(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var m struct {
		Success      bool `json:"success"`
		TotalSteps   int  `json:"total_steps"`
		ChangedSteps int  `json:"changed_steps"`
		FailedSteps  int  `json:"failed_steps"`
		SkippedSteps int  `json:"skipped_steps"`
		DurationMs   int  `json:"duration_ms"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return string(data)
	}
	verdict := "success"
	if !m.Success {
		verdict = "failed"
	}
	return fmt.Sprintf("%s: %d/%d changed, %d failed, %d skipped (%dms)",
		verdict, m.ChangedSteps, m.TotalSteps, m.FailedSteps, m.SkippedSteps, m.DurationMs)
}

// oneLine collapses a multi-line error into a single-line summary suitable
// for the prefix log format. Newlines and indentation are squashed to
// spaces; the result is trimmed.
func oneLine(s string) string {
	if s == "" {
		return s
	}
	out := strings.ReplaceAll(s, "\n", " ")
	out = strings.ReplaceAll(out, "\t", " ")
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	return strings.TrimSpace(out)
}
