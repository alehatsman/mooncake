package discovery

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

// MDNSServiceType is the Bonjour service type agentd advertises and the
// controller queries. Spec-45: a fixed, mooncake-specific type so other
// `_*._tcp.local` services on the LAN don't collide with discovery
// results.
const MDNSServiceType = "_mooncake._tcp"

// MDNSDomain is the standard mDNS local domain. Bonjour callers always
// pair the service type with this domain.
const MDNSDomain = "local."

// AdvertiseOptions configures one mDNS advertisement. agentd's Serve()
// starts an advertise goroutine with these when AdvertiseMDNS is on.
type AdvertiseOptions struct {
	// InstanceName is the operator-visible name shown in `dns-sd` /
	// `avahi-browse` output. Defaults to the trimmed hostname when
	// empty (caller's responsibility — Advertise doesn't substitute).
	InstanceName string

	// Port is the agentd TCP bind port (the SRV record's port field).
	Port int

	// Version is the mooncake version string (TXT `ver=`).
	Version string

	// Hostname is the daemon's OS hostname after first-DNS-label trim
	// (TXT `hn=`). Operators usually want this to match peers.toml
	// names. If empty, the TXT record omits the hn= entry.
	Hostname string

	// SystemMode reports whether agentd is running as system or user
	// (TXT `sm=system` or `sm=user`). Lets controller-side filtering
	// distinguish the two without an extra round-trip.
	SystemMode bool
}

// txtRecord builds the TXT key=value list the controller's Query path
// parses. Spec-45 §mDNS shape — protocol version, hostname, mooncake
// version, mode. NEVER includes the bearer token or any sensitive data;
// discovery is informational only.
func (o AdvertiseOptions) txtRecord() []string {
	rec := []string{"v=1"}
	if o.Hostname != "" {
		rec = append(rec, "hn="+o.Hostname)
	}
	if o.Version != "" {
		rec = append(rec, "ver="+o.Version)
	}
	if o.SystemMode {
		rec = append(rec, "sm=system")
	} else {
		rec = append(rec, "sm=user")
	}
	return rec
}

// Advertise registers a `_mooncake._tcp.local` mDNS service for the
// duration of ctx. It blocks until ctx is canceled and then shuts the
// server down cleanly. Spec-45 §Task 1.
//
// Returns immediately with an error if registration fails (e.g. no
// usable network interface, port collision in zeroconf). Otherwise
// returns ctx.Err() (typically context.Canceled) when ctx unblocks.
//
// The caller is responsible for running this in its own goroutine when
// the daemon needs to keep serving other listeners — Advertise itself
// is synchronous.
func Advertise(ctx context.Context, opts AdvertiseOptions) error {
	if opts.InstanceName == "" {
		return fmt.Errorf("Advertise: InstanceName is required")
	}
	if opts.Port <= 0 {
		return fmt.Errorf("Advertise: Port must be > 0, got %d", opts.Port)
	}
	srv, err := zeroconf.Register(
		opts.InstanceName,
		MDNSServiceType,
		MDNSDomain,
		opts.Port,
		opts.txtRecord(),
		nil, // interfaces=nil → all multicast-capable interfaces
	)
	if err != nil {
		return fmt.Errorf("zeroconf register: %w", err)
	}
	defer srv.Shutdown()

	<-ctx.Done()
	return ctx.Err()
}

// MDNSQueryOptions tunes a controller-side mDNS lookup. The defaults
// match the spec-45 success-criteria timing.
type MDNSQueryOptions struct {
	// Timeout caps the wall-clock the query waits for responses.
	// Default: 3 seconds (mDNS responders on a LAN typically reply
	// in <100ms, but a few stragglers + cache misses can push it).
	Timeout time.Duration
}

// QueryMDNS sends a `_mooncake._tcp.local` Bonjour query and returns one
// Candidate per responder. Spec-45 §Task 2: the controller fans out a
// browse, waits up to Timeout, then closes. Each responder yields a
// candidate with Source=mdns and the TXT-record `hn=`/`ver=` echoed
// onto the candidate's fields.
//
// A timeout with zero results is not an error — it means "nothing on
// the LAN announced itself within the window." Most home networks
// won't have mDNS at all on the first run.
//
// Loopback-only test environments often return no results because mDNS
// requires multicast-capable interfaces; tests should be tolerant.
func QueryMDNS(ctx context.Context, opts MDNSQueryOptions) ([]Candidate, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("zeroconf resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 16)
	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := resolver.Browse(browseCtx, MDNSServiceType, MDNSDomain, entries); err != nil {
		return nil, fmt.Errorf("zeroconf browse: %w", err)
	}

	var out []Candidate
	for {
		select {
		case <-browseCtx.Done():
			return out, nil
		case e, ok := <-entries:
			if !ok {
				return out, nil
			}
			out = append(out, entryToCandidate(e))
		}
	}
}

// entryToCandidate parses one mDNS ServiceEntry into the Candidate
// shape Aggregate uses. The TXT record fields (v=, hn=, ver=, sm=) get
// pulled into Candidate's Hostname/AgentdVersion-equivalent fields when
// present; the address is composed from the entry's host+port for the
// peers.toml-compatible `addr` field.
func entryToCandidate(e *zeroconf.ServiceEntry) Candidate {
	// Prefer the operator-supplied hostname from TXT. Falls back to the
	// instance name when absent.
	name := e.Instance
	hn := ""
	ver := ""
	for _, t := range e.Text {
		k, v, ok := splitKV(t)
		if !ok {
			continue
		}
		switch k {
		case "hn":
			hn = v
		case "ver":
			ver = v
		}
	}
	if hn != "" {
		name = hn
	}

	// Prefer the first IPv4 the responder gave us. mDNS responders
	// can report multiple addresses (IPv4 + IPv6 + link-local); the
	// controller's transport works fine over either but IPv4 is what
	// most operators are reading in peers.toml.
	addr := ""
	if len(e.AddrIPv4) > 0 {
		addr = net.JoinHostPort(e.AddrIPv4[0].String(), strconv.Itoa(e.Port))
	} else if len(e.AddrIPv6) > 0 {
		addr = net.JoinHostPort(e.AddrIPv6[0].String(), strconv.Itoa(e.Port))
	}

	return Candidate{
		Name:          name,
		Addr:          addr,
		Sources:       []string{SourceMDNS},
		AgentdVersion: ver,
		// AgentdOK is false even when mDNS reported the peer: we
		// haven't probed /v1/version yet (and won't, in the simple
		// discover flow — no bearer token). Aggregate's optional
		// probe pass requires peers.toml for tokens.
	}
}

// splitKV is a tiny helper for TXT-record parsing. Returns (key, val,
// true) when s is `k=v`; otherwise (s, "", false). Mirrors Bonjour
// conventions — keys are case-sensitive lowercase by convention.
func splitKV(s string) (string, string, bool) {
	i := strings.IndexByte(s, '=')
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+1:], true
}
