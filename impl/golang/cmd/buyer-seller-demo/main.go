// buyer-seller-demo demonstrates the full Neuron buyer-seller JPEG exchange
// with negotiated pricing and validator supervision.
//
// Protocol flow: setup → register → discover → negotiate → fund → connect → deliver → settle → validate
//
// Uses real: secp256k1 keys, ECDSA signing, payment state machine, libp2p delivery, evidence envelopes.
// Uses mocks: in-memory topic bus, in-memory registry, in-memory escrow.
package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	paymentbindings "github.com/neuron-sdk/neuron-go-sdk/internal/payment/bindings"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/neuron-sdk/neuron-go-sdk/internal/validation"
)

// ═══════════════════════════════════════════════════════════════════
// In-memory infrastructure mocks
// ═══════════════════════════════════════════════════════════════════

type memoryTopicBus struct {
	mu      sync.Mutex
	topics  map[string][]topic.TopicMessage
	nextSeq map[string]uint64
}

func newMemoryTopicBus() *memoryTopicBus {
	return &memoryTopicBus{
		topics:  make(map[string][]topic.TopicMessage),
		nextSeq: make(map[string]uint64),
	}
}

func (b *memoryTopicBus) CreateTopic(opts topic.CreateTopicOpts) (topic.TopicRef, error) {
	b.mu.Lock()
	locator := fmt.Sprintf("mem-topic-%d", len(b.topics)+1)
	b.topics[locator] = nil
	b.nextSeq[locator] = 0
	b.mu.Unlock()
	return topic.NewTopicRef(topic.BackendHCS, locator)
}

func (b *memoryTopicBus) Publish(ref topic.TopicRef, msg topic.TopicMessage, _ topic.PublishOpts) (topic.PublishResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	loc := ref.Locator()
	b.topics[loc] = append(b.topics[loc], msg)
	b.nextSeq[loc]++
	ts := uint64(time.Now().UnixNano())
	seq := b.nextSeq[loc]
	return topic.PublishResult{
		TransactionRef:     fmt.Sprintf("mem-tx-%s-%d", loc, seq),
		ConsensusTimestamp: &ts,
		SequenceNumber:     &seq,
		Confirmed:          true,
	}, nil
}

func (b *memoryTopicBus) Subscribe(_ context.Context, _ topic.TopicRef, _ topic.SubscribeOpts) (<-chan topic.MessageDelivery, error) {
	return nil, nil
}

func (b *memoryTopicBus) Resolve(_ topic.TopicRef) (topic.TopicMetadata, error) {
	return topic.TopicMetadata{}, nil
}

func (b *memoryTopicBus) MaxMessageSize() uint64                { return 128 * 1024 }
func (b *memoryTopicBus) SupportedTransport() topic.BackendKind { return topic.BackendHCS }
func (b *memoryTopicBus) EstimatePublishCost(_ uint64) (topic.CostEstimate, error) {
	return topic.CostEstimate{Amount: 1, Unit: "tinybar"}, nil
}

func (b *memoryTopicBus) getMessages(ref topic.TopicRef) []topic.TopicMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]topic.TopicMessage(nil), b.topics[ref.Locator()]...)
}

// demoBus is the interface both mock and testnet buses implement.
// Extends TopicAdapter with getMessages for the VALIDATE phase polling pattern.
type demoBus interface {
	topic.TopicAdapter
	getMessages(ref topic.TopicRef) []topic.TopicMessage
}

// ═══════════════════════════════════════════════════════════════════
// HCS-backed topic bus (testnet mode)
// ═══════════════════════════════════════════════════════════════════

// hcsTopicBus wraps a real HCSAdapter and maintains a local message log
// so the demo's VALIDATE phase can poll messages without HCS subscription.
type hcsTopicBus struct {
	adapter *topic.HCSAdapter
	mu      sync.Mutex
	msgLog  map[string][]topic.TopicMessage // locator → published messages
	txLog   []txRecord                      // all transactions for summary
}

type txRecord struct {
	topicID string
	txRef   string
	seq     uint64
}

func newHCSTopicBus(adapter *topic.HCSAdapter) *hcsTopicBus {
	return &hcsTopicBus{
		adapter: adapter,
		msgLog:  make(map[string][]topic.TopicMessage),
	}
}

func (b *hcsTopicBus) CreateTopic(opts topic.CreateTopicOpts) (topic.TopicRef, error) {
	ref, err := b.adapter.CreateTopic(opts)
	if err != nil {
		return ref, err
	}
	info("  HCS topic created: %s", ref.Locator())
	info("  HashScan: https://hashscan.io/testnet/topic/%s", ref.Locator())
	return ref, nil
}

func (b *hcsTopicBus) Publish(ref topic.TopicRef, msg topic.TopicMessage, opts topic.PublishOpts) (topic.PublishResult, error) {
	// Always use WaitForConsensus in testnet mode for real confirmation.
	opts.ConfirmationMode = topic.WaitForConsensus
	result, err := b.adapter.Publish(ref, msg, opts)
	if err != nil {
		return result, err
	}
	// Store locally for getMessages polling.
	b.mu.Lock()
	loc := ref.Locator()
	b.msgLog[loc] = append(b.msgLog[loc], msg)
	b.txLog = append(b.txLog, txRecord{topicID: loc, txRef: result.TransactionRef, seq: func() uint64 {
		if result.SequenceNumber != nil {
			return *result.SequenceNumber
		}
		return 0
	}()})
	b.mu.Unlock()
	return result, nil
}

func (b *hcsTopicBus) Subscribe(ctx context.Context, ref topic.TopicRef, opts topic.SubscribeOpts) (<-chan topic.MessageDelivery, error) {
	return b.adapter.Subscribe(ctx, ref, opts)
}

func (b *hcsTopicBus) Resolve(ref topic.TopicRef) (topic.TopicMetadata, error) {
	return b.adapter.Resolve(ref)
}

func (b *hcsTopicBus) MaxMessageSize() uint64                { return b.adapter.MaxMessageSize() }
func (b *hcsTopicBus) SupportedTransport() topic.BackendKind { return b.adapter.SupportedTransport() }
func (b *hcsTopicBus) EstimatePublishCost(size uint64) (topic.CostEstimate, error) {
	return b.adapter.EstimatePublishCost(size)
}

func (b *hcsTopicBus) getMessages(ref topic.TopicRef) []topic.TopicMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]topic.TopicMessage(nil), b.msgLog[ref.Locator()]...)
}

func (b *hcsTopicBus) transactions() []txRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]txRecord(nil), b.txLog...)
}

// ═══════════════════════════════════════════════════════════════════
// Mock registry
// ═══════════════════════════════════════════════════════════════════

type mockRegistry struct {
	agents map[string]registryEntry
	nextID int
}

type registryEntry struct {
	agentID  string
	agentURI string
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{agents: make(map[string]registryEntry)}
}

func (r *mockRegistry) register(evmAddress, agentURI string) string {
	r.nextID++
	id := fmt.Sprintf("%d", r.nextID)
	r.agents[evmAddress] = registryEntry{agentID: id, agentURI: agentURI}
	return id
}

func (r *mockRegistry) lookup(evmAddress string) (registryEntry, bool) {
	e, ok := r.agents[evmAddress]
	return e, ok
}

// ═══════════════════════════════════════════════════════════════════
// Agent
// ═══════════════════════════════════════════════════════════════════

type agent struct {
	name    string
	key     keylib.NeuronPrivateKey
	agentID string
	stdIn   topic.TopicRef // Inbound: other agents publish here to send messages TO this agent.
	stdOut  topic.TopicRef // Outbound: this agent publishes here (heartbeats, evidence envelopes).
	stdErr  topic.TopicRef // Diagnostic: this agent publishes error/SLA reports here.
	seqNum  uint64
}

// Channel usage in the demo:
//
//   seller.stdIn  ← buyer sends:   serviceRequest, escrowCreated, invoiceAck   (phases 4, 5, 8)
//   buyer.stdIn   ← seller sends:  serviceResponse, connectionSetup, invoice   (phases 4, 6, 8)
//   validator.stdOut ← validator publishes: 2 evidence envelopes               (phase 9)
//
//   seller.stdOut, buyer.stdOut, *.stdErr are created but unused in this demo.
//   In production, agents publish heartbeats to their own stdOut (spec 005).

func (a *agent) evmAddress() string { return a.key.PublicKey().EVMAddress().Hex() }
func (a *agent) nextSeq() uint64    { a.seqNum++; return a.seqNum }

func (a *agent) publishPayload(bus demoBus, targetTopic topic.TopicRef, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	now := uint64(time.Now().UnixNano())
	msg, err := topic.NewTopicMessage(&a.key, now, a.nextSeq(), data)
	if err != nil {
		return fmt.Errorf("create topic message: %w", err)
	}
	_, err = bus.Publish(targetTopic, msg, topic.PublishOpts{ConfirmationMode: topic.FireAndForget})
	return err
}

func createAgent(name string, bus demoBus) *agent {
	key, err := keylib.NewNeuronPrivateKey()
	must(err)
	stdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: topic.BackendHCS, Memo: name + "-stdIn"})
	if err != nil {
		log.Fatalf("FATAL: create %s-stdIn topic: %v", name, err)
	}
	stdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: topic.BackendHCS, Memo: name + "-stdOut"})
	if err != nil {
		log.Fatalf("FATAL: create %s-stdOut topic: %v", name, err)
	}
	stdErr, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: topic.BackendHCS, Memo: name + "-stdErr"})
	if err != nil {
		log.Fatalf("FATAL: create %s-stdErr topic: %v", name, err)
	}
	return &agent{name: name, key: key, stdIn: stdIn, stdOut: stdOut, stdErr: stdErr}
}

// ═══════════════════════════════════════════════════════════════════
// Logging helpers
// ═══════════════════════════════════════════════════════════════════

var verbose bool
var testnetMode bool
var evmTestnetMode bool

// relayAddrs holds any --relay multiaddrs provided on the command line.
// When non-empty, buyer and seller libp2p hosts are configured with
// autorelay + AutoNAT v2 + hole punching via delivery.WithRelay, so peers
// behind independent NATs can reserve a slot on the relay and reach each
// other via DCUtR-upgraded or relayed /p2p-circuit paths.
var relayAddrs []string

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("FATAL: required env var %s not set", key)
	}
	return v
}

func phase(name string) {
	fmt.Printf("\n── %s ─────────────────────────────────────────\n", name)
}
func real(f string, a ...any) { fmt.Printf("  [REAL] "+f+"\n", a...) }
func mock(f string, a ...any) { fmt.Printf("  [MOCK] "+f+"\n", a...) }
func info(f string, a ...any) { fmt.Printf("         "+f+"\n", a...) }
func detail(f string, a ...any) {
	if verbose {
		fmt.Printf("         "+f+"\n", a...)
	}
}

func hashScan(label, value string) {
	if testnetMode && strings.HasPrefix(value, "0.0.") {
		info("HashScan: https://hashscan.io/testnet/topic/%s", value)
	} else if testnetMode {
		info("HashScan: testnet | %s=%s", label, shortHash(value))
	} else {
		info("HashScan: N/A (mock mode) | %s=%s", label, shortHash(value))
	}
}

func transition(who string, sm *payment.AgreementStateMachine, event payment.AgreementEvent) {
	from := sm.State()
	newState, err := sm.Transition(event)
	if err != nil {
		log.Fatalf("FATAL: %s transition failed: %s + %s -> err: %v", who, from, event, err)
	}
	info("%s: %s + %s -> %s", who, from, event, newState)
}

func shortAddr(addr string) string {
	if len(addr) > 14 {
		return addr[:8] + "..." + addr[len(addr)-4:]
	}
	return addr
}

func shortHash(h string) string {
	if len(h) > 18 {
		return h[:18] + "..."
	}
	return h
}

func must(err error) {
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}
}

// neuronHostOptions returns the delivery.HostOption slice applied to buyer
// and seller libp2p hosts. When --relay is supplied, WithRelay enables the
// Neuron NAT-traversal stack (autorelay + AutoNAT v2 + hole punching + UPnP).
// WithForcedReachability(Private) is also set because the --relay flag is
// itself the user's declaration that this host is behind a NAT — without it,
// AutoNAT v2 won't converge to Private in a small network (no seed probers)
// and autorelay never fires.
func neuronHostOptions() []delivery.HostOption {
	if len(relayAddrs) == 0 {
		return nil
	}
	return []delivery.HostOption{
		delivery.WithRelay(relayAddrs...),
		delivery.WithForcedReachability(network.ReachabilityPrivate),
	}
}

