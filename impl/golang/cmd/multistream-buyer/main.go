// Command multistream-buyer is the canonical reference demo buyer.
//
// Topology — one libp2p host (one peerID) holds N parallel seller sessions
// (one per --seller flag) and fans the resulting TaggedFrames in to a
// single consolidated JSONL output sink (typically tcp:127.0.0.1:19090
// where cmd/fid-display is listening).
//
// It is the current canonical replacement for prior side-by-side demo captures.
//
// SCOPE — supports two operational modes:
//
//  1. --mode=fixture-direct (default) — operator passes seller multiaddrs
//     directly via repeated --seller=role=...,multiaddr=...,protocol=...
//     flags. No on-chain discovery.
//  2. --mode=eip8004-registry --commerce-mode=registration-only — operator
//     passes seller EVM addresses. The buyer resolves each via the
//     EIP-8004 Identity Registry (LookupRegistration ByEVMAddress), reads
//     the seller's peerID + multiaddr from the AgentURI's
//     neuron-p2p-exchange service, validates peerID consistency, then
//     dials. Used by the VPS 1 runbook.
//
// --commerce-mode=full runs one Spec 008 negotiation + escrow +
// settlement session per seller through the same buyer host. The mode is
// one-shot by protocol design; use registration-only fixture-direct mode
// for the continuously refreshed FID display.
//
// Per Spec 018 FR-F-02 + spec 017 FR-R05 the TaggedFrame envelope is:
//
//	{"source": "adsb"|"remote-id", "type": "normalized-track"|"drone",
//	 "sellerPeerID": "...", "receivedAt": "...",
//	 "frame": { ... inner canonical JSON ... }}
//
// The inner-frame bytes are forwarded verbatim from the stream — the
// buyer does NOT re-marshal. This preserves the seller-stamped
// frame.source (e.g. "basestation-tcp-synthetic") so fid-display's SYN
// badge keeps working unchanged.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	libp2phost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/adsb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/edgeapp"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

const (
	modeFixtureDirect   = "fixture-direct"
	modeEIP8004Registry = "eip8004-registry"

	commerceModeRegistrationOnly = "registration-only"
	commerceModeFull             = "full"

	roleAdsb     = "adsb"
	roleRemoteID = "remoteid"

	// Outer TaggedFrame.source vocabulary per fid-display's switch
	// (cmd/fid-display/main.go:318). Held here as constants so the
	// flag-parsing branches stay literal-free.
	sourceAdsb     = "adsb"
	sourceRemoteID = "remote-id"

	// Outer TaggedFrame.type per the per-binary buyer convention.
	typeNormalizedTrack = "normalized-track"
	typeDrone           = "drone"

	defaultChainID  = uint64(296)
	defaultRPCURL   = "https://testnet.hashio.io/api"
	defaultOutput   = "tcp:127.0.0.1:19090"
	// defaultListenMA listens on all interfaces with an ephemeral UDP/QUIC
	// port. The 0.0.0.0 bind enumerates every reachable address via
	// host.Addrs(); delivery.FilterPublicMultiaddrs prunes private/Docker/
	// loopback addresses before any ConnectionSetup payload is built so the
	// resulting ECIES-wrapped multiaddrs stay under Hedera HCS's 1024-byte
	// per-message limit. Do not narrow this default without keeping the
	// filter in lock-step.
	defaultListenMA = "/ip4/0.0.0.0/udp/0/quic-v1"
)

// TaggedFrame is the on-the-wire envelope this binary emits. The Frame
// field is json.RawMessage so each seller's inner-frame bytes are
// forwarded verbatim — no re-marshalling, no schema coupling to the
// adsb / remoteid package types. This is the same shape fid-display
// already consumes (cmd/fid-display/main.go:73).
type TaggedFrame struct {
	Source       string          `json:"source"`
	Type         string          `json:"type"`
	SellerPeerID string          `json:"sellerPeerID"`
	ReceivedAt   time.Time       `json:"receivedAt"`
	Frame        json.RawMessage `json:"frame"`
}

// SellerSpec describes one parsed --seller flag value. ParseSeller is the
// canonical constructor; this struct is immutable after construction.
type SellerSpec struct {
	// Role is "adsb" or "remoteid". Validated by ParseSeller.
	Role string
	// Multiaddr is non-empty in fixture-direct mode (the full /p2p/<pid>
	// dial string the buyer parses with peer.AddrInfoFromString). In
	// eip8004-registry + commerce-mode=full mode, when paired with
	// AllowOverride=true, this same field carries an OVERRIDE multiaddr
	// the buyer uses INSTEAD of the registry-derived AgentURI multiaddr
	// (the Stage 3B Rehearsal C debug path; required for JV when the
	// seller binds to loopback and the registry strips the address).
	Multiaddr string
	// EVM is non-empty in eip8004-registry mode (the seller's on-chain
	// EVM address; passed to LookupRegistration ByEVMAddress).
	EVM string
	// Protocol is the libp2p stream protocol id the buyer opens against
	// the seller. Defaults are role-dependent:
	//   role=adsb     → /jetvision/basestation/1.0.0
	//   role=remoteid → /ds240/basestation/1.0.0
	Protocol string
	// AllowOverride opts the seller into Stage 3B Rehearsal C: when true,
	// commerce-mode=full uses Multiaddr as a dial override and bypasses
	// the registry-derived multiaddrs (PeerID must still match — RunFullCommerceFlow
	// validates that). Only honoured in eip8004-registry + commerce-mode=full.
	AllowOverride bool
	// Raw is the unparsed --seller value, kept for log lines.
	Raw string
}

// ContractFactoryOpts is the multistream-buyer test-injection shape for
// switching between a real EVM registry client and an in-memory mock.
type ContractFactoryOpts struct {
	RPCURL          string
	RegistryAddress common.Address
	ChainID         uint64
	SignerKey       *ecdsa.PrivateKey
}

