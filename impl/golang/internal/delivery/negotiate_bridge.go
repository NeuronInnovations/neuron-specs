package delivery

import (
	"crypto/ecdsa"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// BuildSetupOption configures a ConnectionSetup builder.
//
// Phase-4 introduces the streams[] catalog (008 FR-P33a). To preserve
// every existing call site verbatim, the catalog is wired via an
// optional functional option rather than a new mandatory parameter.
// Callers that want a single-entry catalog for back-compat advertisement
// pass WithStreams(...); callers that don't pass it get the legacy
// behavior (Protocol field only).
type BuildSetupOption func(*payment.ConnectionSetup)

// WithStreams attaches a streams[] catalog to the ConnectionSetup
// alongside the legacy Protocol field. Per 008 FR-P33a both fields MAY
// be present at once: Phase-2 buyers continue to read Protocol; Phase-5+
// buyers prefer Streams when present.
func WithStreams(entries []payment.StreamCatalogEntry) BuildSetupOption {
	return func(s *payment.ConnectionSetup) {
		// Copy to defend against caller mutation post-build.
		if len(entries) == 0 {
			return
		}
		s.Streams = append(s.Streams[:0:0], entries...)
	}
}

// BuildConnectionSetup creates a payment.ConnectionSetup for the seller to
// send to the buyer after negotiation completes. It collects the seller's
// libp2p listen addresses, encrypts them with the buyer's public key using
// the ECIES profile (FR-D11), and populates all required ConnectionSetup
// fields (FR-P33).
//
// FR-D15: Bridges spec 008 negotiation to spec 009 P2P delivery.
// FR-P33: connectionSetup carries encrypted multiaddrs for peer address exchange.
func BuildConnectionSetup(
	requestID string,
	h host.Host,
	protocol string,
	buyerPubKey *ecdsa.PublicKey,
	opts ...BuildSetupOption,
) (*payment.ConnectionSetup, error) {
	const op = "BuildConnectionSetup"

	if buyerPubKey == nil {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op,
			"buyer public key is nil")
	}

	// Collect multiaddrs from the libp2p host.
	//
	// 009 FR-D11a: filter out loopback / Docker bridge / link-local before
	// advertisement. The publisher SHOULD listen on all interfaces so
	// host.Addrs() yields the full reachable set.
	//
	// Phase 5 B3 (2026-05-20): for ConnectionSetup specifically, apply the
	// STRICTER FilterPublicMultiaddrs which ALSO drops RFC1918 LAN ranges.
	// Rationale: ConnectionSetup payload is ECIES-encrypted-per-multiaddr
	// then HCS-published, and the HCS 1024-byte per-message limit caps how
	// many multiaddrs can fit. A live smoke run had a seller bind
	// 0.0.0.0 → 4 multiaddrs → 1109 bytes encrypted → seller crashed
	// with `[MessageTooLarge]`. The public-only filter keeps just the
	// externally-reachable IP(s).
	//
	// Fallback chain: try the strict (public-only) filter first; if it
	// would yield zero addresses (e.g., a same-host loopback-only test, or
	// a production host with only LAN addresses), fall back to the lenient
	// (FilterMultiaddrs) result. If even THAT is empty, retain the raw
	// unfiltered set rather than fail. Filtering is HCS-budget + leak
	// hygiene, not a hard block.
	raw := h.Addrs()
	if len(raw) == 0 {
		return nil, NewDeliveryError(ErrNoCompatibleTransport, op,
			"host has no listen addresses")
	}
	addrs := FilterPublicMultiaddrs(raw)
	if len(addrs) == 0 {
		// Fall back to the lenient filter (loopback/Docker/link-local
		// dropped; LAN retained). This is the legacy behavior — useful
		// for same-LAN smoke harnesses.
		addrs = FilterMultiaddrs(raw)
	}
	if len(addrs) == 0 {
		// Fall all the way back to raw (loopback-only test paths).
		addrs = raw
	}

	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	// Encrypt multiaddrs with buyer's public key (FR-D11, FR-D12, FR-D13).
	encrypted, err := EncryptMultiaddrs(addrStrings, buyerPubKey)
	if err != nil {
		return nil, err // Already a DeliveryError from ecies.go
	}

	// Get PeerID from the host (derived from the seller's secp256k1 key).
	peerID := h.ID().String()

	setup := &payment.ConnectionSetup{
		Type:                "connectionSetup",
		Version:             "1.0.0",
		RequestID:           requestID,
		PeerID:              peerID,
		EncryptedMultiaddrs: encrypted,
		Protocol:            protocol,
	}

	// Apply Phase-4 streams[] catalog and any other functional options.
	for _, opt := range opts {
		opt(setup)
	}

	return setup, nil
}

// ConnectFromSetup processes a payment.ConnectionSetup message received by the
// buyer, decrypts the seller's multiaddrs, and establishes a delivery channel
// via the provided adapter.
//
// FR-D15: decrypt → validate → connect.
// FR-P33: Completes the connectionSetup → delivery channel handoff.
func ConnectFromSetup(
	adapter DeliveryAdapter,
	setup *payment.ConnectionSetup,
	recipientPrivKey *ecdsa.PrivateKey,
) (*DeliveryChannel, error) {
	const op = "ConnectFromSetup"

	if setup == nil {
		return nil, NewDeliveryError(ErrDialFailed, op,
			"ConnectionSetup is nil")
	}

	if recipientPrivKey == nil {
		return nil, NewDeliveryError(ErrConnectionSetupEncryptionFailed, op,
			"recipient private key is nil")
	}

	// Step 1: Decrypt and validate via ProcessConnectionSetup (FR-D15).
	result, err := ProcessConnectionSetup(
		setup.PeerID,
		setup.EncryptedMultiaddrs,
		setup.Protocol,
		setup.NATStatus,
		recipientPrivKey,
	)
	if err != nil {
		return nil, err // Already a DeliveryError from setup.go
	}

	// Step 2: Establish the delivery channel via the adapter (FR-D02).
	channel, err := adapter.Connect(result.PeerID, result.Multiaddrs, result.Protocol, nil)
	if err != nil {
		return nil, err // Already a DeliveryError from adapter
	}

	return channel, nil
}