// awaitRelayReservation seeds the peerstore by dialing each configured relay
// and then waits up to 20 seconds for the host to advertise a /p2p-circuit
// multiaddr. The explicit Connect is necessary because autorelay's static-
// relays mode only reserves once a reachability=Private event fires AND the
// candidate's addresses are in the peerstore; without an explicit dial,
// autorelay can idle on a candidate it has never contacted.
func awaitRelayReservation(h host.Host) {
	if len(relayAddrs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, a := range relayAddrs {
		m, err := ma.NewMultiaddr(a)
		if err != nil {
			info("relay addr %q unparseable: %v", a, err)
			continue
		}
		info, err := peer.AddrInfoFromP2pAddr(m)
		if err != nil {
			// `info` shadows the outer logger here intentionally for err only.
			fmt.Printf("relay addr %q missing /p2p: %v\n", a, err)
			continue
		}
		if err := h.Connect(ctx, *info); err != nil {
			fmt.Printf("dial to relay %s failed: %v\n", info.ID, err)
		}
	}

	if err := delivery.WaitForRelayReservation(ctx, h); err != nil {
		fmt.Println("         relay reservation not acquired within 20s; connectionSetup will omit /p2p-circuit addrs (DCUtR still possible if buyer independently reserves)")
		return
	}
	real("autorelay acquired /p2p-circuit reservation via configured --relay")
}

// ═══════════════════════════════════════════════════════════════════
// Demo orchestration
// ═══════════════════════════════════════════════════════════════════

func main() {
	jpegPath := flag.String("jpeg", "", "Path to JPEG file to transfer (default: bundled testdata/photo.jpg)")
	price := flag.String("price", "500000000", "Price in tinybar (default: 500000000 = 5 HBAR)")
	listenAddr := flag.String("listen", "/ip4/0.0.0.0/udp/0/quic-v1", "Seller listen address")
	mode := flag.String("mode", "mock", "Demo mode: mock (default; zero-friction local, no infra), testnet (real HCS), or evm-testnet (real HCS + on-chain escrow)")
	role := flag.String("role", "all", "Demo role: all (single-process, default), seller (split mode), or buyer (split mode)")
	bootstrapPath := flag.String("bootstrap", "", "Path to bootstrap JSON file (required for --role=seller|buyer)")
	relayFlag := flag.String("relay", "", "Comma-separated Circuit Relay v2 multiaddrs (e.g., /ip4/PUB/udp/4001/quic-v1/p2p/RELAY_ID). When set, buyer+seller use autorelay + DCUtR + hole punching.")
	flag.BoolVar(&verbose, "verbose", false, "Show full hashes, payload details, and DID:key")
	flag.Parse()

	if *relayFlag != "" {
		for _, a := range strings.Split(*relayFlag, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				relayAddrs = append(relayAddrs, a)
			}
		}
	}

	switch *mode {
	case "mock":
		// default
	case "testnet":
		testnetMode = true
	case "evm-testnet":
		testnetMode = true
		evmTestnetMode = true
	default:
		log.Fatalf("invalid --mode=%s (must be 'mock', 'testnet', or 'evm-testnet')", *mode)
	}

	// Split-mode dispatch. --role=all falls through to the original inline
	// orchestration below (single-process behavior, unchanged).
	switch *role {
	case "all":
		// fall through
	case "seller":
		if !testnetMode {
			log.Fatalf("--role=seller requires --mode=testnet or --mode=evm-testnet (in-memory mock bus cannot cross processes)")
		}
		if *bootstrapPath == "" {
			log.Fatalf("--role=seller requires --bootstrap=<path> (the JSON file seller writes and buyer reads)")
		}
		runSeller(*jpegPath, *price, *listenAddr, *bootstrapPath)
		return
	case "buyer":
		if !testnetMode {
			log.Fatalf("--role=buyer requires --mode=testnet or --mode=evm-testnet (in-memory mock bus cannot cross processes)")
		}
		if *bootstrapPath == "" {
			log.Fatalf("--role=buyer requires --bootstrap=<path> (the JSON file written by the seller process)")
		}
		runBuyer(*bootstrapPath)
		return
	default:
		log.Fatalf("invalid --role=%s (must be 'all', 'seller', or 'buyer')", *role)
	}

	if *jpegPath == "" {
		exe, _ := os.Executable()
		*jpegPath = filepath.Join(filepath.Dir(exe), "testdata", "photo.jpg")
		if _, err := os.Stat(*jpegPath); err != nil {
			*jpegPath = filepath.Join("cmd", "buyer-seller-demo", "testdata", "photo.jpg")
			if _, err := os.Stat(*jpegPath); err != nil {
				log.Fatalf("JPEG file not found. Use --jpeg to specify path.")
			}
		}
	}

	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("  Neuron SDK -- Buyer-Seller JPEG Demo")
	if evmTestnetMode {
		fmt.Println("  Mode: evm-testnet (REAL HCS + REAL EVM registry + REAL EVM escrow)")
	} else if testnetMode {
		fmt.Println("  Mode: testnet (REAL HCS topics, mock registry + escrow)")
	} else {
		fmt.Println("  Mode: mock (in-memory topic bus, registry, escrow)")
	}
	fmt.Println("================================================================")

	demoProtocol := "/neuron/jpeg-transfer/1.0.0"

	// ── Phase 1: SETUP ────────────────────────────────────────────
	phase("1: SETUP")

	var bus demoBus
	if testnetMode {
		real("Initializing Hedera testnet client...")
		client, operatorID, err := topic.NewTestnetClientFromEnv()
		if err != nil {
			log.Fatalf("FATAL: %v\n"+
				"  In testnet mode, set %s (e.g. 0.0.X) and %s (ECDSA secp256k1 hex)\n"+
				"  before running. Operator credentials must never be hardcoded in source.",
				err, topic.HederaOperatorEnvAccountID, topic.HederaOperatorEnvPrivateKey)
		}
		info("operator = %s", operatorID.String())
		info("network  = Hedera Testnet")
		info("HashScan: https://hashscan.io/testnet/account/%s", operatorID.String())

		hcsClient := topic.NewRealHCSClient(client)
		hcsAdapter := topic.NewHCSAdapter(hcsClient)
		bus = newHCSTopicBus(hcsAdapter)
	} else {
		bus = newMemoryTopicBus()
	}

	// Create agents and HCS topics BEFORE EVM initialization.
	// Hedera SDK topic creation can fail if done after heavy EVM RPC usage.
	if testnetMode {
		real("Creating 9 HCS topics on Hedera testnet...")
	}
	seller := createAgent("Seller", bus)
	buyer := createAgent("Buyer", bus)
	validator := createAgent("Validator", bus)

	for _, a := range []*agent{seller, buyer, validator} {
		pub := a.key.PublicKey()
		pid, _ := pub.PeerID()
		real("secp256k1 identity created: %s", a.name)
		info("EVM     = %s", a.evmAddress())
		info("PeerID  = %s", pid.String())
		if verbose {
			detail("DID:key = %s", pub.DIDKey())
		}
	}

	// EVM contract adapters (nil unless evm-testnet mode).
	var evmRegistry *registry.EVMRegistryContract
	var evmEscrow *payment.EVMEscrowAdapter
	var operatorAddr common.Address

	if evmTestnetMode {
		rpcURL := os.Getenv("HEDERA_EVM_RPC")
		if rpcURL == "" {
			rpcURL = "https://testnet.hashio.io/api"
		}
		real("Connecting to Hedera EVM JSON-RPC: %s", rpcURL)
		ethClient, err := ethclient.Dial(rpcURL)
		must(err)

		// EVM private key for contract calls. Defaults to HEDERA_OPERATOR_KEY
		// but can be overridden via NEURON_EVM_PRIVATE_KEY if the HCS operator
		// key differs from the EVM deployer key (common with Hedera accounts).
		evmKeyHex := os.Getenv("NEURON_EVM_PRIVATE_KEY")
		if evmKeyHex == "" {
			evmKeyHex = os.Getenv(topic.HederaOperatorEnvPrivateKey)
		}
		operatorECDSA, err := ethcrypto.HexToECDSA(strings.TrimPrefix(evmKeyHex, "0x"))
		if err != nil {
			log.Fatalf("FATAL: cannot parse EVM private key as ECDSA hex: %v", err)
		}
		operatorAddr = ethcrypto.PubkeyToAddress(operatorECDSA.PublicKey)
		chainID := big.NewInt(296) // Hedera testnet
		auth, err := bind.NewKeyedTransactorWithChainID(operatorECDSA, chainID)
		must(err)
		real("EVM operator: %s", operatorAddr.Hex())

		registryAddr := common.HexToAddress(requireEnv("NEURON_REGISTRY_CONTRACT"))
		evmRegistry, err = registry.NewEVMRegistryContract(ethClient, registryAddr, auth)
		must(err)
		real("Identity Registry contract: %s", registryAddr.Hex())

		escrowAddr := common.HexToAddress(requireEnv("NEURON_ESCROW_CONTRACT"))
		tokenAddr := common.HexToAddress(requireEnv("NEURON_TOKEN_CONTRACT"))
		evmEscrow, err = payment.NewEVMEscrowAdapter(ethClient, escrowAddr, tokenAddr, auth)
		must(err)
		real("Escrow contract: %s", escrowAddr.Hex())
		real("TestToken contract: %s", tokenAddr.Hex())

		// Mint test tokens to operator if needed (idempotent — TestToken has unlimited mint).
		tokenContract, err := paymentbindings.NewTestToken(tokenAddr, ethClient)
		must(err)
		mintTx, err := tokenContract.Mint(auth, operatorAddr, new(big.Int).SetUint64(10_000_000_000))
		if err != nil {
			info("Token mint skipped (may already have balance): %v", err)
		} else {
			_, err = bind.WaitMined(context.Background(), ethClient, mintTx)
			must(err)
			real("Minted 10B TestToken to operator")
		}
	}

	mockReg := newMockRegistry() // used for non-EVM modes and as fallback for validator/buyer
	var escrow payment.EscrowAdapter
	if evmTestnetMode {
		escrow = evmEscrow
	} else {
		escrow = payment.NewMemoryEscrow()
	}
	_ = evmRegistry // used below in register/discover phases

	if evmTestnetMode {
		real("Topic bus: Hedera Consensus Service (9 HCS topics on testnet)")
		real("Registry: NeuronIdentityRegistry ERC-721 on Hedera EVM")
		real("Escrow: NeuronEscrow + TestToken on Hedera EVM")
	} else if testnetMode {
		real("Topic bus: Hedera Consensus Service (9 HCS topics on testnet)")
		mock("Registry: in-memory")
		mock("Escrow: in-memory (MemoryEscrow)")
	} else {
		mock("Topic bus: in-memory (9 topics: 3 agents x stdIn/stdOut/stdErr)")
		mock("Registry: in-memory")
		mock("Escrow: in-memory (MemoryEscrow)")
	}

	// ── Phase 2: REGISTER ─────────────────────────────────────────
	phase("2: REGISTER")

	settlementBinding := "memory"
	if evmTestnetMode {
		settlementBinding = "evm-escrow"
	}
	sellerCommerceJSON := fmt.Sprintf(`{"type":"neuron-commerce","name":"jpeg-delivery","version":"1.0.0","delivery":{"mode":"p2p"},"settlement":{"binding":"%s"},"pricing":{"amount":"%s","currency":"tinybar","unit":"per-file","interval":"0"}}`, settlementBinding, *price)
	sellerURI := fmt.Sprintf(`{"services":[%s]}`, sellerCommerceJSON)

	if evmTestnetMode {
		// Real on-chain registration via ERC-721 Identity Registry.
		// Uses the operator address as msg.sender (proof-of-control).
		real("Registering Seller on-chain via NeuronIdentityRegistry...")
		ctx := context.Background()
		agentIdBig, txHash, err := evmRegistry.Register(ctx, operatorAddr, sellerURI)
		must(err)
		seller.agentID = agentIdBig.String()
		real("Seller registered ON-CHAIN (ERC-721 NFT minted)")
		info("agentId    = %s", seller.agentID)
		info("txHash     = %s", shortHash(txHash))
		info("settlement = %s", settlementBinding)
		// Also register in mockReg so runAll's discover phase can find the seller.
		mockReg.register(seller.evmAddress(), sellerURI)
	} else {
		seller.agentID = mockReg.register(seller.evmAddress(), sellerURI)
		mock("Seller registered in-memory registry")
		info("agentId  = %s", seller.agentID)
	}
	info("services = neuron-commerce (jpeg-delivery, p2p, %s tinybar)", *price)

	validatorSvc, _ := validation.NewNeuronValidatorService("validation", "1.0.0",
		[]string{"008-payment", "009-delivery"}, "topic")
	validatorSvcJSON, _ := json.Marshal(validatorSvc)
	validatorURI := fmt.Sprintf(`{"services":[%s]}`, string(validatorSvcJSON))
	validator.agentID = mockReg.register(validator.evmAddress(), validatorURI)
	mock("Validator registered in-memory registry (not on-chain)")
	info("agentId  = %s", validator.agentID)
	info("services = neuron-validator (domains: [008-payment, 009-delivery])")

	buyer.agentID = mockReg.register(buyer.evmAddress(), `{"services":[]}`)
	mock("Buyer registered in-memory registry")
	info("agentId  = %s", buyer.agentID)

	// ── Phase 3: DISCOVER ─────────────────────────────────────────
	phase("3: DISCOVER")

	if evmTestnetMode {
		// Verify seller's on-chain registration via AgentURIOf().
		real("Verifying Seller registration on-chain...")
		ctx := context.Background()
		sellerAgentIdBig := new(big.Int)
		sellerAgentIdBig.SetString(seller.agentID, 10)
		onChainURI, err := evmRegistry.AgentURIOf(ctx, sellerAgentIdBig)
		if err != nil {
			info("On-chain lookup error (non-fatal): %v", err)
		} else {
			real("On-chain agentURI verified (agentId=%s, length=%d bytes)", seller.agentID, len(onChainURI))
		}
	}
	sellerEntry, _ := mockReg.lookup(seller.evmAddress())
	if !evmTestnetMode {
		mock("Buyer looked up Seller via in-memory registry")
	}
	info("found agentId=%s  EVM=%s", sellerEntry.agentID, shortAddr(seller.evmAddress()))
	info("offer: JPEG delivery, %s tinybar, one-time, p2p-stream", *price)

	// ── Phase 4: NEGOTIATE ────────────────────────────────────────
	phase("4: NEGOTIATE")

	requestID := "req-001"
	buyerSM := payment.NewAgreementStateMachine(requestID)
	sellerSM := payment.NewAgreementStateMachine(requestID)

	// Buyer → seller.stdIn: serviceRequest (spec 008 FR-P06)
	svcReq := payment.ServiceRequest{
		Type: "serviceRequest", Version: "1.0.0", RequestID: requestID,
		ServiceRef: "jpeg-delivery", SettlementBinding: settlementBinding,
		ProposedAmount: *price, ProposedCurrency: "tinybar", ProposedInterval: "0",
		BuyerStdIn: buyer.stdIn.Locator(),
	}
	must(buyer.publishPayload(bus, seller.stdIn, svcReq))
	real("Buyer -> Seller: serviceRequest")
	info("requestId=%s  service=jpeg-delivery  amount=%s tinybar", requestID, *price)
	info("sender=%s -> Seller.stdIn (%s)", shortAddr(buyer.evmAddress()), seller.stdIn.Locator())

	real("Agreement state machine transitions:")
	transition("  Buyer ", buyerSM, payment.EventServiceRequest)
	transition("  Seller", sellerSM, payment.EventServiceRequest)

	// Seller → buyer.stdIn: serviceResponse (spec 008 FR-P08)
	svcResp := payment.ServiceResponse{
		Type: "serviceResponse", Version: "1.0.0", RequestID: requestID, Action: "accept",
	}
	must(seller.publishPayload(bus, buyer.stdIn, svcResp))
	real("Seller -> Buyer: serviceResponse (action=accept)")
	info("sender=%s -> Buyer.stdIn (%s)", shortAddr(seller.evmAddress()), buyer.stdIn.Locator())

	transition("  Buyer ", buyerSM, payment.EventAccept)
	transition("  Seller", sellerSM, payment.EventAccept)

	respJSON, _ := json.Marshal(svcResp)
	agreementHash := payment.ComputeAgreementHash(respJSON)
	agreementHashHex := fmt.Sprintf("0x%x", agreementHash[:])
	real("agreementHash = %s (keccak256 of canonical serviceResponse)", shortHash(agreementHashHex))
	detail("full agreementHash = %s", agreementHashHex)

	// ── Phase 5: FUND ─────────────────────────────────────────────
	phase("5: FUND")

	ctx := context.Background()

	// In evm-testnet mode, use operator address for buyer/seller (single funded account).
	// In mock/testnet mode, use agent EVM addresses (no on-chain interaction).
	escrowBuyer := buyer.evmAddress()
	escrowSeller := seller.evmAddress()
	if evmTestnetMode {
		escrowBuyer = operatorAddr.Hex()
		escrowSeller = operatorAddr.Hex()
	}

	escrowRef, err := escrow.CreateEscrow(ctx, escrowBuyer, escrowSeller, nil,
		"tinybar", 1, agreementHash, uint64(time.Now().Unix())+3600)
	must(err)
	if evmTestnetMode {
		real("Escrow created ON-CHAIN")
	} else {
		mock("Escrow created in-memory")
	}
	info("ref=%s  buyer=%s  seller=%s", escrowRef.Locator, shortAddr(escrowBuyer), shortAddr(escrowSeller))
	info("agreementHash anchored to escrow")

	depositResult, err := escrow.Deposit(ctx, escrowRef, *price)
	must(err)
	if evmTestnetMode {
		real("Buyer deposited %s tinybar ON-CHAIN (balance=%s)", *price, depositResult.NewBalance)
	} else {
		mock("Buyer deposited %s tinybar (balance=%s)", *price, depositResult.NewBalance)
	}
	hashScan("escrowRef", escrowRef.Locator)

	transition("  Buyer ", buyerSM, payment.EventEscrowCreated)
	transition("  Seller", sellerSM, payment.EventEscrowCreated)

	// Buyer → seller.stdIn: escrowCreated (spec 008 FR-P18a)
	escrowCreated := payment.EscrowCreated{
		Type: "escrowCreated", Version: "1.0.0", RequestID: requestID,
		EscrowRef: escrowRef.Locator, DepositAmount: *price, DepositCurrency: "tinybar",
	}
	must(buyer.publishPayload(bus, seller.stdIn, escrowCreated))
	real("Buyer -> Seller: escrowCreated (notifying seller of funding)")

	// ── Phase 6: CONNECT ──────────────────────────────────────────
	phase("6: CONNECT")

	sellerECDSA, err := ethcrypto.GenerateKey()
	must(err)
	buyerECDSA, err := ethcrypto.GenerateKey()
	must(err)

	sellerHost, err := delivery.NewLibp2pHost(sellerECDSA, *listenAddr, neuronHostOptions()...)
	must(err)
	defer sellerHost.Close()
	sellerAdapter := delivery.NewLibp2pAdapter(sellerHost)

	real("Seller libp2p host started")
	for _, addr := range sellerHost.Addrs() {
		info("listening: %s/p2p/%s", addr, sellerHost.ID())
	}
	info("protocol: %s", demoProtocol)

	awaitRelayReservation(sellerHost)

	// Seller → buyer.stdIn: connectionSetup with ECIES-encrypted multiaddrs (spec 008 FR-P33)
	connSetup, err := delivery.BuildConnectionSetup(requestID, sellerHost, demoProtocol, &buyerECDSA.PublicKey)
	must(err)
	must(seller.publishPayload(bus, buyer.stdIn, connSetup))
	real("Seller -> Buyer: connectionSetup")
	info("ECIES encryption: secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM")
	info("encryptedMultiaddrs = %s", shortHash(connSetup.EncryptedMultiaddrs))
	detail("full encryptedMultiaddrs = %s", connSetup.EncryptedMultiaddrs)

	sellerReadyCh := make(chan *delivery.DeliveryChannel, 1)
	sellerAdapter.HandleIncoming(protocol.ID(demoProtocol), func(ch *delivery.DeliveryChannel) {
		sellerReadyCh <- ch
	})

	buyerHost, err := delivery.NewLibp2pHost(buyerECDSA, "/ip4/0.0.0.0/udp/0/quic-v1", neuronHostOptions()...)
	must(err)
	defer buyerHost.Close()
	buyerAdapter := delivery.NewLibp2pAdapter(buyerHost)

	buyerChannel, err := delivery.ConnectFromSetup(buyerAdapter, connSetup, buyerECDSA)
	must(err)
	_, err = buyerAdapter.Send(buyerChannel, []byte{})
	must(err)

	real("Buyer decrypted connectionSetup and connected")
	info("transport = %s", buyerChannel.Transport)
	info("remote PeerID = %s", buyerChannel.PeerID)

	var sellerChannel *delivery.DeliveryChannel
	select {
	case sellerChannel = <-sellerReadyCh:
	case <-time.After(10 * time.Second):
		log.Fatal("timeout waiting for seller to accept connection")
	}

	transition("  Buyer ", buyerSM, payment.EventDeliveryStarted)
	transition("  Seller", sellerSM, payment.EventDeliveryStarted)

	// ── Phase 7: DELIVER ──────────────────────────────────────────
	phase("7: DELIVER")

	sendResult, err := delivery.SendFile(sellerAdapter, sellerChannel, *jpegPath)
	must(err)
	real("Seller sent file via libp2p framing protocol (FR-D22)")
	info("file       = %s", sendResult.Filename)
	info("size       = %d bytes", sendResult.Size)
	info("frames     = %d data frame(s) (max 4 MiB each)", sendResult.FrameCount)
	info("SHA256     = %s", shortHash(sendResult.SHA256))
	detail("full SHA256 = %s", sendResult.SHA256)

	outputDir, _ := os.MkdirTemp("", "neuron-demo-*")
	defer os.RemoveAll(outputDir)

	recvResult, err := delivery.ReceiveFile(buyerAdapter, buyerChannel, outputDir)
	must(err)
	// Independently compute the SHA256 match at the demo layer so the logged
	// result is a real comparison, not a hardcoded success string.
	// ReceiveFile already verifies internally and would have returned an
	// error above on mismatch — this is defense in depth plus honest output.
	sha256Match := sendResult.SHA256 == recvResult.SHA256
	if !sha256Match {
		log.Fatalf("FATAL: SHA256 mismatch after receive: sender=%s receiver=%s",
			sendResult.SHA256, recvResult.SHA256)
	}
	real("Buyer received and independently verified file (SHA256)")
	info("SHA256 match = %s (sender=%s, receiver=%s)",
		boolOK(sha256Match),
		shortHash(sendResult.SHA256),
		shortHash(recvResult.SHA256))

	// ── Phase 8: SETTLE ───────────────────────────────────────────
	phase("8: SETTLE")

	deliveryProof := []byte(fmt.Sprintf(`{"requestId":"%s","file":"%s","sha256":"%s","bytes":%d}`,
		requestID, sendResult.Filename, sendResult.SHA256, sendResult.Size))
	evidenceHash := validation.ComputeEvidenceHash(deliveryProof)
	evidenceHashHex := validation.FormatEvidenceHash(evidenceHash)

	releaseRecipient := seller.evmAddress()
	if evmTestnetMode {
		releaseRecipient = operatorAddr.Hex()
	}
	releaseRef, err := escrow.RequestRelease(ctx, escrowRef, *price, releaseRecipient, evidenceHash)
	must(err)
	if evmTestnetMode {
		real("Seller requested release ON-CHAIN")
	}

	// Seller → buyer.stdIn: invoice with evidenceHash linking to delivery proof (spec 008 FR-P24)
	invoice := payment.Invoice{
		Type: "invoice", Version: "1.0.0", RequestID: requestID,
		ReleaseRequestRef: releaseRef.Locator, EscrowRef: escrowRef.Locator,
		Amount: *price, Currency: "tinybar",
	}
	must(seller.publishPayload(bus, buyer.stdIn, invoice))
	real("Seller -> Buyer: invoice")
	info("amount       = %s tinybar", *price)
	info("evidenceHash = %s", shortHash(evidenceHashHex))
	detail("full evidenceHash = %s", evidenceHashHex)

	transition("  Buyer ", buyerSM, payment.EventInvoice)
	transition("  Seller", sellerSM, payment.EventInvoice)

	// Buyer → seller.stdIn: invoiceAck (spec 008 FR-P26)
	invoiceAck := payment.InvoiceAck{
		Type: "invoiceAck", Version: "1.0.0", RequestID: requestID,
		ReleaseRequestRef: releaseRef.Locator, Action: "approved",
	}
	must(buyer.publishPayload(bus, seller.stdIn, invoiceAck))
	real("Buyer -> Seller: invoiceAck (action=approved)")

	transition("  Buyer ", buyerSM, payment.EventInvoiceApproved)
	transition("  Seller", sellerSM, payment.EventInvoiceApproved)

	releaseResult, err := escrow.ApproveRelease(ctx, escrowRef, releaseRef)
	must(err)
	if evmTestnetMode {
		real("Escrow released %s tinybar ON-CHAIN to %s", releaseResult.Released, shortAddr(releaseResult.Recipient))
	} else {
		mock("Escrow released %s tinybar to %s", releaseResult.Released, shortAddr(releaseResult.Recipient))
	}
	hashScan("releaseRef", releaseRef.Locator)

	transition("  Buyer ", buyerSM, payment.EventComplete)
	transition("  Seller", sellerSM, payment.EventComplete)

	// ── Phase 9: VALIDATE ─────────────────────────────────────────
	phase("9: VALIDATE")

	// Validator reads all messages from both stdIn channels to observe the full protocol flow.
	// seller.stdIn has: serviceRequest, escrowCreated, invoiceAck  (3 msgs from buyer)
	// buyer.stdIn  has: serviceResponse, connectionSetup, invoice  (3 msgs from seller)
	//
	// Honest demo caveat: in testnet mode, the underlying messages were
	// published to real HCS topics (with WaitForConsensus, real signatures,
	// real consensus timestamps), but the validator's *observation* loop
	// reads them from a local cache populated at publish time, NOT from a
	// fresh mirror-node query. A production validator would round-trip
	// through https://testnet.mirrornode.hedera.com/api/v1/topics/{id}/messages
	// instead. Signatures are still verified for real on every observation.
	sellerInMsgs := bus.getMessages(seller.stdIn)
	buyerInMsgs := bus.getMessages(buyer.stdIn)
	allMsgs := append(sellerInMsgs, buyerInMsgs...)
	real("Validator observing protocol messages")
	info("Seller.stdIn: %d messages (buyer -> seller)", len(sellerInMsgs))
	info("Buyer.stdIn:  %d messages (seller -> buyer)", len(buyerInMsgs))
	info("total:        %d messages", len(allMsgs))
	if testnetMode {
		info("source:       local cache (publish-time mirror); not a fresh mirror-node read")
	} else {
		info("source:       in-memory mock bus")
	}

	// Verify signatures.
	real("Verifying ECDSA signatures (ecrecover + sender match)")
	for i, msg := range allMsgs {
		if err := topic.ValidateTopicMessage(msg); err != nil {
			log.Fatalf("FATAL: validator found invalid signature on message %d: %v", i, err)
		}
	}
	info("result: all %d signatures VALID", len(allMsgs))

	// Categorize observed messages.
	type observedMsg struct {
		hash    string
		msgType string
		sender  string
	}
	var observations []observedMsg
	for _, msg := range allMsgs {
		payloadHash := ethcrypto.Keccak256(msg.Payload())
		var parsed struct {
			Type string `json:"type"`
		}
		json.Unmarshal(msg.Payload(), &parsed)
		obs := observedMsg{
			hash:    "0x" + hex.EncodeToString(payloadHash),
			msgType: parsed.Type,
			sender:  msg.SenderAddress(),
		}
		observations = append(observations, obs)
		detail("  observed: type=%-20s sender=%s hash=%s", obs.msgType, shortAddr(obs.sender), shortHash(obs.hash))
	}

	// Build payment evidence.
	type paymentEvidenceDoc struct {
		Domain          string   `json:"domain"`
		ObservedHashes  []string `json:"observedHashes"`
		ObservedTypes   []string `json:"observedTypes"`
		RequestID       string   `json:"requestId"`
		SellerAddress   string   `json:"sellerAddress"`
		SignaturesValid int      `json:"signaturesValid"`
	}
	var paymentHashes, paymentTypes []string
	for _, obs := range observations {
		if obs.msgType == "serviceRequest" || obs.msgType == "serviceResponse" ||
			obs.msgType == "escrowCreated" || obs.msgType == "invoice" || obs.msgType == "invoiceAck" {
			paymentHashes = append(paymentHashes, obs.hash)
			paymentTypes = append(paymentTypes, obs.msgType)
		}
	}
	paymentDoc, _ := json.Marshal(paymentEvidenceDoc{
		Domain: "008-payment", ObservedHashes: paymentHashes, ObservedTypes: paymentTypes,
		RequestID: requestID, SellerAddress: seller.evmAddress(), SignaturesValid: len(paymentHashes),
	})
	paymentEvidenceHash := validation.ComputeEvidenceHash(paymentDoc)

	// Build delivery evidence.
	type deliveryEvidenceDoc struct {
		Domain         string   `json:"domain"`
		ObservedHashes []string `json:"observedHashes"`
		ObservedTypes  []string `json:"observedTypes"`
		FileReceived   string   `json:"fileReceived"`
		FileSHA256     string   `json:"fileSHA256"`
		FileSize       int64    `json:"fileSize"`
	}
	var deliveryHashes, deliveryTypes []string
	for _, obs := range observations {
		if obs.msgType == "connectionSetup" {
			deliveryHashes = append(deliveryHashes, obs.hash)
			deliveryTypes = append(deliveryTypes, obs.msgType)
		}
	}
	deliveryDoc, _ := json.Marshal(deliveryEvidenceDoc{
		Domain: "009-delivery", ObservedHashes: deliveryHashes, ObservedTypes: deliveryTypes,
		FileReceived: recvResult.Filename, FileSHA256: recvResult.SHA256, FileSize: recvResult.Size,
	})
	deliveryEvidenceHash := validation.ComputeEvidenceHash(deliveryDoc)

	// Validator → validator.stdOut: evidence envelopes (spec 010 FR-V16)
	env1, err := validation.NewEvidenceEnvelope(
		validator.agentID, seller.agentID, "008-payment",
		validation.VerdictCompliant,
		validation.FormatEvidenceHash(paymentEvidenceHash),
		"mem://evidence/payment-001",
	)
	must(err)
	_, err = validation.PublishEvidence(env1, &validator.key, validator.stdOut, bus,
		uint64(time.Now().UnixNano()), validator.nextSeq())
	must(err)

	env2, err := validation.NewEvidenceEnvelope(
		validator.agentID, seller.agentID, "009-delivery",
		validation.VerdictCompliant,
		validation.FormatEvidenceHash(deliveryEvidenceHash),
		"mem://evidence/delivery-001",
	)
	must(err)
	_, err = validation.PublishEvidence(env2, &validator.key, validator.stdOut, bus,
		uint64(time.Now().UnixNano()), validator.nextSeq())
	must(err)

	real("Evidence envelopes published to Validator.stdOut")
	info("Verdict #1: 008-payment  -> COMPLIANT (%d negotiation messages)", len(paymentHashes))
	info("  evidenceHash = %s", shortHash(validation.FormatEvidenceHash(paymentEvidenceHash)))
	info("Verdict #2: 009-delivery -> COMPLIANT (%d delivery messages)", len(deliveryHashes))
	info("  evidenceHash = %s", shortHash(validation.FormatEvidenceHash(deliveryEvidenceHash)))

	// Verify evidence chains.
	paymentChainOK := validation.VerifyEvidenceHash(paymentDoc, env1)
	deliveryChainOK := validation.VerifyEvidenceHash(deliveryDoc, env2)
	real("Evidence hash chain verification:")
	info("payment  chain: keccak256(doc) == envelope.evidenceHash ? %s", boolOK(paymentChainOK))
	info("delivery chain: keccak256(doc) == envelope.evidenceHash ? %s", boolOK(deliveryChainOK))

	hash1, _ := validation.ComputeEnvelopeHash(env1)
	hash2, _ := validation.ComputeEnvelopeHash(env2)
	responseHash1 := fmt.Sprintf("0x%x", hash1[:])
	responseHash2 := fmt.Sprintf("0x%x", hash2[:])
	real("Envelope responseHash (would anchor on-chain in Validation Registry):")
	info("#1: %s", shortHash(responseHash1))
	info("#2: %s", shortHash(responseHash2))
	detail("full responseHash #1 = %s", responseHash1)
	detail("full responseHash #2 = %s", responseHash2)

	// ── Phase 10: DONE ────────────────────────────────────────────

	_ = buyerAdapter.Disconnect(buyerChannel)
	_ = sellerAdapter.Disconnect(sellerChannel)

	// ── SUMMARY ───────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("  DEMO SUMMARY")
	fmt.Println("================================================================")
	fmt.Println()
	if testnetMode {
		fmt.Println("  Mode:       testnet (REAL HCS topics, mock registry + escrow)")
	} else {
		fmt.Println("  Mode:       mock (in-memory topic bus, registry, escrow)")
	}
	fmt.Printf("  Agents:     3 (Seller=%s, Buyer=%s, Validator=%s)\n", seller.agentID, buyer.agentID, validator.agentID)
	fmt.Println()
	busLabel := "MOCK bus"
	if testnetMode {
		busLabel = "REAL HCS"
	}
	// Phase 9 observation source label: in testnet mode, publishes are real
	// HCS but the validator still observes from a local cache (not a fresh
	// mirror-node read). Be explicit about that in the summary.
	validateLabel := "REAL validation + " + busLabel
	if testnetMode {
		validateLabel = "REAL validation + REAL HCS publish, local cache observation"
	}
	fmt.Println("  Phase Results:")
	fmt.Println("    1  SETUP       OK   3 identities created             [REAL crypto]")
	fmt.Println("    2  REGISTER    OK   3 agents registered              [MOCK registry]")
	fmt.Println("    3  DISCOVER    OK   Seller found via lookup          [MOCK registry]")
	fmt.Printf("    4  NEGOTIATE   OK   IDLE -> AGREED (2 msgs)          [REAL state machine + %s]\n", busLabel)
	fmt.Printf("    5  FUND        OK   %s tinybar deposited     [MOCK escrow]\n", *price)
	fmt.Printf("    6  CONNECT     OK   %s, ECIES encrypted     [REAL libp2p + ECIES + %s]\n", buyerChannel.Transport, busLabel)
	fmt.Printf("    7  DELIVER     OK   %s %d bytes             [REAL libp2p + SHA256]\n", recvResult.Filename, recvResult.Size)
	fmt.Printf("    8  SETTLE      OK   %s tinybar released     [MOCK escrow + %s]\n", *price, busLabel)
	fmt.Printf("    9  VALIDATE    OK   %d sigs, 2 COMPLIANT verdicts  [%s]\n", len(allMsgs), validateLabel)
	fmt.Println("   10  DONE       OK")
	fmt.Println()
	fmt.Println("  File Transfer:")
	fmt.Printf("    File:       %s\n", recvResult.Filename)
	fmt.Printf("    Size:       %d bytes (%d data frame)\n", recvResult.Size, recvResult.FrameCount)
	fmt.Printf("    SHA256:     %s\n", shortHash(recvResult.SHA256))
	// Integrity line reflects the actual send/receive SHA256 comparison
	// computed in the DELIVER phase, not a hardcoded success string.
	if sha256Match {
		fmt.Printf("    Integrity:  SHA256 match (sender == receiver)\n")
	} else {
		fmt.Printf("    Integrity:  SHA256 MISMATCH\n")
	}
	fmt.Println()
	fmt.Println("  Validation:")
	fmt.Printf("    Verdict #1: 008-payment  -> COMPLIANT (%d messages)\n", len(paymentHashes))
	fmt.Printf("    Verdict #2: 009-delivery -> COMPLIANT (%d message)\n", len(deliveryHashes))
	fmt.Printf("    Evidence chain: %s\n", boolResult(paymentChainOK && deliveryChainOK))
	fmt.Println()
	fmt.Println("  Explorer:")
	if testnetMode {
		fmt.Println("    Topics and messages visible on Hedera testnet HashScan")
		fmt.Printf("    Seller.stdIn:    https://hashscan.io/testnet/topic/%s\n", seller.stdIn.Locator())
		fmt.Printf("    Buyer.stdIn:     https://hashscan.io/testnet/topic/%s\n", buyer.stdIn.Locator())
		fmt.Printf("    Validator.stdOut: https://hashscan.io/testnet/topic/%s\n", validator.stdOut.Locator())
		fmt.Printf("    Mirror API:      https://testnet.mirrornode.hedera.com/api/v1/topics/%s/messages\n", seller.stdIn.Locator())
	} else {
		fmt.Println("    All artifacts local/mock -- no HashScan links available")
		fmt.Println("    To get real links: run with --mode=testnet")
	}
}