// Deps is the test-injection surface. All fields are optional; nil
// defaults to the live-network factory + OS signal channel.
type Deps struct {
	ContractFactory func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error)
	SignalCh        <-chan os.Signal
}

func main() {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	os.Exit(run(os.Args[1:], env, os.Stdout, os.Stderr, Deps{}))
}

// sellerFlag is the flag.Value implementation that lets --seller be
// repeated (the standard library's flag.StringVar can't hold a slice).
type sellerFlag []string

func (s *sellerFlag) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ", ")
}

func (s *sellerFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// run is the testable entry point — returns an exit code (0 = ok, 1 = run
// failure, 2 = bad flags / unrecoverable misconfiguration).
//
// Full-commerce runs only in eip8004-registry mode. Invalid flag
// combinations fail loudly instead of falling back to direct dial.
func run(args []string, env map[string]string, stdout, stderr io.Writer, deps Deps) int {
	logger := log.New(stderr, "", log.LstdFlags|log.Lmicroseconds)

	fs := flag.NewFlagSet("multistream-buyer", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var sellersFlag sellerFlag
	fs.Var(&sellersFlag, "seller", "repeatable; role=ROLE,multiaddr=MA,protocol=PROTO  OR  role=ROLE,evm=0xADDR,protocol=PROTO")

	var (
		mode             = fs.String("mode", modeFixtureDirect, "operation mode: fixture-direct | eip8004-registry")
		commerceMode     = fs.String("commerce-mode", commerceModeRegistrationOnly, "FR-P58 disclosure: registration-only (Phase 2) | full (Phase 3B; N concurrent Spec 008 sessions through one libp2p host)")
		output           = fs.String("output", defaultOutput, "TaggedFrame sink: stdout | file:PATH | file+:PATH | tcp:HOST:PORT (default tcp:127.0.0.1:19090)")
		listen           = fs.String("listen", defaultListenMA, "libp2p host listen multiaddr")
		keyHex           = fs.String("key-hex", "", "32-byte hex secp256k1 private key; defaults to ephemeral")
		frameLimit       = fs.Uint64("frame-limit", 0, "per-session frame cap (0 = unlimited; sessions stop after N frames each)")
		registryAddrFlag = fs.String("registry-address", "", "EIP-8004 Identity Registry contract address (eip8004-registry mode)")
		rpcURL           = fs.String("rpc-url", "", "EVM JSON-RPC endpoint (defaults to env HEDERA_EVM_RPC, then https://testnet.hashio.io/api)")
		chainIDFlag      = fs.Uint64("chain-id", 0, "EVM chain id (defaults to env NEURON_CHAIN_ID, then 296)")
		// Phase 3B (--commerce-mode=full only) — backend selection + pricing
		topicBackend  = fs.String("topic-backend", "memory", "topic adapter (commerce-mode=full only): memory | hcs")
		escrowBackend = fs.String("escrow-backend", "memory", "escrow adapter (commerce-mode=full only): memory | evm")
		pricingAmount = fs.String("pricing-amount", "0", "decimal-string pricing amount per session (commerce-mode=full only; --escrow-backend=evm requires > 0)")
	)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		logger.Printf("multistream-buyer: parse flags: %v", err)
		return 2
	}

	// --commerce-mode validation. Phase 3B unblocks --commerce-mode=full:
	// the multistream buyer now runs N concurrent Spec 008 commerce
	// sessions through ONE libp2p host (via adsb/remoteid.RunFullCommerceFlow).
	// full requires --mode=eip8004-registry (the on-chain registry is the
	// only seller-discovery path the orchestrator supports).
	switch *commerceMode {
	case commerceModeRegistrationOnly:
		// ok
	case commerceModeFull:
		if *mode != modeEIP8004Registry {
			logger.Printf("multistream-buyer: --commerce-mode=full requires --mode=eip8004-registry")
			return 2
		}
	default:
		logger.Printf("multistream-buyer: unknown --commerce-mode=%q (expected: registration-only | full)", *commerceMode)
		return 2
	}

	switch *mode {
	case modeFixtureDirect, modeEIP8004Registry:
		// ok
	default:
		logger.Printf("multistream-buyer: unknown --mode=%q (expected: fixture-direct | eip8004-registry)", *mode)
		return 2
	}

	if len(sellersFlag) == 0 {
		logger.Printf("multistream-buyer: at least one --seller required")
		return 2
	}

	// Parse and validate each --seller spec per the active mode + commerce-mode.
	sellers := make([]SellerSpec, 0, len(sellersFlag))
	for _, raw := range sellersFlag {
		spec, err := ParseSeller(raw)
		if err != nil {
			logger.Printf("multistream-buyer: --seller=%q: %v", raw, err)
			return 2
		}
		if err := spec.validateForMode(*mode, *commerceMode); err != nil {
			logger.Printf("multistream-buyer: --seller=%q: %v", raw, err)
			return 2
		}
		sellers = append(sellers, spec)
	}

	// eip8004-registry mode requires the registry address up front. No
	// silent fallback to fixture-direct when the address is missing.
	if *mode == modeEIP8004Registry && *registryAddrFlag == "" {
		logger.Printf("multistream-buyer: --mode=eip8004-registry requires --registry-address <0x...>")
		return 2
	}

	// Build the shared buyer libp2p host. ONE host across all sellers per
	// the Phase 2 design — single peerID, single signing key, single
	// connection-manager / peerstore footprint.
	privKey, err := resolveKey(*keyHex)
	if err != nil {
		logger.Printf("multistream-buyer: resolve --key-hex: %v", err)
		return 2
	}
	host, err := delivery.NewLibp2pHost(privKey, *listen)
	if err != nil {
		logger.Printf("multistream-buyer: build libp2p host: %v", err)
		return 2
	}
	defer host.Close()
	logger.Printf("[buyer] libp2p host ready peerID=%s addrs=%v sellers=%d mode=%s commerce-mode=%s",
		host.ID(), host.Addrs(), len(sellers), *mode, *commerceMode)

	// Resolve the registry contract once when in eip8004-registry mode —
	// the buyer reads each seller from the same on-chain registry.
	var contract registry.RegistryContract
	var chainID uint64
	var registryAddr keylib.EVMAddress
	if *mode == modeEIP8004Registry {
		registryAddr, err = keylib.EVMAddressFromHex(*registryAddrFlag)
		if err != nil {
			logger.Printf("multistream-buyer: invalid --registry-address %q: %v", *registryAddrFlag, err)
			return 2
		}
		chainID = *chainIDFlag
		if chainID == 0 {
			if v := env["NEURON_CHAIN_ID"]; v != "" {
				n, perr := strconv.ParseUint(v, 10, 64)
				if perr != nil {
					logger.Printf("multistream-buyer: NEURON_CHAIN_ID=%q: %v", v, perr)
					return 2
				}
				chainID = n
			} else {
				chainID = defaultChainID
			}
		}
		rpc := *rpcURL
		if rpc == "" {
			if v := env["HEDERA_EVM_RPC"]; v != "" {
				rpc = v
			} else {
				rpc = defaultRPCURL
			}
		}
		factory := deps.ContractFactory
		if factory == nil {
			factory = defaultContractFactory
		}
		contract, err = factory(context.Background(), ContractFactoryOpts{
			RPCURL:          rpc,
			RegistryAddress: common.HexToAddress(registryAddr.Hex()),
			ChainID:         chainID,
		})
		if err != nil {
			logger.Printf("multistream-buyer: build registry contract: %v", err)
			return 1
		}
		logger.Printf("[registry] connected rpc=%s chainID=%d registry=%s", rpc, chainID, registryAddr.Hex())
	}

	// Build the consolidated TaggedFrame sink. One sink — one TCP socket
	// or one file — drained by a single writer goroutine.
	sink, err := edgeapp.NewTaggedJSONLSink(*output)
	if err != nil {
		logger.Printf("multistream-buyer: build sink: %v", err)
		return 2
	}
	defer sink.Close()

	// Phase 3B (--commerce-mode=full only) — build SHARED commerce
	// infrastructure once at the top of run() so all N seller sessions
	// reuse the same topic adapter + EVM escrow + buyer signing key.
	// The Escrow is wrapped in a serializedEscrow shim (mutex) so
	// concurrent on-chain CreateEscrow/Deposit txs from the same buyer
	// key don't race for the nonce (defensive; Phase 3A empirically did
	// not see collisions with an 8s manual stagger but the shim costs
	// nothing in correctness terms).
	var (
		fcAdapter       topic.TopicAdapter
		fcEscrow        payment.EscrowAdapter
		fcEscrowBinding string
		fcBuyerKey      keylib.NeuronPrivateKey
		fcBuyerPrivKey  *ecdsa.PrivateKey
	)
	if *commerceMode == commerceModeFull {
		envLookup := func(k string) string { return env[k] }

		switch *topicBackend {
		case "hcs":
			be, herr := adsb.NewHCSBackend(context.Background(), adsb.HCSBackendOptions{
				Role:      adsb.HCSRoleBuyer,
				LookupEnv: envLookup,
			})
			if herr != nil {
				logger.Printf("multistream-buyer: HCS backend: %v", herr)
				return 1
			}
			fcAdapter = be.Adapter
			logger.Printf("[hcs] buyer connected; operator=%s", be.OperatorID)
		case "memory":
			fcAdapter = topic.NewMemoryTopicAdapter()
		default:
			logger.Printf("multistream-buyer: --topic-backend=%q (expected: memory | hcs)", *topicBackend)
			return 2
		}

		var rawEscrow payment.EscrowAdapter
		switch *escrowBackend {
		case "evm":
			be, eerr := adsb.NewEVMBackend(context.Background(), adsb.EVMBackendOptions{
				LookupEnv:      envLookup,
				DefaultRPCURL:  defaultRPCURL,
				DefaultChainID: defaultChainID,
			})
			if eerr != nil {
				logger.Printf("multistream-buyer: EVM backend: %v", eerr)
				return 1
			}
			rawEscrow = be.Escrow
			fcEscrowBinding = be.EscrowBinding
			logger.Printf("[evm] buyer connected; rpc=%s chainId=%d escrowContract=%s tokenContract=%s operator=%s",
				be.RPCURL, be.ChainID, be.EscrowContract, be.TokenContract, be.OperatorAddr)
		case "memory":
			rawEscrow = payment.NewMemoryEscrow()
			fcEscrowBinding = "memory"
		default:
			logger.Printf("multistream-buyer: --escrow-backend=%q (expected: memory | evm)", *escrowBackend)
			return 2
		}
		fcEscrow = &serializedEscrow{inner: rawEscrow}

		var werr error
		fcBuyerKey, werr = keylib.NeuronPrivateKeyFromBytes(ethcrypto.FromECDSA(privKey))
		if werr != nil {
			logger.Printf("multistream-buyer: wrap signing key: %v", werr)
			return 1
		}
		fcBuyerPrivKey = privKey
	}

	// Resolve each seller. Two paths:
	//   commerce-mode=registration-only: resolveSeller hits the registry
	//     (or parses the fixture multiaddr) up front so the goroutine just
	//     dials + reads. Strict — empty AgentURI multiaddrs fail.
	//   commerce-mode=full: skip the pre-flight; adsb/remoteid.RunFullCommerceFlow
	//     does its own DiscoverSeller and honours --seller multiaddr=
	//     overrides (Stage 3B Rehearsal C, needed for JV's loopback bind).
	type resolved struct {
		Spec     SellerSpec
		AddrInfo peer.AddrInfo
		Protocol string
	}
	resolvedSellers := make([]resolved, 0, len(sellers))
	if *commerceMode == commerceModeRegistrationOnly {
		for _, spec := range sellers {
			rs, rerr := resolveSeller(context.Background(), spec, *mode, contract, registryAddr, chainID, logger)
			if rerr != nil {
				logger.Printf("multistream-buyer: resolve --seller=%q: %v", spec.Raw, rerr)
				return 1
			}
			resolvedSellers = append(resolvedSellers, resolved{Spec: spec, AddrInfo: rs.AddrInfo, Protocol: rs.Protocol})
		}
	} else {
		// commerce-mode=full: defer discovery to RunFullCommerceFlow.
		for _, spec := range sellers {
			resolvedSellers = append(resolvedSellers, resolved{Spec: spec})
		}
	}

	// Root context drives all session goroutines + the writer.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wire signal handling. Tests inject deps.SignalCh; production builds
	// register the standard SIGINT/SIGTERM notifier.
	sigCh := deps.SignalCh
	if sigCh == nil {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		sigCh = ch
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-sigCh:
			logger.Printf("[buyer] received shutdown signal — cancelling sessions")
			cancel()
		}
	}()

	// Shared output channel (bounded so a slow sink applies back-pressure
	// to the per-session readers rather than dropping frames silently).
	frameCh := make(chan []byte, 256)

	// Writer goroutine. Drains frameCh; emits each TaggedFrame JSON blob
	// to the consolidated sink. Wraps each blob in a sink.Emit so the
	// existing TaggedSink interface stays bound to "any JSON-marshallable
	// value" — for us, the blob is already canonical so we forward it via
	// a small json.RawMessage shim.
	var writerDone sync.WaitGroup
	writerDone.Go(func() {
		for blob := range frameCh {
			// Forward verbatim via a json.RawMessage shim so the sink
			// doesn't re-marshal — the per-session reader has already
			// produced a canonical JSONL line.
			if err := sink.Emit(ctx, json.RawMessage(blob)); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				logger.Printf("[buyer] sink emit error: %v", err)
			}
		}
	})

	// One goroutine per seller. Each runs an independent
	// connect → open stream → frame-loop pipeline against the shared host.
	// Errors are logged but never abort siblings — a flaky seller must
	// not take down the consolidated output.
	var sessionDone sync.WaitGroup
	var (
		sessionMu      sync.Mutex
		completedCount int
		failedCount    int
	)
	for _, rs := range resolvedSellers {
		spec := rs.Spec
		addrInfo := rs.AddrInfo
		proto := rs.Protocol
		sessionDone.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Printf("[session role=%s] PANIC: %v", spec.Role, r)
					sessionMu.Lock()
					failedCount++
					sessionMu.Unlock()
				}
			}()
			if *commerceMode == commerceModeFull {
				ok := runSellerFullCommerce(ctx, spec, host, fcAdapter, fcEscrow, fcEscrowBinding,
					contract, registryAddr, chainID, &fcBuyerKey, fcBuyerPrivKey,
					*pricingAmount, *frameLimit, frameCh, logger)
				sessionMu.Lock()
				if ok {
					completedCount++
				} else {
					failedCount++
				}
				sessionMu.Unlock()
			} else {
				// Phase 5 B4: reconnect supervision. If the session ends
				// (EOF, dial failure, transient transport drop), the
				// supervisor reconnects with bounded exponential backoff.
				// Sibling sessions are unaffected. Termination on
				// ctx.Done() or hitting frame-limit.
				received, attempts := runSellerSessionWithReconnect(
					ctx, runSellerSession,
					host, spec, addrInfo, proto, frameCh, *frameLimit, logger,
				)
				logger.Printf("[session role=%s] supervisor exit; lifetime_emitted=%d attempts=%d",
					spec.Role, received, attempts)
				sessionMu.Lock()
				completedCount++
				sessionMu.Unlock()
			}
		})
	}

	sessionDone.Wait()
	close(frameCh)
	writerDone.Wait()

	logger.Printf("[buyer] all sessions done; completed=%d failed=%d (of %d)", completedCount, failedCount, len(resolvedSellers))
	_ = stdout
	// Phase 3B partial-failure rule: exit 0 if at least ONE session
	// reached COMPLETED (the user's brief: "one seller session failing
	// must not crash the whole buyer"). exit 1 only if ALL sessions failed.
	if completedCount == 0 && len(resolvedSellers) > 0 {
		return 1
	}
	return 0
}

