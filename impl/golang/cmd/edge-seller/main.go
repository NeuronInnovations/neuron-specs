// edge-seller is the spec-built reference seller for the reverse-connect
// topology used by NAT-shielded edge devices (e.g. JetVision Air!Squitter).
//
// It reads BEAST Mode-S frames from a TCP source (default 127.0.0.1:10003),
// dials a publicly-reachable buyer announced via a SellerBootstrap-like
// rendezvous file, and forwards each frame as a length-prefixed binary
// envelope (feeds.EncodeFeedFrame) over a libp2p QUIC stream.
//
// Run modes:
//
//	--mode=mock    in-memory topic bus; both seller and buyer must be
//	               in the same process (use the edgeapp E2E test).
//	--mode=testnet real Hedera testnet HCS via topic.NewTestnetClientFromEnv.
//	               Required env vars are HEDERA_OPERATOR_ID and HEDERA_OPERATOR_KEY.
//
// Other env / flags:
//
//	NEURON_EDGE_FEED_HOSTPORT      (default 127.0.0.1:10003)
//	NEURON_EDGE_FEED_SOURCE        tcp | replay | synth (default tcp)
//	NEURON_EDGE_FEED_REPLAY_FILE   path to a .beast capture (when source=replay)
//	NEURON_EDGE_FEED_SYNTH_FPS     int (when source=synth, default 50)
//	NEURON_EDGE_LIBP2P_LISTEN      multiaddr (default /ip4/0.0.0.0/udp/0/quic-v1)
//	NEURON_EDGE_HEARTBEAT_PERIOD   duration string (default 60s)
//	NEURON_EDGE_PRIVATE_KEY        secp256k1 hex (32 bytes, no 0x prefix)
//	NEURON_EDGE_BOOTSTRAP_OUT      path to write SellerBootstrap JSON (default ./seller-bootstrap.json)
//	NEURON_EDGE_STATE_PATH         (D1, opt-in) path to persistent state JSON. When set,
//	                               the seller reuses HCS topic IDs across restarts. Empty
//	                               (default) preserves Phase C.2 fresh-topics-every-restart.
//	NEURON_EDGE_PROFILE_DESCRIPTOR_PUBLISH (D2, opt-in) when "true", publishes a spec-013
//	                               Profile E descriptor on stdOut at startup. Default off.
//	NEURON_EDGE_REGISTRATION_MODE  (D2, opt-in) one of skip|auto|force-testnet. Default skip.
//	                               In iteration 3, "auto" wires a MemoryRegistry — the
//	                               registration is in-process only (no on-chain calls).
//	                               "force-testnet" returns ErrFeatureNotImplemented at
//	                               every adapter call until iteration 4 wires EVMRegistryAdapter.
//	NEURON_EDGE_PAYMENT_MODE       (D3, opt-in) one of disabled|mock|testnet. Default disabled.
//	                               In iteration 3, "mock" wires payment.MemoryEscrow + the
//	                               BuyerNegotiateAndFund / SellerObserveAndAccept gate.
//	                               "testnet" returns ErrFeatureNotImplemented.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/neuron-sdk/neuron-go-sdk/internal/edgeapp"
	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