func boolOK(b bool) string {
	if b {
		return "YES"
	}
	return "NO"
}

func boolResult(b bool) string {
	if b {
		return "verified (keccak256)"
	}
	return "FAILED"
}

// ═══════════════════════════════════════════════════════════════════
// Split-mode helpers: mirror-node poller, wait helper, bootstrap I/O
// ═══════════════════════════════════════════════════════════════════

// hederaMirrorNodeBaseURL is the Hedera testnet mirror node REST API base.
// Used by split-mode (--role=seller|buyer) to fetch HCS topic messages
// since HCSClient.SubscribeTopic is not implemented in this codebase.
const hederaMirrorNodeBaseURL = "https://testnet.mirrornode.hedera.com/api/v1"

// mirrorPollInterval is how often the wait helper polls the mirror node
// for new messages. The mirror node typically lags consensus by ~1-3s,
// so polling more aggressively wastes API quota without reducing latency.
const mirrorPollInterval = 2 * time.Second

// mirrorWaitDefaultTimeout is the per-message wait budget for split-mode
// payment messages. Hedera consensus + mirror lag is usually 3-7s; the
// long budget covers manual orchestration delays (e.g., the seller
// process starting before the buyer process is launched on another
// machine) and intermittent mirror node hiccups.
const mirrorWaitDefaultTimeout = 10 * time.Minute