// resolvedSeller is the output of resolveSeller — an AddrInfo ready for
// host.Connect plus the protocol id the buyer should open against it.
type resolvedSeller struct {
	AddrInfo peer.AddrInfo
	Protocol string
}

// resolveSeller turns a SellerSpec into an AddrInfo + Protocol.
//
// In fixture-direct mode: peer.AddrInfoFromString(spec.Multiaddr). The
// Protocol is spec.Protocol (defaulted by ParseSeller per role).
//
// In eip8004-registry mode: LookupRegistration ByEVMAddress through the
// shared contract; read the seller's peerID + multiaddrs out of the
// AgentURI's neuron-p2p-exchange service; cross-validate peerID
// consistency (defence-in-depth — the contract may have rewritten
// fields between register and lookup).
//
// Always uses the seller-supplied Protocol when present; in registry
// mode this is the operator-asserted stream protocol the seller
// advertises (the AgentURI's primary p2p protocol id is read for log
// purposes but the operator's value wins — sellers advertising both
// /ds240/raw/1.0.0 and /ds240/basestation/1.0.0 need this so the
// buyer can pick the basestation companion explicitly).
func resolveSeller(
	ctx context.Context,
	spec SellerSpec,
	mode string,
	contract registry.RegistryContract,
	registryAddr keylib.EVMAddress,
	chainID uint64,
	logger *log.Logger,
) (resolvedSeller, error) {
	switch mode {
	case modeFixtureDirect:
		info, err := peer.AddrInfoFromString(spec.Multiaddr)
		if err != nil {
			return resolvedSeller{}, fmt.Errorf("parse multiaddr %q: %w", spec.Multiaddr, err)
		}
		return resolvedSeller{AddrInfo: *info, Protocol: spec.Protocol}, nil

	case modeEIP8004Registry:
		sellerEVM, err := keylib.EVMAddressFromHex(spec.EVM)
		if err != nil {
			return resolvedSeller{}, fmt.Errorf("invalid evm=%q: %w", spec.EVM, err)
		}
		reg, err := registry.LookupRegistration(ctx, registryAddr, chainID, registry.ByEVMAddress(sellerEVM), contract)
		if err != nil {
			return resolvedSeller{}, fmt.Errorf("LookupRegistration %s: %w", sellerEVM.Hex(), err)
		}
		p2p, err := registry.ResolveP2PExchange(reg)
		if err != nil {
			return resolvedSeller{}, fmt.Errorf("resolve neuron-p2p-exchange for %s: %w", sellerEVM.Hex(), err)
		}
		if len(p2p.Multiaddrs) == 0 {
			return resolvedSeller{}, fmt.Errorf("seller %s: AgentURI has no multiaddrs", sellerEVM.Hex())
		}
		pid, err := peer.Decode(p2p.PeerID)
		if err != nil {
			return resolvedSeller{}, fmt.Errorf("decode peerID %q: %w", p2p.PeerID, err)
		}

		// Parse the registered multiaddrs and assert each one is internally
		// consistent (its /p2p/<id> suffix, if present, matches the
		// registered peerID).
		info, err := buildAddrInfo(pid, p2p.Multiaddrs)
		if err != nil {
			return resolvedSeller{}, err
		}

		logger.Printf("[registry] role=%s evm=%s peerID=%s registered-protocol=%s using-protocol=%s multiaddrs=%d",
			spec.Role, sellerEVM.Hex(), pid, p2p.Protocol, spec.Protocol, len(p2p.Multiaddrs))
		return resolvedSeller{AddrInfo: info, Protocol: spec.Protocol}, nil

	default:
		// run() already validated *mode — this is defence in depth.
		return resolvedSeller{}, fmt.Errorf("unknown mode %q", mode)
	}
}

