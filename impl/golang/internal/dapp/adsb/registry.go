package adsb

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

// RegisterResult summarises the on-chain side effects of an ADS-B BaseStation
// seller registration. Evidence-grade record for TEVV artefacts; never
// includes private key material.
type RegisterResult struct {
	SellerEVM       keylib.EVMAddress
	RegistryAddress keylib.EVMAddress
	ChainID         uint64
	TokenID         *big.Int
	TransactionHash string
	AgentURIJSON    string
	AgentURISha256  string
	// Outcome records whether this registration was freshly minted, had a
	// stale AgentURI refreshed, or was reused unchanged — so idempotent
	// re-runs (restart / post-rename) can be logged for evidence.
	Outcome    registry.RegisterOutcome
	Descriptor ServiceDescriptor
}

// DiscoverResult summarises what a buyer learned about an ADS-B seller via
// the EIP-8004 registry.
type DiscoverResult struct {
	SellerEVM       keylib.EVMAddress
	RegistryAddress keylib.EVMAddress
	ChainID         uint64
	TokenID         *big.Int
	PeerID          string
	Protocol        string
	CommerceService *payment.NeuronCommerceService
	AgentURIJSON    string
	AgentURISha256  string

	agentURI registry.AgentURI
}

// AgentURI returns the parsed AgentURI the buyer read off-chain.
func (d *DiscoverResult) AgentURI() registry.AgentURI { return d.agentURI }

// TopicConfigFor returns the locator the seller embedded in the named
// standard channel's NeuronTopicService.Config["topicId"]. Returns "" when
// the channel is missing or has no string topicId.
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
// carry any usable multiaddrs in its NeuronP2PExchangeService.
var ErrNoRegistryMultiaddrs = errors.New("adsb: AgentURI carries no usable multiaddrs (Stage 3B requires the seller to embed listen addresses)")

// ResolveDialAddrs composes a libp2p AddrInfo from the discovered AgentURI's
// neuron-p2p-exchange service. Returns ErrNoRegistryMultiaddrs when the
// service has zero multiaddrs.
func (d *DiscoverResult) ResolveDialAddrs() (peer.AddrInfo, error) {
	p2pServices := d.agentURI.P2PServices()
	if len(p2pServices) == 0 {
		return peer.AddrInfo{}, errors.New("adsb.ResolveDialAddrs: AgentURI has no neuron-p2p-exchange service")
	}
	p2p := p2pServices[0]
	if len(p2p.Multiaddrs) == 0 {
		return peer.AddrInfo{}, ErrNoRegistryMultiaddrs
	}
	pid, err := peer.Decode(p2p.PeerID)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("adsb.ResolveDialAddrs: decode PeerID %q: %w", p2p.PeerID, err)
	}
	addrs := make([]ma.Multiaddr, 0, len(p2p.Multiaddrs))
	for _, s := range p2p.Multiaddrs {
		m, perr := ma.NewMultiaddr(s)
		if perr != nil {
			return peer.AddrInfo{}, fmt.Errorf("adsb.ResolveDialAddrs: parse multiaddr %q: %w", s, perr)
		}
		addrs = append(addrs, m)
	}
	return peer.AddrInfo{ID: pid, Addrs: addrs}, nil
}

// RegisterSeller registers an ADS-B BaseStation seller in the EIP-8004
// Identity Registry. Wraps registry.Register so callers don't repeat the
// permissionless-admission / agentURI-validation boilerplate.
//
// FR anchors:
//   - 016 FR-A01: seller MUST register with neuron-commerce name="adsb",
//     pricing.unit="frame".
//   - 003 FR-R02 / V-REG-12: childAddress comes from the same key that
//     produced the descriptor's PeerID.
func RegisterSeller(
	ctx context.Context,
	childKey *keylib.NeuronPrivateKey,
	descriptor ServiceDescriptor,
	registryAddr keylib.EVMAddress,
	chainID uint64,
	contract registry.RegistryContract,
) (RegisterResult, error) {
	if childKey == nil {
		return RegisterResult{}, errors.New("adsb.RegisterSeller: childKey is required")
	}
	if contract == nil {
		return RegisterResult{}, errors.New("adsb.RegisterSeller: contract is required")
	}

	// RegisterOrUpdate is idempotent: it mints when unregistered, refreshes a
	// stale AgentURI (e.g. after the /jetvision rename), and reuses an
	// up-to-date registration — so a re-run never fails with "duplicate
	// registration".
	result, outcome, err := registry.RegisterOrUpdate(
		ctx, childKey, registryAddr, chainID,
		descriptor.AgentURI, contract,
		registry.PermissionlessPolicy{}, "",
	)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("adsb.RegisterSeller: %w", err)
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

// DiscoverSeller looks an ADS-B seller up in the EIP-8004 registry by its
// EVM address and extracts the ADS-B-specific service descriptor.
func DiscoverSeller(
	ctx context.Context,
	sellerEVM keylib.EVMAddress,
	registryAddr keylib.EVMAddress,
	chainID uint64,
	contract registry.RegistryContract,
) (DiscoverResult, error) {
	if contract == nil {
		return DiscoverResult{}, errors.New("adsb.DiscoverSeller: contract is required")
	}

	reg, err := registry.LookupRegistration(
		ctx, registryAddr, chainID,
		registry.ByEVMAddress(sellerEVM), contract,
	)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("adsb.DiscoverSeller: %w", err)
	}

	p2p, err := registry.ResolveP2PExchange(reg)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("adsb.DiscoverSeller: resolve p2p service: %w", err)
	}

	uri := reg.AgentURI()
	commerceServices := payment.FilterByName(uri.CommerceServices(), CommerceServiceName)
	var commerce *payment.NeuronCommerceService
	if len(commerceServices) > 0 {
		c := commerceServices[0]
		commerce = &c
	}

	agentURIJSON, marshalErr := uri.ToJSON()
	if marshalErr != nil {
		return DiscoverResult{}, fmt.Errorf("adsb.DiscoverSeller: serialize agentURI for evidence: %w", marshalErr)
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