// errMirrorWaitTimeout is returned by waitForPayloadType when the deadline
// elapses before a matching message arrives.
var errMirrorWaitTimeout = errors.New("waitForPayloadType: timeout waiting for message")

// mirrorMessage is the subset of the Hedera mirror REST API
// /api/v1/topics/{id}/messages response we need.
type mirrorMessage struct {
	ConsensusTimestamp string `json:"consensus_timestamp"`
	TopicID            string `json:"topic_id"`
	Message            string `json:"message"` // base64-encoded raw payload
	SequenceNumber     uint64 `json:"sequence_number"`
}

type mirrorResponse struct {
	Messages []mirrorMessage `json:"messages"`
}

// pollMirrorNodeSince fetches HCS topic messages from the Hedera testnet
// mirror node REST API with sequence number > sinceSeq. Returns the parsed
// TopicMessages and the highest sequence number observed. The caller is
// responsible for tracking sinceSeq across calls. This is a one-shot fetch
// (no goroutines, no background subscription).
//
// The mirror node lags consensus by ~1-3 seconds, so callers should expect
// a brief delay between Publish on one machine and the message appearing
// here on another machine.
func pollMirrorNodeSince(topicID string, sinceSeq uint64) ([]topic.TopicMessage, uint64, error) {
	// The Hedera mirror node /topics/{id}/messages endpoint does NOT accept
	// operator-based filters on sequencenumber (only equality). The
	// timestamp parameter accepts gt:/gte:/lt:/lte: but seq does not.
	// We fetch all messages on the topic in ascending order and filter
	// client-side by seq > sinceSeq. Each topic has only a handful of
	// messages in this demo so the cost is trivial.
	q := url.Values{}
	q.Set("order", "asc")
	q.Set("limit", "100")
	endpoint := fmt.Sprintf("%s/topics/%s/messages?%s", hederaMirrorNodeBaseURL, topicID, q.Encode())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, sinceSeq, fmt.Errorf("pollMirrorNodeSince: GET %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, sinceSeq, fmt.Errorf("pollMirrorNodeSince: GET %s returned %d: %s",
			endpoint, resp.StatusCode, string(body))
	}

	var parsed mirrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, sinceSeq, fmt.Errorf("pollMirrorNodeSince: decode response: %w", err)
	}

	highest := sinceSeq
	out := make([]topic.TopicMessage, 0, len(parsed.Messages))
	for _, m := range parsed.Messages {
		if m.SequenceNumber <= sinceSeq {
			// Already seen on a previous poll.
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(m.Message)
		if err != nil {
			return nil, sinceSeq, fmt.Errorf("pollMirrorNodeSince: base64 decode seq=%d: %w", m.SequenceNumber, err)
		}
		var msg topic.TopicMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			// Mirror may return non-Neuron messages (e.g. raw bytes from another
			// publisher). Skip with a warning rather than failing the whole demo.
			info("mirror: skipping non-Neuron message at seq=%d on topic %s (%v)", m.SequenceNumber, topicID, err)
			if m.SequenceNumber > highest {
				highest = m.SequenceNumber
			}
			continue
		}
		out = append(out, msg)
		if m.SequenceNumber > highest {
			highest = m.SequenceNumber
		}
	}
	return out, highest, nil
}

