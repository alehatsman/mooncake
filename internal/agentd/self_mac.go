package agentd

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// macResponse is the wire shape for GET /v1/self/mac. Interface is
// informational — callers (fleet shutdown / fleet mac-refresh) store
// only MAC in peers.toml. EmptyMAC daemons (no usable NIC visible) reply
// 404 rather than emit a placeholder; an empty string in peers.toml
// would be a footgun.
type macResponse struct {
	MAC       string `json:"mac"`
	Interface string `json:"interface"`
}

// selfMACHandler picks the MAC of the interface that owns the inbound
// connection's local IP. That's the right choice for Wake-on-LAN: the
// controller will broadcast on whichever NIC is on the same subnet as
// the peer's reachable IP, so the matching MAC is the one we want to
// store in peers.toml.
//
// Fallbacks (unix-socket clients, or a TCP client whose local addr
// can't be matched to an interface) walk net.Interfaces() and pick
// the first up, non-loopback interface with an IPv4 address and a
// usable hardware addr.
func (s *Server) selfMACHandler(w http.ResponseWriter, r *http.Request) {
	mac, ifname, err := pickPeerMAC(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "no_mac", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, macResponse{MAC: mac, Interface: ifname})
}

// pickPeerMAC returns (mac, ifname) for the interface most useful to
// reach this daemon, given the inbound request. Error means no
// candidate interface was found.
func pickPeerMAC(r *http.Request) (mac, ifname string, err error) {
	// Try inbound-connection match first. http.LocalAddrContextKey is
	// set by net/http for each accepted conn.
	if v := r.Context().Value(http.LocalAddrContextKey); v != nil {
		if la, ok := v.(net.Addr); ok {
			if ip := extractIP(la); ip != nil && !ip.IsLoopback() {
				if iface := interfaceForIP(ip); iface != nil {
					if hw := normalizeHW(iface.HardwareAddr); hw != "" {
						return hw, iface.Name, nil
					}
				}
			}
		}
	}
	// Fallback: first usable non-loopback IPv4 interface.
	ifs, err := net.Interfaces()
	if err != nil {
		return "", "", fmt.Errorf("enumerate interfaces: %w", err)
	}
	for i := range ifs {
		iface := ifs[i]
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		hw := normalizeHW(iface.HardwareAddr)
		if hw == "" {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if ip := extractIP(a); ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				return hw, iface.Name, nil
			}
		}
	}
	return "", "", fmt.Errorf("no non-loopback interface with a usable MAC found")
}

// extractIP returns the IP component of a net.Addr regardless of its
// concrete type (TCPAddr, UDPAddr, IPAddr, IPNet).
func extractIP(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.TCPAddr:
		return v.IP
	case *net.UDPAddr:
		return v.IP
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	}
	return nil
}

// interfaceForIP returns the interface whose address list contains ip,
// or nil if none matches. Caller is expected to have already filtered
// out loopback / unset cases.
func interfaceForIP(ip net.IP) *net.Interface {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for i := range ifs {
		iface := ifs[i]
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if cand := extractIP(a); cand != nil && cand.Equal(ip) {
				return &iface
			}
		}
	}
	return nil
}

// normalizeHW lowercases hw.String() so the wire shape matches the
// canonical form fleet.NormalizeMAC produces; returns "" when hw is a
// zero-length addr (loopback, tun, p2p without a real link layer).
func normalizeHW(hw net.HardwareAddr) string {
	if len(hw) == 0 {
		return ""
	}
	return strings.ToLower(hw.String())
}
