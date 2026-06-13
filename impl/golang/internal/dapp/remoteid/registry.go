package remoteid

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// RegisterResult summarises the on-chain side effects of a Remote ID
// seller registration. It's the evidence-grade record that gets
// written into a Level-R1 evidence artefact (registry evidence template).
// Notably: never includes the seller's private key or the operator's
// secret material. The seller's EVM is public; the agentURI SHA256 is a
// content-hash digest of what landed on-chain.
type RegisterResult struct {
	// SellerEVM is the seller's EVM address (childAddress).
	SellerEVM keylib.EVMAddress

	// RegistryAddress is the on-chain Identity Registry contract address.
	RegistryAddress keylib.EVMAddress

	// ChainID is the EVM chain id where the registry lives.
	ChainID uint64

	// TokenID is the EIP-8004 NFT tokenId minted by Register(). May be nil
	// if the underlying contract returned no tokenId (shouldn't happen on
	// a correctly-implemented contract; defensive against mocks).
	TokenID *big.Int

	// TransactionHash is the tx hash from the underlying contract's
	// Register call. May be a synthetic "0xtxhash_register" string when
	// running against MemoryRegistryContract.
	TransactionHash string

	// AgentURIJSON is the canonical JSON string that was written on-chain.
	AgentURIJSON string

	// AgentURISha256 is the hex-encoded SHA-256 of the AgentURI JSON.
	// Captured for evidence so a TEVV reviewer can confirm what the seller
	// asserts off-chain matches what landed on-chain.
	AgentURISha256 string

	// Outcome records whether this registration was freshly minted, had a
	// stale AgentURI refreshed, or was reused unchanged — so idempotent
	// re-runs (restart / post-rename) can be logged for evidence.
	Outcome registry.RegisterOutcome

	// Descriptor is the descriptor that backed the registration. Captured
	// so callers don't have to re-derive feedSource / commerceMode /
	// profileID for evidence logging.
	Descriptor ServiceDescriptor
}

// DiscoverResult summarises what a buyer learned about a seller through
// the EIP-8004 registry. Identity binding (the SellerEVM ↔ PeerID link)
// is the load-bearing fact for the Level-R1 demo.
type DiscoverResult struct {
	// SellerEVM is the EVM address the buyer looked up by.
	SellerEVM keylib.EVMAddress

	// RegistryAddress is the registry contract the buyer queried.
	RegistryAddress keylib.EVMAddress

	// ChainID is the chain id the buyer queried.
	ChainID uint64

	// TokenID is the seller's EIP-8004 token id.
	TokenID *big.Int

	// PeerID is the seller's libp2p PeerID, taken from the AgentURI's
	// neuron-p2p-exchange service. The buyer MUST cross-check this against
	// any multiaddr supplied out-of-band before dialling (per the R1
	// "no silent fallback" rule).
	PeerID string

	// Protocol is the libp2p stream protocol ID the seller advertises
	// (e.g. /ds240/raw/1.0.0). Equals ProtocolRaw for a conforming
	// Remote ID seller.
	Protocol string

	// CommerceService is the Remote ID neuron-commerce entry from the
	// AgentURI, or nil if none was published. Per FR-R01 a conforming
	// Remote ID seller MUST publish exactly one entry with name="remote-id".
	CommerceService *payment.NeuronCommerceService

	// AgentURIJSON is the raw JSON string read off-chain. Captured so
	// evidence artefacts can quote the exact bytes the buyer saw.
	AgentURIJSON string

	// AgentURISha256 is the hex SHA-256 of AgentURIJSON.
	AgentURISha256 string

	// agentURI is the parsed AgentURI retained so Stage 2b's CLI can
	// resolve seller stdIn/stdOut/stdErr topic locators (out of
	// NeuronTopicService.Config["topicId"]) and so Stage 3B's
	// ResolveDialAddrs can read the seller's listen multiaddrs out of
	// NeuronP2PExchangeService.Multiaddrs. Unexported — read via the
	// accessor helpers below.
	agentURI registry.AgentURI
}

