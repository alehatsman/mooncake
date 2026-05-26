package fleet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet"
)

func TestTCPHintFor_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantParts []string
	}{
		{
			name:      "connection refused",
			err:       errors.New("dial tcp 127.0.0.1:7878: connect: connection refused"),
			wantParts: []string{"TCP RST", "nothing is listening", "mooncake-agentd"},
		},
		{
			name:      "host unreachable",
			err:       errors.New("dial tcp 192.168.5.1:7878: connect: no route to host"),
			wantParts: []string{"No route", "off the network"},
		},
		{
			name:      "timeout",
			err:       timeoutErr{},
			wantParts: []string{"timed out", "powered off", "firewall"},
		},
		{
			name:      "unknown",
			err:       errors.New("dial tcp: weird thing happened"),
			wantParts: []string{"Generic dial failure"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tcpHintFor(tc.err)
			for _, want := range tc.wantParts {
				if !strings.Contains(got, want) {
					t.Errorf("hint for %s missing %q: %s", tc.name, want, got)
				}
			}
		})
	}
}

func TestRunDoctorLadder_HealthyAgentd(t *testing.T) {
	// httptest server speaking the minimum of /v1/version + /v1/facts
	// to exercise all five rungs as ✓.
	const tok = "shhh"
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+tok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version":"dev","uptime_sec":42}`)
	})
	mux.HandleFunc("/v1/facts", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+tok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"os":"linux","arch":"amd64"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// httptest server URL → strip "http://"; that leaves "host:port".
	addr := strings.TrimPrefix(srv.URL, "http://")
	peer := fleet.Peer{Name: "tester", Addr: addr, Token: tok}
	r := runDoctorLadder(context.Background(), peer, 2*time.Second)
	if !r.Healthy {
		t.Fatalf("expected healthy report, got %+v", r)
	}
	wantRungs := []rungName{rungResolve, rungTCP, rungHTTP, rungAuth, rungFacts}
	if len(r.Rungs) != len(wantRungs) {
		t.Fatalf("expected %d rungs, got %d", len(wantRungs), len(r.Rungs))
	}
	for i, n := range wantRungs {
		if r.Rungs[i].Name != n {
			t.Errorf("rung %d: got %q, want %q", i, r.Rungs[i].Name, n)
		}
		if !r.Rungs[i].OK {
			t.Errorf("rung %s: expected OK, got %+v", n, r.Rungs[i])
		}
	}
}

func TestRunDoctorLadder_TCPRefused_StopsLadder(t *testing.T) {
	// 127.0.0.1:1 — privileged port, no listener; kernel returns RST
	// (loopback isn't firewalled). Reliable refused signal.
	peer := fleet.Peer{Name: "deadbeef", Addr: "127.0.0.1:1", Token: "x"}
	r := runDoctorLadder(context.Background(), peer, 500*time.Millisecond)
	if r.Healthy {
		t.Fatalf("expected unhealthy on closed port")
	}
	// resolve OK, tcp fails, http/auth/facts skipped — 5 rungs total.
	if len(r.Rungs) != 5 {
		t.Fatalf("expected 5 rungs, got %d: %+v", len(r.Rungs), r.Rungs)
	}
	if !r.Rungs[0].OK {
		t.Fatalf("resolve should succeed for 127.0.0.1")
	}
	if r.Rungs[1].OK {
		t.Fatalf("tcp should fail for 127.0.0.1:1")
	}
	if !strings.Contains(r.Rungs[1].Hint, "TCP RST") {
		t.Errorf("tcp hint missing TCP RST line: %s", r.Rungs[1].Hint)
	}
	for _, rg := range r.Rungs[2:] {
		if !rg.Skipped {
			t.Errorf("rung %s should be skipped, got %+v", rg.Name, rg)
		}
	}
}

func TestRunDoctorLadder_AuthFailure(t *testing.T) {
	// Server that always returns 401, simulating a wrong token.
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="x"`)
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	peer := fleet.Peer{Name: "p", Addr: addr, Token: "wrong"}
	r := runDoctorLadder(context.Background(), peer, 2*time.Second)
	if r.Healthy {
		t.Fatalf("expected unhealthy on auth failure")
	}
	// resolve ✓, tcp ✓, http ✓ (401 is expected anon), auth ✗, facts skipped.
	if r.Rungs[2].Name != rungHTTP || !r.Rungs[2].OK {
		t.Fatalf("http rung should be ✓ (401 ok anon): %+v", r.Rungs[2])
	}
	if r.Rungs[3].Name != rungAuth || r.Rungs[3].OK {
		t.Fatalf("auth rung should fail: %+v", r.Rungs[3])
	}
	if !strings.Contains(r.Rungs[3].Hint, "Re-pair") {
		t.Errorf("auth hint missing 'Re-pair': %s", r.Rungs[3].Hint)
	}
	if !r.Rungs[4].Skipped {
		t.Errorf("facts should be skipped: %+v", r.Rungs[4])
	}
}

func TestRunDoctorLadder_EmptyTokenSurfacesHint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized) // anon → 401, fine
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	peer := fleet.Peer{Name: "p", Addr: addr, Token: ""}
	r := runDoctorLadder(context.Background(), peer, 2*time.Second)
	if r.Healthy {
		t.Fatalf("expected unhealthy with empty token")
	}
	auth := r.Rungs[3]
	if auth.Name != rungAuth || auth.OK {
		t.Fatalf("auth rung should fail: %+v", auth)
	}
	if !strings.Contains(auth.Detail, "no token") {
		t.Errorf("auth detail should mention no token: %q", auth.Detail)
	}
	if !strings.Contains(auth.Hint, "agentd.token") {
		t.Errorf("auth hint should reference agentd.token: %q", auth.Hint)
	}
}

func TestRunDoctorLadder_BadAddr(t *testing.T) {
	peer := fleet.Peer{Name: "p", Addr: "not-a-host-port", Token: "x"}
	r := runDoctorLadder(context.Background(), peer, time.Second)
	if r.Healthy {
		t.Fatal("expected unhealthy on malformed addr")
	}
	if r.Rungs[0].OK || !strings.Contains(r.Rungs[0].Hint, "host:port") {
		t.Errorf("expected addr-shape hint on resolve rung: %+v", r.Rungs[0])
	}
}

// timeoutErr is a tiny net.Error whose Timeout() is true; used to feed
// tcpHintFor in the table-driven test without hitting the network.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

var _ net.Error = timeoutErr{}
