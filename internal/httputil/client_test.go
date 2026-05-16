package httputil_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/httputil"
)

func TestGet_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "mooncake") {
			t.Errorf("User-Agent = %q, want mooncake prefix", got)
		}
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	body, err := httputil.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", body, "hello")
	}
}

func TestGet_NonSuccessStatusIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	body, err := httputil.Get(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("expected error on 404; got body %q", body)
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status; got %v", err)
	}
}

// TestGet_CancelInterruptsInflight pins the F016 cancellation contract:
// a ctx cancel mid-request must abort the call.
func TestGet_CancelInterruptsInflight(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow handler: never returns until the request is cancelled
		// from the client side.
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := httputil.Get(ctx, srv.URL)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// TestNewRequest_SetsUserAgent ensures the canonical UA is on every
// outbound call. Handlers that build their own req via NewRequest get
// the UA for free; handlers that go around (raw http.NewRequest)
// don't — F012's migration audit catches them.
func TestNewRequest_SetsUserAgent(t *testing.T) {
	req, err := httputil.NewRequest(context.Background(), http.MethodGet, "http://example.test/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("User-Agent"); !strings.Contains(got, "mooncake") {
		t.Errorf("User-Agent = %q, want mooncake prefix", got)
	}
}

// TestClient_TransportHasBoundedTimeouts checks the load-bearing
// Transport tunables — a regression that resets Client.Transport to
// http.DefaultTransport would silently re-introduce the F012 hazard.
func TestClient_TransportHasBoundedTimeouts(t *testing.T) {
	tr, ok := httputil.Client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Client.Transport is %T, want *http.Transport", httputil.Client.Transport)
	}
	if tr.TLSHandshakeTimeout <= 0 {
		t.Errorf("TLSHandshakeTimeout = 0; F012 regression")
	}
	if tr.ResponseHeaderTimeout <= 0 {
		t.Errorf("ResponseHeaderTimeout = 0; F012 regression")
	}
	if tr.IdleConnTimeout <= 0 {
		t.Errorf("IdleConnTimeout = 0; F012 regression")
	}
}
