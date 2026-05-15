package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
)

// makeOpErr fabricates the error shape that http.Client.Do returns when
// the underlying net.Dialer fails. Mirrors the chain real failures
// produce: *url.Error -> *net.OpError -> *os.SyscallError -> syscall.Errno.
func makeOpErr(op string, syscallName string, errno syscall.Errno) error {
	inner := &os.SyscallError{Syscall: syscallName, Err: errno}
	opErr := &net.OpError{
		Op:     op,
		Net:    "tcp",
		Addr:   &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 7878},
		Err:    inner,
	}
	return &url.Error{Op: "Get", URL: "http://192.0.2.1:7878/v1/version", Err: opErr}
}

func TestClassifyNetErr_ConnectionRefused(t *testing.T) {
	err := makeOpErr("dial", "connect", syscall.ECONNREFUSED)
	got := classifyNetErr(err)
	if !strings.Contains(got, "connection refused") {
		t.Fatalf("expected 'connection refused' label, got %q", got)
	}
	if !strings.Contains(got, "agentd not running") {
		t.Fatalf("expected hint about no listener, got %q", got)
	}
}

func TestClassifyNetErr_ConnectionReset(t *testing.T) {
	err := makeOpErr("read", "read", syscall.ECONNRESET)
	got := classifyNetErr(err)
	if !strings.Contains(got, "reset by peer") {
		t.Fatalf("expected 'reset by peer' label, got %q", got)
	}
}

func TestClassifyNetErr_HostUnreachable(t *testing.T) {
	err := makeOpErr("dial", "connect", syscall.EHOSTUNREACH)
	got := classifyNetErr(err)
	if !strings.Contains(got, "host unreachable") {
		t.Fatalf("expected 'host unreachable' label, got %q", got)
	}
}

func TestClassifyNetErr_NetworkUnreachable(t *testing.T) {
	err := makeOpErr("dial", "connect", syscall.ENETUNREACH)
	got := classifyNetErr(err)
	if !strings.Contains(got, "network unreachable") {
		t.Fatalf("expected 'network unreachable' label, got %q", got)
	}
}

func TestClassifyNetErr_ContextDeadline(t *testing.T) {
	// Wrap context.DeadlineExceeded in the *url.Error layer the way the
	// real http.Client does (when the request context's deadline trips
	// during dial).
	err := &url.Error{Op: "Get", URL: "http://x/", Err: context.DeadlineExceeded}
	got := classifyNetErr(err)
	if !strings.Contains(got, "timed out") {
		t.Fatalf("expected 'timed out' label, got %q", got)
	}
	if !strings.Contains(got, "firewall") {
		t.Fatalf("expected timeout label to mention firewall as a candidate cause, got %q", got)
	}
}

func TestClassifyNetErr_DialTimeout(t *testing.T) {
	// Build an OpError whose Timeout() returns true via the synthetic
	// timeoutErr below. Mirrors what *net.Dialer produces when its own
	// dial-timeout fires (separate from a context deadline).
	inner := timeoutErr{}
	opErr := &net.OpError{Op: "dial", Net: "tcp", Err: inner}
	err := &url.Error{Op: "Get", URL: "http://x/", Err: opErr}
	got := classifyNetErr(err)
	if !strings.Contains(got, "TCP connect timed out") {
		t.Fatalf("expected 'TCP connect timed out' label, got %q", got)
	}
}

func TestClassifyNetErr_ReadTimeout(t *testing.T) {
	inner := timeoutErr{}
	opErr := &net.OpError{Op: "read", Net: "tcp", Err: inner}
	err := &url.Error{Op: "Get", URL: "http://x/", Err: opErr}
	got := classifyNetErr(err)
	if !strings.Contains(got, "read timeout") {
		t.Fatalf("expected 'read timeout' label, got %q", got)
	}
}

func TestClassifyNetErr_DNSNotFound(t *testing.T) {
	err := &net.DNSError{Err: "no such host", Name: "bogus.invalid", IsNotFound: true}
	got := classifyNetErr(err)
	if !strings.Contains(got, "host not found") {
		t.Fatalf("expected 'host not found' label, got %q", got)
	}
}

func TestClassifyNetErr_DNSGeneric(t *testing.T) {
	err := &net.DNSError{Err: "server misbehaving", Name: "foo.example.com"}
	got := classifyNetErr(err)
	if !strings.Contains(got, "DNS lookup failed") {
		t.Fatalf("expected 'DNS lookup failed' label, got %q", got)
	}
	if !strings.Contains(got, "server misbehaving") {
		t.Fatalf("expected DNS label to include underlying reason, got %q", got)
	}
}

func TestClassifyNetErr_UnknownReturnsEmpty(t *testing.T) {
	// A generic non-network error shouldn't get a network label slapped
	// on it. The caller (wrap) falls back to bare-%w in that case.
	got := classifyNetErr(errors.New("decode: unexpected EOF"))
	if got != "" {
		t.Fatalf("expected empty label for unknown error, got %q", got)
	}
}

func TestClassifyNetErr_Nil(t *testing.T) {
	if got := classifyNetErr(nil); got != "" {
		t.Fatalf("expected empty label for nil, got %q", got)
	}
}

// TestWrap_IncludesClassification confirms the public-facing path:
// transport.Client.wrap should embed the classifier output between the
// op string and the underlying error.
func TestWrap_IncludesClassification(t *testing.T) {
	c := &Client{Name: "main_pc"}
	err := makeOpErr("dial", "connect", syscall.ECONNREFUSED)
	wrapped := c.wrap("GET /v1/version", err)
	msg := wrapped.Error()
	for _, want := range []string{
		"peer main_pc",
		"GET /v1/version",
		"connection refused",
		// Original error should still be in the chain via %w.
		"192.0.2.1:7878",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("wrapped error missing %q: %s", want, msg)
		}
	}
	// errors.Is/As still works through the chain.
	if !errors.Is(wrapped, syscall.ECONNREFUSED) {
		t.Fatal("wrap broke errors.Is for ECONNREFUSED")
	}
}

// timeoutErr is a tiny error whose Timeout() method returns true,
// matching the net.Error interface used by *net.OpError.Timeout().
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

// Confirm at compile-time that timeoutErr satisfies net.Error so
// future refactors don't silently break the dial-timeout test.
var _ net.Error = timeoutErr{}

// Sanity: makeOpErr should produce an error where errors.Is can pick
// up the syscall constant — protects against the test helper drifting
// out of sync with the chain shape the real http.Client emits.
func TestMakeOpErrIsChainable(t *testing.T) {
	err := makeOpErr("dial", "connect", syscall.ECONNREFUSED)
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("test helper produced unchainable error: %v", err)
	}
	// Also exposes net.OpError via errors.As.
	var op *net.OpError
	if !errors.As(err, &op) {
		t.Fatalf("test helper hid *net.OpError: %v", err)
	}
	_ = fmt.Sprintf // keep fmt import even if no Sprintf usage in this test
}