func main() {
	mode := flag.String("mode", "testnet", "Bus mode: testnet (real HCS) or mock (in-memory; for dev only)")
	bootstrapOut := flag.String("bootstrap-out", envOr("NEURON_EDGE_BOOTSTRAP_OUT", "./seller-bootstrap.json"),
		"Path to write SellerBootstrap JSON (so the buyer can read seller pubkey + stdIn topic)")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)

	// 1. Load private key.
	priv, err := loadPrivateKey()
	if err != nil {
		logger.Fatalf("load private key: %v", err)
	}
	pub := priv.PublicKey()

	// 2. Build bus.
	bus, err := makeBus(*mode, logger)
	if err != nil {
		logger.Fatalf("build bus (%s): %v", *mode, err)
	}

	// 3. Resolve topics. If NEURON_EDGE_STATE_PATH is set and a previous
	// state file matches the running identity, reuse those topic IDs across
	// the restart (D1). Otherwise create three fresh topics on the bus.
	statePath := strings.TrimSpace(os.Getenv("NEURON_EDGE_STATE_PATH"))
	stdIn, stdOut, stdErr, persistedState, freshTopics, err := resolveTopicsFromState(bus, statePath, &priv)
	if err != nil {
		logger.Fatalf("resolve topics: %v", err)
	}
	if statePath != "" {
		if freshTopics {
			if persistedState != nil {
				if saveErr := edgeapp.SaveEdgeState(statePath, persistedState); saveErr != nil {
					logger.Printf("state: persist failed (continuing): %v", saveErr)
				} else {
					logger.Printf("state: wrote %s (fresh topics)", statePath)
				}
			}
		} else {
			logger.Printf("state: reusing topics from %s", statePath)
		}
	}

	// 4. Write bootstrap from the resolved refs (fresh or persisted).
	pubBlock, err := pub.ToBlockchainKey()
	if err != nil {
		logger.Fatalf("derive pubkey: %v", err)
	}
	bootstrap := edgeapp.SellerBootstrap{
		EVMAddress:    pub.EVMAddress().Hex(),
		PublicKeyHex:  hex.EncodeToString(ethcrypto.FromECDSAPub(pubBlock)),
		StdInLocator:  stdIn.Locator(),
		StdOutLocator: stdOut.Locator(),
		StdErrLocator: stdErr.Locator(),
		BackendKind:   string(bus.SupportedTransport()),
		NetworkLabel:  *mode,
	}
	if err := edgeapp.WriteSellerBootstrap(*bootstrapOut, bootstrap); err != nil {
		logger.Fatalf("write bootstrap: %v", err)
	}
	logger.Printf("wrote bootstrap: %s", *bootstrapOut)

	// 5. Build feed source.
	feedSource, err := buildFeedSource()
	if err != nil {
		logger.Fatalf("build feed source: %v", err)
	}

	// 6. Spec-feature flags.
	publishDescriptor := parseBoolFlag(os.Getenv("NEURON_EDGE_PROFILE_DESCRIPTOR_PUBLISH"))
	bgCtx := context.Background()
	registry := selectRegistryFromEnv(bgCtx, &priv, logger)
	escrow := selectEscrowFromEnv(bgCtx, &priv, logger)

	cfg := edgeapp.SellerConfig{
		Bus:              bus,
		PrivateKey:       &priv,
		StdIn:            stdIn,
		StdOut:           stdOut,
		StdErr:           stdErr,
		LibP2PListenAddr: envOr("NEURON_EDGE_LIBP2P_LISTEN", "/ip4/0.0.0.0/udp/0/quic-v1"),
		Protocol:         edgeapp.DefaultProtocol,
		HeartbeatPeriod:  parseDuration(envOr("NEURON_EDGE_HEARTBEAT_PERIOD", "60s")),
		FeedSource:       feedSource,
		Logger:           logger,

		PublishProfileDescriptor: publishDescriptor,
		Registry:                 registry,
		Escrow:                   escrow,

		// Iter-7: period-driven Invoice + idle-FUNDED timeout. Both default
		// off (Phase D3 single-shot SIGINT-driven settlement). Set
		// NEURON_EDGE_AGREEMENT_PERIOD=600s to demo 3 sequential 10-min
		// agreements; NEURON_EDGE_IDLE_FUNDED_TIMEOUT=300s avoids deadlock
		// if the buyer crashes between EscrowCreated and ReverseConnectionSetup.
		AgreementPeriod:   parseDuration(envOr("NEURON_EDGE_AGREEMENT_PERIOD", "0s")),
		IdleFundedTimeout: parseDuration(envOr("NEURON_EDGE_IDLE_FUNDED_TIMEOUT", "0s")),
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := edgeapp.RunSeller(ctx, cfg); err != nil && err != context.Canceled {
		logger.Fatalf("run seller: %v", err)
	}
}

func loadPrivateKey() (keylib.NeuronPrivateKey, error) {
	hexKey := strings.TrimSpace(strings.TrimPrefix(os.Getenv("NEURON_EDGE_PRIVATE_KEY"), "0x"))
	if hexKey == "" {
		return keylib.NeuronPrivateKey{}, fmt.Errorf("NEURON_EDGE_PRIVATE_KEY required (32-byte secp256k1 hex)")
	}
	bs, err := hex.DecodeString(hexKey)
	if err != nil {
		return keylib.NeuronPrivateKey{}, fmt.Errorf("decode NEURON_EDGE_PRIVATE_KEY: %w", err)
	}
	return keylib.NeuronPrivateKeyFromBytes(bs)
}

