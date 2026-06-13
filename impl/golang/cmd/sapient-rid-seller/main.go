// Command sapient-rid-seller is the sensor-side PUSHER in the local SAPIENT
// Remote ID demo. Per the Neuron reverse-connect topology the seller DIALS the
// reachable buyer (--buyer), opens /sapient/detection/2.0.0, and pushes
// SapientMessages sourced from the live neuron-rid-bridge SAPIENT feed
// (--bridge-addr), re-stamping node_id with its Neuron identity. It never
// listens, so a NAT'd sensor needs no port-forwarding.
//
//	sapient-rid-seller --bridge-addr 127.0.0.1:9999 --buyer <buyer-multiaddr>
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/auditlane"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/sapientpb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient/tasking"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

const (
	// EIP-8004 registry backends. memory is the testnet-free SIM default
	// (in-process MemoryRegistryContract, evidence labelled simulated:true);
	// evm registers on a real Identity Registry over JSON-RPC.
	registryBackendMemory = "memory"
	registryBackendEVM    = "evm"

	defaultChainID = uint64(296) // Hedera testnet
	defaultRPCURL  = "https://testnet.hashio.io/api"

	envRegistryContract = "NEURON_REGISTRY_CONTRACT"
	envHederaRPC        = "HEDERA_EVM_RPC"

	// SAPIENT audit lane (004) backends. file keeps the FileLane behaviour
	// byte-identical (no topic, no heartbeat); memory|hcs put the lane on a real
	// TopicAdapter (TopicLane) and enable the 005 heartbeat + Registration.
	auditBackendFile   = "file"
	auditBackendMemory = "memory"
	auditBackendHCS    = "hcs"

	// bridgeStaleAfter is how long the upstream bridge feed may be silent before
	// the seller declares its data plane degraded (heartbeat) / disconnected
	// (StatusReport).
	bridgeStaleAfter = 20 * time.Second
)

// Deps holds dependency-injection seams so tests can drive run() without a
// real EVM RPC. Mirrors cmd/remoteid-seller's Deps. All fields optional:
// production uses defaults that hit the configured chain.
type Deps struct {
	// ContractFactory builds the RegistryContract for --registry-backend=evm.
	// nil => defaultContractFactory (dials RPC, real EVMRegistryContract).
	ContractFactory func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error)

	// TopicAdapter is the commerce bus for --commerce-mode=full with
	// --topic-backend=memory. nil => a fresh MemoryTopicAdapter (useless
	// cross-process; tests inject the bus they share with the buyer).
	TopicAdapter topic.TopicAdapter

	// EscrowAdapter is the settlement adapter for --commerce-mode=full with
	// --escrow-backend=memory. nil => a fresh MemoryEscrow (tests inject the
	// instance they share with the buyer).
	EscrowAdapter payment.EscrowAdapter
}

// ContractFactoryOpts is what run() hands to the contract factory once it
// has parsed flags + env.
type ContractFactoryOpts struct {
	RPCURL          string
	RegistryAddress common.Address
	ChainID         uint64
	SignerKey       *ecdsa.PrivateKey
}