// syncFromMirror fetches new messages from the Hedera testnet mirror node
// for the given topic and merges them into the local hcsTopicBus.msgLog.
// Returns the new highest sequence number seen. The validator's existing
// bus.getMessages(ref) call sees the merged result without modification.
//
// IMPORTANT: this is the only path by which messages published from
// another process appear in this process's local cache. Locally-published
// messages are added to msgLog by Publish() at the time of submission.
func (b *hcsTopicBus) syncFromMirror(ref topic.TopicRef, sinceSeq uint64) (uint64, error) {
	msgs, highest, err := pollMirrorNodeSince(ref.Locator(), sinceSeq)
	if err != nil {
		return sinceSeq, err
	}
	if len(msgs) == 0 {
		return highest, nil
	}
	b.mu.Lock()
	loc := ref.Locator()
	b.msgLog[loc] = append(b.msgLog[loc], msgs...)
	b.mu.Unlock()
	return highest, nil
}

// waitForPayloadType polls the mirror node for new messages on the given
// topic until a message with payload {"type":"<payloadType>"} arrives or
// the timeout elapses. *lastSeq is updated each iteration so subsequent
// calls resume from where the previous one left off.
//
// On success returns the matching TopicMessage and the parsed payload bytes.
// On timeout returns errMirrorWaitTimeout wrapped with topic and type context.
//
// Newly fetched messages are merged into the bus.msgLog regardless of
// match — this is what populates the validator's observation cache.
func waitForPayloadType(
	bus *hcsTopicBus,
	ref topic.TopicRef,
	payloadType string,
	lastSeq *uint64,
	timeout time.Duration,
) (topic.TopicMessage, []byte, error) {
	deadline := time.Now().Add(timeout)
	scanned := 0 // index into bus.msgLog[loc] of next unscanned message
	loc := ref.Locator()

	for {
		newSeq, err := bus.syncFromMirror(ref, *lastSeq)
		if err != nil {
			return topic.TopicMessage{}, nil, fmt.Errorf("waitForPayloadType: mirror sync failed for topic %s: %w", loc, err)
		}
		*lastSeq = newSeq

		// Scan only the messages we haven't seen yet on this call.
		bus.mu.Lock()
		cached := bus.msgLog[loc]
		bus.mu.Unlock()

		for ; scanned < len(cached); scanned++ {
			msg := cached[scanned]
			payload := msg.Payload()
			var head struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(payload, &head); err != nil {
				continue // not a typed payment payload, skip
			}
			if head.Type == payloadType {
				return msg, payload, nil
			}
		}

		if time.Now().After(deadline) {
			return topic.TopicMessage{}, nil, fmt.Errorf("%w: type=%q topic=%s after %s",
				errMirrorWaitTimeout, payloadType, loc, timeout)
		}
		time.Sleep(mirrorPollInterval)
	}
}

// ─────────── bootstrap file ──────────────────────────────────────
//
// The bootstrap file is a small JSON document the seller writes after
// creating its HCS topics. The buyer reads the same file (transferred
// out-of-band, e.g. via SCP) to learn the seller's topic IDs and
// identity. This is the minimum side-channel needed for two split
// processes to coordinate.
//
// Format is intentionally tiny and stable. version=1.0.0 is required.
// Future fields should be added without breaking parses.

const bootstrapFormatVersion = "1.0.0"

type bootstrapFile struct {
	Version   string            `json:"version"`
	RequestID string            `json:"requestId"`
	Protocol  string            `json:"protocol"`
	Price     string            `json:"price"`
	Currency  string            `json:"currency"`
	CreatedAt string            `json:"createdAt"` // RFC3339, informational
	Seller    bootstrapIdentity `json:"seller"`
}

type bootstrapIdentity struct {
	AgentID    string          `json:"agentId"`
	EVMAddress string          `json:"evmAddress"`
	DID        string          `json:"did"`
	Topics     bootstrapTopics `json:"topics"`
}

type bootstrapTopics struct {
	StdIn  string `json:"stdIn"`
	StdOut string `json:"stdOut"`
	StdErr string `json:"stdErr"`
}

// writeBootstrap serializes the bootstrap file as pretty JSON and writes
// it to path with 0644 permissions. Fails fast if the file cannot be
// created or marshalled.
func writeBootstrap(path string, b bootstrapFile) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("writeBootstrap: marshal: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writeBootstrap: write %s: %w", path, err)
	}
	return nil
}

// readBootstrap loads and validates a bootstrap file. Returns clear errors
// for missing files, parse failures, and version mismatches so split-mode
// failures are obvious to operators.
func readBootstrap(path string) (bootstrapFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return bootstrapFile{}, fmt.Errorf("readBootstrap: %s: %w (did the seller process write it yet? scp it from the server?)", path, err)
	}
	var b bootstrapFile
	if err := json.Unmarshal(data, &b); err != nil {
		return bootstrapFile{}, fmt.Errorf("readBootstrap: parse %s: %w", path, err)
	}
	if b.Version != bootstrapFormatVersion {
		return bootstrapFile{}, fmt.Errorf("readBootstrap: incompatible bootstrap format version %q in %s (expected %q)",
			b.Version, path, bootstrapFormatVersion)
	}
	if b.Seller.Topics.StdIn == "" || b.Seller.EVMAddress == "" {
		return bootstrapFile{}, fmt.Errorf("readBootstrap: %s missing required seller.topics.stdIn or seller.evmAddress", path)
	}
	return b, nil
}

// ─────────── multiaddr filter ────────────────────────────────────
//
// Drop loopback and RFC1918 / unique-local addresses from the seller's
// libp2p multiaddr list before encrypting it into the connectionSetup.
// Two reasons:
//
//  1. The HCS 1024-byte message limit is tight. A 4-multiaddr ECIES-
//     encrypted ConnectionSetup wrapped in a TopicMessage envelope can
//     hit ~700-900 bytes. Filtering to just the public address keeps
//     the message comfortably under the limit.
//
//  2. The buyer dials the multiaddrs in order. Without filtering, the
//     dialer wastes ~1-2 seconds per failed private/loopback attempt
//     before reaching the public IP.
//
// Used ONLY in split-mode runSeller — runAll preserves its existing
// behavior of encrypting all host.Addrs().

func filterPublicMultiaddrStrings(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if isPublicMultiaddr(a) {
			out = append(out, a)
		}
	}
	return out
}

// filterNonLoopbackMultiaddrStrings is the runSeller fallback when no public
// addresses are found (e.g. single-machine split-mode test on a laptop with
// only LAN interfaces). It keeps everything except loopback so the buyer can
// still dial via 192.168.x.x / 10.x.x.x / fe80:: when both processes are on
// the same machine or LAN.
func filterNonLoopbackMultiaddrStrings(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if !isLoopbackMultiaddr(a) {
			out = append(out, a)
		}
	}
	return out
}

func isLoopbackMultiaddr(addr string) bool {
	parts := strings.SplitN(addr, "/", 4)
	if len(parts) < 4 {
		return false
	}
	switch parts[1] {
	case "ip4":
		return strings.HasPrefix(parts[2], "127.") || parts[2] == "0.0.0.0"
	case "ip6":
		return parts[2] == "::1" || parts[2] == "::"
	}
	return false
}

func isPublicMultiaddr(addr string) bool {
	// Relay-assisted multiaddrs carrying /p2p-circuit are always keepable:
	// the relay itself is public and the buyer needs the circuit hint to
	// dial a NAT'd seller. Autorelay-emitted circuit addrs begin with the
	// relay's public /ip4/.../p2p/<RELAY_ID>/p2p-circuit prefix, which the
	// /ip4 case below would also accept — this branch additionally covers
	// any bare /p2p-circuit form without an IP prefix.
	if strings.Contains(addr, "/p2p-circuit") {
		return true
	}
	// Match the IP component of common multiaddr forms:
	//   /ip4/A.B.C.D/...
	//   /ip6/X:X:.../...
	parts := strings.SplitN(addr, "/", 4)
	if len(parts) < 4 {
		return false
	}
	switch parts[1] {
	case "ip4":
		return isPublicIPv4(parts[2])
	case "ip6":
		return isPublicIPv6(parts[2])
	default:
		return false
	}
}

func isPublicIPv4(ip string) bool {
	if strings.HasPrefix(ip, "127.") || ip == "0.0.0.0" {
		return false
	}
	if strings.HasPrefix(ip, "10.") {
		return false
	}
	if strings.HasPrefix(ip, "192.168.") {
		return false
	}
	if strings.HasPrefix(ip, "172.") {
		// 172.16.0.0/12 → second octet 16..31
		dot := strings.IndexByte(ip[4:], '.')
		if dot < 0 {
			return false
		}
		oct, err := strconv.Atoi(ip[4 : 4+dot])
		if err == nil && oct >= 16 && oct <= 31 {
			return false
		}
	}
	if strings.HasPrefix(ip, "169.254.") { // link-local
		return false
	}
	if strings.HasPrefix(ip, "100.64.") { // CGNAT (rough match for /10)
		return false
	}
	return true
}

func isPublicIPv6(ip string) bool {
	low := strings.ToLower(ip)
	if low == "::1" || low == "::" {
		return false
	}
	if strings.HasPrefix(low, "fe80:") { // link-local
		return false
	}
	if strings.HasPrefix(low, "fc") || strings.HasPrefix(low, "fd") { // unique local fc00::/7
		return false
	}
	return true
}

// ═══════════════════════════════════════════════════════════════════
// Split-mode orchestration (--role=seller|buyer)
// ═══════════════════════════════════════════════════════════════════
//
// Split mode allows buyer and seller to run as separate processes on
// separate machines, coordinating via:
//
//   1. A JSON bootstrap file written by the seller and read by the
//      buyer. This carries the seller's HCS topic IDs and identity.
//      Transport is out-of-band (SCP, shared filesystem, etc.).
//
//   2. Hedera Consensus Service topics for payment protocol messages.
//      Since HCSClient.SubscribeTopic is not implemented, split mode
//      polls the Hedera testnet mirror node REST API to fetch new
//      messages (see pollMirrorNodeSince below). All polled messages
//      are merged into the local hcsTopicBus.msgLog so the validator's
//      existing getMessages() observation loop works unchanged.
//
//   3. libp2p QUIC for the actual file delivery (same as --role=all).
//
// Split mode requires --mode=testnet. The in-memory mock bus cannot
// cross process boundaries.
//
// The validator runs colocated with the seller process (--role=seller)
// because the seller's local msgLog already contains seller-published
// messages via the normal Publish path; the mirror poller fills in the
// buyer-published half. --role=buyer skips the validator phase.

