package delivery

import (
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FR-D11a: the advertised array MUST include LAN (RFC1918), public, and
// active relay-circuit addresses; MUST exclude loopback, Docker bridge,
// virtual interfaces, and link-local.

func mustAddr(t *testing.T, s string) ma.Multiaddr {
	t.Helper()
	addr, err := ma.NewMultiaddr(s)
	require.NoError(t, err, "failed to parse multiaddr %q", s)
	return addr
}

func TestFilterMultiaddrs_DropsLoopbackIPv4(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip4/127.0.0.1/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Empty(t, got, "loopback IPv4 MUST be filtered")
}

func TestFilterMultiaddrs_DropsLoopbackIPv6(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip6/::1/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Empty(t, got, "IPv6 loopback MUST be filtered")
}

func TestFilterMultiaddrs_DropsIPv4LinkLocal(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip4/169.254.1.5/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Empty(t, got, "IPv4 link-local 169.254/16 MUST be filtered")
}

func TestFilterMultiaddrs_DropsIPv6LinkLocal(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip6/fe80::1/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Empty(t, got, "IPv6 link-local fe80::/10 MUST be filtered")
}

func TestFilterMultiaddrs_DropsDockerBridge(t *testing.T) {
	t.Parallel()
	// 172.17.0.0/16 is the Docker default bridge.
	in := []ma.Multiaddr{mustAddr(t, "/ip4/172.17.0.2/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Empty(t, got, "Docker default bridge 172.17/16 MUST be filtered")
}

func TestFilterMultiaddrs_DropsDockerComposeBridges(t *testing.T) {
	t.Parallel()
	// 172.18.0.0 .. 172.31.255.255 — typical docker-compose user networks.
	for _, addr := range []string{
		"/ip4/172.18.0.5/udp/4001/quic-v1",
		"/ip4/172.20.1.1/udp/4001/quic-v1",
		"/ip4/172.31.255.10/udp/4001/quic-v1",
	} {
		got := FilterMultiaddrs([]ma.Multiaddr{mustAddr(t, addr)})
		assert.Empty(t, got, "%q should be filtered as docker-compose range", addr)
	}
}

func TestFilterMultiaddrs_KeepsLAN10(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip4/10.0.5.42/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Len(t, got, 1, "RFC1918 10.0.0.0/8 MUST be retained")
}

func TestFilterMultiaddrs_KeepsLAN192_168(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip4/192.168.1.100/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Len(t, got, 1, "RFC1918 192.168.0.0/16 MUST be retained")
}

func TestFilterMultiaddrs_KeepsLAN172_16(t *testing.T) {
	t.Parallel()
	// 172.16.0.0/16 (NOT the Docker bridge range) — should pass.
	in := []ma.Multiaddr{mustAddr(t, "/ip4/172.16.1.50/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Len(t, got, 1, "RFC1918 172.16.0.0/16 (non-Docker) MUST be retained")
}

func TestFilterMultiaddrs_KeepsPublicIPv4(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip4/203.0.113.42/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Len(t, got, 1, "globally-routable IPv4 MUST be retained")
}

func TestFilterMultiaddrs_KeepsPublicIPv6(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip6/2001:db8::1/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Len(t, got, 1, "globally-routable IPv6 MUST be retained")
}

func TestFilterMultiaddrs_KeepsDNS(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/dns/example.com/udp/4001/quic-v1")}
	got := FilterMultiaddrs(in)
	assert.Len(t, got, 1, "DNS multiaddrs (no IP component) MUST be retained")
}

func TestFilterMultiaddrs_KeepsRelayCircuit(t *testing.T) {
	t.Parallel()
	// Relay-circuit multiaddrs carry the relay's IP in their first segment;
	// we keep them regardless because the relay is the routing layer.
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWAAAA/p2p-circuit/p2p/12D3KooWBBBB"),
	}
	got := FilterMultiaddrs(in)
	assert.Len(t, got, 1, "relay-circuit multiaddrs MUST be retained per FR-D11a")
}

func TestFilterMultiaddrs_MixedSet(t *testing.T) {
	t.Parallel()
	// Realistic host.Addrs() output combining loopback + Docker + LAN +
	// public + relay-circuit. Result must contain ONLY the three valid
	// entries.
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/127.0.0.1/udp/4001/quic-v1"),                                                              // loopback — drop
		mustAddr(t, "/ip4/172.17.0.2/udp/4001/quic-v1"),                                                             // docker0 — drop
		mustAddr(t, "/ip4/192.168.1.100/udp/4001/quic-v1"),                                                          // LAN — keep
		mustAddr(t, "/ip4/203.0.113.42/udp/4001/quic-v1"),                                                           // public — keep
		mustAddr(t, "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/12D3KooWAAAA/p2p-circuit/p2p/12D3KooWBBBB"),              // relay circuit — keep
		mustAddr(t, "/ip6/::1/udp/4001/quic-v1"),                                                                    // IPv6 loopback — drop
	}
	got := FilterMultiaddrs(in)
	require.Len(t, got, 3)
	assert.Contains(t, got[0].String(), "192.168.1.100")
	assert.Contains(t, got[1].String(), "203.0.113.42")
	assert.Contains(t, got[2].String(), "p2p-circuit")
}

func TestFilterMultiaddrStrings_RoundTrip(t *testing.T) {
	t.Parallel()
	in := []string{
		"/ip4/127.0.0.1/udp/4001/quic-v1",
		"/ip4/10.0.0.5/udp/4001/quic-v1",
		"/ip4/172.17.0.1/udp/4001/quic-v1",
		"/ip4/8.8.8.8/udp/4001/quic-v1",
		"not-a-multiaddr-at-all",
	}
	got := FilterMultiaddrStrings(in)
	assert.Equal(t, []string{
		"/ip4/10.0.0.5/udp/4001/quic-v1",
		"/ip4/8.8.8.8/udp/4001/quic-v1",
	}, got, "string filter drops loopback + Docker + malformed; keeps LAN + public")
}

func TestFilterMultiaddrs_EmptyInput(t *testing.T) {
	t.Parallel()
	got := FilterMultiaddrs(nil)
	assert.Empty(t, got)
}

func TestShouldAdvertise_RetainsLAN172_16_DropsDocker172_17(t *testing.T) {
	t.Parallel()
	// Boundary case: 172.16/16 is the lone /16 retained from the 172.16/12
	// RFC1918 block; 172.17–31/16 are dropped as docker-like.
	keep := mustAddr(t, "/ip4/172.16.255.255/udp/4001/quic-v1")
	drop := mustAddr(t, "/ip4/172.17.0.0/udp/4001/quic-v1")

	assert.True(t, shouldAdvertise(keep), "172.16/16 must be retained")
	assert.False(t, shouldAdvertise(drop), "172.17/16 must be dropped")
}

// FR-D11a (advertisement hygiene) when a host listens on a real mix
// (loopback + LAN + public) — the filter MUST prune the loopback while
// retaining LAN and public.
func TestFilterMultiaddrs_RealisticHostAddrsSet(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/127.0.0.1/tcp/4001"),
		mustAddr(t, "/ip4/127.0.0.1/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/10.0.0.5/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/192.0.2.42/udp/4001/quic-v1"),
		mustAddr(t, "/ip6/2001:db8::abcd/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/172.17.0.99/udp/4001/quic-v1"),
	}
	got := FilterMultiaddrs(in)
	require.Len(t, got, 3, "expected loopback + docker dropped; LAN + public retained")
}

// Filter MUST be idempotent — applying twice produces the same set.
func TestFilterMultiaddrs_Idempotent(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/127.0.0.1/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/10.0.0.5/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/172.17.0.1/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/8.8.8.8/udp/4001/quic-v1"),
	}
	once := FilterMultiaddrs(in)
	twice := FilterMultiaddrs(once)
	assert.Equal(t, once, twice)
}

// ============================================================================
// Phase 5 B3 — FilterPublicMultiaddrs (stricter variant for ConnectionSetup)
// ============================================================================

func TestFilterPublicMultiaddrs_DropsLoopback(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/127.0.0.1/udp/4001/quic-v1"),
		mustAddr(t, "/ip6/::1/udp/4001/quic-v1"),
	}
	got := FilterPublicMultiaddrs(in)
	assert.Empty(t, got, "loopback MUST be dropped (inherited from FilterMultiaddrs)")
}