// buildAddrInfo composes a peer.AddrInfo from a registered peerID + a
// list of multiaddr strings. Multiaddrs may be bare (no /p2p/<id>
// suffix) or fully-qualified — both shapes are accepted. Any /p2p/<id>
// component MUST match the registered peerID; this is the defence
// against a registry serving multiaddrs for a different peer.
func buildAddrInfo(pid peer.ID, addrs []string) (peer.AddrInfo, error) {
	out := peer.AddrInfo{ID: pid}
	for _, s := range addrs {
		mm, err := ma.NewMultiaddr(s)
		if err != nil {
			return peer.AddrInfo{}, fmt.Errorf("parse multiaddr %q: %w", s, err)
		}
		// SplitAddr returns (transport, peerID). When the multiaddr has
		// no /p2p/... suffix, info.ID is "" and we use only the
		// transport part. When it does, the embedded peerID MUST equal
		// the registered one (defence-in-depth against registry rewrites).
		transport, embedded := peer.SplitAddr(mm)
		if embedded != "" && embedded != pid {
			return peer.AddrInfo{}, fmt.Errorf("multiaddr %q peerID=%s != registered=%s",
				s, embedded, pid)
		}
		if transport != nil {
			out.Addrs = append(out.Addrs, transport)
		}
	}
	if len(out.Addrs) == 0 {
		return peer.AddrInfo{}, errors.New("no usable multiaddrs after parsing")
	}
	return out, nil
}

