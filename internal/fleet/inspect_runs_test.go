package fleet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// runsFake answers GET /v1/runs?status=…&limit=…, optionally with a
// per-status response. Tracks every call so tests can assert that
// multi-status invocations actually issue one request per status.
type runsFake struct {
	mu        sync.Mutex
	calls     []string                    // raw RawQuery captured per request
	perStatus map[string][]map[string]any // status → runs payload; "" key = no-status-filter
	fail      bool
}

func newRunsFake() *runsFake {
	return &runsFake{
		perStatus: map[string][]map[string]any{},
	}
}

func (f *runsFake) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.calls = append(f.calls, r.URL.RawQuery)
		runs, ok := f.perStatus[r.URL.Query().Get("status")]
		fail := f.fail
		f.mu.Unlock()
		if fail {
			http.Error(w, "boom", 500)
			return
		}
		if !ok {
			runs = nil
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": runs})
	})
	return mux
}

func (f *runsFake) start(t *testing.T) (addr, token string) {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://"), "tok"
}

func (f *runsFake) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *runsFake) sawQueryContaining(substr string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, q := range f.calls {
		if strings.Contains(q, substr) {
			return true
		}
	}
	return false
}

// --- FetchRuns -----------------------------------------------------------

func TestFetchRuns_SingleStatusFiltersByQuery(t *testing.T) {
	fa := newRunsFake()
	fa.perStatus["running"] = []map[string]any{
		{"id": "01H1", "status": "running", "queued_at": "2026-05-15T00:00:00Z"},
	}
	addr, tok := fa.start(t)
	peer := Peer{Name: "p1", Addr: addr, Token: tok}

	runs, err := FetchRuns(context.Background(), peer, FetchOpts{
		Statuses: []string{"running"},
	})
	if err != nil {
		t.Fatalf("FetchRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != "running" {
		t.Errorf("runs = %+v", runs)
	}
	if !fa.sawQueryContaining("status=running") {
		t.Errorf("daemon should have received ?status=running; calls=%v", fa.calls)
	}
}

func TestFetchRuns_MultiStatusIssuesOneCallPerStatus(t *testing.T) {
	fa := newRunsFake()
	fa.perStatus["running"] = []map[string]any{{"id": "R1", "status": "running"}}
	fa.perStatus["queued"] = []map[string]any{{"id": "Q1", "status": "queued"}}
	addr, tok := fa.start(t)
	peer := Peer{Name: "p1", Addr: addr, Token: tok}

	runs, err := FetchRuns(context.Background(), peer, FetchOpts{
		Statuses: []string{"running", "queued"},
	})
	if err != nil {
		t.Fatalf("FetchRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs from merged status calls, got %d", len(runs))
	}
	if fa.callCount() != 2 {
		t.Errorf("expected 2 daemon calls, got %d", fa.callCount())
	}
}

func TestFetchRuns_AllStatusesEmptyFilterOneCall(t *testing.T) {
	fa := newRunsFake()
	fa.perStatus[""] = []map[string]any{
		{"id": "A", "status": "success"},
		{"id": "B", "status": "running"},
	}
	addr, tok := fa.start(t)
	peer := Peer{Name: "p1", Addr: addr, Token: tok}

	runs, err := FetchRuns(context.Background(), peer, FetchOpts{}) // empty Statuses
	if err != nil {
		t.Fatalf("FetchRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
	if fa.callCount() != 1 {
		t.Errorf("empty-filter should be one call, got %d", fa.callCount())
	}
	// Confirm no `?status=` was sent.
	if fa.sawQueryContaining("status=") {
		t.Errorf("empty filter should not send ?status=; calls=%v", fa.calls)
	}
}

func TestFetchRuns_DaemonErrorBubbles(t *testing.T) {
	fa := newRunsFake()
	fa.fail = true
	addr, tok := fa.start(t)
	peer := Peer{Name: "p1", Addr: addr, Token: tok}

	if _, err := FetchRuns(context.Background(), peer, FetchOpts{}); err == nil {
		t.Fatal("expected error when daemon returns 500")
	}
}

// --- FetchRunsAll --------------------------------------------------------

func TestFetchRunsAll_PreservesPeerOrder_SurfacesErrors(t *testing.T) {
	ok := newRunsFake()
	ok.perStatus["running"] = []map[string]any{{"id": "R1", "status": "running"}}
	okAddr, okTok := ok.start(t)

	broken := newRunsFake()
	broken.fail = true
	brokenAddr, _ := broken.start(t)

	peers := []Peer{
		{Name: "broken", Addr: brokenAddr, Token: "tok"},
		{Name: "alive", Addr: okAddr, Token: okTok},
	}
	res := FetchRunsAll(context.Background(), peers, FetchOpts{Statuses: []string{"running"}})

	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if res[0].Name != "broken" || res[1].Name != "alive" {
		t.Errorf("result order: %s,%s — must match input", res[0].Name, res[1].Name)
	}
	if res[0].Error == nil {
		t.Error("broken peer must surface Error != nil")
	}
	if res[1].Error != nil {
		t.Errorf("alive peer should have no error, got %v", res[1].Error)
	}
	if len(res[1].Runs) != 1 {
		t.Errorf("alive peer should have 1 run, got %d", len(res[1].Runs))
	}
}

func TestFetchRunsAll_ParallelismCap(t *testing.T) {
	var inflight, peakInflight int32

	fa := newRunsFake()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := atomic.AddInt32(&inflight, 1)
		for {
			peak := atomic.LoadInt32(&peakInflight)
			if cur <= peak || atomic.CompareAndSwapInt32(&peakInflight, peak, cur) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{}})
	}))
	t.Cleanup(srv.Close)
	_ = fa // unused

	peers := make([]Peer, 6)
	addr := strings.TrimPrefix(srv.URL, "http://")
	for i := range peers {
		peers[i] = Peer{Name: "p", Addr: addr, Token: "tok"}
	}
	_ = FetchRunsAll(context.Background(), peers, FetchOpts{
		MaxParallel: 2,
		Timeout:     2 * time.Second,
	})
	if peak := atomic.LoadInt32(&peakInflight); peak > 2 {
		t.Errorf("MaxParallel=2 but observed peak %d inflight", peak)
	}
}

// --- sortRunsNewestFirst -------------------------------------------------

func TestSortRunsNewestFirst_PutsNewerFirst(t *testing.T) {
	now := time.Now().UTC()
	mk := func(id string, ago time.Duration) transport.RunRecord {
		return transport.RunRecord{
			ID:         id,
			FinishedAt: now.Add(-ago).Format(time.RFC3339Nano),
		}
	}
	runs := []transport.RunRecord{
		mk("old", 10*time.Minute),
		mk("young", time.Minute),
		mk("mid", 5*time.Minute),
	}
	sortRunsNewestFirst(runs)
	if runs[0].ID != "young" || runs[1].ID != "mid" || runs[2].ID != "old" {
		t.Errorf("sort order: %s,%s,%s", runs[0].ID, runs[1].ID, runs[2].ID)
	}
}