func makeBus(mode string, logger *log.Logger) (topic.TopicAdapter, error) {
	switch mode {
	case "mock":
		logger.Printf("WARNING: --mode=mock is in-memory only; the buyer must be in the same process")
		return edgeapp.NewMemoryBus(), nil
	case "testnet":
		client, operatorID, err := topic.NewTestnetClientFromEnv()
		if err != nil {
			return nil, fmt.Errorf("hedera testnet client: %w", err)
		}
		logger.Printf("hedera operator: %s", operatorID.String())
		return topic.NewHCSAdapter(topic.NewRealHCSClient(client)), nil
	default:
		return nil, fmt.Errorf("unknown mode %q (want testnet or mock)", mode)
	}
}

func buildFeedSource() (edgeapp.FeedSource, error) {
	source := envOr("NEURON_EDGE_FEED_SOURCE", "tcp")
	switch source {
	case "tcp":
		hostPort := envOr("NEURON_EDGE_FEED_HOSTPORT", "127.0.0.1:10003")
		return func(ctx context.Context, out chan<- feeds.FeedFrame) error {
			return feeds.RunBeastTCP(ctx, hostPort, out)
		}, nil
	case "replay":
		path := os.Getenv("NEURON_EDGE_FEED_REPLAY_FILE")
		if path == "" {
			return nil, fmt.Errorf("NEURON_EDGE_FEED_REPLAY_FILE required when source=replay")
		}
		speedup, _ := strconv.ParseFloat(envOr("NEURON_EDGE_FEED_REPLAY_SPEEDUP", "1.0"), 64)
		return func(ctx context.Context, out chan<- feeds.FeedFrame) error {
			return feeds.RunBeastReplay(ctx, path,
				feeds.ReplayOptions{Speedup: speedup, Loop: true}, out)
		}, nil
	case "synth":
		fps, _ := strconv.Atoi(envOr("NEURON_EDGE_FEED_SYNTH_FPS", "50"))
		return func(ctx context.Context, out chan<- feeds.FeedFrame) error {
			return feeds.RunSynth(ctx, fps, out)
		}, nil
	default:
		return nil, fmt.Errorf("unknown NEURON_EDGE_FEED_SOURCE %q", source)
	}
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 60 * time.Second
	}
	return d
}

// parseBoolFlag returns true for "1" / "true" / "yes" / "on" (case-insensitive).
func parseBoolFlag(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	}
	return false
}

// selectRegistryFromEnv reads NEURON_EDGE_REGISTRATION_MODE and returns the
// corresponding RegistryAdapter or nil.
//
// Modes:
//   - skip / empty       ⇒ nil (registration disabled).
//   - auto / mock        ⇒ in-process MemoryRegistry (no chain calls).
//   - testnet            ⇒ real EVMRegistryAdapter; requires
//                          NEURON_EDGE_REGISTRY_ADDR + NEURON_EDGE_CHAIN_ID
//                          + NEURON_EDGE_HEDERA_EVM_RPC. Will send actual
//                          on-chain Register / SetAgentURI transactions
//                          with the configured signing key.
//   - force-testnet      ⇒ explicit-opt-in synonym for testnet (kept so
//                          the operator's deliberate intent is captured
//                          in env). Same behavior as "testnet".
//   - any other value    ⇒ logged + treated as skip (fail-safe).
func selectRegistryFromEnv(ctx context.Context, signingKey *keylib.NeuronPrivateKey, logger *log.Logger) edgeapp.RegistryAdapter {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv("NEURON_EDGE_REGISTRATION_MODE")))
	switch mode {
	case "", "skip":
		return nil
	case "auto", "mock":
		logger.Printf("registration mode: auto (in-process MemoryRegistry — no chain calls)")
		return edgeapp.NewMemoryRegistry()
	case "testnet", "force-testnet", "force-evm":
		ad, err := buildEVMRegistry(ctx, signingKey, logger)
		if err != nil {
			logger.Printf("registration mode: testnet — construct failed: %v ⇒ falling back to DisabledRegistry", err)
			return edgeapp.NewDisabledRegistry("evm-construct-failed-" + err.Error())
		}
		return ad
	default:
		logger.Printf("registration mode: unknown %q, treating as skip", mode)
		return nil
	}
}

