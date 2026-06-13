// Command sapient-jv-seller is the JetVision-side PUSHER in the multi-source
// SAPIENT demo — the ADS-B sibling of cmd/sapient-rid-seller. Per the Neuron
// reverse-connect topology the seller DIALS the reachable buyer (--buyer),
// opens /sapient/detection/2.0.0, and pushes SapientMessages sourced from the
// neuron-jv-bridge SAPIENT feed (--bridge-addr, LE-framed BSI Flex 335 v2.0
// protobuf), re-stamping node_id with its Neuron identity. It never listens,
// so a NAT'd sensor host needs no port-forwarding.
//
// The data plane is byte-for-byte the rid seller's (opaque SapientMessage
// forwarding); only the identity surface differs: the Agent Card advertises
// the jetvision-adsb-sapient service with the neuron.adsb/1 extension and the
// JV capability vocabulary (sapient.JetVisionProfile). The commerce path is
// deliberately ABSENT — this seller is advertisement-only (no escrow flags).
//
//	sapient-jv-seller --bridge-addr 127.0.0.1:40005 --buyer <buyer-multiaddr>
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

	// SAPIENT audit lane (004) backends — same family as the rid seller.
	auditBackendFile   = "file"
	auditBackendMemory = "memory"
	auditBackendHCS    = "hcs"

	// bridgeStaleAfter is how long the upstream jv-bridge feed may be silent
	// before the seller declares its data plane degraded. The bridge dedups a
	// static sky, so quiet air (few aircraft, no change) can legitimately pause
	// the feed — this is a heartbeat disclosure, not an error.
	bridgeStaleAfter = 60 * time.Second
)

// Deps holds dependency-injection seams so tests can drive run() without a
// real EVM RPC. Mirrors cmd/sapient-rid-seller's Deps minus the commerce
// adapters (this seller has no commerce path).
type Deps struct {
	// ContractFactory builds the RegistryContract for --registry-backend=evm.
	// nil => defaultContractFactory (dials RPC, real EVMRegistryContract).
	ContractFactory func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error)

	// TopicAdapter backs --auditlane-backend=memory (tests inject the bus they
	// observe). nil => a fresh MemoryTopicAdapter.
	TopicAdapter topic.TopicAdapter
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

// registryConfig is the resolved registry-backend selection.
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
// and the SAPIENT StatusReport bridge entry. Seeded healthy at construction.
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

// auditLaneSetup is the resolved SAPIENT control/audit lane (same shape as the
// rid seller's).
type auditLaneSetup struct {
	lane        auditlane.Lane
	adapter     topic.TopicAdapter        // nil in file mode
	stdOutRef   topic.TopicRef            // zero in file mode
	transport   string                    // "" in file mode (card stays auditlane-file)
	topicConfig map[string]map[string]any // nil in file mode
	topicBacked bool
}

// setupAuditLane builds the SAPIENT control/audit lane for the chosen backend.
func setupAuditLane(ctx context.Context, backend, controlLanePath, evmHex8 string, nk *keylib.NeuronPrivateKey, deps Deps, logger *log.Logger) (auditLaneSetup, error) {
	switch backend {
	case auditBackendFile:
		if controlLanePath == "" {
			return auditLaneSetup{}, nil // data-only: no control plane
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
				TopicMemoPrefix: "sapient-jv-" + evmHex8 + "-",
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
			if stdInRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-jv-stdin-" + evmHex8}); err != nil {
				return auditLaneSetup{}, fmt.Errorf("create stdIn topic: %w", err)
			}
			if stdOutRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-jv-stdout-" + evmHex8}); err != nil {
				return auditLaneSetup{}, fmt.Errorf("create stdOut topic: %w", err)
			}
			if stdErrRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "sapient-jv-stderr-" + evmHex8}); err != nil {
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
		log.Fatalf("sapient-jv-seller: %v", err)
	}
}

