// Command sapient-buyer is the generic SAPIENT Buyer Proxy in the local SAPIENT
// Remote ID demo. Per the Neuron reverse-connect topology the seller always
// dials the buyer (buyers are assumed reachable), so the proxy LISTENS on
// /sapient/detection/2.0.0; the seller dials in and pushes SapientMessages.
//
// It is vendor- and Neuron-blind by design (015 FR-S90): it does NOT parse
// object_info / rid.*. It strips the Neuron envelope and FORWARDS each
// SapientMessage onto the downstream SAPIENT edge (--sapient-edge), the local
// realisation of the FR-S91 Buyer-Proxy↔HLDMM edge. All rid.*-aware display
// projection (remote-id TaggedFrame, Cursor-on-Target) lives in the separate
// 018/FID consumer (cmd/sapient-fid-consumer), which dials that edge.
//
//	sapient-buyer --listen <multiaddr> --sapient-edge HOST:PORT
//
// It prints its dialable multiaddr(s) on stdout for the seller's --buyer flag.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	lphost "github.com/libp2p/go-libp2p/core/host"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
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
	defaultChainID = uint64(296) // Hedera testnet
	defaultRPCURL  = "https://testnet.hashio.io/api"

	envRegistryContract = "NEURON_REGISTRY_CONTRACT"
	envHederaRPC        = "HEDERA_EVM_RPC"
)

// publisher is the downstream SAPIENT edge the proxy forwards onto — a
// *sapient.FeedServer in production, a capture in tests.
type publisher interface {
	Publish(*sapientpb.SapientMessage) error
}

// Deps holds dependency-injection seams so tests can drive run() without a
// real chain or HCS. All fields optional.
type Deps struct {
	// TopicAdapter is the commerce bus for --commerce-mode=full with
	// --topic-backend=memory (tests inject the bus they share with the seller).
	TopicAdapter topic.TopicAdapter

	// EscrowAdapter is the settlement adapter for --escrow-backend=memory
	// (tests inject the instance they share with the seller).
	EscrowAdapter payment.EscrowAdapter

	// ContractFactory builds the (read-only) RegistryContract the buyer uses
	// for seller discovery. nil => read-only EVMRegistryContract over RPC.
	ContractFactory func(ctx context.Context, rpcURL string, addr keylib.EVMAddress) (registry.RegistryContract, error)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, Deps{}); err != nil {
		log.Fatalf("sapient-buyer: %v", err)
	}
}