// runSellerSession dials one seller, opens the chosen stream protocol,
// reads frames until ctx cancels or the stream EOFs, and forwards each
// frame wrapped in a TaggedFrame envelope to the shared frameCh.
//
// Frame-limit semantics: 0 = unlimited. When > 0, the session stops
// cleanly after emitting frameLimit frames. Sibling sessions are
// unaffected.
//
// Returns the number of frames emitted on the shared channel — used by
// runSellerSessionWithReconnect (Phase 5 B4) to decide whether to reset
// the reconnect backoff (received > 0 = successful session).
func runSellerSession(
	ctx context.Context,
	host libp2phost.Host,
	spec SellerSpec,
	addrInfo peer.AddrInfo,
	protocolID string,
	frameCh chan<- []byte,
	frameLimit uint64,
	logger *log.Logger,
) uint64 {
	if err := host.Connect(ctx, addrInfo); err != nil {
		logger.Printf("[session role=%s] connect: %v", spec.Role, err)
		return 0
	}
	logger.Printf("[session role=%s] connected peerID=%s protocol=%s", spec.Role, addrInfo.ID, protocolID)

	stream, err := host.NewStream(ctx, addrInfo.ID, protocol.ID(protocolID))
	if err != nil {
		logger.Printf("[session role=%s] open stream %s: %v", spec.Role, protocolID, err)
		return 0
	}
	defer stream.Close()
	logger.Printf("[session role=%s] stream open protocol=%s", spec.Role, protocolID)

	source, frameType, err := envelopeKeys(spec.Role)
	if err != nil {
		logger.Printf("[session role=%s] %v", spec.Role, err)
		return 0
	}

	reader := delivery.NewFrameReader(stream)
	sellerPID := addrInfo.ID.String()
	var received uint64
	for {
		if ctx.Err() != nil {
			break
		}
		data, rerr := reader.ReadFrame()
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				logger.Printf("[session role=%s] stream EOF after %d frames", spec.Role, received)
				break
			}
			if errors.Is(rerr, context.Canceled) {
				break
			}
			logger.Printf("[session role=%s] read error after %d frames: %v", spec.Role, received, rerr)
			break
		}

		// Wrap the inner-frame bytes in a TaggedFrame and forward as a
		// single canonical JSONL blob. json.RawMessage forwards data
		// verbatim without re-parsing — preserving the seller-stamped
		// frame.source (e.g. "basestation-tcp-synthetic" → fid-display
		// SYN badge).
		tagged := TaggedFrame{
			Source:       source,
			Type:         frameType,
			SellerPeerID: sellerPID,
			ReceivedAt:   time.Now().UTC(),
			Frame:        json.RawMessage(data),
		}
		blob, merr := json.Marshal(tagged)
		if merr != nil {
			logger.Printf("[session role=%s] marshal envelope error: %v", spec.Role, merr)
			continue
		}

		select {
		case <-ctx.Done():
			return received
		case frameCh <- blob:
		}
		received++

		if frameLimit > 0 && received >= frameLimit {
			logger.Printf("[session role=%s] frame-limit reached after %d frames", spec.Role, received)
			return received
		}
	}
	logger.Printf("[session role=%s] session done; emitted=%d", spec.Role, received)
	return received
}

