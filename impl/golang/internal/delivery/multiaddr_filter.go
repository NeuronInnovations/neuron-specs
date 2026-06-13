package delivery

import (
	"net"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
)

// FilterMultiaddrs returns a copy of addrs with addresses that 009 FR-D11a
// declares ineligible for advertisement removed. The rule set, in order of
// evaluation per address:
//
//  1. Drop loopback addresses: IPv4 in `127.0.0.0/8`, IPv6 `::1`.
//  2. Drop IPv4 link-local: `169.254.0.0/16`.
//  3. Drop IPv6 link-local: `fe80::/10`.
//  4. Drop common virtual-interface prefixes when the multiaddr's IP falls
//     in the Docker default bridge range `172.17.0.0/16` or other private
//     ranges that are typically Docker/virtual on the seller side
//     (`172.18.0.0/16` … `172.31.0.0/16`). RFC1918 LAN ranges OUTSIDE the
//     Docker default `172.17–31/16` (i.e. `10/8`, `172.16/16`,
//     `192.168/16`) are retained because they are valid LAN-reachable
//     addresses for buyers on the same network.
//  5. Keep all other addresses, including:
//       - globally-routable IPv4 / IPv6,
//       - RFC1918 LAN (`10.0.0.0/8`, `192.168.0.0/16`, `172.16.0.0/16` —
//         the lone /16 retained from the 172.16/12 block),
//       - p2p-circuit relay paths (which begin `/p2p/.../p2p-circuit/...`),
//       - non-IP multiaddrs (DNS, onion, etc.).
//
// Per FR-D11a the publisher SHOULD listen on all interfaces
// (`/ip4/0.0.0.0/...`, `/ip6/::/...`) so `host.Addrs()` enumerates the full
// reachable set; this filter then prunes the set for advertisement.
//
// Filtering rules are evaluated against the multiaddr's IP component. The
// OS-interface-name dimension referenced in 009 FR-D11a (e.g. dropping
// addresses sourced from a `veth*` or `docker0` interface) cannot be
// inferred from the multiaddr alone — libp2p's `host.Addrs()` returns
// multiaddrs, not interface names. Callers that want interface-level
// filtering MUST consult `net.Interfaces()` separately and prune before
// passing to this helper; this helper covers the IP-based subset that is
// inferable from the multiaddr.
//
// The filter is pure (no side effects, no I/O) and safe for concurrent use.
func FilterMultiaddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		if !shouldAdvertise(addr) {
			continue
		}
		out = append(out, addr)
	}
	return out
}

// FilterMultiaddrStrings is the string-typed convenience over FilterMultiaddrs.
// Strings that fail to parse as a multiaddr are dropped silently (a malformed
// entry that can't be parsed cannot be reasoned about; callers needing strict
// validation should call FilterMultiaddrs directly on parsed multiaddrs).
func FilterMultiaddrStrings(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, s := range addrs {
		addr, err := ma.NewMultiaddr(s)
		if err != nil {
			continue
		}
		if !shouldAdvertise(addr) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// FilterPublicMultiaddrs is a STRICTER variant of FilterMultiaddrs for use
// in ConnectionSetup payloads, which must fit inside Hedera's HCS 1024-byte
// per-message limit after per-multiaddr ECIES encryption. In addition to
// everything FilterMultiaddrs drops (loopback, Docker bridge, link-local),
// this filter ALSO drops the remaining RFC1918 LAN ranges
// (10.0.0.0/8, 172.16.0.0/16, 192.168.0.0/16) which the AgentURI-advertisement
// filter retains for LAN-peer discovery.
//
// Rationale (Phase 3A live-smoke evidence, 2026-05-19):
// VPS 1 listened on 0.0.0.0 → enumerated 4 multiaddrs (10.15.0.5, 10.104.0.2,
// 127.0.0.1, 203.0.113.20). After FilterMultiaddrs: 3 addrs (loopback dropped,
// LAN retained). After ECIES-per-multiaddr encryption + envelope: 1109 bytes
// → exceeded HCS 1024 limit → seller crashed with `[MessageTooLarge]`.
// FilterPublicMultiaddrs would drop the two 10.x addresses and leave just
// the public 203.0.113.20 → ~330 bytes encrypted → well under the limit.
//
// Non-IP multiaddrs (DNS names, p2p-circuit relay paths) and globally-routable
// IPv4/IPv6 are KEPT — only the IP-based RFC1918 ranges are pruned beyond the
// FilterMultiaddrs baseline.
//
// The filter is pure (no side effects, no I/O) and safe for concurrent use.
func FilterPublicMultiaddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		if !shouldAdvertise(addr) {
			continue
		}
		if isRFC1918LAN(extractIP(addr)) {
			continue
		}
		out = append(out, addr)
	}
	return out
}

// isRFC1918LAN reports whether ip is in one of the three RFC1918 LAN blocks
// (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16). Used by FilterPublicMultiaddrs
// to drop addresses that are FilterMultiaddrs-eligible (LAN-reachable for
// the advertisement layer) but inappropriate for the HCS-bounded ConnectionSetup.
func isRFC1918LAN(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	// 10.0.0.0/8
	if ip4[0] == 10 {
		return true
	}
	// 172.16.0.0/12 (covers .16-.31; isDockerBridgeIP already dropped .17-.31,
	// but the publishable .16/16 lives here and should be dropped at the
	// public-only tier).
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	// 192.168.0.0/16
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	return false
}

// shouldAdvertise returns true if a multiaddr should be included in the
// advertised array per FR-D11a.
func shouldAdvertise(addr ma.Multiaddr) bool {
	// Non-IP multiaddrs (DNS, onion, p2p-circuit-only, etc.) advertise.
	// A p2p-circuit address may carry an IP segment for the relay; we
	// preserve those because the relay is the routing layer, not the
	// final destination.
	if strings.Contains(addr.String(), "/p2p-circuit") {
		return true
	}

	ip := extractIP(addr)
	if ip == nil {
		// No IP component (e.g. /dns/example.com/...) — advertise.
		return true
	}

	if ip.IsLoopback() {
		return false
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	if isDockerBridgeIP(ip) {
		return false
	}
	return true
}

// extractIP walks a multiaddr's components and returns the first IPv4 or
// IPv6 address found, or nil if none is present.
func extractIP(addr ma.Multiaddr) net.IP {
	var found net.IP
	ma.ForEach(addr, func(c ma.Component) bool {
		switch c.Protocol().Code {
		case ma.P_IP4:
			found = net.IP(c.RawValue()).To4()
			return false
		case ma.P_IP6:
			found = net.IP(c.RawValue()).To16()
			return false
		}
		return true
	})
	return found
}

// isDockerBridgeIP reports whether ip falls in the common Docker / virtual-
// bridge ranges. Specifically: `172.17.0.0/16` is the Docker default
// `docker0` range; `172.18.0.0/16` … `172.31.0.0/16` are the typical
// docker-compose user-defined-network ranges. We drop the entire
// `172.17.0.0/12` minus `172.16.0.0/16` block as "virtual unless proven
// otherwise"; operators that actually use `172.18.x.y` for a real LAN
// can override via their explicit listen configuration.
//
// RFC1918 ranges OUTSIDE this block (10/8, 192.168/16, 172.16/16) are
// retained: those are conventional LAN addressing for real networks.
func isDockerBridgeIP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	// 172.17.0.0 .. 172.31.255.255 (Docker default + compose user networks).
	if ip4[0] == 172 && ip4[1] >= 17 && ip4[1] <= 31 {
		return true
	}
	return false
}