func run(args []string, stdout io.Writer, deps Deps) error {
	fs := flag.NewFlagSet("sapient-buyer", flag.ContinueOnError)
	var (
		listen      = fs.String("listen", "/ip4/127.0.0.1/udp/19192/quic-v1", "libp2p host listen multiaddr (must be reachable by the seller)")
		sapientEdge = fs.String("sapient-edge", "127.0.0.1:19193", "TCP address to serve the downstream SAPIENT edge (FR-S91: 4-byte LE length-prefixed protobuf) for the FID/HLDMM consumer")
		keyHex      = fs.String("key-hex", "", "32-byte hex secp256k1 key; defaults to ephemeral")
		// Optional auditable-lane (004) observability (additive; routes by message
		// type only — never parses object_info, so FR-S90-compliant).
		controlLane = fs.String("control-lane", "", "auditable-lane stub file path to observe TaskAck/StatusReport (needs --asm-node-id)")
		asmNodeID   = fs.String("asm-node-id", "", "the seller's ASM node_id, for --control-lane observability")
		// Multi-source observability (additive; open mode only).
		sessionsHTTP = fs.String("sessions-http", "", "loopback address to serve the read-only GET /sessions endpoint (seller sessions: peerID, node_id, counts); empty disables it")
		// Full-payment commerce mode (additive). off = byte-identical proxy.
		commerceMode        = fs.String("commerce-mode", sapient.CommerceModeOff, "008 commerce posture: off (default; open proxy) | full (escrow-funded, admission-gated session; exits after settlement)")
		topicBackend        = fs.String("topic-backend", "memory", "commerce bus for --commerce-mode=full: memory (in-process) | hcs (env HEDERA_OPERATOR_ID/HEDERA_OPERATOR_KEY)")
		escrowBackend       = fs.String("escrow-backend", "memory", "settlement for --commerce-mode=full: memory | evm (env NEURON_ESCROW_CONTRACT + NEURON_TOKEN_CONTRACT + signer key)")
		sellerEVM           = fs.String("seller-evm", "", "seller EVM address to engage (registry discovery key; required for --commerce-mode=full)")
		registryAddress     = fs.String("registry-address", "", "EIP-8004 Identity Registry address for seller discovery (env fallback "+envRegistryContract+")")
		rpcURL              = fs.String("rpc-url", "", "EVM JSON-RPC endpoint for discovery/settlement (env fallback "+envHederaRPC+"; default "+defaultRPCURL+")")
		chainIDFlag         = fs.Uint64("chain-id", defaultChainID, "EVM chain id (Hedera testnet = 296)")
		pricingAmount       = fs.String("pricing-amount", "", "proposed price (decimal integer); empty = adopt the card's advertised pricing")
		frameLimit          = fs.Uint64("frame-limit", 25, "stop the session after this many forwarded frames (--commerce-mode=full)")
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
	logger := log.New(os.Stderr, "[sapient-buyer] ", log.LstdFlags)

	key, err := resolveKey(keyHexOrEnv(*keyHex))
	if err != nil {
		return err
	}
	host, err := delivery.NewLibp2pHost(key, *listen)
	if err != nil {
		return fmt.Errorf("create host: %w", err)
	}
	defer host.Close()

	feed, err := sapient.ServeFeed(*sapientEdge)
	if err != nil {
		return fmt.Errorf("serve sapient edge %q: %w", *sapientEdge, err)
	}
	defer feed.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Optional: observe the auditable lane (TaskAck/StatusReport) for visibility.
	if *controlLane != "" && *asmNodeID != "" {
		observeControlLane(ctx, strings.TrimPrefix(*controlLane, "file:"), *asmNodeID, logger)
	}

	// Full-payment commerce mode: discovery → escrow funding → reverse
	// ConnectionSetup → admission-gated stream → settlement → exit.
	if *commerceMode == sapient.CommerceModeFull {
		return runBuyerCommerce(ctx, key, host, feed, stdout, buyerCommerceArgs{
			topicBackend:    *topicBackend,
			escrowBackend:   *escrowBackend,
			sellerEVMHex:    *sellerEVM,
			registryAddress: *registryAddress,
			rpcURL:          *rpcURL,
			chainID:         *chainIDFlag,
			pricingAmount:   *pricingAmount,
			frameLimit:      *frameLimit,
			evidenceOut:     *commerceEvidenceOut,
		}, deps, logger)
	}

	// Open mode is multi-source: every inbound seller stream gets its own
	// forwardStream goroutine (libp2p spawns one per stream) and its own
	// session-registry entry — no global "current seller" state.
	reg := newSessionRegistry()
	host.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(stream libp2pnetwork.Stream) {
		forwardStream(stream, feed, logger, reg)
	})

	if *sessionsHTTP != "" {
		if _, _, herr := startSessionsHTTP(ctx, *sessionsHTTP, reg, feed, logger); herr != nil {
			return herr
		}
	}
	go sessionSummaryLoop(ctx, reg, feed, 30*time.Second, logger)

	for _, a := range host.Addrs() {
		fmt.Fprintf(stdout, "%s/p2p/%s\n", a, host.ID())
	}
	logger.Printf("listening peerID=%s protocol=%s sapient-edge=%s", host.ID(), sapient.ProtocolDetection, feed.Addr())

	<-ctx.Done()
	logger.Printf("shutting down")
	return nil
}