// sessionRunner is the testable signature of runSellerSession so the
// reconnect supervisor (runSellerSessionWithReconnect) can be unit-tested
// against a fake that doesn't require a real libp2p host.
type sessionRunner func(
	ctx context.Context,
	host libp2phost.Host,
	spec SellerSpec,
	addrInfo peer.AddrInfo,
	protocolID string,
	frameCh chan<- []byte,
	frameLimit uint64,
	logger *log.Logger,
) uint64

// reconnectBackoffSequence is the exponential-backoff sequence used by
// runSellerSessionWithReconnect. Exposed for tests; production must use
// these specific values to keep operator runbook expectations consistent.
var reconnectBackoffSequence = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
}

// runSellerSessionWithReconnect (Phase 5 B4) wraps `runSellerSession` in a
// retry loop that reconnects after EOF / read error / dial failure with
// exponential backoff (1s → 2s → 5s → 10s → 30s cap). Used ONLY for
// commerce-mode=registration-only (the continuous Phase 2 demo path);
// commerce-mode=full is intentionally one-shot per Spec 008 design.
//
// Termination conditions (in priority order):
//  1. ctx.Done() — clean shutdown
//  2. frameLimit > 0 AND last session reached the limit — intentional
//     stop (test fixtures, capped evidence runs)
//
// Backoff reset rule: if a session emitted ≥1 frame, the backoff is
// reset to the start of the sequence (1s) for the next reconnect. This
// distinguishes "connection blip mid-stream" (heal fast) from "seller
// gone, repeated dial failures" (back off).
//
// Per-session isolation: the supervisor for seller A is independent of
// the supervisor for seller B; one seller flapping doesn't extend the
// other's backoff.
func runSellerSessionWithReconnect(
	ctx context.Context,
	runner sessionRunner,
	host libp2phost.Host,
	spec SellerSpec,
	addrInfo peer.AddrInfo,
	protocolID string,
	frameCh chan<- []byte,
	frameLimit uint64,
	logger *log.Logger,
) (totalReceived uint64, attempts int) {
	backoffIdx := 0
	for {
		if ctx.Err() != nil {
			return totalReceived, attempts
		}
		attempts++
		received := runner(ctx, host, spec, addrInfo, protocolID, frameCh, frameLimit, logger)
		totalReceived += received

		if ctx.Err() != nil {
			return totalReceived, attempts
		}
		if frameLimit > 0 && received >= frameLimit {
			logger.Printf("[session role=%s] frame-limit reached; not reconnecting", spec.Role)
			return totalReceived, attempts
		}

		if received > 0 {
			backoffIdx = 0
		}
		wait := reconnectBackoffSequence[backoffIdx]
		logger.Printf("[session role=%s] reconnecting in %s (attempt #%d; previous emitted=%d; lifetime=%d)",
			spec.Role, wait, attempts+1, received, totalReceived)

		select {
		case <-ctx.Done():
			return totalReceived, attempts
		case <-time.After(wait):
		}

		if backoffIdx < len(reconnectBackoffSequence)-1 {
			backoffIdx++
		}
	}
}

// envelopeKeys returns the (TaggedFrame.source, TaggedFrame.type) pair
// for a parsed role. Errors if the role is somehow unknown (run() and
// ParseSeller both validate this; defence in depth).
func envelopeKeys(role string) (source, frameType string, err error) {
	switch role {
	case roleAdsb:
		return sourceAdsb, typeNormalizedTrack, nil
	case roleRemoteID:
		return sourceRemoteID, typeDrone, nil
	default:
		return "", "", fmt.Errorf("envelopeKeys: unknown role %q", role)
	}
}

// defaultProtocolForRole maps a role to the protocol id ParseSeller
// applies when the operator omits `protocol=...` from --seller.
//
//	role=adsb     → /jetvision/basestation/1.0.0  (per spec 018 FR-F-02)
//	role=remoteid → /ds240/basestation/1.0.0 (per spec 017 FR-R05
//	                companion stream; the original /ds240/raw/1.0.0
//	                path remains accessible by passing protocol= explicitly)
func defaultProtocolForRole(role string) string {
	switch role {
	case roleAdsb:
		return adsb.ProtocolBaseStation
	case roleRemoteID:
		return remoteid.ProtocolBasestation
	default:
		return ""
	}
}

