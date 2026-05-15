package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// watchFake is a tiny agentd substitute tailored to the fleet watch
// state machine. Each test plans the sequence of run-list snapshots
// the controller will see, plus per-run SSE event scripts.
type watchFake struct {
	mu         sync.Mutex
	listCalls  int32
	listSeq    []listResp                       // one entry per poll; last entry is sticky
	streams    map[string][]map[string]any      // runID → event bodies emitted on SSE
	failList   bool
}

type listResp struct {
	Runs []map[string]any
	Err  bool // when true, /v1/runs returns 500
}

func newWatchFake() *watchFake {
	return &watchFake{streams: map[string][]map[string]any{}}
}

// nextListResp returns the run snapshot for this poll. After the
// scripted slice is exhausted the last entry is returned indefinitely,
// so a test can end with "nothing new to see" indefinitely without
// hitting an index-out-of-range.
func (f *watchFake) nextListResp() listResp {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := atomic.AddInt32(&f.listCalls, 1)
	idx := int(n) - 1
	if idx >= len(f.listSeq) {
		idx = len(f.listSeq) - 1
	}
	if idx < 0 {
		return listResp{}
	}
	return f.listSeq[idx]
}

func (f *watchFake) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/events") {
			return // handled by the next pattern
		}
		resp := f.nextListResp()
		if resp.Err {
			http.Error(w, "boom", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": resp.Runs})
	})
	mux.HandleFunc("/v1/runs/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/events") {
			http.NotFound(w, r)
			return
		}
		runID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/runs/"), "/events")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		events := f.streams[runID]
		for i, ev := range events {
			body, _ := json.Marshal(map[string]any{
				"seq":       i + 1,
				"type":      ev["type"],
				"timestamp": "2026-05-15T00:00:00Z",
				"data":      ev["data"],
			})
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", i+1, body)
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		}
	})
	return mux
}

func (f *watchFake) start(t *testing.T) (addr, token string) {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://"), "tok"
}

// collectPeerEvents reads every PeerEvent until the channel is closed and
// returns the collected slice. Used by tests to inspect what flowed
// through the multiplexer / JSON sink.
func collectPeerEvents(ch <-chan fleet.PeerEvent) []fleet.PeerEvent {
	var out []fleet.PeerEvent
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

// --- Per-peer state machine ----------------------------------------------

func TestWatchOnePeer_AttachesAndStreamsThenReturnsOnCtxCancel(t *testing.T) {
	fa := newWatchFake()
	fa.listSeq = []listResp{
		// First poll: one running run.
		{Runs: []map[string]any{{"id": "R1", "status": "running"}}},
		// After R1's SSE closes: nothing else (sticky empty).
		{},
	}
	fa.streams["R1"] = []map[string]any{
		{"type": "run.started", "data": map[string]any{}},
		{"type": "step.started", "data": map[string]any{"name": "install"}},
		{"type": "step.completed", "data": map[string]any{"name": "install"}},
		{"type": "run.completed", "data": map[string]any{"success": true}},
	}
	addr, tok := fa.start(t)
	peer := fleet.Peer{Name: "p1", Addr: addr, Token: tok, Transport: fleet.TransportAgentd}

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan fleet.PeerEvent, 32)
	done := make(chan struct{})
	go func() {
		watchOnePeer(ctx, peer, 50*time.Millisecond, events)
		close(done)
	}()

	// Give the goroutine enough time to attach + drain SSE + return to POLLING.
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	close(events)

	all := collectPeerEvents(events)

	var sawAttached, sawRunStarted, sawRunCompleted bool
	for _, ev := range all {
		switch ev.Kind {
		case fleet.KindSubmit:
			if strings.Contains(ev.Message, "R1") {
				sawAttached = true
			}
		case fleet.KindEvent:
			switch ev.Event.Type {
			case "run.started":
				sawRunStarted = true
			case "run.completed":
				sawRunCompleted = true
			}
		}
	}
	if !sawAttached {
		t.Errorf("expected 'attached to run R1' control event, got %+v", all)
	}
	if !sawRunStarted || !sawRunCompleted {
		t.Errorf("expected to see run.started + run.completed; events=%+v", all)
	}
}