// buildEVMRegistry constructs the EVMRegistryAdapter from env vars.
// **WARNING: this prepares to send real testnet transactions.** It does
// NOT itself send anything — the first Register / SetAgentURI call will.
// Operators must have explicitly authorized testnet activation before
// running with NEURON_EDGE_REGISTRATION_MODE=testnet.
func buildEVMRegistry(ctx context.Context, signingKey *keylib.NeuronPrivateKey, logger *log.Logger) (edgeapp.RegistryAdapter, error) {
	rpc := strings.TrimSpace(os.Getenv("NEURON_EDGE_HEDERA_EVM_RPC"))
	if rpc == "" {
		rpc = "https://testnet.hashio.io/api"
	}
	chainStr := strings.TrimSpace(os.Getenv("NEURON_EDGE_CHAIN_ID"))
	if chainStr == "" {
		return nil, fmt.Errorf("NEURON_EDGE_CHAIN_ID required")
	}
	chainID, perr := strconv.ParseUint(chainStr, 10, 64)
	if perr != nil {
		return nil, fmt.Errorf("invalid NEURON_EDGE_CHAIN_ID %q: %w", chainStr, perr)
	}
	addr := strings.TrimSpace(os.Getenv("NEURON_EDGE_REGISTRY_ADDR"))
	if addr == "" {
		return nil, fmt.Errorf("NEURON_EDGE_REGISTRY_ADDR required")
	}
	logger.Printf("registration mode: testnet — RPC=%s chainID=%d registry=%s", rpc, chainID, addr)
	logger.Printf("WARNING: real on-chain transactions will be sent for Register/SetAgentURI")
	return edgeapp.NewEVMRegistryAdapter(ctx, edgeapp.EVMRegistryAdapterConfig{
		RPC:          rpc,
		ChainID:      chainID,
		ContractAddr: addr,
		SigningKey:   signingKey,
	})
}

// selectEscrowFromEnv reads NEURON_EDGE_PAYMENT_MODE and returns:
//   - disabled / empty   ⇒ nil (commerce gate skipped).
//   - mock               ⇒ in-process payment.MemoryEscrow.
//   - testnet            ⇒ real EVMEscrowAdapter; requires
//                          NEURON_EDGE_ESCROW_ADDR + NEURON_EDGE_TOKEN_ADDR
//                          + NEURON_EDGE_CHAIN_ID + NEURON_EDGE_HEDERA_EVM_RPC.
//                          ERC-20 (TestToken or USDC-compatible) is the only
//                          supported currency — NeuronEscrow contract has no
//                          native HBAR support.
func selectEscrowFromEnv(ctx context.Context, signingKey *keylib.NeuronPrivateKey, logger *log.Logger) payment.EscrowAdapter {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv("NEURON_EDGE_PAYMENT_MODE")))
	switch mode {
	case "", "disabled":
		return nil
	case "mock":
		logger.Printf("payment mode: mock (in-process MemoryEscrow — no chain calls)")
		return payment.NewMemoryEscrow()
	case "testnet":
		ad, err := buildEVMEscrow(ctx, signingKey, logger)
		if err != nil {
			logger.Printf("payment mode: testnet — construct failed: %v ⇒ commerce gate disabled", err)
			return nil
		}
		return ad
	default:
		logger.Printf("payment mode: unknown %q, treating as disabled", mode)
		return nil
	}
}