// AgentURI returns the parsed AgentURI the buyer read off-chain.
func (d *DiscoverResult) AgentURI() registry.AgentURI { return d.agentURI }

// TopicConfigFor returns the locator the seller embedded in
// NeuronTopicService.Config["topicId"] for the given standard channel
// ("stdIn" / "stdOut" / "stdErr"). Returns "" when the channel is
// missing or the Config entry has no string topicId.
//
// Stage 2b: the seller populates this when it creates topics on the
// memory adapter; the buyer uses it to subscribe to the right topic
// ref without out-of-band coordination.
func (d *DiscoverResult) TopicConfigFor(channel string) string {
	for _, svc := range d.agentURI.TopicServices() {
		if svc.Channel != channel {
			continue
		}
		if svc.Config == nil {
			return ""
		}
		if v, ok := svc.Config["topicId"].(string); ok {
			return v
		}
	}
	return ""
}

// ErrNoRegistryMultiaddrs signals that the discovered AgentURI does not
// carry any usable multiaddrs in its NeuronP2PExchangeService — Stage 3B
// requires the seller to embed listen addresses so the buyer can dial
// without `--seller-multiaddr`. Callers branch on this error to drop to
// debug-override mode or refuse.
var ErrNoRegistryMultiaddrs = errors.New("remoteid: AgentURI carries no usable multiaddrs (Stage 3B requires the seller to embed listen addresses)")

// ResolveDialAddrs composes a libp2p AddrInfo from the discovered
// AgentURI's neuron-p2p-exchange service. Returns ErrNoRegistryMultiaddrs
// when the service has zero multiaddrs (the Stage 3B failure mode the
// buyer must surface clearly). Returns a parse error on a malformed
// PeerID or multiaddr.
//
// FR anchors:
//   - 003 (registry) — optional `multiaddrs` field on neuron-p2p-exchange
//     populated by the seller at register time.
//   - 017 (Remote ID DApp) — Stage 3B amendment requiring Remote ID
//     sellers to populate the field for registry-backed discovery.
func (d *DiscoverResult) ResolveDialAddrs() (peer.AddrInfo, error) {
	p2pServices := d.agentURI.P2PServices()
	if len(p2pServices) == 0 {
		return peer.AddrInfo{}, errors.New("remoteid.ResolveDialAddrs: AgentURI has no neuron-p2p-exchange service")
	}
	p2p := p2pServices[0]
	if len(p2p.Multiaddrs) == 0 {
		return peer.AddrInfo{}, ErrNoRegistryMultiaddrs
	}
	pid, err := peer.Decode(p2p.PeerID)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("remoteid.ResolveDialAddrs: decode PeerID %q: %w", p2p.PeerID, err)
	}
	addrs := make([]ma.Multiaddr, 0, len(p2p.Multiaddrs))
	for _, s := range p2p.Multiaddrs {
		m, perr := ma.NewMultiaddr(s)
		if perr != nil {
			return peer.AddrInfo{}, fmt.Errorf("remoteid.ResolveDialAddrs: parse multiaddr %q: %w", s, perr)
		}
		addrs = append(addrs, m)
	}
	return peer.AddrInfo{ID: pid, Addrs: addrs}, nil
}