func TestWatchOnePeer_ReattachesToNewRunAfterStreamCloses(t *testing.T) {
	fa := newWatchFake()
	fa.listSeq = []listResp{
		{Runs: []map[string]any{{"id": "R1", "status": "running"}}}, // first poll: R1
		{Runs: []map[string]any{{"id": "R1", "status": "running"}}}, // R1 still listed but already attached
		{Runs: []map[string]any{{"id": "R2", "status": "running"}}}, // later: R2 appears
		{},
	}
	fa.streams["R1"] = []map[string]any{
		{"type": "run.completed", "data": map[string]any{"success": true}},
	}
	fa.streams["R2"] = []map[string]any{
		{"type": "run.completed", "data": map[string]any{"success": true}},
	}
	addr, tok := fa.start(t)
	peer := fleet.Peer{Name: "p1", Addr: addr, Token: tok, Transport: fleet.TransportAgentd}

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan fleet.PeerEvent, 32)
	done := make(chan struct{})
	go func() {
		watchOnePeer(ctx, peer, 30*time.Millisecond, events)
		close(done)
	}()

	time.Sleep(400 * time.Millisecond)
	cancel()
	<-done
	close(events)

	attached := map[string]bool{}
	for _, ev := range collectPeerEvents(events) {
		if ev.Kind == fleet.KindSubmit && strings.HasPrefix(ev.Message, "attached to run ") {
			attached[strings.TrimPrefix(ev.Message, "attached to run ")] = true
		}
	}
	if !attached["R1"] || !attached["R2"] {
		t.Errorf("expected attaches to both R1 and R2, got %v", attached)
	}
}

func TestWatchOnePeer_DoesNotReAttachToSameRun(t *testing.T) {
	fa := newWatchFake()
	fa.listSeq = []listResp{
		{Runs: []map[string]any{{"id": "R1", "status": "running"}}},
		{Runs: []map[string]any{{"id": "R1", "status": "running"}}},
		{Runs: []map[string]any{{"id": "R1", "status": "running"}}},
		{Runs: []map[string]any{{"id": "R1", "status": "running"}}},
	}
	fa.streams["R1"] = []map[string]any{
		{"type": "run.completed", "data": map[string]any{"success": true}},
	}
	addr, tok := fa.start(t)
	peer := fleet.Peer{Name: "p1", Addr: addr, Token: tok, Transport: fleet.TransportAgentd}

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan fleet.PeerEvent, 64)
	done := make(chan struct{})
	go func() {
		watchOnePeer(ctx, peer, 25*time.Millisecond, events)
		close(done)
	}()
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	close(events)

	attaches := 0
	for _, ev := range collectPeerEvents(events) {
		if ev.Kind == fleet.KindSubmit && strings.Contains(ev.Message, "R1") {
			attaches++
		}
	}
	if attaches != 1 {
		t.Errorf("expected exactly one attach to R1 (attached_set dedup), got %d", attaches)
	}
}

func TestWatchOnePeer_BackoffSurfacesListErrorThenRecovers(t *testing.T) {
	fa := newWatchFake()
	fa.listSeq = []listResp{
		{Err: true},
		{Err: true},
		{Runs: []map[string]any{{"id": "R1", "status": "running"}}},
		{},
	}
	fa.streams["R1"] = []map[string]any{
		{"type": "run.completed", "data": map[string]any{"success": true}},
	}
	addr, tok := fa.start(t)
	peer := fleet.Peer{Name: "p1", Addr: addr, Token: tok, Transport: fleet.TransportAgentd}

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan fleet.PeerEvent, 64)
	done := make(chan struct{})
	go func() {
		watchOnePeer(ctx, peer, 20*time.Millisecond, events)
		close(done)
	}()
	time.Sleep(2 * time.Second) // backoff caps at 1s, so 2s covers two errors + a recovery
	cancel()
	<-done
	close(events)

	var sawErr, sawAttach bool
	for _, ev := range collectPeerEvents(events) {
		switch ev.Kind {
		case fleet.KindError:
			sawErr = true
		case fleet.KindSubmit:
			if strings.Contains(ev.Message, "R1") {
				sawAttach = true
			}
		}
	}
	if !sawErr {
		t.Error("expected KindError from failing /v1/runs polls")
	}
	if !sawAttach {
		t.Error("expected eventual attach after backoff recovery")
	}
}

// --- Backoff helpers ----------------------------------------------------