// ParseSeller parses one --seller flag value.
//
// Syntax: comma-separated key=value pairs. Recognised keys:
//   - role     (required) — "adsb" | "remoteid"
//   - multiaddr (one of) — full /p2p/<peerID>/<addr> dial string
//   - evm       (one of) — seller's EVM address (registry-mode lookup)
//   - protocol (optional) — libp2p stream protocol id; defaults per role
//
// Exactly one of multiaddr / evm is required. Mode-specific validation
// happens later in validateForMode — ParseSeller only checks
// syntactic well-formedness so a single unknown key short-circuits the
// CLI cleanly.
func ParseSeller(raw string) (SellerSpec, error) {
	spec := SellerSpec{Raw: raw}
	if strings.TrimSpace(raw) == "" {
		return SellerSpec{}, errors.New("empty --seller value")
	}

	for kv := range strings.SplitSeq(raw, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			return SellerSpec{}, fmt.Errorf("malformed key=value pair %q (want key=value)", kv)
		}
		key := strings.TrimSpace(kv[:i])
		val := strings.TrimSpace(kv[i+1:])
		if val == "" {
			return SellerSpec{}, fmt.Errorf("empty value for key %q", key)
		}
		switch key {
		case "role":
			spec.Role = val
		case "multiaddr":
			spec.Multiaddr = val
		case "evm":
			spec.EVM = val
		case "protocol":
			spec.Protocol = val
		case "allowOverride":
			switch strings.ToLower(val) {
			case "true", "1", "yes":
				spec.AllowOverride = true
			case "false", "0", "no":
				spec.AllowOverride = false
			default:
				return SellerSpec{}, fmt.Errorf("allowOverride=%q must be true/false", val)
			}
		default:
			return SellerSpec{}, fmt.Errorf("unknown key %q (want: role|multiaddr|evm|protocol|allowOverride)", key)
		}
	}

	// Required: role + one of {multiaddr, evm} (or BOTH in registry+full mode).
	switch spec.Role {
	case roleAdsb, roleRemoteID:
		// ok
	case "":
		return SellerSpec{}, errors.New("missing required key: role")
	default:
		return SellerSpec{}, fmt.Errorf("unknown role %q (want: adsb | remoteid)", spec.Role)
	}
	if spec.Multiaddr == "" && spec.EVM == "" {
		return SellerSpec{}, errors.New("either multiaddr= or evm= is required")
	}

	if spec.Protocol == "" {
		spec.Protocol = defaultProtocolForRole(spec.Role)
	}
	return spec, nil
}

// validateForMode returns an error when this spec is incompatible with
// the active --mode + --commerce-mode pair. Mode-specific validation
// lives outside ParseSeller so the same parser handles all valid
// (mode, commerce-mode) combinations.
//
// Rules:
//   - fixture-direct: multiaddr= REQUIRED, evm= REJECTED (no on-chain).
//   - eip8004-registry + commerce-mode=registration-only: evm= REQUIRED,
//     multiaddr= REJECTED (resolver pulls multiaddrs from on-chain AgentURI).
//   - eip8004-registry + commerce-mode=full: evm= REQUIRED; multiaddr=
//     is OPTIONAL — when supplied alongside allowOverride=true it becomes
//     a Stage 3B Rehearsal C dial override (used when registry-stripped
//     loopback addresses leave the AgentURI without a dialable multiaddr).
//     allowOverride=true without multiaddr= is rejected.
func (s SellerSpec) validateForMode(mode, commerceMode string) error {
	switch mode {
	case modeFixtureDirect:
		if s.Multiaddr == "" {
			return errors.New("fixture-direct mode requires multiaddr= per --seller")
		}
		if s.EVM != "" {
			return errors.New("fixture-direct mode rejects evm= per --seller")
		}
		if s.AllowOverride {
			return errors.New("fixture-direct mode rejects allowOverride= per --seller (registry-mode debug path only)")
		}
	case modeEIP8004Registry:
		if s.EVM == "" {
			return errors.New("eip8004-registry mode requires evm= per --seller")
		}
		switch commerceMode {
		case commerceModeRegistrationOnly:
			if s.Multiaddr != "" {
				return errors.New("eip8004-registry + commerce-mode=registration-only rejects multiaddr= per --seller")
			}
			if s.AllowOverride {
				return errors.New("eip8004-registry + commerce-mode=registration-only rejects allowOverride= per --seller")
			}
		case commerceModeFull:
			// multiaddr= alone is allowed but inert without allowOverride.
			if s.AllowOverride && s.Multiaddr == "" {
				return errors.New("eip8004-registry + commerce-mode=full: allowOverride=true requires multiaddr= per --seller")
			}
		default:
			return fmt.Errorf("unknown commerce-mode %q", commerceMode)
		}
	default:
		return fmt.Errorf("unknown mode %q", mode)
	}
	return nil
}

// resolveKey parses --key-hex into a secp256k1 *ecdsa.PrivateKey, or
// generates an ephemeral key when hexStr is empty.
func resolveKey(hexStr string) (*ecdsa.PrivateKey, error) {
	if hexStr == "" {
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return nil, fmt.Errorf("generate ephemeral key: %w", err)
		}
		raw[0] &^= 0x80
		neuronKey, err := keylib.NeuronPrivateKeyFromBytes(raw[:])
		if err != nil {
			return nil, fmt.Errorf("wrap ephemeral key: %w", err)
		}
		return neuronKey.ToBlockchainKey()
	}
	neuronKey, err := keylib.NeuronPrivateKeyFromHex(hexStr)
	if err != nil {
		return nil, err
	}
	return neuronKey.ToBlockchainKey()
}

// defaultContractFactory dials the configured RPC endpoint and returns
// an auth-less EVMRegistryContract. Buyer lookups are read-only so the
// no-private-key TransactOpts is fine.
func defaultContractFactory(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
	client, err := ethclient.DialContext(ctx, opts.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", opts.RPCURL, err)
	}
	auth := &bind.TransactOpts{Context: ctx}
	return registry.NewEVMRegistryContract(client, opts.RegistryAddress, auth)
}