func run(args []string, deps Deps) error {
	fs := flag.NewFlagSet("sapient-jv-seller", flag.ContinueOnError)
	var (
		bridgeAddr = fs.String("bridge-addr", "127.0.0.1:40005", "neuron-jv-bridge SAPIENT feed (its --sapient-listen, LE-framed BSI Flex 335 v2.0 protobuf)")
		buyer      = fs.String("buyer", "", "reachable buyer multiaddr to dial, e.g. /ip4/127.0.0.1/udp/19192/quic-v1/p2p/<id> [required unless --register-only]")
		listen     = fs.String("listen", "/ip4/127.0.0.1/udp/0/quic-v1", "libp2p host listen multiaddr (ephemeral)")
		keyHex     = fs.String("key-hex", "", "32-byte hex secp256k1 key; defaults to ephemeral (env NEURON_KEY_HEX)")
		// SAPIENT control plane (additive). Empty --control-lane => data-only.
		controlLane    = fs.String("control-lane", "", "auditable-lane stub file path (file:PATH or PATH) for Task/TaskAck/StatusReport; empty disables the control plane (only meaningful with --auditlane-backend=file)")
		feedSource     = fs.String("feed-source", "synthetic", "feedSource advertised in StatusReport/evidence: live|replay|synthetic|placeholder (017 FR-R-E02)")
		statusInterval = fs.Duration("status-interval", 5*time.Second, "SAPIENT StatusReport cadence on the auditable lane")
		sessionID      = fs.String("session-id", "hldmm", "consumer/session id this seller serves (the key a Task addresses for per-session STOP/START)")
		// SAPIENT control/evidence plane backend (additive).
		auditlaneBackend  = fs.String("auditlane-backend", auditBackendFile, "SAPIENT audit lane (004) backend: file (default) | memory (in-process) | hcs (real Hedera topics; env HEDERA_OPERATOR_ID/HEDERA_OPERATOR_KEY)")
		heartbeatInterval = fs.Duration("heartbeat-interval", sapient.DefaultHeartbeatInterval, "spec-005 heartbeat cadence on stdOut (only when --auditlane-backend is memory|hcs)")
		// EIP-8004 Agent Card evidence (additive).
		register     = fs.Bool("register", false, "build + register the seller's EIP-8004 Agent Card (backend per --registry-backend) before pushing")
		agentCardOut = fs.String("agent-card-out", "", "write the registered Agent Card (agentURI JSON) to this path; implies --register")
		registryOut  = fs.String("registry-out", "", "write the agent evidence record (agentId + identity binding + card) to this path; implies --register")
		// EIP-8004 registry backend selection.
		registryBackend = fs.String("registry-backend", registryBackendMemory, "EIP-8004 registry backend: memory (in-process SIM, default) | evm (real chain; requires --registry-address)")
		registryAddress = fs.String("registry-address", "", "Identity Registry contract address for --registry-backend=evm (env fallback "+envRegistryContract+")")
		rpcURL          = fs.String("rpc-url", "", "EVM JSON-RPC endpoint for --registry-backend=evm (env fallback "+envHederaRPC+"; default "+defaultRPCURL+")")
		chainIDFlag     = fs.Uint64("chain-id", defaultChainID, "EVM chain id for --registry-backend=evm (Hedera testnet = 296)")
		registerOnly    = fs.Bool("register-only", false, "register the Agent Card, write the requested artefacts, then exit 0 WITHOUT dialing the buyer (implies --register; --buyer not required)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *buyer == "" && !*registerOnly {
		return errors.New("--buyer <multiaddr> is required (the seller dials the buyer)")
	}
	logger := log.New(os.Stderr, "[sapient-jv-seller] ", log.LstdFlags)

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

	logger.Printf("identity evm=%s node_id=%s peerID=%s service=%s", evmHex, nodeID, host.ID(), sapient.JVCommerceServiceName)

	// SAPIENT control/evidence plane backend (additive). Built BEFORE
	// registration so a topic-backed lane can advertise its real transport +
	// topic IDs in the Agent Card.
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

	// EIP-8004 Agent Card evidence mode (additive, opt-in) with the JetVision
	// ADS-B profile (jetvision-adsb-sapient service, neuron.adsb/1 extension,
	// the 4 capability strings, Air!Squitter sensor model).
	var agentURISha string
	if *register || *registerOnly || *agentCardOut != "" || *registryOut != "" {
		res, rerr := registerAgentCard(ctx, &nk, host.ID().String(), *agentCardOut, *registryOut, *feedSource, regCfg, deps, logger, audit.transport, audit.topicConfig)
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

	// SAPIENT control/evidence plane (additive) — identical machinery to the
	// rid seller, with the ADS-B Registration identity and heartbeat profile.
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
			Registration: &tasking.RegistrationIdentity{
				Name:        "Neuron SAPIENT ADS-B seller",
				ShortName:   "neuron-adsb",
				Model:       "sapient-jv-seller",
				NodeSubType: []string{sapient.JVExtensionID},
			},
		})
		mgr.RegisterSession(*sessionID, frameSink{w: writer})
		if err := mgr.Start(mgrCtx); err != nil {
			return fmt.Errorf("start tasking manager: %w", err)
		}
		logger.Printf("control plane ON: backend=%s session=%s feedSource=%s status=%s",
			*auditlaneBackend, *sessionID, *feedSource, *statusInterval)

		if audit.topicBacked {
			if rerr := mgr.EmitRegistration(mgrCtx); rerr != nil {
				logger.Printf("emit Registration: %v", rerr)
			}
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
				TopicBackend:   *auditlaneBackend,
				AgentURISha256: agentURISha,
				ServiceName:    sapient.JVCommerceServiceName,
				Profile:        sapient.ProfileSAPIENTADSB,
				DegradedFunc:   func() bool { return !bh.connected() },
			})
			if herr != nil {
				return fmt.Errorf("start heartbeat: %w", herr)
			}
			hbLoop = hb
			logger.Printf("005 heartbeat ON: stdOut=%s interval=%s profile=%s", audit.stdOutRef.Locator(), *heartbeatInterval, sapient.ProfileSAPIENTADSB)
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
const localRegistryAddr = "0x0000000000000000000000000000000000008004"

// registerAgentCard builds the JetVision seller's EIP-8004 Agent Card
// (sapient.JetVisionProfile), registers it on the selected registry backend,
// and writes the requested evidence artefacts. It asserts the dial host and
// the card advertise the same PeerID (V-REG-12). Registration is idempotent
// (registry.RegisterOrUpdate): re-runs reuse or refresh, never double-mint.
func registerAgentCard(ctx context.Context, nk *keylib.NeuronPrivateKey, hostPeerID, cardOut, registryOut, feedSource string, cfg registryConfig, deps Deps, logger *log.Logger, topicTransport string, topicConfig map[string]map[string]any) (sapient.RegisterResult, error) {
	prof := sapient.JetVisionProfile()
	card, err := sapient.BuildSellerCard(sapient.SellerCardOptions{
		ChildKey: nk, ChainID: cfg.chainID, Profile: &prof,
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
	default: // memory — the local/simulated path.
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
		ev := sapient.EvidenceFromResultProfile(res, cfg.simulated, sapient.JVCommerceServiceName)
		ev.FeedSource = feedSource // runtime provenance (017 FR-R-E02), not part of the card
		if werr := sapient.WriteEvidence(registryOut, ev); werr != nil {
			return res, werr
		}
		logger.Printf("wrote agent evidence -> %s", registryOut)
	}
	return res, nil
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
