// Package wol builds and sends Wake-on-LAN magic packets.
//
// A magic packet is a 102-byte UDP payload: six 0xFF bytes followed by
// 16 repetitions of the target's 6-byte MAC. NICs configured for WoL
// scan inbound frames for this pattern at the link layer (before the
// host wakes), so the packet must reach the target's broadcast domain
// — typically that means the controller is on the same LAN segment.
//
// This package deliberately stays small and dependency-free: the fleet
// CLI (cmd/fleet_up.go) does selection and config; tests construct a
// loopback UDP server to verify the byte pattern without touching the
// real network.
package wol

import (
	"fmt"
	"net"
)

// DefaultBroadcast is the destination most consumer routers and NICs
// accept for WoL. Port 9 (discard) is conventional; the payload is
// inspected by the NIC, not by any listening service, so the port is
// effectively a label.
const DefaultBroadcast = "255.255.255.255:9"

// BuildMagicPacket returns the 102-byte WoL payload for mac. The
// returned slice is owned by the caller.
func BuildMagicPacket(mac net.HardwareAddr) ([]byte, error) {
	if len(mac) != 6 {
		return nil, fmt.Errorf("wol: MAC must be 6 bytes, got %d", len(mac))
	}
	pkt := make([]byte, 6+16*6)
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(pkt[6+i*6:], mac)
	}
	return pkt, nil
}

// Send dials broadcastAddr as UDP, enables SO_BROADCAST, and writes a
// magic packet for mac. broadcastAddr may be a directed broadcast
// (e.g. "10.0.0.255:9") for routed networks where 255.255.255.255 is
// blocked by the gateway. Pass "" to use DefaultBroadcast.
func Send(mac net.HardwareAddr, broadcastAddr string) error {
	if broadcastAddr == "" {
		broadcastAddr = DefaultBroadcast
	}
	pkt, err := BuildMagicPacket(mac)
	if err != nil {
		return err
	}
	raddr, err := net.ResolveUDPAddr("udp", broadcastAddr)
	if err != nil {
		return fmt.Errorf("wol: resolve %s: %w", broadcastAddr, err)
	}
	// net.DialUDP sets SO_BROADCAST implicitly when raddr is a broadcast
	// address on Linux/macOS; explicit setsockopt isn't needed for the
	// standard "255.255.255.255" case. For directed broadcasts the kernel
	// also routes correctly without extra socket options.
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return fmt.Errorf("wol: dial %s: %w", broadcastAddr, err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write(pkt); err != nil {
		return fmt.Errorf("wol: write to %s: %w", broadcastAddr, err)
	}
	return nil
}
