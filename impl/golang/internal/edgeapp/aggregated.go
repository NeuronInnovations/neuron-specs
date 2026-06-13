package edgeapp

import (
	"crypto/ecdsa"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// SellerEntry identifies one seller a buyer should connect to. Multi-seller
// configurations pass a list of these in BuyerConfig.Sellers.
type SellerEntry struct {
	// StdIn is the seller's stdIn HCS topic — where the buyer publishes
	// its ReverseConnectionSetup payload. Required.
	StdIn topic.TopicRef

	// StdOut is the seller's stdOut topic — where the seller publishes
	// its spec-005 heartbeats. Optional today; required when
	// BuyerConfig.EnforceDeadlines=true so the buyer can observe seller
	// liveness independent of stream death.
	StdOut topic.TopicRef

	// PubKey is the seller's secp256k1 ECDSA public key. The buyer
	// ECIES-encrypts its multiaddrs to this key so only this seller can
	// dial back. Required.
	PubKey *ecdsa.PublicKey

	// DisplayName is an optional human-readable label that appears in logs
	// and aggregated frame output. If empty, the seller's EVM-address
	// prefix is used instead.
	DisplayName string
}

// AggregatedFrame is one Mode-S record received from a seller, decorated
// with seller identity and a best-effort Mode-S meta decoding (DF + ICAO
// when extractable).
//
// AggregatedFrame is the unit the buyer hands to OnAggregatedFrame and
// the OutputSink layer.
type AggregatedFrame struct {
	// SellerEVM is the seller's EVM address ("0x…40 hex chars"). It is
	// derived from the seller's PeerID via the libp2p secp256k1 → PeerID
	// mapping; for any active stream this matches a SellerEntry the buyer
	// was configured with.
	SellerEVM string `json:"sellerEVM"`

	// SellerName is the SellerEntry.DisplayName, or the lowercase 4-hex
	// EVM-address prefix if no display name was set.
	SellerName string `json:"sellerName"`

	// SellerPeerID is the libp2p multihash form of the seller's identity
	// ("12D3KooW…"), useful for cross-referencing libp2p logs.
	SellerPeerID string `json:"sellerPeerID"`

	// Frame carries the raw Mode-S bytes plus the GPS timestamp the
	// receiver attached. See feeds.FeedFrame for the field semantics.
	Frame feeds.FeedFrame `json:"frame"`

	// Meta is the best-effort Mode-S parse: the DF (downlink format) and,
	// for DF 11/17/18, the ICAO24 in 6-hex form.
	Meta feeds.ModeSMeta `json:"meta"`

	// ReceivedAt is the wall-clock time the buyer enqueued this frame for
	// downstream processing — distinct from Frame.Rx (set on dequeue from
	// the wire) by goroutine-scheduling latency only.
	ReceivedAt time.Time `json:"receivedAt"`
}

// SellerState classifies a seller's connection status from the buyer's
// perspective.
type SellerState string

const (
	// SellerStateConnecting — buyer has published its ReverseConnectionSetup
	// to the seller's stdIn and is waiting for the seller to dial in.
	SellerStateConnecting SellerState = "connecting"

	// SellerStateConnected — the seller has dialed in and an active libp2p
	// stream is delivering frames.
	SellerStateConnected SellerState = "connected"

	// SellerStateDisconnected — a previously-connected seller's stream
	// closed gracefully (peer closed, ctx cancelled, etc.).
	SellerStateDisconnected SellerState = "disconnected"

	// SellerStateError — the seller's stream closed with an error or the
	// initial setup-publish / dial-wait failed.
	SellerStateError SellerState = "error"
)

// SellerStatus is a snapshot of one seller's connection from the buyer's
// perspective. RunBuyer fires OnSellerStatus on each state transition.
type SellerStatus struct {
	EVM            string      `json:"evm"`
	DisplayName    string      `json:"displayName"`
	PeerID         string      `json:"peerID"`
	State          SellerState `json:"state"`
	FramesReceived uint64      `json:"framesReceived"`
	LastFrameAt    time.Time   `json:"lastFrameAt,omitzero"`
	LastError      string      `json:"lastError,omitempty"`
}

// abbrEVM returns a lowercase 4-hex prefix of an EVM address for use as a
// fallback display name.
func abbrEVM(evm string) string {
	s := evm
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	if len(s) > 6 {
		s = s[:6]
	}
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'F' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}
