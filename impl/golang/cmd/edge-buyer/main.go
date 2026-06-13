// edge-buyer is the spec-built reference buyer for the reverse-connect
// topology used by NAT-shielded edge devices. It listens on a publicly
// reachable libp2p multiaddr, publishes ReverseConnectionSetup to one or
// more sellers' stdIn topics announcing its multiaddrs (encrypted to each
// seller's pubkey), accepts incoming streams as the sellers dial in, and
// emits one JSONL record per received frame to the configured output sink.
//
// Run modes mirror cmd/edge-seller. The buyer reads SellerBootstrap JSON
// files produced by each seller to learn that seller's pubkey + stdIn topic.
// Multiple bootstraps are supported via NEURON_EDGE_SELLERS_BOOTSTRAP=path1,path2,...
// (or the legacy --bootstrap-in / NEURON_EDGE_BOOTSTRAP_IN flag for one seller).
//
// Other env / flags:
//
//	NEURON_EDGE_SELLERS_BOOTSTRAP      comma-separated paths to SellerBootstrap files (multi-seller)
//	NEURON_EDGE_BOOTSTRAP_IN           legacy single-seller path (used when SELLERS_BOOTSTRAP unset)
//	NEURON_EDGE_OUTPUT                 sink spec — stdout (default), file:PATH, file+:PATH, tcp:HOST:PORT
//	NEURON_EDGE_LIBP2P_LISTEN          multiaddr (default /ip4/0.0.0.0/udp/0/quic-v1)
//	NEURON_EDGE_LIBP2P_ADVERTISED      comma-separated multiaddrs to override host.Addrs()
//	NEURON_EDGE_HEARTBEAT_PERIOD       duration string (default 60s)
//	NEURON_EDGE_PRIVATE_KEY            secp256k1 hex (32 bytes, no 0x prefix)
//	NEURON_EDGE_REQUEST_ID             arbitrary string (default edge-feed-001)
//	NEURON_EDGE_RECONNECT_BACKOFF      duration string (default 10s)
//	NEURON_EDGE_SELLER_DIAL_TIMEOUT    duration string (default 60s)
//	NEURON_EDGE_ENFORCE_DEADLINES      bool ("true" / "1") — opt-in to spec-005
//	                                   deadline observation per seller. Default off
//	                                   (Phase C.2: rely on libp2p stream death).
//	                                   When true, every SellerBootstrap MUST carry a
//	                                   non-empty StdOutLocator.
//	NEURON_EDGE_REGISTRATION_MODE      one of skip|auto|mock|force-testnet. Default skip.
//	                                   Mirrors edge-seller's flag.
//	NEURON_EDGE_PAYMENT_MODE           one of disabled|mock|testnet. Default disabled.
//	                                   When mock, the buyer runs BuyerNegotiateAndFund
//	                                   per seller before publishing ReverseConnectionSetup
//	                                   and Settle after the stream ends.
//	NEURON_EDGE_PAYMENT_PRICE          decimal string (default "100"); deposit per agreement.
//	NEURON_EDGE_PAYMENT_CURRENCY       currency label (default "tinybar").
package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/neuron-sdk/neuron-go-sdk/internal/edgeapp"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

