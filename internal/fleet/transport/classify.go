package transport

// classify.go turns the opaque errors returned by net/http into short,
// human-friendly cause labels so `fleet status` and similar callers can
// distinguish "the host is down" from "the firewall is dropping packets"
// from "agentd crashed" without forcing the user to read a stack trace.
//
// Anything we can identify from the error chain is named explicitly; the
// ambiguous case ("timed out — could be three things") is named that way
// so the caller knows to fall through to `fleet doctor` for a real probe
// ladder rather than guessing.

import (
	"context"
	"errors"
	"net"
	"syscall"
)

// classifyNetErr inspects the error chain returned by http.Client.Do and
// returns a short cause label, or "" when the error doesn't match a
// known network-level failure (HTTP-status / decode errors are handled
// separately via httpErr).
func classifyNetErr(err error) string {
	if err == nil {
		return ""
	}

	// DNS first — independent of the *net.OpError path that follows for
	// dial-level failures, and the message benefits from including the
	// underlying resolver reason.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return "DNS lookup failed: host not found"
		}
		return "DNS lookup failed: " + dnsErr.Err
	}

	// Concrete syscall mapping. errors.Is(err, syscall.E*) works across
	// linux/darwin/windows since Go 1.21 — the standard library
	// translates WSAE* into the portable constants on Windows.
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused (port open but no listener — agentd not running?)"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection reset by peer (agentd crashed mid-response?)"
	case errors.Is(err, syscall.EHOSTUNREACH):
		return "host unreachable (routing / link down)"
	case errors.Is(err, syscall.ENETUNREACH):
		return "network unreachable"
	}

	// Context-level: caller's timeout fired before we got an answer.
	// We can't tell from this alone whether the SYN was dropped
	// (firewall filtering) or the host was down or the peer was just
	// slow — that's what `fleet doctor` is for. Name the ambiguity
	// explicitly so the user knows to escalate.
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out (host down, firewall filtering SYN, or peer overloaded)"
	}

	// Generic net.OpError with Timeout() — same ambiguity as above but
	// reported with finer granularity by the dialer. Recognise it so
	// the label doesn't become "i/o timeout" verbatim.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Timeout() {
			switch opErr.Op {
			case "dial":
				return "TCP connect timed out (host down or firewall filtering SYN)"
			case "read":
				return "read timeout (peer accepted connection but stopped responding)"
			}
			return "network timeout"
		}
	}

	return ""
}
