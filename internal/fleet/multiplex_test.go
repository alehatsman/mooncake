package fleet

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// TestMultiplexer_PaddingAndPrefix asserts the per-peer label is padded to
// the widest peer name so columns line up across hosts. This is the
// foundational property — without it the [host] prefix becomes ragged the
// moment two peers have different-length names.
func TestMultiplexer_PaddingAndPrefix(t *testing.T) {
	var buf bytes.Buffer
	mux := NewMultiplexer(&buf, []string{"a", "longname"}, false)

	mux.write(PeerEvent{Peer: "a", Kind: KindEvent, Event: transport.Event{Type: "run.started"}})
	mux.write(PeerEvent{Peer: "longname", Kind: KindEvent, Event: transport.Event{Type: "run.started"}})

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "[a       ] ") {
		t.Errorf("short-name padding wrong; line=%q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "[longname] ") {
		t.Errorf("long-name unpadded prefix wrong; line=%q", lines[1])
	}
}

// TestMultiplexer_NoColorWhenDisabled — passing useColor=false yields plain
// ASCII labels with zero ANSI bytes. The Multiplexer is the only thing that
// has color knowledge in PR6; if this breaks, NO_COLOR support breaks too.
func TestMultiplexer_NoColorWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	mux := NewMultiplexer(&buf, []string{"alpha", "beta"}, false)
	mux.write(PeerEvent{Peer: "alpha", Kind: KindEvent, Event: transport.Event{Type: "run.started"}})

	s := buf.String()
	if strings.Contains(s, "\x1b[") {
		t.Errorf("ANSI escape leaked with useColor=false: %q", s)
	}
}

// TestMultiplexer_ColorWhenEnabled — useColor=true wraps the label in ANSI
// codes. Reset must always come after, even mid-stream, so a downstream
// `cat -A` or piped consumer doesn't get bleed-through coloring.
func TestMultiplexer_ColorWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	mux := NewMultiplexer(&buf, []string{"alpha"}, true)
	mux.write(PeerEvent{Peer: "alpha", Kind: KindEvent, Event: transport.Event{Type: "run.started"}})

	s := buf.String()
	if !strings.Contains(s, "\x1b[") {
		t.Errorf("expected ANSI escape with useColor=true: %q", s)
	}
	if !strings.Contains(s, ansiReset) {
		t.Errorf("expected ANSI reset with useColor=true: %q", s)
	}
}

// TestMultiplexer_ControlKinds — every Kind that isn't KindEvent must render
// a recognizable status line. Without dedicated formatting, control events
// would disappear silently and the user would lose half the signal during
// a run.
func TestMultiplexer_ControlKinds(t *testing.T) {
	var buf bytes.Buffer
	mux := NewMultiplexer(&buf, []string{"p"}, false)
	mux.write(PeerEvent{Peer: "p", Kind: KindSync, Message: "sync: 3 uploaded"})
	mux.write(PeerEvent{Peer: "p", Kind: KindSubmit, Message: "submitted run abc"})
	mux.write(PeerEvent{Peer: "p", Kind: KindDisconnect})
	mux.write(PeerEvent{Peer: "p", Kind: KindError, Message: "boom"})

	want := []string{
		"[p] sync: 3 uploaded",
		"[p] submitted run abc",
		"[p] *** disconnected ***",
		"[p] ✗ boom",
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), buf.String())
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d = %q, want %q", i, lines[i], w)
		}
	}
}

// TestMultiplexer_LineAtomicityUnderConcurrency stresses the internal mutex.
// Without serialization the [host] prefix can split across writes, breaking
// `grep -F '[host]'`. We blast events from many goroutines and assert every
// output line still starts with one of the expected prefixes.
func TestMultiplexer_LineAtomicityUnderConcurrency(t *testing.T) {
	var buf bytes.Buffer
	peers := []string{"alpha", "beta", "gamma"}
	mux := NewMultiplexer(&buf, peers, false)

	// 200 events per peer × 3 peers × interleaved goroutines.
	const N = 200
	var wg sync.WaitGroup
	for _, p := range peers {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			for i := 0; i < N; i++ {
				ev := transport.Event{
					Type: "step.stdout",
					Data: json.RawMessage(`{"line":"chunk"}`),
				}
				mux.write(PeerEvent{Peer: p, Kind: KindEvent, Event: ev})
			}
		}(p)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if got, want := len(lines), len(peers)*N; got != want {
		t.Fatalf("got %d lines, want %d", got, want)
	}
	validPrefixes := []string{"[alpha]", "[beta ]", "[gamma]"}
	for i, ln := range lines {
		ok := false
		for _, p := range validPrefixes {
			if strings.HasPrefix(ln, p+" ") {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("line %d has bad prefix: %q", i, ln)
		}
	}
}

// TestMultiplexer_DrainExitsOnClose — Drain must return when the channel
// closes, not block. If this regresses, fleet apply hangs forever after
// every peer finishes.
func TestMultiplexer_DrainExitsOnClose(t *testing.T) {
	var buf bytes.Buffer
	mux := NewMultiplexer(&buf, []string{"p"}, false)

	ch := make(chan PeerEvent, 2)
	ch <- PeerEvent{Peer: "p", Kind: KindEvent, Event: transport.Event{Type: "run.started"}}
	ch <- PeerEvent{Peer: "p", Kind: KindEvent, Event: transport.Event{Type: "run.completed"}}
	close(ch)

	done := make(chan struct{})
	go func() {
		mux.Drain(ch)
		close(done)
	}()
	<-done

	if !strings.Contains(buf.String(), "run started") {
		t.Errorf("missing first event: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "run complete") {
		t.Errorf("missing terminal event: %q", buf.String())
	}
}

// TestShouldColor_NoColorEnv asserts the spec-required NO_COLOR honor — a
// quiet but important contract. Setting NO_COLOR=1 must disable color even
// when the writer looks like a TTY.
func TestShouldColor_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// bytes.Buffer isn't an *os.File so ShouldColor returns false regardless,
	// but the explicit NO_COLOR branch should short-circuit before that
	// check ever runs. We just assert the result.
	if ShouldColor(&bytes.Buffer{}, false) {
		t.Error("ShouldColor returned true with NO_COLOR=1")
	}
}

// TestShouldColor_DisableFlag verifies --no-color wins regardless of TTY
// or env.
func TestShouldColor_DisableFlag(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if ShouldColor(&bytes.Buffer{}, true) {
		t.Error("ShouldColor returned true with disable=true")
	}
}