func main() {
	mode := flag.String("mode", "testnet", "Bus mode: testnet (real HCS) or mock (in-memory; for dev only)")
	bootstrapIn := flag.String("bootstrap-in", envOr("NEURON_EDGE_BOOTSTRAP_IN", "./seller-bootstrap.json"),
		"(legacy single-seller) path to a SellerBootstrap JSON file. Ignored when NEURON_EDGE_SELLERS_BOOTSTRAP is set.")
	output := flag.String("output", envOr("NEURON_EDGE_OUTPUT", "stdout"),
		"Output sink — stdout (default) | file:PATH | file+:PATH | tcp:HOST:PORT")
	// Phase 5 — optional v2-tagged sink for the dual-stream display path.
	// When set, every AggregatedFrame is ALSO emitted to this sink wrapped
	// in {source:"adsb", type:"aircraft", sellerPeerID, receivedAt, frame}.
	// Legacy --output continues to emit untagged AggregatedFrame.
	taggedOutput := flag.String("tagged-output", envOr("NEURON_EDGE_TAGGED_OUTPUT", ""),
		"v2-tagged dual-stream sink (additive to --output). Same spec syntax. Empty = no tagged emit.")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)

	priv, err := loadPrivateKey()
	if err != nil {
		logger.Fatalf("load private key: %v", err)
	}

	sellers, err := loadSellers(*bootstrapIn)
	if err != nil {
		logger.Fatalf("load sellers: %v", err)
	}
	for _, s := range sellers {
		logger.Printf("loaded seller: name=%s evm=%s stdIn=%s",
			s.DisplayName, displayEVM(s.PubKey), s.StdIn.Locator())
	}

	bus, err := makeBus(*mode, logger)
	if err != nil {
		logger.Fatalf("build bus (%s): %v", *mode, err)
	}

	sink, err := edgeapp.NewOutputSink(*output)
	if err != nil {
		logger.Fatalf("output sink: %v", err)
	}
	defer sink.Close()
	logger.Printf("output sink: %s", *output)

	// Phase 5 — optional v2-tagged sink. nil sink ⇒ tagged emission
	// disabled; the OnTaggedAdsb hook isn't wired into BuyerConfig.
	var taggedSink edgeapp.TaggedSink
	if *taggedOutput != "" {
		taggedSink, err = edgeapp.NewTaggedJSONLSink(*taggedOutput)
		if err != nil {
			logger.Fatalf("tagged output sink: %v", err)
		}
		defer taggedSink.Close()
		logger.Printf("tagged output sink: %s (Phase 5 dual-stream)", *taggedOutput)
	}

	advertised := splitCSV(os.Getenv("NEURON_EDGE_LIBP2P_ADVERTISED"))

	var received atomic.Uint64
	perSeller := &sync.Map{} // sellerEVM → *atomic.Uint64

	onAgg := func(af edgeapp.AggregatedFrame) {
		received.Add(1)
		v, _ := perSeller.LoadOrStore(af.SellerEVM, &atomic.Uint64{})
		v.(*atomic.Uint64).Add(1)

		if err := sink.Emit(context.Background(), af); err != nil {
			logger.Printf("output sink emit: %v", err)
		}
		// Log the first 5 frames per seller so the operator can see each
		// stream came alive.
		n := v.(*atomic.Uint64).Load()
		if n <= 5 && len(af.Frame.Raw) > 0 {
			logger.Printf("frame#%d from %s: bytes=%d df=%d icao=%q",
				n, af.SellerName, len(af.Frame.Raw), af.Meta.DF, af.Meta.ICAO)
		}
	}

	onStatus := func(s edgeapp.SellerStatus) {
		logger.Printf("seller %s [%s] state=%s frames=%d err=%q",
			s.DisplayName, s.EVM[:10], s.State, s.FramesReceived, s.LastError)
	}

	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		var prev uint64
		for range t.C {
			cur := received.Load()
			perSellerStr := ""
			perSeller.Range(func(k, v any) bool {
				if perSellerStr != "" {
					perSellerStr += " "
				}
				perSellerStr += fmt.Sprintf("%s=%d", abbreviate(k.(string)), v.(*atomic.Uint64).Load())
				return true
			})
			logger.Printf("rate: total=%d delta=%d (last 5s) [%s]", cur, cur-prev, perSellerStr)
			prev = cur
		}
	}()

	enforceDeadlines := parseBool(os.Getenv("NEURON_EDGE_ENFORCE_DEADLINES"))
	if enforceDeadlines {
		for i, s := range sellers {
			if s.StdOut.Locator() == "" {
				logger.Fatalf("NEURON_EDGE_ENFORCE_DEADLINES=true but seller[%d] (%s) bootstrap has no stdOutLocator",
					i, s.DisplayName)
			}
		}
		logger.Printf("spec-005 deadline observation: ENABLED")
	}

	bgCtx := context.Background()
	registry := selectRegistryFromEnv(bgCtx, &priv, logger)
	escrow := selectEscrowFromEnv(bgCtx, &priv, logger)

	cfg := edgeapp.BuyerConfig{
		Bus:                        bus,
		PrivateKey:                 &priv,
		Sellers:                    sellers,
		LibP2PListenAddr:           envOr("NEURON_EDGE_LIBP2P_LISTEN", "/ip4/0.0.0.0/udp/0/quic-v1"),
		LibP2PAdvertisedMultiaddrs: advertised,
		Protocol:                   edgeapp.DefaultProtocol,
		RequestID:                  envOr("NEURON_EDGE_REQUEST_ID", "edge-feed-001"),
		HeartbeatPeriod:            parseDuration(envOr("NEURON_EDGE_HEARTBEAT_PERIOD", "60s")),
		ReconnectBackoff:           parseDuration(envOr("NEURON_EDGE_RECONNECT_BACKOFF", "10s")),
		SellerDialTimeout:          parseDuration(envOr("NEURON_EDGE_SELLER_DIAL_TIMEOUT", "60s")),
		NegotiationTimeout:         parseDuration(envOr("NEURON_EDGE_BUYER_NEGOTIATION_TIMEOUT", "120s")),
		EnforceDeadlines:           enforceDeadlines,
		StatePath:                  os.Getenv("NEURON_EDGE_BUYER_STATE_PATH"),
		OnAggregatedFrame:          onAgg,
		OnTaggedAdsb: func(tf edgeapp.TaggedAdsbFrame) {
			if taggedSink == nil {
				return
			}
			if err := taggedSink.Emit(context.Background(), tf); err != nil {
				logger.Printf("tagged sink emit: %v", err)
			}
		},
		OnSellerStatus: onStatus,
		Logger:                     logger,

		Registry:            registry,
		Escrow:              escrow,
		PaymentPriceTinybar: envOr("NEURON_EDGE_PAYMENT_PRICE", "100"),
		PaymentCurrency:     envOr("NEURON_EDGE_PAYMENT_CURRENCY", "tinybar"),
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := edgeapp.RunBuyer(ctx, cfg); err != nil && err != context.Canceled {
		logger.Fatalf("run buyer: %v", err)
	}
}