func TestFilterPublicMultiaddrs_DropsDockerBridge(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/172.17.0.1/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/172.20.0.5/udp/4001/quic-v1"),
	}
	got := FilterPublicMultiaddrs(in)
	assert.Empty(t, got, "Docker bridge MUST be dropped")
}

func TestFilterPublicMultiaddrs_DropsLAN10_8(t *testing.T) {
	t.Parallel()
	// 10.0.0.0/8 is KEPT by FilterMultiaddrs (LAN-reachable for advertisement)
	// but MUST be dropped by FilterPublicMultiaddrs (HCS-bounded ConnectionSetup).
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/10.15.0.5/udp/41001/quic-v1"),
		mustAddr(t, "/ip4/10.104.0.2/udp/41001/quic-v1"),
	}
	got := FilterPublicMultiaddrs(in)
	assert.Empty(t, got, "10/8 LAN MUST be dropped at the public-only tier")
}

func TestFilterPublicMultiaddrs_DropsLAN192_168(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{mustAddr(t, "/ip4/192.168.1.42/udp/4001/quic-v1")}
	got := FilterPublicMultiaddrs(in)
	assert.Empty(t, got, "192.168/16 MUST be dropped at the public-only tier")
}

func TestFilterPublicMultiaddrs_DropsLAN172_16(t *testing.T) {
	t.Parallel()
	// 172.16.0.0/16 is the singleton /16 retained by FilterMultiaddrs from
	// the broader 172.16/12 block. FilterPublicMultiaddrs drops the full
	// 172.16/12 (16-31).
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/172.16.0.1/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/172.31.255.254/udp/4001/quic-v1"),
	}
	got := FilterPublicMultiaddrs(in)
	assert.Empty(t, got, "172.16/12 MUST be dropped at the public-only tier")
}