// buildEVMEscrow constructs the payment.EVMEscrowAdapter from env vars.
// **WARNING: this prepares to send real testnet transactions for
// CreateEscrow / Deposit / RequestRelease / ApproveRelease / ClaimRefund.**
func buildEVMEscrow(ctx context.Context, signingKey *keylib.NeuronPrivateKey, logger *log.Logger) (payment.EscrowAdapter, error) {
	rpc := strings.TrimSpace(os.Getenv("NEURON_EDGE_HEDERA_EVM_RPC"))
	if rpc == "" {
		rpc = "https://testnet.hashio.io/api"
	}
	chainStr := strings.TrimSpace(os.Getenv("NEURON_EDGE_CHAIN_ID"))
	if chainStr == "" {
		return nil, fmt.Errorf("NEURON_EDGE_CHAIN_ID required")
	}
	chainID, perr := strconv.ParseUint(chainStr, 10, 64)
	if perr != nil {
		return nil, fmt.Errorf("invalid NEURON_EDGE_CHAIN_ID %q: %w", chainStr, perr)
	}
	escrowAddrStr := strings.TrimSpace(os.Getenv("NEURON_EDGE_ESCROW_ADDR"))
	tokenAddrStr := strings.TrimSpace(os.Getenv("NEURON_EDGE_TOKEN_ADDR"))
	if escrowAddrStr == "" || tokenAddrStr == "" {
		return nil, fmt.Errorf("NEURON_EDGE_ESCROW_ADDR + NEURON_EDGE_TOKEN_ADDR required")
	}

	client, err := ethclient.DialContext(ctx, rpc)
	if err != nil {
		return nil, fmt.Errorf("dial RPC: %w", err)
	}
	ecdsaPriv, err := signingKey.ToBlockchainKey()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("convert signing key: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(ecdsaPriv, new(big.Int).SetUint64(chainID))
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("new transactor: %w", err)
	}
	auth.Context = ctx
	logger.Printf("payment mode: testnet — RPC=%s chainID=%d escrow=%s token=%s",
		rpc, chainID, escrowAddrStr, tokenAddrStr)
	logger.Printf("WARNING: real on-chain transactions will be sent for CreateEscrow/Deposit/etc.")

	return payment.NewEVMEscrowAdapter(client,
		common.HexToAddress(escrowAddrStr),
		common.HexToAddress(tokenAddrStr),
		auth,
	)
}

// resolveTopicsFromState returns the three topic refs the seller will use
// for stdIn / stdOut / stdErr. Behavior:
//
//   - statePath empty (Phase C.2 default): create three fresh topics on the
//     bus, return (refs, nil, true, nil) — caller skips the SaveEdgeState
//     step since persistState is nil.
//   - statePath set + file present + Identity matches: parse the persisted
//     locators, return them with persistState non-nil + fresh=false. Caller
//     may still re-save persistState to refresh UpdatedAt.
//   - statePath set + file missing / mismatched / parse-error-tolerant cases:
//     create fresh topics, build a new EdgeState, return it with fresh=true.
//     Caller is expected to SaveEdgeState. A real I/O / unrecoverable parse
//     error from LoadEdgeState surfaces as a non-nil err.
func resolveTopicsFromState(
	bus topic.TopicAdapter,
	statePath string,
	priv *keylib.NeuronPrivateKey,
) (in, out, errRef topic.TopicRef, persistState *edgeapp.EdgeState, fresh bool, err error) {
	if statePath != "" {
		loaded, loadErr := edgeapp.LoadEdgeState(statePath)
		if loadErr != nil {
			return in, out, errRef, nil, false, fmt.Errorf("load state: %w", loadErr)
		}
		pubHex := priv.PublicKey().Hex()
		if loaded != nil && loaded.MatchesIdentity(pubHex) {
			ri, ro, re, refErr := loaded.TopicRefs()
			if refErr == nil && ri.Transport() == bus.SupportedTransport() {
				return ri, ro, re, loaded, false, nil
			}
		}
	}

	in, err = bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport(), Memo: "edge-seller-stdIn"})
	if err != nil {
		return in, out, errRef, nil, true, fmt.Errorf("create stdIn topic: %w", err)
	}
	out, err = bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport(), Memo: "edge-seller-stdOut"})
	if err != nil {
		return in, out, errRef, nil, true, fmt.Errorf("create stdOut topic: %w", err)
	}
	errRef, err = bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport(), Memo: "edge-seller-stdErr"})
	if err != nil {
		return in, out, errRef, nil, true, fmt.Errorf("create stdErr topic: %w", err)
	}

	if statePath == "" {
		return in, out, errRef, nil, true, nil
	}

	pub := priv.PublicKey()
	pid, _ := pub.PeerID() // best-effort PeerID for state file (not load-bearing)
	persistState = edgeapp.BuildEdgeState(
		pub.EVMAddress().Hex(),
		pub.Hex(),
		pid.String(),
		in, out, errRef,
	)
	return in, out, errRef, persistState, true, nil
}