// defaultContractFactory dials the configured RPC and returns a real
// EVMRegistryContract. Used in production; tests inject a memory factory.
func defaultContractFactory(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error) {
	client, err := ethclient.DialContext(ctx, opts.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", opts.RPCURL, err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(opts.SignerKey, new(big.Int).SetUint64(opts.ChainID))
	if err != nil {
		return nil, fmt.Errorf("build transactor: %w", err)
	}
	return registry.NewEVMRegistryContract(client, opts.RegistryAddress, auth)
}

// registryConfig is the resolved registry-backend selection. memory keeps
// the original local/simulated behaviour byte-identical; evm carries the
// validated chain coordinates.
type registryConfig struct {
	backend   string
	addr      keylib.EVMAddress // evm only
	rpc       string            // evm only
	chainID   uint64            // evm only (0 for memory)
	simulated bool              // true => evidence labelled simulated
}

// resolveRegistryConfig validates the --registry-backend flag family. The
// lookup func abstracts os.Getenv so tests can inject env. Key material is
// never part of this config and never appears in errors (SEC-003).
func resolveRegistryConfig(backend, addrFlag, rpcFlag string, chainID uint64, lookup func(string) string) (registryConfig, error) {
	switch backend {
	case registryBackendMemory:
		return registryConfig{backend: registryBackendMemory, simulated: true}, nil
	case registryBackendEVM:
		addrHex := addrFlag
		if addrHex == "" {
			addrHex = lookup(envRegistryContract)
		}
		if addrHex == "" {
			return registryConfig{}, fmt.Errorf("--registry-backend=evm requires --registry-address <0x...> (or env %s)", envRegistryContract)
		}
		addr, err := keylib.EVMAddressFromHex(addrHex)
		if err != nil {
			return registryConfig{}, fmt.Errorf("invalid --registry-address %q: %w", addrHex, err)
		}
		rpc := rpcFlag
		if rpc == "" {
			rpc = lookup(envHederaRPC)
		}
		if rpc == "" {
			rpc = defaultRPCURL
		}
		if chainID == 0 {
			return registryConfig{}, errors.New("--chain-id must be non-zero for --registry-backend=evm")
		}
		return registryConfig{backend: registryBackendEVM, addr: addr, rpc: rpc, chainID: chainID, simulated: false}, nil
	default:
		return registryConfig{}, fmt.Errorf("unknown --registry-backend=%q (want %s|%s)", backend, registryBackendMemory, registryBackendEVM)
	}
}

// frameSink wraps a 009 FrameWriter as a tasking.Forwarder: marshal the
// SapientMessage and write one length-prefixed frame to the buyer stream.
type frameSink struct{ w *delivery.FrameWriter }

func (s frameSink) Send(m *sapientpb.SapientMessage) error {
	b, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	return s.w.WriteFrame(b)
}

// bridgeHealth tracks upstream feed liveness for the 005 heartbeat DegradedFunc
// and the SAPIENT StatusReport bridge entry. Seeded healthy at construction so a
// just-started seller is not reported degraded during warm-up; it flips degraded
// only after staleAfter of silence. Safe for concurrent use.
type bridgeHealth struct {
	last       atomic.Int64 // UnixNano of the most recent frame; seeded at start
	staleAfter time.Duration
}

func newBridgeHealth() *bridgeHealth {
	b := &bridgeHealth{staleAfter: bridgeStaleAfter}
	b.last.Store(time.Now().UnixNano())
	return b
}

func (b *bridgeHealth) markFrame() { b.last.Store(time.Now().UnixNano()) }

func (b *bridgeHealth) connected() bool {
	return time.Since(time.Unix(0, b.last.Load())) <= b.staleAfter
}

// auditLaneSetup is the resolved SAPIENT control/audit lane. In file mode the
// lane is a FileLane (or nil when --control-lane is empty) with no topic, so the
// heartbeat is disabled; in memory|hcs mode it is a TopicLane and the adapter +
// stdOut ref + card disclosure (transport + topic ids) are populated.
type auditLaneSetup struct {
	lane        auditlane.Lane
	adapter     topic.TopicAdapter        // nil in file mode
	stdOutRef   topic.TopicRef            // zero in file mode
	transport   string                    // "" in file mode (card stays auditlane-file)
	topicConfig map[string]map[string]any // nil in file mode
	topicBacked bool
}

// setupAuditLane builds the SAPIENT control/audit lane for the chosen backend.
// file → FileLane (or no control plane when controlLanePath is empty), byte-
// identical to the original; memory|hcs → TopicLane over a real TopicAdapter,
// also yielding the stdOut ref for the heartbeat and the transport + topic-id
// disclosure for the Agent Card. The hcs path NEVER falls back to memory.
func setupAuditLane(ctx context.Context, backend, controlLanePath, evmHex8 string, nk *keylib.NeuronPrivateKey, deps Deps, logger *log.Logger) (auditLaneSetup, error) {
	switch backend {
	case auditBackendFile:
		if controlLanePath == "" {
			return auditLaneSetup{}, nil // data-only: no control plane (unchanged)
		}
		return auditLaneSetup{lane: auditlane.NewFileLane(controlLanePath)}, nil

	case auditBackendMemory, auditBackendHCS:
		var (
			adapter                        topic.TopicAdapter
			stdInRef, stdOutRef, stdErrRef topic.TopicRef
		)
		if backend == auditBackendHCS {
			be, err := remoteid.NewHCSBackend(ctx, remoteid.HCSBackendOptions{
				Role:            remoteid.HCSRoleSeller,
				TopicMemoPrefix: "sapient-" + evmHex8 + "-",
			})
			if err != nil {
				return auditLaneSetup{}, err
			}
			adapter, stdInRef, stdOutRef, stdErrRef = be.Adapter, be.StdInRef, be.StdOutRef, be.StdErrRef
			logger.Printf("[auditlane:hcs] operator=%s topics: stdIn=%s stdOut=%s stdErr=%s",
				be.OperatorID, stdInRef.Locator(), stdOutRef.Locator(), stdErrRef.Locator())
		} else {
			adapter = deps.TopicAdapter
			if adapter == nil {
				adapter = topic.NewMemoryTopicAdapter()
			}
			var err error
			if stdInRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdin-" + evmHex8}); err != nil {
				return auditLaneSetup{}, fmt.Errorf("create stdIn topic: %w", err)
			}
			if stdOutRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdout-" + evmHex8}); err != nil {
				return auditLaneSetup{}, fmt.Errorf("create stdOut topic: %w", err)
			}
			if stdErrRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stderr-" + evmHex8}); err != nil {
				return auditLaneSetup{}, fmt.Errorf("create stdErr topic: %w", err)
			}
		}
		lane, err := auditlane.NewTopicLane(adapter, nk, map[auditlane.Role]topic.TopicRef{
			auditlane.RoleStdIn:  stdInRef,
			auditlane.RoleStdOut: stdOutRef,
			auditlane.RoleStdErr: stdErrRef,
		})
		if err != nil {
			return auditLaneSetup{}, err
		}
		return auditLaneSetup{
			lane:      lane,
			adapter:   adapter,
			stdOutRef: stdOutRef,
			transport: backend,
			topicConfig: map[string]map[string]any{
				"stdIn":  {"topicId": stdInRef.Locator()},
				"stdOut": {"topicId": stdOutRef.Locator()},
				"stdErr": {"topicId": stdErrRef.Locator()},
			},
			topicBacked: true,
		}, nil

	default:
		return auditLaneSetup{}, fmt.Errorf("unknown --auditlane-backend=%q (want %s|%s|%s)", backend, auditBackendFile, auditBackendMemory, auditBackendHCS)
	}
}