// RegisterSeller registers a Remote ID seller in an EIP-8004 Identity
// Registry. It wraps registry.Register so callers in cmd/remoteid-seller
// (and tests) don't have to re-state the permissionless-admission /
// agentURI-validation boilerplate.
//
// FR anchors:
//   - 017 FR-R01: seller MUST register with a neuron-commerce entry whose
//     name is "remote-id" and pricing.unit is "frame".
//   - 003 FR-R02 / V-REG-12: childAddress comes from the same key that
//     produced the descriptor's PeerID.
//
// The registry contract is taken by interface so callers pass either a
// real EVMRegistryContract or a MemoryRegistryContract.
func RegisterSeller(
	ctx context.Context,
	childKey *keylib.NeuronPrivateKey,
	descriptor ServiceDescriptor,
	registryAddr keylib.EVMAddress,
	chainID uint64,
	contract registry.RegistryContract,
) (RegisterResult, error) {
	if childKey == nil {
		return RegisterResult{}, errors.New("remoteid.RegisterSeller: childKey is required")
	}
	if contract == nil {
		return RegisterResult{}, errors.New("remoteid.RegisterSeller: contract is required")
	}

	// RegisterOrUpdate is idempotent: it mints when unregistered, refreshes a
	// stale AgentURI (e.g. after the /ds240 rename), and reuses an up-to-date
	// registration — so a re-run never fails with "duplicate registration".
	result, outcome, err := registry.RegisterOrUpdate(
		ctx, childKey, registryAddr, chainID,
		descriptor.AgentURI, contract,
		registry.PermissionlessPolicy{}, "",
	)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("remoteid.RegisterSeller: %w", err)
	}

	agentURIJSON := result.AgentURIString()
	sha := sha256.Sum256([]byte(agentURIJSON))

	return RegisterResult{
		SellerEVM:       result.ChildAddress(),
		RegistryAddress: result.RegistryAddress(),
		ChainID:         result.ChainId(),
		TokenID:         result.TokenId(),
		TransactionHash: result.TransactionHash(),
		AgentURIJSON:    agentURIJSON,
		AgentURISha256:  hex.EncodeToString(sha[:]),
		Outcome:         outcome,
		Descriptor:      descriptor,
	}, nil
}

// DiscoverSeller looks a Remote ID seller up in the EIP-8004 registry by
// its EVM address and extracts the Remote ID-specific service descriptor
// view (PeerID, protocol id, commerce entry).
//
// FR anchors:
//   - 003 FR-R02 + FR-R05: lookup by EVM address is the canonical buyer-side
//     discovery path.
//   - 017 FR-R01: returned CommerceService is filtered by name="remote-id".
//   - 009 FR-D-*: caller is expected to use Protocol to open the libp2p
//     stream; PeerID lets the caller validate any out-of-band multiaddr
//     before dialling (the R1 "no silent fallback" discipline).
func DiscoverSeller(
	ctx context.Context,
	sellerEVM keylib.EVMAddress,
	registryAddr keylib.EVMAddress,
	chainID uint64,
	contract registry.RegistryContract,
) (DiscoverResult, error) {
	if contract == nil {
		return DiscoverResult{}, errors.New("remoteid.DiscoverSeller: contract is required")
	}

	reg, err := registry.LookupRegistration(
		ctx, registryAddr, chainID,
		registry.ByEVMAddress(sellerEVM), contract,
	)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("remoteid.DiscoverSeller: %w", err)
	}

	p2p, err := registry.ResolveP2PExchange(reg)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("remoteid.DiscoverSeller: resolve p2p service: %w", err)
	}

	uri := reg.AgentURI()
	commerceServices := payment.FilterByName(uri.CommerceServices(), CommerceServiceName)
	var commerce *payment.NeuronCommerceService
	if len(commerceServices) > 0 {
		c := commerceServices[0]
		commerce = &c
	}

	// Read the agentURI JSON for evidence. The registry already gave us a
	// parsed AgentURI, but TEVV evidence wants the bytes-as-stored.
	agentURIJSON, marshalErr := uri.ToJSON()
	if marshalErr != nil {
		return DiscoverResult{}, fmt.Errorf("remoteid.DiscoverSeller: serialize agentURI for evidence: %w", marshalErr)
	}
	sha := sha256.Sum256([]byte(agentURIJSON))

	return DiscoverResult{
		SellerEVM:       sellerEVM,
		RegistryAddress: registryAddr,
		ChainID:         chainID,
		TokenID:         reg.TokenId(),
		PeerID:          p2p.PeerID,
		Protocol:        p2p.Protocol,
		CommerceService: commerce,
		AgentURIJSON:    agentURIJSON,
		AgentURISha256:  hex.EncodeToString(sha[:]),
		agentURI:        uri,
	}, nil
}