// runSeller is the split-mode seller orchestration. It is invoked when
// --role=seller and creates the Seller + Validator agents (no Buyer).
// Side-channel coordination with the buyer process happens through:
//
//   - bootstrapPath: a JSON file written here, transferred out-of-band
//     (SCP), and read by the buyer's runBuyer.
//   - HCS topics + mirror node polling: payment messages from the buyer
//     arrive on Seller.stdIn via Hedera consensus, fetched here through
//     pollMirrorNodeSince + waitForPayloadType.
//
// The validator runs in this process. Locally-published messages
// (serviceResponse, connectionSetup, invoice) are already in
// hcsTopicBus.msgLog via Publish; mirror-node polling adds the buyer's
// half (serviceRequest, escrowCreated, invoiceAck), giving the existing
// validator phase the full 6-message observation set unchanged.
func runSeller(jpegPath, price, listenAddr, bootstrapPath string) {
	if jpegPath == "" {
		exe, _ := os.Executable()
		jpegPath = filepath.Join(filepath.Dir(exe), "testdata", "photo.jpg")
		if _, err := os.Stat(jpegPath); err != nil {
			jpegPath = filepath.Join("cmd", "buyer-seller-demo", "testdata", "photo.jpg")
			if _, err := os.Stat(jpegPath); err != nil {
				log.Fatalf("JPEG file not found. Use --jpeg to specify path.")
			}
		}
	}

	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("  Neuron SDK -- Buyer-Seller JPEG Demo (split mode: SELLER)")
	fmt.Println("================================================================")
	fmt.Printf("  Bootstrap file: %s\n", bootstrapPath)
	fmt.Printf("  Listen address: %s\n", listenAddr)
	fmt.Printf("  Price: %s tinybar\n", price)

	demoProtocol := "/neuron/jpeg-transfer/1.0.0"
	requestID := "req-001"

	// ── Phase 1: SETUP ──────────────────────────────────────────
	phase("1: SETUP")
	real("Initializing Hedera testnet client...")
	client, operatorID, err := topic.NewTestnetClientFromEnv()
	if err != nil {
		log.Fatalf("FATAL: %v\n  In split mode set %s and %s before running --role=seller --mode=testnet",
			err, topic.HederaOperatorEnvAccountID, topic.HederaOperatorEnvPrivateKey)
	}
	info("operator = %s", operatorID.String())
	info("network  = Hedera Testnet (HCS publish + mirror node poll)")
	info("HashScan: https://hashscan.io/testnet/account/%s", operatorID.String())

	hcsClient := topic.NewRealHCSClient(client)
	hcsAdapter := topic.NewHCSAdapter(hcsClient)
	bus := newHCSTopicBus(hcsAdapter)

	real("Creating 6 HCS topics on Hedera testnet (Seller + Validator, no Buyer in this process)")
	seller := createAgent("Seller", bus)
	validator := createAgent("Validator", bus)

	for _, a := range []*agent{seller, validator} {
		pub := a.key.PublicKey()
		pid, _ := pub.PeerID()
		real("secp256k1 identity created: %s", a.name)
		info("EVM     = %s", a.evmAddress())
		info("PeerID  = %s", pid.String())
		if verbose {
			detail("DID:key = %s", pub.DIDKey())
		}
	}

	mockReg := newMockRegistry()
	real("Topic bus: Hedera Consensus Service (REAL HCS publish + mirror polling)")
	mock("Registry: in-memory (local to this process)")

	// ── Phase 2: REGISTER ──────────────────────────────────────
	phase("2: REGISTER")
	sellerCommerceJSON := fmt.Sprintf(`{"type":"neuron-commerce","name":"jpeg-delivery","version":"1.0.0","delivery":{"mode":"p2p"},"settlement":{"binding":"memory"},"pricing":{"amount":"%s","currency":"tinybar","unit":"per-file","interval":"0"}}`, price)
	sellerURI := fmt.Sprintf(`{"services":[%s]}`, sellerCommerceJSON)
	seller.agentID = mockReg.register(seller.evmAddress(), sellerURI)
	mock("Seller registered in local mock registry")
	info("agentId  = %s", seller.agentID)
	info("services = neuron-commerce (jpeg-delivery, p2p, %s tinybar)", price)
	hashScan("agentId", seller.agentID)

	validatorSvc, _ := validation.NewNeuronValidatorService("validation", "1.0.0",
		[]string{"008-payment", "009-delivery"}, "topic")
	validatorSvcJSON, _ := json.Marshal(validatorSvc)
	validatorURI := fmt.Sprintf(`{"services":[%s]}`, string(validatorSvcJSON))
	validator.agentID = mockReg.register(validator.evmAddress(), validatorURI)
	mock("Validator registered in local mock registry")
	info("agentId  = %s", validator.agentID)

	// ── Phase 2.5: BOOTSTRAP ──────────────────────────────────
	phase("2.5: BOOTSTRAP")
	sellerPub := seller.key.PublicKey()
	bf := bootstrapFile{
		Version:   bootstrapFormatVersion,
		RequestID: requestID,
		Protocol:  demoProtocol,
		Price:     price,
		Currency:  "tinybar",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Seller: bootstrapIdentity{
			AgentID:    seller.agentID,
			EVMAddress: seller.evmAddress(),
			DID:        sellerPub.DIDKey(),
			Topics: bootstrapTopics{
				StdIn:  seller.stdIn.Locator(),
				StdOut: seller.stdOut.Locator(),
				StdErr: seller.stdErr.Locator(),
			},
		},
	}
	must(writeBootstrap(bootstrapPath, bf))
	real("Bootstrap file written: %s", bootstrapPath)
	info("seller.stdIn  = %s", bf.Seller.Topics.StdIn)
	info("seller.stdOut = %s", bf.Seller.Topics.StdOut)
	info("Next step: scp %s to the buyer machine, then run with --role=buyer --bootstrap=<localpath>", bootstrapPath)

	sellerSM := payment.NewAgreementStateMachine(requestID)

	// ── Phase 4: NEGOTIATE (wait for buyer) ────────────────────
	phase("4: NEGOTIATE (wait for buyer)")
	var sellerStdInSeq uint64 = 0
	real("Waiting for serviceRequest on seller.stdIn (HCS mirror poll, ~3-7s consensus lag)")
	srMsg, srPayload, err := waitForPayloadType(bus, seller.stdIn, "serviceRequest", &sellerStdInSeq, mirrorWaitDefaultTimeout)
	must(err)
	if err := topic.ValidateTopicMessage(srMsg); err != nil {
		log.Fatalf("FATAL: serviceRequest signature invalid: %v", err)
	}
	var svcReq payment.ServiceRequest
	must(json.Unmarshal(srPayload, &svcReq))
	real("[HCS mirror] Received serviceRequest from %s", shortAddr(srMsg.SenderAddress()))
	info("requestId   = %s", svcReq.RequestID)
	info("buyerStdIn  = %s", svcReq.BuyerStdIn)
	info("source      = HCS mirror node (live read, signature VALID)")

	// Recover buyer's pubkey from the serviceRequest signature so we can
	// ECIES-encrypt connectionSetup later. The buyer signs with its
	// NeuronPrivateKey and uses ToBlockchainKey() to get the matching
	// libp2p/ECIES *ecdsa.PrivateKey.
	buyerSig, err := keylib.SignatureFromBytes(srMsg.Signature())
	must(err)
	signingInput := topic.TopicMessageSigningInput(srMsg.Timestamp(), srMsg.SequenceNumber(), srMsg.Payload())
	buyerNeuronPub, err := buyerSig.RecoverPublicKey(signingInput)
	must(err)
	buyerECDSAPub, err := buyerNeuronPub.ToBlockchainKey()
	must(err)
	info("recovered buyer ECDSA pubkey for ECIES (%d bytes uncompressed)", len(buyerNeuronPub.Uncompressed()))

	buyerStdIn, err := topic.NewTopicRef(topic.BackendHCS, svcReq.BuyerStdIn)
	must(err)

	transition("  Seller", sellerSM, payment.EventServiceRequest)

	svcResp := payment.ServiceResponse{
		Type: "serviceResponse", Version: "1.0.0", RequestID: requestID, Action: "accept",
	}
	must(seller.publishPayload(bus, buyerStdIn, svcResp))
	real("Seller -> Buyer: serviceResponse (action=accept) [HCS publish, WaitForConsensus]")
	info("sender=%s -> Buyer.stdIn (%s)", shortAddr(seller.evmAddress()), buyerStdIn.Locator())

	transition("  Seller", sellerSM, payment.EventAccept)

	respJSON, _ := json.Marshal(svcResp)
	agreementHash := payment.ComputeAgreementHash(respJSON)
	agreementHashHex := fmt.Sprintf("0x%x", agreementHash[:])
	real("agreementHash = %s (keccak256 of canonical serviceResponse)", shortHash(agreementHashHex))
	detail("full agreementHash = %s", agreementHashHex)

	// ── Phase 5: FUND (wait for buyer) ──────────────────────────
	phase("5: FUND (wait for buyer)")
	real("Waiting for escrowCreated on seller.stdIn (HCS mirror poll)")
	ecMsg, ecPayload, err := waitForPayloadType(bus, seller.stdIn, "escrowCreated", &sellerStdInSeq, mirrorWaitDefaultTimeout)
	must(err)
	if err := topic.ValidateTopicMessage(ecMsg); err != nil {
		log.Fatalf("FATAL: escrowCreated signature invalid: %v", err)
	}
	var escrowCreated payment.EscrowCreated
	must(json.Unmarshal(ecPayload, &escrowCreated))
	real("[HCS mirror] Received escrowCreated")
	info("escrowRef     = %s", escrowCreated.EscrowRef)
	info("depositAmount = %s tinybar", escrowCreated.DepositAmount)
	info("source        = HCS mirror node (live read, signature VALID)")
	transition("  Seller", sellerSM, payment.EventEscrowCreated)

	// ── Phase 6: CONNECT ────────────────────────────────────────
	phase("6: CONNECT")
	sellerECDSA, err := seller.key.ToBlockchainKey()
	must(err)
	sellerHost, err := delivery.NewLibp2pHost(sellerECDSA, listenAddr, neuronHostOptions()...)
	must(err)
	defer sellerHost.Close()
	sellerAdapter := delivery.NewLibp2pAdapter(sellerHost)
	real("Seller libp2p host started (key derived from seller NeuronPrivateKey)")
	for _, addr := range sellerHost.Addrs() {
		info("listening: %s/p2p/%s", addr, sellerHost.ID())
	}
	info("protocol: %s", demoProtocol)

	awaitRelayReservation(sellerHost)

	// Filter out loopback + RFC1918 + IPv6 unique-local before encrypting.
	// Keeps connectionSetup small enough for HCS 1024-byte limit and makes
	// the buyer dial the public address first.
	allAddrStrings := make([]string, 0, len(sellerHost.Addrs()))
	for _, a := range sellerHost.Addrs() {
		allAddrStrings = append(allAddrStrings, a.String())
	}
	publicAddrs := filterPublicMultiaddrStrings(allAddrStrings)
	if len(publicAddrs) == 0 {
		// Fall back to non-loopback (LAN / private). This is correct for
		// single-machine split tests and LAN deployments where neither side
		// has a routable public IP. On a cloud server with a 1:1 NAT public
		// IP this branch is not taken because filterPublicMultiaddrStrings
		// will already include the public address.
		info("no public multiaddrs found; falling back to non-loopback (LAN/private)")
		publicAddrs = filterNonLoopbackMultiaddrStrings(allAddrStrings)
		if len(publicAddrs) == 0 {
			log.Fatalf("FATAL: no reachable multiaddrs (only loopback).\n  All addrs: %v\n  Use --listen on an externally-reachable interface.", allAddrStrings)
		}
		real("Filtered multiaddrs: %d/%d non-loopback (LAN fallback, single-machine or LAN test)", len(publicAddrs), len(allAddrStrings))
	} else {
		real("Filtered multiaddrs: %d/%d are public (drops loopback + RFC1918)", len(publicAddrs), len(allAddrStrings))
	}
	for _, a := range publicAddrs {
		info("public: %s", a)
	}

	encrypted, err := delivery.EncryptMultiaddrs(publicAddrs, buyerECDSAPub)
	must(err)
	connSetup := &payment.ConnectionSetup{
		Type:                "connectionSetup",
		Version:             "1.0.0",
		RequestID:           requestID,
		PeerID:              sellerHost.ID().String(),
		EncryptedMultiaddrs: encrypted,
		Protocol:            demoProtocol,
	}
	must(seller.publishPayload(bus, buyerStdIn, connSetup))
	real("Seller -> Buyer: connectionSetup [HCS publish, %d filtered multiaddrs]", len(publicAddrs))
	info("ECIES encryption: secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM")
	info("encryptedMultiaddrs = %s", shortHash(connSetup.EncryptedMultiaddrs))
	detail("full encryptedMultiaddrs = %s", connSetup.EncryptedMultiaddrs)

	sellerReadyCh := make(chan *delivery.DeliveryChannel, 1)
	sellerAdapter.HandleIncoming(protocol.ID(demoProtocol), func(ch *delivery.DeliveryChannel) {
		sellerReadyCh <- ch
	})

	var sellerChannel *delivery.DeliveryChannel
	select {
	case sellerChannel = <-sellerReadyCh:
	case <-time.After(120 * time.Second):
		log.Fatal("FATAL: timeout waiting for buyer libp2p connection (120s)")
	}
	real("Buyer connected via libp2p")
	info("transport = %s", sellerChannel.Transport)
	info("remote PeerID = %s", sellerChannel.PeerID)
	transition("  Seller", sellerSM, payment.EventDeliveryStarted)

	// ── Phase 7: DELIVER ────────────────────────────────────────
	phase("7: DELIVER")
	sendResult, err := delivery.SendFile(sellerAdapter, sellerChannel, jpegPath)
	must(err)
	real("Seller sent file via libp2p framing protocol (FR-D22)")
	info("file       = %s", sendResult.Filename)
	info("size       = %d bytes", sendResult.Size)
	info("frames     = %d data frame(s)", sendResult.FrameCount)
	info("SHA256     = %s", shortHash(sendResult.SHA256))
	detail("full SHA256 = %s", sendResult.SHA256)

	// ── Phase 8: SETTLE ─────────────────────────────────────────
	phase("8: SETTLE")
	deliveryProof := []byte(fmt.Sprintf(`{"requestId":"%s","file":"%s","sha256":"%s","bytes":%d}`,
		requestID, sendResult.Filename, sendResult.SHA256, sendResult.Size))
	evidenceHash := validation.ComputeEvidenceHash(deliveryProof)
	evidenceHashHex := validation.FormatEvidenceHash(evidenceHash)

	// In split mode the seller does NOT touch escrow — the buyer process
	// owns the mock escrow state. Seller publishes the invoice and waits
	// for the ack message, then trusts the buyer to release locally.
	releaseRefLocator := "split-release-" + requestID
	invoice := payment.Invoice{
		Type: "invoice", Version: "1.0.0", RequestID: requestID,
		ReleaseRequestRef: releaseRefLocator, EscrowRef: escrowCreated.EscrowRef,
		Amount: price, Currency: "tinybar",
	}
	must(seller.publishPayload(bus, buyerStdIn, invoice))
	real("Seller -> Buyer: invoice [HCS publish]")
	info("amount       = %s tinybar", price)
	info("evidenceHash = %s", shortHash(evidenceHashHex))
	detail("full evidenceHash = %s", evidenceHashHex)

	transition("  Seller", sellerSM, payment.EventInvoice)

	real("Waiting for invoiceAck on seller.stdIn (HCS mirror poll)")
	iaMsg, iaPayload, err := waitForPayloadType(bus, seller.stdIn, "invoiceAck", &sellerStdInSeq, mirrorWaitDefaultTimeout)
	must(err)
	if err := topic.ValidateTopicMessage(iaMsg); err != nil {
		log.Fatalf("FATAL: invoiceAck signature invalid: %v", err)
	}
	var invoiceAck payment.InvoiceAck
	must(json.Unmarshal(iaPayload, &invoiceAck))
	real("[HCS mirror] Received invoiceAck (action=%s)", invoiceAck.Action)
	transition("  Seller", sellerSM, payment.EventInvoiceApproved)
	transition("  Seller", sellerSM, payment.EventComplete)

	// ── Phase 9: VALIDATE ────────────────────────────────────────
	phase("9: VALIDATE")
	// At this point seller's local msgLog contains:
	//   - seller.stdIn:  3 messages from buyer (mirror-fetched)
	//   - buyer.stdIn:   3 messages from seller (locally published)
	// Same shape as runAll's local cache, so the validator code below is
	// IDENTICAL to runAll's Phase 9 — we reuse it verbatim.
	sellerInMsgs := bus.getMessages(seller.stdIn)
	buyerInMsgs := bus.getMessages(buyerStdIn)
	allMsgs := append(sellerInMsgs, buyerInMsgs...)
	real("Validator observing protocol messages")
	info("Seller.stdIn: %d messages (buyer -> seller, source: HCS mirror)", len(sellerInMsgs))
	info("Buyer.stdIn:  %d messages (seller -> buyer, source: local publish cache)", len(buyerInMsgs))
	info("total:        %d messages", len(allMsgs))

	real("Verifying ECDSA signatures (ecrecover + sender match)")
	for i, msg := range allMsgs {
		if err := topic.ValidateTopicMessage(msg); err != nil {
			log.Fatalf("FATAL: validator found invalid signature on message %d: %v", i, err)
		}
	}
	info("result: all %d signatures VALID", len(allMsgs))

	type observedMsg struct {
		hash    string
		msgType string
		sender  string
	}
	var observations []observedMsg
	for _, msg := range allMsgs {
		payloadHash := ethcrypto.Keccak256(msg.Payload())
		var parsed struct {
			Type string `json:"type"`
		}
		json.Unmarshal(msg.Payload(), &parsed)
		obs := observedMsg{
			hash:    "0x" + hex.EncodeToString(payloadHash),
			msgType: parsed.Type,
			sender:  msg.SenderAddress(),
		}
		observations = append(observations, obs)
		detail("  observed: type=%-20s sender=%s hash=%s", obs.msgType, shortAddr(obs.sender), shortHash(obs.hash))
	}

	type paymentEvidenceDoc struct {
		Domain          string   `json:"domain"`
		ObservedHashes  []string `json:"observedHashes"`
		ObservedTypes   []string `json:"observedTypes"`
		RequestID       string   `json:"requestId"`
		SellerAddress   string   `json:"sellerAddress"`
		SignaturesValid int      `json:"signaturesValid"`
	}
	var paymentHashes, paymentTypes []string
	for _, obs := range observations {
		if obs.msgType == "serviceRequest" || obs.msgType == "serviceResponse" ||
			obs.msgType == "escrowCreated" || obs.msgType == "invoice" || obs.msgType == "invoiceAck" {
			paymentHashes = append(paymentHashes, obs.hash)
			paymentTypes = append(paymentTypes, obs.msgType)
		}
	}
	paymentDoc, _ := json.Marshal(paymentEvidenceDoc{
		Domain: "008-payment", ObservedHashes: paymentHashes, ObservedTypes: paymentTypes,
		RequestID: requestID, SellerAddress: seller.evmAddress(), SignaturesValid: len(paymentHashes),
	})
	paymentEvidenceHash := validation.ComputeEvidenceHash(paymentDoc)

	type deliveryEvidenceDoc struct {
		Domain         string   `json:"domain"`
		ObservedHashes []string `json:"observedHashes"`
		ObservedTypes  []string `json:"observedTypes"`
		FileSent       string   `json:"fileSent"`
		FileSHA256     string   `json:"fileSHA256"`
		FileSize       int64    `json:"fileSize"`
	}
	var deliveryHashes, deliveryTypes []string
	for _, obs := range observations {
		if obs.msgType == "connectionSetup" {
			deliveryHashes = append(deliveryHashes, obs.hash)
			deliveryTypes = append(deliveryTypes, obs.msgType)
		}
	}
	deliveryDoc, _ := json.Marshal(deliveryEvidenceDoc{
		Domain: "009-delivery", ObservedHashes: deliveryHashes, ObservedTypes: deliveryTypes,
		FileSent: sendResult.Filename, FileSHA256: sendResult.SHA256, FileSize: sendResult.Size,
	})
	deliveryEvidenceHash := validation.ComputeEvidenceHash(deliveryDoc)

	env1, err := validation.NewEvidenceEnvelope(
		validator.agentID, seller.agentID, "008-payment",
		validation.VerdictCompliant,
		validation.FormatEvidenceHash(paymentEvidenceHash),
		"mem://evidence/payment-001",
	)
	must(err)
	_, err = validation.PublishEvidence(env1, &validator.key, validator.stdOut, bus,
		uint64(time.Now().UnixNano()), validator.nextSeq())
	must(err)

	env2, err := validation.NewEvidenceEnvelope(
		validator.agentID, seller.agentID, "009-delivery",
		validation.VerdictCompliant,
		validation.FormatEvidenceHash(deliveryEvidenceHash),
		"mem://evidence/delivery-001",
	)
	must(err)
	_, err = validation.PublishEvidence(env2, &validator.key, validator.stdOut, bus,
		uint64(time.Now().UnixNano()), validator.nextSeq())
	must(err)

	real("Evidence envelopes published to Validator.stdOut")
	info("Verdict #1: 008-payment  -> COMPLIANT (%d negotiation messages)", len(paymentHashes))
	info("  evidenceHash = %s", shortHash(validation.FormatEvidenceHash(paymentEvidenceHash)))
	info("Verdict #2: 009-delivery -> COMPLIANT (%d delivery messages)", len(deliveryHashes))
	info("  evidenceHash = %s", shortHash(validation.FormatEvidenceHash(deliveryEvidenceHash)))

	paymentChainOK := validation.VerifyEvidenceHash(paymentDoc, env1)
	deliveryChainOK := validation.VerifyEvidenceHash(deliveryDoc, env2)
	real("Evidence hash chain verification:")
	info("payment  chain: keccak256(doc) == envelope.evidenceHash ? %s", boolOK(paymentChainOK))
	info("delivery chain: keccak256(doc) == envelope.evidenceHash ? %s", boolOK(deliveryChainOK))

	_ = sellerAdapter.Disconnect(sellerChannel)

	// ── SUMMARY ─────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("  SELLER SUMMARY (split mode)")
	fmt.Println("================================================================")
	fmt.Println()
	fmt.Println("  Mode:       testnet split (--role=seller)")
	fmt.Printf("  Agents:     2 (Seller=%s, Validator=%s)\n", seller.agentID, validator.agentID)
	fmt.Println("  Buyer:      separate process (--role=buyer)")
	fmt.Println()
	fmt.Println("  Phase Results:")
	fmt.Println("    1  SETUP        OK   2 identities created             [REAL crypto]")
	fmt.Println("    2  REGISTER     OK   Seller + Validator registered    [MOCK registry]")
	fmt.Println("    2.5 BOOTSTRAP   OK   Bootstrap file written           [SCP transport]")
	fmt.Println("    4  NEGOTIATE    OK   serviceRequest received via HCS  [HCS mirror poll]")
	fmt.Println("    5  FUND         OK   escrowCreated received via HCS   [HCS mirror poll]")
	fmt.Printf("    6  CONNECT      OK   %s, ECIES encrypted              [REAL libp2p + ECIES]\n", sellerChannel.Transport)
	fmt.Printf("    7  DELIVER      OK   %s %d bytes                       [REAL libp2p + SHA256]\n", sendResult.Filename, sendResult.Size)
	fmt.Println("    8  SETTLE       OK   invoiceAck received via HCS      [HCS mirror poll, no escrow on seller]")
	fmt.Printf("    9  VALIDATE     OK   %d sigs, 2 COMPLIANT verdicts    [REAL validation]\n", len(allMsgs))
	fmt.Println()
	fmt.Println("  Explorer:")
	fmt.Printf("    Seller.stdIn:    https://hashscan.io/testnet/topic/%s\n", seller.stdIn.Locator())
	fmt.Printf("    Buyer.stdIn:     https://hashscan.io/testnet/topic/%s\n", buyerStdIn.Locator())
	fmt.Printf("    Validator.stdOut: https://hashscan.io/testnet/topic/%s\n", validator.stdOut.Locator())
}