func TestFilterPublicMultiaddrs_KeepsPublicIPv4(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/8.8.8.8/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/203.0.113.20/udp/41001/quic-v1"),
	}
	got := FilterPublicMultiaddrs(in)
	require.Len(t, got, 2, "public IPv4 MUST be kept")
}

func TestFilterPublicMultiaddrs_KeepsDNS(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{
		mustAddr(t, "/dns4/example.com/udp/4001/quic-v1"),
		mustAddr(t, "/dnsaddr/relay.libp2p.example/tcp/443/wss"),
	}
	got := FilterPublicMultiaddrs(in)
	require.Len(t, got, 2, "DNS-based multiaddrs MUST be kept (no IP component to filter)")
}

func TestFilterPublicMultiaddrs_RealisticVPS1Mix(t *testing.T) {
	t.Parallel()
	// This is the exact host.Addrs() shape that crashed VPS 1 in Phase 3A
	// attempt #1 (limitations.md gotcha #1): seller bound to 0.0.0.0 and
	// libp2p enumerated 2 private + 1 loopback + 1 public.
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/10.15.0.5/udp/41001/quic-v1"),
		mustAddr(t, "/ip4/10.104.0.2/udp/41001/quic-v1"),
		mustAddr(t, "/ip4/127.0.0.1/udp/41001/quic-v1"),
		mustAddr(t, "/ip4/203.0.113.20/udp/41001/quic-v1"),
	}
	got := FilterPublicMultiaddrs(in)
	require.Len(t, got, 1, "only the public addr should remain")
	assert.Contains(t, got[0].String(), "203.0.113.20", "public IP retained")
}

func TestFilterPublicMultiaddrs_Idempotent(t *testing.T) {
	t.Parallel()
	in := []ma.Multiaddr{
		mustAddr(t, "/ip4/127.0.0.1/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/10.0.0.5/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/192.168.1.1/udp/4001/quic-v1"),
		mustAddr(t, "/ip4/8.8.8.8/udp/4001/quic-v1"),
	}
	once := FilterPublicMultiaddrs(in)
	twice := FilterPublicMultiaddrs(once)
	assert.Equal(t, once, twice)
}