// runSellerFullCommerce runs ONE Spec 008 full-commerce session against
// one seller. Dispatches per-role to adsb.RunFullCommerceFlow or
// remoteid.RunFullCommerceFlow. Returns true on COMPLETED + action=approved;
// false on any error (which is logged but never propagated — partial-failure
// isolation per the user's Phase 3B brief).
//
// All inputs other than spec are SHARED across sessions:
//   - host: one libp2p host (one peerID) for all dial-outs.
//   - adapter: one topic adapter (HCS or memory).
//   - escrow: one EscrowAdapter (memory or EVM), wrapped in serializedEscrow.
//   - contract: one registry contract (read-only).
//   - key + privKey: one buyer signing identity for both escrows.
//
// frameCh receives JSON-marshalled TaggedFrame blobs (verbatim wire shape
// matching runSellerSession + per-binary buyers).
func runSellerFullCommerce(
	ctx context.Context,
	spec SellerSpec,
	host libp2phost.Host,
	adapter topic.TopicAdapter,
	escrow payment.EscrowAdapter,
	escrowBinding string,
	contract registry.RegistryContract,
	registryAddr keylib.EVMAddress,
	chainID uint64,
	buyerKey *keylib.NeuronPrivateKey,
	buyerPrivKey *ecdsa.PrivateKey,
	pricingAmount string,
	frameLimit uint64,
	frameCh chan<- []byte,
	logger *log.Logger,
) bool {
	sellerEVM, err := keylib.EVMAddressFromHex(spec.EVM)
	if err != nil {
		logger.Printf("[session role=%s] invalid evm=%q: %v", spec.Role, spec.EVM, err)
		return false
	}

	switch spec.Role {
	case roleAdsb:
		cb := func(track adsb.NormalizedTrack, sellerPID peer.ID) error {
			tagged := TaggedFrame{
				Source:       sourceAdsb,
				Type:         typeNormalizedTrack,
				SellerPeerID: sellerPID.String(),
				ReceivedAt:   time.Now().UTC(),
				Frame:        nil, // populated via json.RawMessage below
			}
			inner, merr := json.Marshal(track)
			if merr != nil {
				return fmt.Errorf("marshal adsb frame: %w", merr)
			}
			tagged.Frame = json.RawMessage(inner)
			blob, merr := json.Marshal(tagged)
			if merr != nil {
				return fmt.Errorf("marshal tagged frame: %w", merr)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case frameCh <- blob:
				return nil
			}
		}
		final, ferr := adsb.RunFullCommerceFlow(ctx, adsb.FullCommerceFlowOpts{
			Logger:           logger,
			Key:              buyerKey,
			EcdsaPriv:        buyerPrivKey,
			BuyerHost:        host,
			Adapter:          adapter,
			Escrow:           escrow,
			EscrowBinding:    escrowBinding,
			Contract:         contract,
			RegistryAddress:  registryAddr,
			ChainID:          chainID,
			SellerEVM:        sellerEVM,
			PricingAmount:    pricingAmount,
			FrameLimit:       frameLimit,
			SellerMaOverride: spec.Multiaddr,
			AllowMaOverride:  spec.AllowOverride,
		}, cb)
		if ferr != nil {
			logger.Printf("[session role=adsb] ERROR: %v", ferr)
			return false
		}
		return final.FinalAction == "approved"

	case roleRemoteID:
		cb := func(frame remoteid.RemoteIdFrame, sellerPID peer.ID) error {
			tagged := TaggedFrame{
				Source:       sourceRemoteID,
				Type:         typeDrone,
				SellerPeerID: sellerPID.String(),
				ReceivedAt:   time.Now().UTC(),
			}
			inner, merr := json.Marshal(frame)
			if merr != nil {
				return fmt.Errorf("marshal remoteid frame: %w", merr)
			}
			tagged.Frame = json.RawMessage(inner)
			blob, merr := json.Marshal(tagged)
			if merr != nil {
				return fmt.Errorf("marshal tagged frame: %w", merr)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case frameCh <- blob:
				return nil
			}
		}
		final, ferr := remoteid.RunFullCommerceFlow(ctx, remoteid.FullCommerceFlowOpts{
			Logger:           logger,
			Key:              buyerKey,
			EcdsaPriv:        buyerPrivKey,
			BuyerHost:        host,
			Adapter:          adapter,
			Escrow:           escrow,
			EscrowBinding:    escrowBinding,
			Contract:         contract,
			RegistryAddress:  registryAddr,
			ChainID:          chainID,
			SellerEVM:        sellerEVM,
			PricingAmount:    pricingAmount,
			FrameLimit:       frameLimit,
			SellerMaOverride: spec.Multiaddr,
			AllowMaOverride:  spec.AllowOverride,
		}, cb)
		if ferr != nil {
			logger.Printf("[session role=remoteid] ERROR: %v", ferr)
			return false
		}
		return final.FinalAction == "approved"

	default:
		logger.Printf("[session role=%s] unsupported role for commerce-mode=full", spec.Role)
		return false
	}
}

// serializedEscrow wraps a payment.EscrowAdapter with a sync.Mutex so
// concurrent on-chain tx submissions from the same buyer EVM key are
// serialized — defensive against go-ethereum's bind.TransactOpts nonce
// race when multiple sessions issue CreateEscrow / Deposit / RequestRelease /
// ApproveRelease in parallel. Phase 3A empirically saw no collision with
// an 8-second manual stagger; the shim costs nothing in correctness terms
// and removes the operational gap.
type serializedEscrow struct {
	inner payment.EscrowAdapter
	mu    sync.Mutex
}

func (s *serializedEscrow) CreateEscrow(ctx context.Context, buyer, seller string, arbiter *string,
	currency string, threshold uint64, agreementHash [32]byte, timeout uint64,
) (payment.EscrowRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.CreateEscrow(ctx, buyer, seller, arbiter, currency, threshold, agreementHash, timeout)
}

func (s *serializedEscrow) Deposit(ctx context.Context, ref payment.EscrowRef, amount string) (payment.DepositResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.Deposit(ctx, ref, amount)
}

func (s *serializedEscrow) GetBalance(ctx context.Context, ref payment.EscrowRef) (payment.Balance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.GetBalance(ctx, ref)
}

func (s *serializedEscrow) RequestRelease(ctx context.Context, ref payment.EscrowRef, amount string,
	recipient string, evidenceHash [32]byte,
) (payment.ReleaseRequestRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.RequestRelease(ctx, ref, amount, recipient, evidenceHash)
}

func (s *serializedEscrow) ApproveRelease(ctx context.Context, ref payment.EscrowRef,
	releaseRef payment.ReleaseRequestRef,
) (payment.ReleaseResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.ApproveRelease(ctx, ref, releaseRef)
}

func (s *serializedEscrow) ClaimRefund(ctx context.Context, ref payment.EscrowRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inner.ClaimRefund(ctx, ref)
}