func TestBackoff_DoublesUntilCap(t *testing.T) {
	b := newBackoff(100*time.Millisecond, 800*time.Millisecond)
	// internal.current marches 100 → 200 → 400 → 800 → 800 (cap)
	want := []time.Duration{100, 200, 400, 800, 800}
	for i, w := range want {
		d := b.next()
		// jittered → ±25%; verify within band.
		low, high := time.Duration(float64(w)*time.Millisecond.Seconds()*1e9*0.75), time.Duration(float64(w)*time.Millisecond.Seconds()*1e9*1.25)
		_ = low
		_ = high
		// Cleaner: assert d is within 75–125% of w*ms.
		gotMs := float64(d) / float64(time.Millisecond)
		if gotMs < float64(w)*0.7 || gotMs > float64(w)*1.3 {
			t.Errorf("backoff[%d]=%.0fms, want ~%dms", i, gotMs, w)
		}
	}
}

func TestBackoff_ResetReturnsToBase(t *testing.T) {
	b := newBackoff(100*time.Millisecond, 1*time.Second)
	for i := 0; i < 5; i++ {
		_ = b.next()
	}
	b.reset()
	d := b.next()
	if float64(d)/float64(time.Millisecond) > 200 {
		t.Errorf("reset failed: next backoff %dms (want ≤ ~125ms)", d/time.Millisecond)
	}
}

// --- JSON sink ----------------------------------------------------------

func TestEmitJSONEvents_LabelsKindsCorrectly(t *testing.T) {
	ch := make(chan fleet.PeerEvent, 4)
	ch <- fleet.PeerEvent{Peer: "p1", Kind: fleet.KindSubmit, Message: "attached to run R1"}
	ch <- fleet.PeerEvent{Peer: "p1", Kind: fleet.KindEvent, Event: transport.Event{Seq: 1, Type: "run.started"}}
	ch <- fleet.PeerEvent{Peer: "p1", Kind: fleet.KindDisconnect, Message: "connection reset"}
	ch <- fleet.PeerEvent{Peer: "p1", Kind: fleet.KindError, Message: "boom"}
	close(ch)

	var buf bytes.Buffer
	emitJSONEvents(&buf, ch)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 JSONL lines, got %d:\n%s", len(lines), buf.String())
	}
	wantKinds := []string{"attached", "event", "disconnected", "error"}
	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %d not JSON: %v\n%s", i, err, line)
		}
		if m["kind"] != wantKinds[i] {
			t.Errorf("line %d kind = %v, want %s", i, m["kind"], wantKinds[i])
		}
		if m["peer"] != "p1" {
			t.Errorf("line %d peer = %v", i, m["peer"])
		}
	}
}

// --- Multiplexed end-to-end -----------------------------------------------

func TestRunWatchMultiplex_RendersEventsFromMultiplePeers(t *testing.T) {
	makeFake := func(rid string) *watchFake {
		fa := newWatchFake()
		fa.listSeq = []listResp{
			{Runs: []map[string]any{{"id": rid, "status": "running"}}},
			{},
		}
		fa.streams[rid] = []map[string]any{
			{"type": "run.started", "data": map[string]any{}},
			{"type": "run.completed", "data": map[string]any{"success": true}},
		}
		return fa
	}
	faA := makeFake("RA")
	faB := makeFake("RB")
	addrA, tokA := faA.start(t)
	addrB, tokB := faB.start(t)
	peers := []fleet.Peer{
		{Name: "alpha", Addr: addrA, Token: tokA, Transport: fleet.TransportAgentd},
		{Name: "beta", Addr: addrB, Token: tokB, Transport: fleet.TransportAgentd},
	}

	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = runWatchMultiplex(ctx, &buf, peers, 30*time.Millisecond, true)
		close(done)
	}()
	time.Sleep(400 * time.Millisecond)
	cancel()
	<-done

	out := buf.String()
	if !strings.Contains(out, "fleet watch: 2 peer(s)") {
		t.Errorf("missing opening banner:\n%s", out)
	}
	if !strings.Contains(out, "[alpha") || !strings.Contains(out, "[beta") {
		t.Errorf("missing per-peer prefixes:\n%s", out)
	}
	if !strings.Contains(out, "fleet watch: stopped.") {
		t.Errorf("missing closing banner:\n%s", out)
	}
}