func main() {
	if err := run(os.Args[1:], Deps{}); err != nil {
		log.Fatalf("sapient-rid-seller: %v", err)
	}
}

func run(args []string, deps Deps) error {
	fs := flag.NewFlagSet("sapient-rid-seller", flag.ContinueOnError)
	var (
		bridgeAddr = fs.String("bridge-addr", "127.0.0.1:9999", "neuron-rid-bridge SAPIENT feed (its --sapient-listen, run with --sapient-format json)")
		buyer      = fs.String("buyer", "", "reachable buyer multiaddr to dial, e.g. /ip4/127.0.0.1/udp/19192/quic-v1/p2p/<id> [required unless --register-only]")
		listen     = fs.String("listen", "/ip4/127.0.0.1/udp/0/quic-v1", "libp2p host listen multiaddr (ephemeral)")
		keyHex     = fs.String("key-hex", "", "32-byte hex secp256k1 key; defaults to ephemeral")
		// SAPIENT control plane (additive). Empty --control-lane => data-only
		// (the seller behaves exactly as without these flags).
		controlLane    = fs.String("control-lane", "", "auditable-lane stub file path (file:PATH or PATH) for Task/TaskAck/StatusReport; empty disables the control plane (only meaningful with --auditlane-backend=file)")
		feedSource     = fs.String("feed-source", "synthetic", "feedSource advertised in StatusReport: live|replay|synthetic|placeholder (017 FR-R-E02)")
		statusInterval = fs.Duration("status-interval", 5*time.Second, "SAPIENT StatusReport cadence on the auditable lane")
		sessionID      = fs.String("session-id", "hldmm", "consumer/session id this seller serves (the key a Task addresses for per-session STOP/START)")
		// SAPIENT control/evidence plane backend (additive). file (default) keeps the
		// FileLane behaviour byte-identical and runs no heartbeat; memory|hcs put the
		// 004 audit lane on a real TopicAdapter and additionally run the 005 heartbeat
		// + a Registration on the seller's stdOut topic.
		auditlaneBackend  = fs.String("auditlane-backend", auditBackendFile, "SAPIENT audit lane (004) backend: file (default) | memory (in-process) | hcs (real Hedera topics; env HEDERA_OPERATOR_ID/HEDERA_OPERATOR_KEY)")
		heartbeatInterval = fs.Duration("heartbeat-interval", sapient.DefaultHeartbeatInterval, "spec-005 heartbeat cadence on stdOut (only when --auditlane-backend is memory|hcs)")
		// EIP-8004 Agent Card evidence (additive). All empty/false => no registry,
		// no card; the seller behaves byte-identically to without these flags.
		register     = fs.Bool("register", false, "build + register the seller's EIP-8004 Agent Card (backend per --registry-backend) before pushing")
		agentCardOut = fs.String("agent-card-out", "", "write the registered Agent Card (agentURI JSON) to this path; implies --register")
		registryOut  = fs.String("registry-out", "", "write the agent evidence record (agentId + identity binding + card) to this path; implies --register")
		// EIP-8004 registry backend selection (additive). Default memory keeps
		// the local/simulated path byte-identical; evm registers on a real
		// Identity Registry (Hedera testnet by default).
		registryBackend = fs.String("registry-backend", registryBackendMemory, "EIP-8004 registry backend: memory (in-process SIM, default) | evm (real chain; requires --registry-address)")
		registryAddress = fs.String("registry-address", "", "Identity Registry contract address for --registry-backend=evm (env fallback "+envRegistryContract+")")
		rpcURL          = fs.String("rpc-url", "", "EVM JSON-RPC endpoint for --registry-backend=evm (env fallback "+envHederaRPC+"; default "+defaultRPCURL+")")
		chainIDFlag     = fs.Uint64("chain-id", defaultChainID, "EVM chain id for --registry-backend=evm (Hedera testnet = 296)")
		registerOnly    = fs.Bool("register-only", false, "register the Agent Card, write the requested artefacts, then exit 0 WITHOUT dialing the buyer (implies --register; --buyer not required)")
		// Full-payment commerce mode (additive). off = byte-identical original
		// behaviour; full = 008 escrow-gated streaming (the buyer's multiaddr
		// arrives via the reverse ConnectionSetup, --buyer is not used).
		commerceMode        = fs.String("commerce-mode", sapient.CommerceModeOff, "008 commerce posture: off (default) | full (escrow-gated streaming + settlement)")
		topicBackend        = fs.String("topic-backend", "memory", "commerce bus for --commerce-mode=full: memory (in-process) | hcs (real Hedera topics; env HEDERA_OPERATOR_ID/HEDERA_OPERATOR_KEY)")
		escrowBackend       = fs.String("escrow-backend", "memory", "settlement for --commerce-mode=full: memory | evm (env NEURON_ESCROW_CONTRACT + NEURON_TOKEN_CONTRACT + signer key)")
		pricingAmount       = fs.String("pricing-amount", "1", "rid service price for --commerce-mode=full (decimal integer, settlement-token base units; the evm escrow rejects 0)")
		commerceEvidenceOut = fs.String("commerce-evidence-out", "", "write the commerce-session evidence JSON to this path (--commerce-mode=full)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch *commerceMode {
	case sapient.CommerceModeOff, sapient.CommerceModeFull:
	default:
		return fmt.Errorf("unknown --commerce-mode=%q (want %s|%s)", *commerceMode, sapient.CommerceModeOff, sapient.CommerceModeFull)
	}
	if *registerOnly && *commerceMode == sapient.CommerceModeFull {
		return errors.New("--register-only and --commerce-mode=full are mutually exclusive (commerce registers its own card with topic locators)")
	}
	if *buyer == "" && !*registerOnly && *commerceMode != sapient.CommerceModeFull {
		return errors.New("--buyer <multiaddr> is required (the seller dials the buyer)")
	}
	logger := log.New(os.Stderr, "[sapient-rid-seller] ", log.LstdFlags)

	regCfg, err := resolveRegistryConfig(*registryBackend, *registryAddress, *rpcURL, *chainIDFlag, os.Getenv)
	if err != nil {
		return err
	}

	key, err := resolveKey(keyHexOrEnv(*keyHex))
	if err != nil {
		return err
	}
	nk, err := keylib.NeuronPrivateKeyFromBytes(ethcrypto.FromECDSA(key))
	if err != nil {
		return fmt.Errorf("derive neuron identity: %w", err)
	}
	evmHex := nk.PublicKey().EVMAddress().Hex()
	nodeID := sapient.NodeIDFromIdentity(evmHex)

	host, err := delivery.NewLibp2pHost(key, *listen)
	if err != nil {
		return fmt.Errorf("create host: %w", err)
	}
	defer host.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Printf("identity evm=%s node_id=%s peerID=%s", evmHex, nodeID, host.ID())

	// Full-payment commerce mode: registration (with topic locators in the
	// card) + escrow-gated streaming + settlement. Replaces the --buyer dial
	// (the buyer's multiaddr arrives via the reverse ConnectionSetup).
	if *commerceMode == sapient.CommerceModeFull {
		return runSellerCommerce(ctx, &nk, host, nodeID, sellerCommerceArgs{
			bridgeAddr:    *bridgeAddr,
			feedSource:    *feedSource,
			topicBackend:  *topicBackend,
			escrowBackend: *escrowBackend,
			pricingAmount: *pricingAmount,
			evidenceOut:   *commerceEvidenceOut,
			agentCardOut:  *agentCardOut,
			registryOut:   *registryOut,
			regCfg:        regCfg,
		}, deps, logger)
	}

	// SAPIENT control/evidence plane backend (additive). Built BEFORE registration
	// so a topic-backed lane (memory|hcs) can advertise its real transport + topic
	// IDs in the Agent Card. file mode is byte-identical to the original.
	if *auditlaneBackend == auditBackendHCS {
		if missing := remoteid.MissingHCSEnvVars(os.Getenv); len(missing) > 0 {
			return fmt.Errorf("--auditlane-backend=hcs requires env %v (no silent fallback to memory)", missing)
		}
	}
	evmHex8 := evmHex[2:10]
	audit, err := setupAuditLane(ctx, *auditlaneBackend, strings.TrimPrefix(*controlLane, "file:"), evmHex8, &nk, deps, logger)
	if err != nil {
		return fmt.Errorf("setup audit lane: %w", err)
	}
	if audit.lane != nil {
		defer audit.lane.Close()
	}

	// EIP-8004 Agent Card evidence mode (additive, opt-in). Builds + registers the
	// seller's card (disclosing the audit-lane transport + topic IDs when topic-
	// backed), writes the evidence, then falls through to the reverse-connect.
	var agentURISha string
	if *register || *registerOnly || *agentCardOut != "" || *registryOut != "" {
		res, rerr := registerAgentCard(ctx, &nk, host.ID().String(), *agentCardOut, *registryOut, *feedSource, regCfg, deps, logger, nil, audit.transport, audit.topicConfig)
		if rerr != nil {
			return fmt.Errorf("register agent card: %w", rerr)
		}
		agentURISha = res.AgentURISha256
	}
	if *registerOnly {
		logger.Printf("register-only: done; exiting before buyer dial")
		return nil
	}

	buyerMA, err := ma.NewMultiaddr(*buyer)
	if err != nil {
		return fmt.Errorf("parse --buyer %q: %w", *buyer, err)
	}
	buyerInfo, err := peer.AddrInfoFromP2pAddr(buyerMA)
	if err != nil {
		return fmt.Errorf("parse buyer addrinfo: %w", err)
	}

	logger.Printf("dialing buyer=%s", buyerInfo.ID)
	if err := host.Connect(ctx, *buyerInfo); err != nil {
		return fmt.Errorf("connect buyer: %w", err)
	}
	stream, err := host.NewStream(ctx, buyerInfo.ID, protocol.ID(sapient.ProtocolDetection))
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer stream.Close()
	logger.Printf("stream open to buyer=%s; sourcing bridge=%s", buyerInfo.ID, *bridgeAddr)

	writer := delivery.NewFrameWriter(stream)
	msgs, errc := sapient.ReadBridgeFeed(ctx, *bridgeAddr)

	// SAPIENT control/evidence plane (additive). A nil lane (file backend with no
	// --control-lane) skips this block entirely — data-only, byte-identical. A
	// topic-backed lane additionally runs the 005 heartbeat + a Registration on
	// stdOut. The shared bridgeHealth drives both the heartbeat DegradedFunc and
	// the StatusReport bridge entry.
	var (
		mgr       *tasking.Manager
		mgrCancel context.CancelFunc
		hbLoop    *sapient.HeartbeatLoop
		bh        *bridgeHealth
		sent      uint64
	)
	if audit.lane != nil {
		if !tasking.ValidFeedSource(*feedSource) {
			return fmt.Errorf("--feed-source %q invalid (want live|replay|synthetic|placeholder)", *feedSource)
		}
		bh = newBridgeHealth()
		mgrCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		mgrCancel = cancel
		mgr = tasking.NewManager(tasking.Options{
			ASMNodeID:      nodeID,
			Lane:           audit.lane,
			FeedSource:     *feedSource,
			StatusInterval: *statusInterval,
			Logger:         logger,
			StatusInputs: func() tasking.StatusSnapshot {
				return tasking.StatusSnapshot{BridgeConnected: bh.connected(), Degraded: !bh.connected()}
			},
		})
		mgr.RegisterSession(*sessionID, frameSink{w: writer})
		if err := mgr.Start(mgrCtx); err != nil {
			return fmt.Errorf("start tasking manager: %w", err)
		}
		logger.Printf("control plane ON: backend=%s session=%s feedSource=%s status=%s",
			*auditlaneBackend, *sessionID, *feedSource, *statusInterval)

		if audit.topicBacked {
			// Registration on the auditable lane (015 FR-S21) — topic-backed only,
			// so the file-mode audit stream stays byte-identical.
			if rerr := mgr.EmitRegistration(mgrCtx); rerr != nil {
				logger.Printf("emit Registration: %v", rerr)
			}
			// 005 heartbeat on the shared stdOut topic.
			hb, herr := sapient.StartHeartbeatLoop(mgrCtx, sapient.HeartbeatOptions{
				Key:            &nk,
				StdOutRef:      audit.stdOutRef,
				Adapter:        audit.adapter,
				Interval:       *heartbeatInterval,
				Logger:         logger,
				SellerEVM:      evmHex,
				SellerPeerID:   host.ID().String(),
				ASMNodeID:      nodeID,
				FeedSource:     *feedSource,
				CommerceMode:   *commerceMode,
				TopicBackend:   *auditlaneBackend,
				AgentURISha256: agentURISha,
				DegradedFunc:   func() bool { return !bh.connected() },
			})
			if herr != nil {
				return fmt.Errorf("start heartbeat: %w", herr)
			}
			hbLoop = hb
			logger.Printf("005 heartbeat ON: stdOut=%s interval=%s", audit.stdOutRef.Locator(), *heartbeatInterval)
		}
	}

	if mgr != nil {
		for msg := range msgs {
			msg.NodeId = proto.String(nodeID) // re-stamp identity at the runtime boundary
			bh.markFrame()
			if ferr := mgr.Forward(msg); ferr != nil {
				logger.Printf("forward error after %d frames: %v", sent, ferr)
				break
			}
			sent++
		}
		mgrCancel()
		mgr.Wait()
		if hbLoop != nil {
			<-hbLoop.Done // let the graceful OFFLINE sentinel publish
		}
	} else {
		for msg := range msgs {
			msg.NodeId = proto.String(nodeID) // re-stamp identity at the runtime boundary
			b, merr := proto.Marshal(msg)
			if merr != nil {
				logger.Printf("marshal error: %v", merr)
				continue
			}
			if werr := writer.WriteFrame(b); werr != nil {
				logger.Printf("write error after %d frames: %v", sent, werr)
				break
			}
			sent++
		}
	}
	_ = stream.CloseWrite() // clean EOF for the buyer

	select {
	case e := <-errc:
		if e != nil && !errors.Is(e, context.Canceled) {
			logger.Printf("bridge feed error after %d frames: %v", sent, e)
		}
	default:
	}
	logger.Printf("done; pushed %d frames", sent)
	return nil
}

// localRegistryAddr is the placeholder Identity Registry address recorded in
// local/simulated evidence (the in-memory contract is not a real deployment).
// 0x…8004 nods to EIP-8004.
const localRegistryAddr = "0x0000000000000000000000000000000000008004"

// registerAgentCard builds the seller's EIP-8004 Agent Card, registers it on
// the selected registry backend (memory = in-process SIM, evm = real chain via
// the Deps.ContractFactory seam), and writes the requested evidence artefacts.
// It asserts the dial host and the card advertise the same PeerID — the
// V-REG-12 invariant that makes the card's identity binding verifiable against
// the seller that dials in. Registration is idempotent either way
// (registry.RegisterOrUpdate): re-runs reuse or refresh, never double-mint.
// commerce, when non-nil, switches the card to the full-payment variant
// (settlement advertisement + topic locators).
func registerAgentCard(ctx context.Context, nk *keylib.NeuronPrivateKey, hostPeerID, cardOut, registryOut, feedSource string, cfg registryConfig, deps Deps, logger *log.Logger, commerce *sapient.CommerceCardOptions, topicTransport string, topicConfig map[string]map[string]any) (sapient.RegisterResult, error) {
	card, err := sapient.BuildSellerCard(sapient.SellerCardOptions{
		ChildKey: nk, ChainID: cfg.chainID, Commerce: commerce,
		TopicTransport: topicTransport, TopicConfig: topicConfig,
	})
	if err != nil {
		return sapient.RegisterResult{}, fmt.Errorf("build card: %w", err)
	}
	if hostPeerID != card.PeerID {
		return sapient.RegisterResult{}, fmt.Errorf("identity mismatch: host PeerID %s != card PeerID %s", hostPeerID, card.PeerID)
	}

	var (
		regAddr  keylib.EVMAddress
		chainID  uint64
		contract registry.RegistryContract
	)
	switch cfg.backend {
	case registryBackendEVM:
		regAddr = cfg.addr
		chainID = cfg.chainID
		signer, kerr := nk.ToBlockchainKey()
		if kerr != nil {
			return sapient.RegisterResult{}, fmt.Errorf("derive signer key: %w", kerr)
		}
		factory := deps.ContractFactory
		if factory == nil {
			factory = defaultContractFactory
		}
		contract, err = factory(ctx, ContractFactoryOpts{
			RPCURL:          cfg.rpc,
			RegistryAddress: common.HexToAddress(cfg.addr.Hex()),
			ChainID:         cfg.chainID,
			SignerKey:       signer,
		})
		if err != nil {
			return sapient.RegisterResult{}, fmt.Errorf("build registry contract: %w", err)
		}
	default: // memory — the original local/simulated path, byte-identical.
		regAddr, err = keylib.EVMAddressFromHex(localRegistryAddr)
		if err != nil {
			return sapient.RegisterResult{}, fmt.Errorf("registry addr: %w", err)
		}
		chainID = 0
		mem := registry.NewMemoryRegistryContract()
		mem.SetPendingOwner(common.BytesToAddress(nk.PublicKey().EVMAddress().Bytes()))
		contract = mem
	}

	res, err := sapient.RegisterSeller(ctx, nk, card, regAddr, chainID, contract)
	if err != nil {
		return sapient.RegisterResult{}, err
	}
	if cfg.backend == registryBackendEVM {
		logger.Printf("agent card registered (on-chain): backend=evm outcome=%s agentId=%v sellerEVM=%s registry=%s chainId=%d node_id=%s peerID=%s txHash=%s agentURISha256=%s",
			res.Outcome, res.TokenID, res.SellerEVM.Hex(), res.RegistryAddress.Hex(), res.ChainID, res.NodeID, res.PeerID, res.TransactionHash, res.AgentURISha256)
	} else {
		logger.Printf("agent card registered (local/simulated): agentId=%v evm=%s node_id=%s peerID=%s outcome=%s",
			res.TokenID, res.SellerEVM.Hex(), res.NodeID, res.PeerID, res.Outcome)
	}

	if cardOut != "" {
		if werr := os.WriteFile(cardOut, append([]byte(res.AgentURIJSON), '\n'), 0o644); werr != nil {
			return res, fmt.Errorf("write agent-card-out: %w", werr)
		}
		logger.Printf("wrote agent card -> %s", cardOut)
	}
	if registryOut != "" {
		ev := sapient.EvidenceFromResult(res, cfg.simulated)
		ev.FeedSource = feedSource // runtime provenance (017 FR-R-E02), not part of the card
		if werr := sapient.WriteEvidence(registryOut, ev); werr != nil {
			return res, werr
		}
		logger.Printf("wrote agent evidence -> %s", registryOut)
	}
	return res, nil
}

// sellerCommerceArgs carries the parsed --commerce-mode=full flags.
type sellerCommerceArgs struct {
	bridgeAddr    string
	feedSource    string
	topicBackend  string
	escrowBackend string
	pricingAmount string
	evidenceOut   string
	agentCardOut  string
	registryOut   string
	regCfg        registryConfig
}

// runSellerCommerce drives the full-payment seller: build backends → create
// the 3 commerce topics → register the COMMERCE card (topic locators +
// settlement advertisement) → await the buyer's session (escrow verified
// BEFORE streaming — sapient.StartSellerCommerce's pre-stream gate) → dial
// the buyer from the reverse ConnectionSetup → pump bridge frames → settle.
func runSellerCommerce(ctx context.Context, nk *keylib.NeuronPrivateKey, h lphost.Host, nodeID string, a sellerCommerceArgs, deps Deps, logger *log.Logger) error {
	evmHex8 := nk.PublicKey().EVMAddress().Hex()[2:10]

	// --- topic backend (the commerce bus + the seller's 3 channels) ---
	var (
		adapter                       topic.TopicAdapter
		stdInRef, stdOutRef, stdErrRef topic.TopicRef
	)
	switch a.topicBackend {
	case "hcs":
		be, err := remoteid.NewHCSBackend(ctx, remoteid.HCSBackendOptions{
			Role:            remoteid.HCSRoleSeller,
			TopicMemoPrefix: "sapient-" + evmHex8 + "-",
		})
		if err != nil {
			return err
		}
		adapter, stdInRef, stdOutRef, stdErrRef = be.Adapter, be.StdInRef, be.StdOutRef, be.StdErrRef
		logger.Printf("[hcs] operator=%s topics: stdIn=%s stdOut=%s stdErr=%s",
			be.OperatorID, stdInRef.Locator(), stdOutRef.Locator(), stdErrRef.Locator())
	case "memory":
		adapter = deps.TopicAdapter
		if adapter == nil {
			adapter = topic.NewMemoryTopicAdapter()
		}
		var err error
		if stdInRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdin-" + evmHex8}); err != nil {
			return fmt.Errorf("create stdIn topic: %w", err)
		}
		if stdOutRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stdout-" + evmHex8}); err != nil {
			return fmt.Errorf("create stdOut topic: %w", err)
		}
		if stdErrRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-stderr-" + evmHex8}); err != nil {
			return fmt.Errorf("create stdErr topic: %w", err)
		}
	default:
		return fmt.Errorf("unknown --topic-backend=%q (want memory|hcs)", a.topicBackend)
	}

	// --- escrow backend ---
	var (
		escrow         payment.EscrowAdapter
		escrowBinding  = sapient.SettlementBindingMemory
		escrowContract string
		tokenContract  string
		tokenBalance   func(ctx context.Context) (*big.Int, error)
	)
	switch a.escrowBackend {
	case "evm":
		if err := remoteid.ValidatePricingForEVM(a.pricingAmount); err != nil {
			return err
		}
		be, err := remoteid.NewEVMBackend(ctx, remoteid.EVMBackendOptions{
			DefaultRPCURL:  defaultRPCURL,
			DefaultChainID: defaultChainID,
		})
		if err != nil {
			return err
		}
		escrow, escrowBinding = be.Escrow, be.EscrowBinding
		escrowContract, tokenContract = be.EscrowContract, be.TokenContract
		tokenBalance = sapient.TokenBalanceProbe(be.RPCURL, common.HexToAddress(be.TokenContract),
			common.BytesToAddress(nk.PublicKey().EVMAddress().Bytes()))
		logger.Printf("[evm] rpc=%s chainId=%d escrowContract=%s tokenContract=%s operator=%s",
			be.RPCURL, be.ChainID, be.EscrowContract, be.TokenContract, be.OperatorAddr)
	case "memory":
		escrow = deps.EscrowAdapter
		if escrow == nil {
			escrow = payment.NewMemoryEscrow()
		}
	default:
		return fmt.Errorf("unknown --escrow-backend=%q (want memory|evm)", a.escrowBackend)
	}

	// --- the COMMERCE card: settlement advertisement + topic locators ---
	commerceCard := &sapient.CommerceCardOptions{
		SettlementBinding: escrowBinding,
		EscrowContract:    escrowContract,
		TokenContract:     tokenContract,
		PricingAmount:     a.pricingAmount,
		PricingCurrency:   sapient.DefaultPricingCurrency,
		TopicConfig: map[string]map[string]any{
			"stdIn":  {"topicId": stdInRef.Locator()},
			"stdOut": {"topicId": stdOutRef.Locator()},
			"stdErr": {"topicId": stdErrRef.Locator()},
		},
		TopicTransport: a.topicBackend,
	}
	// Commerce supplies its topic disclosure via the Commerce card option, so the
	// non-commerce audit-lane transport/config params stay empty here.
	regRes, err := registerAgentCard(ctx, nk, h.ID().String(), a.agentCardOut, a.registryOut, a.feedSource, a.regCfg, deps, logger, commerceCard, "", nil)
	if err != nil {
		return fmt.Errorf("register commerce card: %w", err)
	}
	logger.Printf("[seller-commerce] commerce card live: stdIn=%s stdOut=%s stdErr=%s pricing=%s %s",
		stdInRef.Locator(), stdOutRef.Locator(), stdErrRef.Locator(), a.pricingAmount, sapient.DefaultPricingCurrency)

	// --- the commerce session (blocks awaiting the buyer; the pre-stream
	// escrow gate fires inside StartSellerCommerce) ---
	session, err := sapient.StartSellerCommerce(ctx, sapient.SellerCommerceOptions{
		Key:           nk,
		Adapter:       adapter,
		SellerStdIn:   stdInRef,
		Host:          h,
		Escrow:        escrow,
		EscrowBinding: escrowBinding,
		TokenBalance:  tokenBalance,
		Logger:        logger,
	})
	if err != nil {
		return err
	}

	// --- data plane: dial the buyer (reverse-connect) + pump bridge frames ---
	logger.Printf("dialing buyer=%s (from reverse connectionSetup)", session.BuyerAddr.ID)
	if err := h.Connect(ctx, session.BuyerAddr); err != nil {
		return fmt.Errorf("connect buyer: %w", err)
	}
	stream, err := h.NewStream(ctx, session.BuyerAddr.ID, protocol.ID(sapient.ProtocolDetection))
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer stream.Close()
	writer := delivery.NewFrameWriter(stream)

	pumpCtx, pumpCancel := context.WithCancel(ctx)
	defer pumpCancel()
	msgs, errc := sapient.ReadBridgeFeed(pumpCtx, a.bridgeAddr)
	var sent, firstAt, lastAt atomic.Uint64
	go func() {
		for msg := range msgs {
			msg.NodeId = proto.String(nodeID) // re-stamp identity at the runtime boundary
			b, merr := proto.Marshal(msg)
			if merr != nil {
				logger.Printf("marshal error: %v", merr)
				continue
			}
			if werr := writer.WriteFrame(b); werr != nil {
				logger.Printf("write error after %d frames: %v (buyer closed the stream?)", sent.Load(), werr)
				return
			}
			now := uint64(time.Now().UnixNano())
			if sent.Add(1) == 1 {
				firstAt.Store(now)
			}
			lastAt.Store(now)
		}
	}()

	// --- settlement (blocks on ServiceStop; the pump runs concurrently) ---
	result, err := session.Finalise(ctx, func() (uint64, uint64, uint64) {
		return sent.Load(), firstAt.Load(), lastAt.Load()
	})
	pumpCancel()
	_ = stream.CloseWrite()
	select {
	case e := <-errc:
		if e != nil && !errors.Is(e, context.Canceled) {
			logger.Printf("bridge feed note after %d frames: %v", sent.Load(), e)
		}
	default:
	}
	if err != nil {
		return fmt.Errorf("seller commerce settlement: %w", err)
	}
	logger.Printf("[seller-commerce] %s requestID=%s frames=%d released-to-seller escrowRef=%s ack=%s",
		result.FinalState, result.RequestID, sent.Load(), result.EscrowRef, result.InvoiceAckAction)

	if a.evidenceOut != "" {
		tokenIDStr := ""
		if regRes.TokenID != nil {
			tokenIDStr = regRes.TokenID.String()
		}
		ev := sapient.CommerceEvidence{
			RequestID:         result.RequestID,
			Role:              "seller",
			Service:           sapient.CommerceServiceName,
			Protocol:          sapient.ProtocolDetection,
			BuyerEVM:          result.BuyerEVM,
			SellerEVM:         nk.PublicKey().EVMAddress().Hex(),
			SellerAgentID:     tokenIDStr,
			SellerPeerID:      h.ID().String(),
			BuyerPeerID:       result.BuyerPeerID,
			RegistryAddress:   regRes.RegistryAddress.Hex(),
			EscrowContract:    escrowContract,
			TokenContract:     tokenContract,
			ChainID:           a.regCfg.chainID,
			TopicBackend:      a.topicBackend,
			EscrowBackend:     a.escrowBackend,
			RegistryBackend:   a.regCfg.backend,
			Topics: map[string]string{
				"sellerStdIn": stdInRef.Locator(), "sellerStdOut": stdOutRef.Locator(), "sellerStdErr": stdErrRef.Locator(),
			},
			EscrowRef:         result.EscrowRef,
			EscrowAvailable:   result.EscrowAvailable,
			ReleaseRequestRef: result.ReleaseRequestRef,
			InvoiceAckAction:  result.InvoiceAckAction,
			EvidenceHash:      result.EvidenceHash,
			SellerTokenDelta:  result.TokenDelta,
			FrameCount:        sent.Load(),
			FinalState:        string(result.FinalState),
		}
		if werr := sapient.WriteCommerceEvidence(a.evidenceOut, ev); werr != nil {
			return werr
		}
		logger.Printf("wrote commerce evidence -> %s", a.evidenceOut)
	}
	return nil
}

// keyHexOrEnv returns the --key-hex flag value, falling back to the
// NEURON_KEY_HEX environment variable (the systemd EnvironmentFile path, so
// the key never appears in process argv). An explicit flag wins; both empty
// means ephemeral. The value itself is never logged (SEC-003).
func keyHexOrEnv(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("NEURON_KEY_HEX")
}

// resolveKey parses --key-hex into a secp256k1 *ecdsa.PrivateKey, or generates an
// ephemeral key when hexStr is empty.
func resolveKey(hexStr string) (*ecdsa.PrivateKey, error) {
	if hexStr == "" {
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return nil, fmt.Errorf("generate ephemeral key: %w", err)
		}
		raw[0] &^= 0x80
		nk, err := keylib.NeuronPrivateKeyFromBytes(raw[:])
		if err != nil {
			return nil, fmt.Errorf("wrap ephemeral key: %w", err)
		}
		return nk.ToBlockchainKey()
	}
	nk, err := keylib.NeuronPrivateKeyFromHex(hexStr)
	if err != nil {
		return nil, err
	}
	return nk.ToBlockchainKey()
}