// buyerCommerceArgs carries the parsed --commerce-mode=full flags.
type buyerCommerceArgs struct {
	topicBackend    string
	escrowBackend   string
	sellerEVMHex    string
	registryAddress string
	rpcURL          string
	chainID         uint64
	pricingAmount   string
	frameLimit      uint64
	evidenceOut     string
}

// admissionGate is the inbound-stream admission latch. Streams arriving
// before the commerce handshake reaches FUNDED are HELD (not served); once
// the gate opens, only the expected seller PeerID is admitted — everything
// else is reset. Stream data therefore never flows before payment admission.
type admissionGate struct {
	ready    chan struct{} // closed when the gate opens
	mu       sync.Mutex
	expected string
}

func newAdmissionGate() *admissionGate { return &admissionGate{ready: make(chan struct{})} }

// open sets the expected seller and releases held streams.
func (g *admissionGate) open(expectedPeerID string) {
	g.mu.Lock()
	g.expected = expectedPeerID
	g.mu.Unlock()
	close(g.ready)
}

// admit blocks until the gate opens (or ctx/timeout), then returns whether
// the remote peer is the funded session's seller.
func (g *admissionGate) admit(ctx context.Context, remote string) (bool, string) {
	select {
	case <-g.ready:
	case <-ctx.Done():
		return false, ""
	case <-time.After(30 * time.Second):
		return false, ""
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return remote == g.expected, g.expected
}

// runBuyerCommerce drives the full-payment buyer: resolve backends → install
// the admission-gated stream handler → StartBuyerCommerce (discovery +
// verification + funding + reverse ConnectionSetup) → open the gate for the
// funded seller → forward frames opaquely up to --frame-limit → Finalise
// (ServiceStop → Invoice → ApproveRelease → InvoiceAck) → evidence → exit.
func runBuyerCommerce(ctx context.Context, key *ecdsa.PrivateKey, h lphost.Host, feed publisher, stdout io.Writer, a buyerCommerceArgs, deps Deps, logger *log.Logger) error {
	if a.sellerEVMHex == "" {
		return errors.New("--commerce-mode=full requires --seller-evm <0x...> (the registry discovery key)")
	}
	sellerEVMAddr, err := keylib.EVMAddressFromHex(a.sellerEVMHex)
	if err != nil {
		return fmt.Errorf("invalid --seller-evm %q: %w", a.sellerEVMHex, err)
	}
	regHex := a.registryAddress
	if regHex == "" {
		regHex = os.Getenv(envRegistryContract)
	}
	if regHex == "" {
		return fmt.Errorf("--commerce-mode=full requires --registry-address <0x...> (or env %s)", envRegistryContract)
	}
	regAddr, err := keylib.EVMAddressFromHex(regHex)
	if err != nil {
		return fmt.Errorf("invalid --registry-address %q: %w", regHex, err)
	}
	if a.chainID == 0 {
		return errors.New("--chain-id must be non-zero for --commerce-mode=full")
	}
	rpc := a.rpcURL
	if rpc == "" {
		rpc = os.Getenv(envHederaRPC)
	}
	if rpc == "" {
		rpc = defaultRPCURL
	}

	nk, err := keylib.NeuronPrivateKeyFromBytes(ethCryptoFromECDSA(key))
	if err != nil {
		return fmt.Errorf("derive neuron identity: %w", err)
	}

	// --- registry contract (read-only discovery; the buyer signs nothing
	// on-chain for identity — the buyer card is deferred) ---
	contractFactory := deps.ContractFactory
	if contractFactory == nil {
		contractFactory = func(ctx context.Context, rpcURL string, addr keylib.EVMAddress) (registry.RegistryContract, error) {
			client, derr := ethclient.DialContext(ctx, rpcURL)
			if derr != nil {
				return nil, fmt.Errorf("dial %s: %w", rpcURL, derr)
			}
			return registry.NewEVMRegistryContract(client, common.HexToAddress(addr.Hex()), nil)
		}
	}
	contract, err := contractFactory(ctx, rpc, regAddr)
	if err != nil {
		return fmt.Errorf("build registry contract: %w", err)
	}

	// --- topic backend ---
	var adapter topic.TopicAdapter
	switch a.topicBackend {
	case "hcs":
		be, herr := remoteid.NewHCSBackend(ctx, remoteid.HCSBackendOptions{Role: remoteid.HCSRoleBuyer})
		if herr != nil {
			return herr
		}
		adapter = be.Adapter
		logger.Printf("[hcs] operator=%s (buyer adapter; topics resolved from the seller's card)", be.OperatorID)
	case "memory":
		adapter = deps.TopicAdapter
		if adapter == nil {
			adapter = topic.NewMemoryTopicAdapter()
		}
	default:
		return fmt.Errorf("unknown --topic-backend=%q (want memory|hcs)", a.topicBackend)
	}

	// --- escrow backend + funding ---
	var (
		escrow         payment.EscrowAdapter
		escrowBinding  = sapient.SettlementBindingMemory
		escrowContract string
		tokenContract  string
		ensureFunds    func(ctx context.Context, amount string) (string, error)
	)
	switch a.escrowBackend {
	case "evm":
		be, eerr := remoteid.NewEVMBackend(ctx, remoteid.EVMBackendOptions{
			DefaultRPCURL:  defaultRPCURL,
			DefaultChainID: defaultChainID,
		})
		if eerr != nil {
			return eerr
		}
		escrow, escrowBinding = be.Escrow, be.EscrowBinding
		escrowContract, tokenContract = be.EscrowContract, be.TokenContract
		tokenAddr := common.HexToAddress(be.TokenContract)
		ensureFunds = func(ctx context.Context, amount string) (string, error) {
			needed, ok := new(big.Int).SetString(amount, 10)
			if !ok {
				return "", fmt.Errorf("invalid amount %q", amount)
			}
			return sapient.EnsureTokenBalance(ctx, be.RPCURL, tokenAddr, be.ChainID, key, needed, logger)
		}
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

	// --- admission-gated stream handler (installed BEFORE the handshake so
	// the seller's dial can never race past the gate) ---
	gate := newAdmissionGate()
	var frames atomic.Uint64
	dataDone := make(chan struct{})
	var dataOnce sync.Once
	h.SetStreamHandler(protocol.ID(sapient.ProtocolDetection), func(stream libp2pnetwork.Stream) {
		remote := stream.Conn().RemotePeer().String()
		ok, expected := gate.admit(ctx, remote)
		if !ok {
			logger.Printf("[admission] REJECT stream from %s (expected seller=%s) — resetting", remote, expected)
			_ = stream.Reset()
			return
		}
		logger.Printf("[admission] ADMIT stream from funded seller=%s", remote)
		forwardStreamLimited(stream, feed, logger, &frames, a.frameLimit, func() {
			dataOnce.Do(func() { close(dataDone) })
		})
	})

	for _, addr := range h.Addrs() {
		fmt.Fprintf(stdout, "%s/p2p/%s\n", addr, h.ID())
	}
	logger.Printf("listening peerID=%s protocol=%s (commerce-mode=full)", h.ID(), sapient.ProtocolDetection)

	// --- the commerce handshake ---
	session, err := sapient.StartBuyerCommerce(ctx, sapient.BuyerCommerceOptions{
		Key:             &nk,
		Host:            h,
		Adapter:         adapter,
		Escrow:          escrow,
		EscrowBinding:   escrowBinding,
		Contract:        contract,
		RegistryAddress: regAddr,
		ChainID:         a.chainID,
		SellerEVM:       sellerEVMAddr,
		PricingAmount:   a.pricingAmount,
		EnsureFunds:     ensureFunds,
		Logger:          logger,
	})
	if err != nil {
		return err
	}
	gate.open(session.ExpectedSellerPeerID)
	logger.Printf("[admission] gate OPEN for seller=%s (escrow funded)", session.ExpectedSellerPeerID)

	// --- data plane: wait for the frame-limit (or ctx) ---
	select {
	case <-dataDone:
		logger.Printf("[buyer-commerce] frame-limit reached frames=%d; settling", frames.Load())
	case <-ctx.Done():
		return ctx.Err()
	}

	result, err := session.Finalise(ctx)
	if err != nil {
		return fmt.Errorf("buyer commerce settlement: %w", err)
	}
	logger.Printf("[buyer-commerce] %s requestID=%s frames=%d released=%s -> %s",
		result.FinalAction, result.RequestID, frames.Load(), result.ReleasedAmount, result.ReleaseRecipient)

	if a.evidenceOut != "" {
		ev := sapient.CommerceEvidence{
			RequestID:         result.RequestID,
			Role:              "buyer",
			Service:           sapient.CommerceServiceName,
			Protocol:          sapient.ProtocolDetection,
			BuyerEVM:          nk.PublicKey().EVMAddress().Hex(),
			SellerEVM:         result.SellerEVM,
			SellerAgentID:     result.SellerAgentID,
			SellerPeerID:      result.SellerPeerID,
			BuyerPeerID:       h.ID().String(),
			RegistryAddress:   regAddr.Hex(),
			EscrowContract:    escrowContract,
			TokenContract:     tokenContract,
			ChainID:           a.chainID,
			TopicBackend:      a.topicBackend,
			EscrowBackend:     a.escrowBackend,
			RegistryBackend:   registryBackendLabel(deps),
			Topics:            map[string]string{"sellerStdIn": result.SellerStdIn, "buyerStdIn": result.BuyerStdIn},
			EscrowRef:         result.EscrowRef,
			MintTx:            result.MintTx,
			DepositTx:         result.DepositTx,
			InvoiceAmount:     result.InvoiceAmount,
			InvoiceCurrency:   result.InvoiceCurrency,
			ReleaseRequestRef: result.ReleaseRequestRef,
			InvoiceAckAction:  "approved",
			ApproveTx:         result.ApproveTx,
			ReleasedAmount:    result.ReleasedAmount,
			ReleaseRecipient:  result.ReleaseRecipient,
			FrameCount:        frames.Load(),
			FinalAction:       result.FinalAction,
		}
		if werr := sapient.WriteCommerceEvidence(a.evidenceOut, ev); werr != nil {
			return werr
		}
		logger.Printf("wrote commerce evidence -> %s", a.evidenceOut)
	}
	return nil
}

// registryBackendLabel discloses how the buyer resolved the seller: a real
// read-only RPC contract by default, or an injected (test/memory) contract.
func registryBackendLabel(deps Deps) string {
	if deps.ContractFactory != nil {
		return "injected"
	}
	return "evm"
}

// ethCryptoFromECDSA mirrors ethcrypto.FromECDSA without forcing the import
// into every call site.
func ethCryptoFromECDSA(key *ecdsa.PrivateKey) []byte {
	return ethcrypto.FromECDSA(key)
}

// forwardStreamLimited is forwardStream plus an opaque frame counter and a
// session limit: when `limit` frames have been forwarded the stream is reset
// (the seller sees a write error and stops) and onLimit fires exactly once.
func forwardStreamLimited(stream libp2pnetwork.Stream, feed publisher, logger *log.Logger, frames *atomic.Uint64, limit uint64, onLimit func()) {
	peerID := stream.Conn().RemotePeer().String()
	logger.Printf("stream open from seller=%s", peerID)
	defer stream.Close()

	reader := delivery.NewFrameReader(stream)
	um := proto.UnmarshalOptions{DiscardUnknown: true}
	for {
		data, rerr := reader.ReadFrame()
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				logger.Printf("stream EOF seller=%s frames=%d", peerID, frames.Load())
			} else if !errors.Is(rerr, context.Canceled) {
				logger.Printf("read error seller=%s frames=%d: %v", peerID, frames.Load(), rerr)
			}
			onLimit() // stream ended early — settle with what we got
			return
		}
		var msg sapientpb.SapientMessage
		if err := um.Unmarshal(data, &msg); err != nil {
			logger.Printf("decode SapientMessage error: %v", err)
			continue
		}
		if err := feed.Publish(&msg); err != nil {
			logger.Printf("edge publish error: %v", err)
			continue
		}
		if n := frames.Add(1); limit > 0 && n >= limit {
			logger.Printf("frame-limit %d reached from seller=%s; closing stream", limit, peerID)
			_ = stream.Reset()
			onLimit()
			return
		}
	}
}