// runBuyer is the split-mode buyer orchestration. It is invoked when
// --role=buyer and creates only the Buyer agent (Seller + Validator
// run in the seller process).
//
// Coordination:
//   - bootstrapPath: JSON file written by the seller, transferred via SCP
//   - HCS topics + mirror node polling: seller's responses arrive on
//     buyer.stdIn via Hedera consensus, fetched via waitForPayloadType
//
// Buyer owns the mock escrow in split mode (since the seller process
// doesn't have it). Validator phase is skipped — the seller process
// runs the validator using its colocated msgLog.
func runBuyer(bootstrapPath string) {
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("  Neuron SDK -- Buyer-Seller JPEG Demo (split mode: BUYER)")
	fmt.Println("================================================================")
	fmt.Printf("  Bootstrap file: %s\n", bootstrapPath)

	bf, err := readBootstrap(bootstrapPath)
	must(err)
	fmt.Printf("  Request ID:     %s\n", bf.RequestID)
	fmt.Printf("  Protocol:       %s\n", bf.Protocol)
	fmt.Printf("  Price:          %s %s\n", bf.Price, bf.Currency)
	fmt.Printf("  Seller agentId: %s (%s)\n", bf.Seller.AgentID, shortAddr(bf.Seller.EVMAddress))

	requestID := bf.RequestID
	price := bf.Price

	// ── Phase 1: SETUP ──────────────────────────────────────────
	phase("1: SETUP")
	real("Initializing Hedera testnet client...")
	client, operatorID, err := topic.NewTestnetClientFromEnv()
	if err != nil {
		log.Fatalf("FATAL: %v\n  In split mode set %s and %s before running --role=buyer --mode=testnet",
			err, topic.HederaOperatorEnvAccountID, topic.HederaOperatorEnvPrivateKey)
	}
	info("operator = %s", operatorID.String())
	info("network  = Hedera Testnet (HCS publish + mirror node poll)")

	hcsClient := topic.NewRealHCSClient(client)
	hcsAdapter := topic.NewHCSAdapter(hcsClient)
	bus := newHCSTopicBus(hcsAdapter)

	real("Creating 3 HCS topics on Hedera testnet (Buyer only)")
	buyer := createAgent("Buyer", bus)

	pub := buyer.key.PublicKey()
	pid, _ := pub.PeerID()
	real("secp256k1 identity created: Buyer")
	info("EVM     = %s", buyer.evmAddress())
	info("PeerID  = %s", pid.String())
	if verbose {
		detail("DID:key = %s", pub.DIDKey())
	}

	// Reconstruct seller topic refs from bootstrap.
	sellerStdIn, err := topic.NewTopicRef(topic.BackendHCS, bf.Seller.Topics.StdIn)
	must(err)

	mockReg := newMockRegistry()
	sellerCommerceJSON := fmt.Sprintf(`{"type":"neuron-commerce","name":"jpeg-delivery","version":"1.0.0","delivery":{"mode":"p2p"},"settlement":{"binding":"memory"},"pricing":{"amount":"%s","currency":"tinybar","unit":"per-file","interval":"0"}}`, price)
	sellerURI := fmt.Sprintf(`{"services":[%s]}`, sellerCommerceJSON)
	mockReg.register(bf.Seller.EVMAddress, sellerURI)
	buyer.agentID = mockReg.register(buyer.evmAddress(), `{"services":[]}`)
	mock("Seller + Buyer registered in local mock registry (seller info from bootstrap)")
	info("buyer agentId  = %s", buyer.agentID)
	info("seller agentId = %s (from bootstrap)", bf.Seller.AgentID)

	escrow := payment.NewMemoryEscrow()

	// ── Phase 4: NEGOTIATE ───────────────────────────────────────
	phase("4: NEGOTIATE")
	buyerSM := payment.NewAgreementStateMachine(requestID)

	svcReq := payment.ServiceRequest{
		Type: "serviceRequest", Version: "1.0.0", RequestID: requestID,
		ServiceRef: "jpeg-delivery", SettlementBinding: "memory",
		ProposedAmount: price, ProposedCurrency: "tinybar", ProposedInterval: "0",
		BuyerStdIn: buyer.stdIn.Locator(),
	}
	must(buyer.publishPayload(bus, sellerStdIn, svcReq))
	real("Buyer -> Seller: serviceRequest [HCS publish, WaitForConsensus]")
	info("requestId  = %s", requestID)
	info("buyerStdIn = %s (so seller can reply)", buyer.stdIn.Locator())
	info("target     = seller.stdIn (%s, from bootstrap)", sellerStdIn.Locator())
	transition("  Buyer ", buyerSM, payment.EventServiceRequest)

	var buyerStdInSeq uint64 = 0
	real("Waiting for serviceResponse on buyer.stdIn (HCS mirror poll, ~3-7s lag)")
	srMsg, srPayload, err := waitForPayloadType(bus, buyer.stdIn, "serviceResponse", &buyerStdInSeq, mirrorWaitDefaultTimeout)
	must(err)
	if err := topic.ValidateTopicMessage(srMsg); err != nil {
		log.Fatalf("FATAL: serviceResponse signature invalid: %v", err)
	}
	if !strings.EqualFold(srMsg.SenderAddress(), bf.Seller.EVMAddress) {
		log.Fatalf("FATAL: serviceResponse sender mismatch: got %s, expected %s (from bootstrap)",
			srMsg.SenderAddress(), bf.Seller.EVMAddress)
	}
	var svcResp payment.ServiceResponse
	must(json.Unmarshal(srPayload, &svcResp))
	if svcResp.Action != "accept" {
		log.Fatalf("FATAL: seller did not accept (action=%s)", svcResp.Action)
	}
	real("[HCS mirror] Received serviceResponse (action=accept) from %s", shortAddr(srMsg.SenderAddress()))
	info("source = HCS mirror node (live read, signature VALID, sender == bootstrap.seller)")
	transition("  Buyer ", buyerSM, payment.EventAccept)

	respJSON, _ := json.Marshal(svcResp)
	agreementHash := payment.ComputeAgreementHash(respJSON)
	agreementHashHex := fmt.Sprintf("0x%x", agreementHash[:])
	real("agreementHash = %s (computed locally; must match seller's)", shortHash(agreementHashHex))
	detail("full agreementHash = %s", agreementHashHex)

	// ── Phase 5: FUND ───────────────────────────────────────────
	phase("5: FUND")
	ctx := context.Background()
	escrowRef, err := escrow.CreateEscrow(ctx, buyer.evmAddress(), bf.Seller.EVMAddress, nil,
		"tinybar", 1, agreementHash, uint64(time.Now().Unix())+3600)
	must(err)
	mock("Escrow created in-memory (BUYER side only in split mode)")
	info("ref=%s  buyer=%s  seller=%s", escrowRef.Locator, shortAddr(buyer.evmAddress()), shortAddr(bf.Seller.EVMAddress))

	depositResult, err := escrow.Deposit(ctx, escrowRef, price)
	must(err)
	mock("Buyer deposited %s tinybar (balance=%s)", price, depositResult.NewBalance)

	transition("  Buyer ", buyerSM, payment.EventEscrowCreated)

	escrowCreated := payment.EscrowCreated{
		Type: "escrowCreated", Version: "1.0.0", RequestID: requestID,
		EscrowRef: escrowRef.Locator, DepositAmount: price, DepositCurrency: "tinybar",
	}
	must(buyer.publishPayload(bus, sellerStdIn, escrowCreated))
	real("Buyer -> Seller: escrowCreated [HCS publish]")

	// ── Phase 6: CONNECT ────────────────────────────────────────
	phase("6: CONNECT")
	buyerECDSA, err := buyer.key.ToBlockchainKey()
	must(err)
	buyerHost, err := delivery.NewLibp2pHost(buyerECDSA, "/ip4/0.0.0.0/udp/0/quic-v1", neuronHostOptions()...)
	must(err)
	defer buyerHost.Close()
	buyerAdapter := delivery.NewLibp2pAdapter(buyerHost)
	real("Buyer libp2p host started (key derived from buyer NeuronPrivateKey)")
	info("buyer PeerID = %s", buyerHost.ID())

	real("Waiting for connectionSetup on buyer.stdIn (HCS mirror poll)")
	csMsg, csPayload, err := waitForPayloadType(bus, buyer.stdIn, "connectionSetup", &buyerStdInSeq, mirrorWaitDefaultTimeout)
	must(err)
	if err := topic.ValidateTopicMessage(csMsg); err != nil {
		log.Fatalf("FATAL: connectionSetup signature invalid: %v", err)
	}
	var connSetup payment.ConnectionSetup
	must(json.Unmarshal(csPayload, &connSetup))
	if connSetup.Protocol != bf.Protocol {
		log.Fatalf("FATAL: connectionSetup protocol mismatch: got %q, bootstrap declared %q",
			connSetup.Protocol, bf.Protocol)
	}
	real("[HCS mirror] Received connectionSetup")
	info("seller PeerID       = %s", connSetup.PeerID)
	info("protocol            = %s (matches bootstrap)", connSetup.Protocol)
	info("encryptedMultiaddrs = %s", shortHash(connSetup.EncryptedMultiaddrs))

	buyerChannel, err := delivery.ConnectFromSetup(buyerAdapter, &connSetup, buyerECDSA)
	must(err)
	_, err = buyerAdapter.Send(buyerChannel, []byte{})
	must(err)
	real("Buyer decrypted connectionSetup and connected via libp2p")
	info("transport     = %s", buyerChannel.Transport)
	info("remote PeerID = %s", buyerChannel.PeerID)
	transition("  Buyer ", buyerSM, payment.EventDeliveryStarted)

	// ── Phase 7: DELIVER ─────────────────────────────────────────
	phase("7: DELIVER")
	outputDir, _ := os.MkdirTemp("", "neuron-demo-buyer-*")
	defer os.RemoveAll(outputDir)

	recvResult, err := delivery.ReceiveFile(buyerAdapter, buyerChannel, outputDir)
	must(err)
	real("Buyer received file via libp2p framing protocol")
	info("file       = %s", recvResult.Filename)
	info("size       = %d bytes", recvResult.Size)
	info("frames     = %d data frame(s)", recvResult.FrameCount)
	info("SHA256     = %s", shortHash(recvResult.SHA256))
	detail("full SHA256 = %s", recvResult.SHA256)

	// ── Phase 8: SETTLE ──────────────────────────────────────────
	phase("8: SETTLE")
	deliveryProof := []byte(fmt.Sprintf(`{"requestId":"%s","file":"%s","sha256":"%s","bytes":%d}`,
		requestID, recvResult.Filename, recvResult.SHA256, recvResult.Size))
	evidenceHash := validation.ComputeEvidenceHash(deliveryProof)

	// Buyer waits for invoice from seller, then approves locally and
	// publishes invoiceAck. Buyer holds the mock escrow in split mode
	// and releases it locally after the ack.
	real("Waiting for invoice on buyer.stdIn (HCS mirror poll)")
	invMsg, invPayload, err := waitForPayloadType(bus, buyer.stdIn, "invoice", &buyerStdInSeq, mirrorWaitDefaultTimeout)
	must(err)
	if err := topic.ValidateTopicMessage(invMsg); err != nil {
		log.Fatalf("FATAL: invoice signature invalid: %v", err)
	}
	var invoice payment.Invoice
	must(json.Unmarshal(invPayload, &invoice))
	real("[HCS mirror] Received invoice")
	info("amount   = %s tinybar", invoice.Amount)
	info("escrowRef = %s", invoice.EscrowRef)
	transition("  Buyer ", buyerSM, payment.EventInvoice)

	// Local escrow release.
	releaseRef, err := escrow.RequestRelease(ctx, escrowRef, price, bf.Seller.EVMAddress, evidenceHash)
	must(err)

	invoiceAck := payment.InvoiceAck{
		Type: "invoiceAck", Version: "1.0.0", RequestID: requestID,
		ReleaseRequestRef: releaseRef.Locator, Action: "approved",
	}
	must(buyer.publishPayload(bus, sellerStdIn, invoiceAck))
	real("Buyer -> Seller: invoiceAck (action=approved) [HCS publish]")
	transition("  Buyer ", buyerSM, payment.EventInvoiceApproved)

	releaseResult, err := escrow.ApproveRelease(ctx, escrowRef, releaseRef)
	must(err)
	mock("Escrow released %s tinybar to %s (BUYER side only)", releaseResult.Released, shortAddr(releaseResult.Recipient))
	transition("  Buyer ", buyerSM, payment.EventComplete)

	// ── Phase 9 (skipped) ────────────────────────────────────────
	phase("9: VALIDATE (skipped — runs in seller process)")
	info("In split mode the validator is colocated with the seller. Buyer process exits here.")

	_ = buyerAdapter.Disconnect(buyerChannel)

	// ── SUMMARY ──────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("  BUYER SUMMARY (split mode)")
	fmt.Println("================================================================")
	fmt.Println()
	fmt.Println("  Mode:       testnet split (--role=buyer)")
	fmt.Printf("  Buyer:      %s (%s)\n", buyer.agentID, shortAddr(buyer.evmAddress()))
	fmt.Printf("  Seller:     %s (%s, from bootstrap)\n", bf.Seller.AgentID, shortAddr(bf.Seller.EVMAddress))
	fmt.Println()
	fmt.Println("  Phase Results:")
	fmt.Println("    1  SETUP        OK   Buyer identity created           [REAL crypto]")
	fmt.Println("    1.5 BOOTSTRAP   OK   Loaded seller info               [from JSON]")
	fmt.Println("    2  REGISTER     OK   Local registry populated         [MOCK registry]")
	fmt.Println("    4  NEGOTIATE    OK   serviceResponse received via HCS [HCS mirror poll]")
	fmt.Printf("    5  FUND         OK   %s tinybar deposited             [MOCK escrow, buyer-side]\n", price)
	fmt.Printf("    6  CONNECT      OK   %s, ECIES decrypted              [REAL libp2p + ECIES]\n", buyerChannel.Transport)
	fmt.Printf("    7  DELIVER      OK   %s %d bytes                      [REAL libp2p + SHA256]\n", recvResult.Filename, recvResult.Size)
	fmt.Printf("    8  SETTLE       OK   %s tinybar released              [MOCK escrow + invoiceAck]\n", price)
	fmt.Println("    9  VALIDATE     SKIPPED   (validator runs in seller process)")
	fmt.Println()
	fmt.Println("  File Transfer:")
	fmt.Printf("    File:       %s\n", recvResult.Filename)
	fmt.Printf("    Size:       %d bytes\n", recvResult.Size)
	fmt.Printf("    SHA256:     %s\n", shortHash(recvResult.SHA256))
	fmt.Println()
	fmt.Println("  Explorer:")
	fmt.Printf("    Buyer.stdIn:  https://hashscan.io/testnet/topic/%s\n", buyer.stdIn.Locator())
	fmt.Printf("    Seller.stdIn: https://hashscan.io/testnet/topic/%s\n", sellerStdIn.Locator())
}