// loadSellers resolves the SellerEntry list from env / flag.
//
// Precedence:
//  1. NEURON_EDGE_SELLERS_BOOTSTRAP — comma-separated paths to SellerBootstrap
//     files. The DisplayName per seller is taken from a sibling NAME=path syntax
//     when present (e.g. "alpha=./a.json,beta=./b.json").
//  2. legacyBootstrapPath (the --bootstrap-in flag / NEURON_EDGE_BOOTSTRAP_IN
//     env) — single seller, no display name.
func loadSellers(legacyBootstrapPath string) ([]edgeapp.SellerEntry, error) {
	multi := os.Getenv("NEURON_EDGE_SELLERS_BOOTSTRAP")
	if multi != "" {
		return loadSellersFromMulti(multi)
	}
	bs, err := edgeapp.ReadSellerBootstrap(legacyBootstrapPath)
	if err != nil {
		return nil, fmt.Errorf("read bootstrap %q: %w", legacyBootstrapPath, err)
	}
	pub, err := bs.SellerPubKey()
	if err != nil {
		return nil, fmt.Errorf("parse pubkey from %q: %w", legacyBootstrapPath, err)
	}
	stdIn, err := bs.SellerStdIn()
	if err != nil {
		return nil, fmt.Errorf("parse stdIn from %q: %w", legacyBootstrapPath, err)
	}
	stdOut, err := bs.SellerStdOut()
	if err != nil {
		return nil, fmt.Errorf("parse stdOut from %q: %w", legacyBootstrapPath, err)
	}
	return []edgeapp.SellerEntry{{StdIn: stdIn, StdOut: stdOut, PubKey: pub}}, nil
}

func loadSellersFromMulti(spec string) ([]edgeapp.SellerEntry, error) {
	var out []edgeapp.SellerEntry
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		var name, path string
		if eq := strings.IndexByte(item, '='); eq > 0 {
			name = item[:eq]
			path = item[eq+1:]
		} else {
			path = item
		}
		bs, err := edgeapp.ReadSellerBootstrap(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", path, err)
		}
		pub, err := bs.SellerPubKey()
		if err != nil {
			return nil, fmt.Errorf("parse pubkey %q: %w", path, err)
		}
		stdIn, err := bs.SellerStdIn()
		if err != nil {
			return nil, fmt.Errorf("parse stdIn %q: %w", path, err)
		}
		stdOut, err := bs.SellerStdOut()
		if err != nil {
			return nil, fmt.Errorf("parse stdOut %q: %w", path, err)
		}
		out = append(out, edgeapp.SellerEntry{StdIn: stdIn, StdOut: stdOut, PubKey: pub, DisplayName: name})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("NEURON_EDGE_SELLERS_BOOTSTRAP parsed to zero entries")
	}
	return out, nil
}

func displayEVM(pub *ecdsa.PublicKey) string {
	if pub == nil {
		return "<nil>"
	}
	return ethcrypto.PubkeyToAddress(*pub).Hex()
}

func abbreviate(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
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
		logger.Printf("WARNING: --mode=mock is in-memory only; the seller must be in the same process")
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

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 60 * time.Second
	}
	return d
}

// parseBool returns true for "1", "t", "T", "true", "TRUE" etc.; everything
// else (including empty string) returns false. Mirrors strconv.ParseBool's
// accepted-true set without surfacing the parse error — empty / unknown
// values default to false to preserve the on-by-mistake guarantee for
// new feature flags.
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	}
	return false
}

// selectRegistryFromEnv mirrors the seller's registry-selector.
// See cmd/edge-seller/main.go's selectRegistryFromEnv for the full
// mode semantics. Iteration 4 wires "testnet" to a real EVMRegistryAdapter.
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

// selectEscrowFromEnv mirrors the seller's escrow-selector. mock ⇒
// payment.MemoryEscrow. testnet ⇒ payment.EVMEscrowAdapter (real on-chain
// CreateEscrow/Deposit/RequestRelease/ApproveRelease/ClaimRefund). disabled
// / empty ⇒ nil.
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