// forwardStream reads length-framed SapientMessage protobufs from one seller
// stream and forwards each, unchanged, onto the downstream SAPIENT edge. Per
// FR-S90 the proxy does NOT parse object_info — it is opaque to the detection
// payload (the registry observes only the top-level node_id, which the loop
// already decodes for forwarding). A single decode failure skips that frame; a
// read error (incl. clean io.EOF) ends the stream and closes the session.
// Multi-source: libp2p runs one forwardStream per inbound stream, so N sellers
// forward concurrently with no shared per-seller state beyond the registry.
func forwardStream(stream libp2pnetwork.Stream, feed publisher, logger *log.Logger, reg *sessionRegistry) {
	peerID := stream.Conn().RemotePeer().String()
	sessionID := reg.openSession(peerID, stream.Conn().RemoteMultiaddr().String())
	defer reg.closeSession(sessionID)
	logger.Printf("stream open from seller=%s session=%d", peerID, sessionID)
	defer stream.Close()

	reader := delivery.NewFrameReader(stream)
	um := proto.UnmarshalOptions{DiscardUnknown: true}
	var n uint64
	for {
		data, rerr := reader.ReadFrame()
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				logger.Printf("stream EOF seller=%s session=%d frames=%d", peerID, sessionID, n)
			} else if !errors.Is(rerr, context.Canceled) {
				logger.Printf("read error seller=%s session=%d frames=%d: %v", peerID, sessionID, n, rerr)
			}
			return
		}
		var msg sapientpb.SapientMessage
		if err := um.Unmarshal(data, &msg); err != nil {
			logger.Printf("decode SapientMessage error: %v", err)
			continue
		}
		if err := feed.Publish(&msg); err != nil {
			logger.Printf("edge publish error: %v", err)
			continue
		}
		reg.observe(sessionID, msg.GetNodeId(), time.Now())
		n++
	}
}

// observeControlLane subscribes to the ASM's stdOut and logs received
// TaskAck/StatusReport (read-only visibility; no effect on the data path).
func observeControlLane(ctx context.Context, lanePath, asmNodeID string, logger *log.Logger) {
	lane := auditlane.NewFileLane(lanePath)
	sub, err := lane.Subscribe(ctx, auditlane.Channel{ASMNodeID: asmNodeID, Role: auditlane.RoleStdOut})
	if err != nil {
		logger.Printf("control-lane observe error: %v", err)
		return
	}
	logger.Printf("observing control lane %s (asm=%s) for TaskAck/StatusReport", lanePath, asmNodeID)
	go func() {
		defer lane.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sub:
				if !ok {
					return
				}
				switch {
				case msg.GetTaskAck() != nil:
					ack := msg.GetTaskAck()
					logger.Printf("TaskAck task=%s status=%s reason=%v", ack.GetTaskId(), ack.GetTaskStatus(), ack.GetReason())
				case msg.GetStatusReport() != nil:
					sr := msg.GetStatusReport()
					fs := ""
					for _, st := range sr.GetStatus() {
						if v, ok := tasking.ParseFeedSource(st.GetStatusValue()); ok {
							fs = v
						}
					}
					logger.Printf("StatusReport system=%s mode=%s feedSource=%s", sr.GetSystem(), sr.GetMode(), fs)
				}
			}
		}
	}()
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
