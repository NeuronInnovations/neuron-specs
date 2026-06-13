// Command adsb-seller is the reference MVP ADS-B BaseStation seller.
//
// It runs a libp2p host that serves the `/jetvision/basestation/1.0.0` stream
// protocol per the BaseStation decoded-track fast-path slice
// (BaseStation fast-fusion audit +
// docs/normalized-track-contract.md). Two operation modes are supported via
// the `--mode` flag:
//
//   - `fixture-direct` (default) — Profile F (013 FR-F-01..F-06). No
//     registration, no commerce. The seller prints its libp2p multiaddrs
//     to stdout; a buyer dials them with `--seller=<multiaddr>`. Suitable
//     for CI smoke runs and local demos.
//   - `eip8004-registry` — Level R1+ of the reference demo. The seller
//     registers its AgentURI in an EIP-8004 Identity Registry (003 / 007)
//     before opening its listen socket. A buyer in the same mode looks
//     the seller up by EVM address and dials the cross-checked multiaddr.
//
// Source modes (required `--feed-source`):
//
//   - `basestation-tcp` — dial `--basestation-tcp-host` (default
//     `127.0.0.1:30003`, the canonical JetVision SBS-1 port) and parse
//     incoming records as SBS-1 BaseStation CSV. Read-only.
//   - `replay` — replay an SBS-1 capture file from `--replay <path>` at
//     `--speedup` rate, optionally looping.
//   - `synthetic` — emit `--synth-aircraft` deterministic orbital aircraft
//     at `--synth-fps` records per second.
//
// Heartbeat disclosure:
//   - basestation-tcp → feedSource = "live" (audit Q-5: stay inside the
//     existing four-label vocabulary; lineage goes in feedSourceConfig)
//   - replay         → feedSource = "replay"
//   - synthetic      → feedSource = "synthetic"
//
// On startup the binary prints the libp2p PeerID and listen multiaddrs
// (filtered per 009 FR-D11a). In registry mode the binary ALSO prints the
// on-chain evidence lines (registry, tokenId, transaction hash, agentURI
// SHA-256) for TEVV capture.
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/libp2p/go-libp2p/core/host"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/adsb"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	sbs "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

const (
	modeFixtureDirect   = "fixture-direct"
	modeEIP8004Registry = "eip8004-registry"

	feedBaseStationTCP = "basestation-tcp"
	feedReplay         = "replay"
	feedSynthetic      = "synthetic"

	defaultBaseStationHost = "127.0.0.1:30003"

	defaultChainID = uint64(296) // Hedera testnet
	defaultRPCURL  = "https://testnet.hashio.io/api"
)

// Deps holds dependency-injection seams for tests.
type Deps struct {
	ContractFactory   func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error)
	TopicAdapter      topic.TopicAdapter
	EscrowAdapter     payment.EscrowAdapter
	SkipServe         bool
	SignalCh          <-chan os.Signal
	HeartbeatInterval time.Duration
}

// ContractFactoryOpts is what the CLI hands to the contract factory.
type ContractFactoryOpts struct {
	RPCURL          string
	RegistryAddress common.Address
	ChainID         uint64
	SignerKey       *ecdsa.PrivateKey
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

// run is the testable entry point.
func run(args []string, env map[string]string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("adsb-seller", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		mode = fs.String("mode", modeFixtureDirect, "operation mode: fixture-direct | eip8004-registry")
		// 0.0.0.0 binds all interfaces; delivery.FilterPublicMultiaddrs prunes
		// loopback/Docker/RFC1918 before any ConnectionSetup is built (HCS 1024-byte
		// limit). Operators MAY narrow to a specific IP at deploy time.
		listen             = fs.String("listen", "/ip4/0.0.0.0/udp/0/quic-v1", "libp2p listen multiaddr")
		feedSourceFlag     = fs.String("feed-source", "", "REQUIRED: basestation-tcp | replay | synthetic")
		basestationTCPHost = fs.String("basestation-tcp-host", defaultBaseStationHost, "BaseStation TCP host:port for --feed-source=basestation-tcp")
		replayPath         = fs.String("replay", "", "SBS-1 capture file path for --feed-source=replay")
		speedup            = fs.Float64("speedup", 1.0, "replay speedup factor (with --feed-source=replay)")
		loop               = fs.Bool("loop", false, "restart replay from beginning on EOF (with --feed-source=replay)")
		synthAircraft      = fs.Int("synth-aircraft", 3, "synthetic aircraft count (with --feed-source=synthetic)")
		synthFPS           = fs.Int("synth-fps", 2, "per-aircraft frame rate (with --feed-source=synthetic)")
		keyHex             = fs.String("key-hex", "", "32-byte hex secp256k1 private key; defaults to ephemeral")

		registryAddrFlag   = fs.String("registry-address", "", "EIP-8004 Identity Registry contract address (registry mode)")
		rpcURL             = fs.String("rpc-url", "", "EVM JSON-RPC endpoint (registry mode; defaults to env HEDERA_EVM_RPC)")
		chainIDFlag        = fs.Uint64("chain-id", 0, "EVM chain id (registry mode; defaults to 296)")
		escrowContractFlag = fs.String("escrow-contract", "", "optional escrow contract address; embedded in commerce settlement.config")
		commerceMode       = fs.String("commerce-mode", "", "FR-P58 disclosure: full|registration-only|data-only (default registration-only in registry mode)")
		topicBackend       = fs.String("topic-backend", "memory", "Topic adapter: memory (in-process) or hcs. No silent fallback.")
		escrowBackend      = fs.String("escrow-backend", "memory", "Escrow adapter: memory (in-process) or evm. No silent fallback.")
	)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "adsb-seller: parse flags:", err)
		return 2
	}

	logger := log.New(stderr, "", log.LstdFlags|log.Lmicroseconds)

	// --feed-source is REQUIRED — no default; an empty value is an error.
	switch *feedSourceFlag {
	case feedBaseStationTCP, feedReplay, feedSynthetic:
		// ok
	case "":
		logger.Printf("adsb-seller: --feed-source is required (basestation-tcp | replay | synthetic)")
		return 2
	default:
		logger.Printf("adsb-seller: unknown --feed-source=%q (expected: basestation-tcp | replay | synthetic)", *feedSourceFlag)
		return 2
	}

	// Cross-validate the source-specific flags.
	if *feedSourceFlag == feedReplay && *replayPath == "" {
		logger.Printf("adsb-seller: --feed-source=replay requires --replay <path>")
		return 2
	}
	if *feedSourceFlag == feedSynthetic && (*synthAircraft <= 0 || *synthFPS <= 0) {
		logger.Printf("adsb-seller: --feed-source=synthetic requires --synth-aircraft > 0 and --synth-fps > 0")
		return 2
	}
	if *feedSourceFlag == feedBaseStationTCP && *basestationTCPHost == "" {
		logger.Printf("adsb-seller: --feed-source=basestation-tcp requires --basestation-tcp-host <host:port>")
		return 2
	}

	switch *mode {
	case modeFixtureDirect, modeEIP8004Registry:
	default:
		logger.Printf("adsb-seller: unknown --mode=%q (expected fixture-direct or eip8004-registry)", *mode)
		return 2
	}

	switch *topicBackend {
	case "memory", "hcs":
	default:
		logger.Printf("adsb-seller: unknown --topic-backend=%q (expected: memory | hcs)", *topicBackend)
		return 2
	}
	switch *escrowBackend {
	case "memory", "evm":
	default:
		logger.Printf("adsb-seller: unknown --escrow-backend=%q (expected: memory | evm)", *escrowBackend)
		return 2
	}

	// No-silent-fallback pre-flight for real backends.
	envLookup := func(key string) string { return env[key] }
	if *topicBackend == "hcs" {
		if missing := adsb.MissingHCSEnvVars(envLookup); len(missing) > 0 {
			logger.Printf("adsb-seller: --topic-backend=hcs requires env %v — refusing to fall back to memory; see runbook", missing)
			return 2
		}
	}
	if *escrowBackend == "evm" {
		if missing := adsb.MissingEVMEnvVars(envLookup); len(missing) > 0 {
			logger.Printf("adsb-seller: --escrow-backend=evm requires env %v — refusing to fall back to memory; see runbook", missing)
			return 2
		}
	}

	privKey, err := resolveKey(*keyHex)
	if err != nil {
		logger.Printf("adsb-seller: resolve key: %v", err)
		return 2
	}

	feedSource := deriveFeedSource(*feedSourceFlag)

	// Stage 3B: pre-build libp2p host for registry + commerce=full so the
	// descriptor's NeuronP2PExchangeService can carry live listen multiaddrs.
	var (
		host            host.Host
		hostBuiltEarly  bool
		earlyMultiaddrs []string
	)
	preBuildHost := *mode == modeEIP8004Registry &&
		*commerceMode == adsb.CommerceModeFull &&
		!deps.SkipServe
	if preBuildHost {
		host, err = delivery.NewLibp2pHost(privKey, *listen)
		if err != nil {
			logger.Printf("adsb-seller: build host: %v", err)
			return 2
		}
		hostBuiltEarly = true
		filtered := delivery.FilterMultiaddrs(host.Addrs())
		if len(filtered) == 0 {
			filtered = host.Addrs()
		}
		earlyMultiaddrs = make([]string, len(filtered))
		for i, ma := range filtered {
			earlyMultiaddrs[i] = ma.String()
		}
		if len(earlyMultiaddrs) == 0 {
			logger.Printf("adsb-seller: --commerce-mode=full requires at least one libp2p listen multiaddr; set --listen explicitly")
			_ = host.Close()
			return 2
		}
		logger.Printf("[seller] libp2p host ready peerID=%s multiaddrs=%d", host.ID(), len(earlyMultiaddrs))
	}

	var regOutcome registerOutcome
	if *mode == modeEIP8004Registry {
		regOutcome = runRegister(registerArgs{
			env:                env,
			logger:             logger,
			deps:               deps,
			privKey:            privKey,
			registryHex:        *registryAddrFlag,
			rpcURLFlag:         *rpcURL,
			chainIDFlag:        *chainIDFlag,
			escrowContract:     *escrowContractFlag,
			commerceMode:       *commerceMode,
			feedSource:         feedSource,
			topicBackend:       *topicBackend,
			escrowBackend:      *escrowBackend,
			multiaddrs:         earlyMultiaddrs,
			feedSourceCLI:      *feedSourceFlag,
			basestationTCPHost: *basestationTCPHost,
		})
		if regOutcome.exitCode != 0 {
			if hostBuiltEarly {
				_ = host.Close()
			}
			return regOutcome.exitCode
		}
		if deps.SkipServe {
			return 0
		}
	}

	if !hostBuiltEarly {
		host, err = delivery.NewLibp2pHost(privKey, *listen)
		if err != nil {
			logger.Printf("adsb-seller: build host: %v", err)
			return 2
		}
	}
	defer host.Close()

	source := chooseSource(sourceParams{
		feedSource:         *feedSourceFlag,
		replayPath:         *replayPath,
		speedup:            *speedup,
		loop:               *loop,
		synthAircraft:      *synthAircraft,
		synthFPS:           *synthFPS,
		basestationTCPHost: *basestationTCPHost,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	running, err := adsb.Start(ctx, adsb.SellerConfig{
		Host:   host,
		Source: source,
		Logger: logger,
	})
	if err != nil {
		logger.Printf("adsb-seller: start: %v", err)
		return 1
	}

	logger.Printf("ready: peerID=%s mode=%s feedSource=%s", host.ID(), *mode, *feedSourceFlag)
	advertise := delivery.FilterMultiaddrs(host.Addrs())
	if len(advertise) == 0 {
		advertise = host.Addrs()
	}
	for _, a := range advertise {
		fmt.Fprintf(stdout, "%s/p2p/%s\n", a, host.ID())
	}

	// Heartbeat publisher (only when commerce-mode=full creates a stdOut topic).
	var heartbeatLoop *adsb.HeartbeatLoop
	if regOutcome.adapter != nil && regOutcome.sellerStdOut.Locator() != "" {
		ck := regOutcome.childKey
		hbLoop, hbErr := adsb.StartHeartbeatLoop(ctx, adsb.HeartbeatLoopOptions{
			Key:            &ck,
			StdOutRef:      regOutcome.sellerStdOut,
			Adapter:        regOutcome.adapter,
			Descriptor:     regOutcome.descriptor,
			Interval:       deps.HeartbeatInterval,
			Logger:         logger,
			SellerEVM:      ck.PublicKey().EVMAddress().Hex(),
			SellerPeerID:   host.ID().String(),
			ServiceName:    adsb.CommerceServiceName,
			TopicBackend:   regOutcome.topicBackend,
			EscrowBackend:  regOutcome.escrowBackend,
			AgentURISha256: regOutcome.agentURISha256,
			DegradedFunc:   func() bool { return false },
		})
		if hbErr != nil {
			logger.Printf("adsb-seller: start heartbeat loop: %v", hbErr)
			running.Cancel()
			<-running.Done
			return 1
		}
		heartbeatLoop = hbLoop
		logger.Printf("[heartbeat] loop started sellerEVM=%s sellerPeerID=%s topicBackend=%s escrowBackend=%s",
			ck.PublicKey().EVMAddress().Hex(), host.ID().String(),
			regOutcome.topicBackend, regOutcome.escrowBackend)
	}

	// Per-buyer commerce session goroutine.
	sessionCh := make(chan adsb.SellerSessionResult, 1)
	sessionDone := make(chan struct{})
	hasSessionGoroutine := false
	if regOutcome.adapter != nil {
		hasSessionGoroutine = true
		ck := regOutcome.childKey
		go func() {
			defer close(sessionDone)
			result, err := adsb.RunSellerCLISession(ctx, adsb.SellerCLIOptions{
				Key:           &ck,
				Adapter:       regOutcome.adapter,
				SellerStdIn:   regOutcome.sellerStdIn,
				Descriptor:    regOutcome.descriptor,
				Host:          host,
				Escrow:        regOutcome.escrow,
				EscrowBinding: regOutcome.escrowBinding,
				Mode:          adsb.CommerceModeFull,
				Logger:        logger,
				FrameSummary: func() (uint64, uint64, uint64) {
					return 0, 0, 0
				},
			})
			if err != nil {
				logger.Printf("[seller-session] error: %v", err)
			}
			select {
			case sessionCh <- result:
			default:
			}
		}()
	}

	sigCh := deps.SignalCh
	if sigCh == nil {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		sigCh = ch
	}
	select {
	case sig, ok := <-sigCh:
		if ok {
			logger.Printf("received signal=%v; shutting down", sig)
		}
	case sr := <-sessionCh:
		logger.Printf("[seller-session] complete state=%s requestID=%s", sr.FinalState, sr.RequestID)
	}
	running.Cancel()
	<-running.Done
	cancel()
	if heartbeatLoop != nil {
		<-heartbeatLoop.Done
	}
	if hasSessionGoroutine {
		<-sessionDone
	}
	logger.Printf("shutdown complete")
	return 0
}

// registerArgs / registerOutcome — same shape as remoteid-seller's, with
// the BaseStation lineage threaded through for heartbeat disclosure.
type registerArgs struct {
	env                map[string]string
	logger             *log.Logger
	deps               Deps
	privKey            *ecdsa.PrivateKey
	registryHex        string
	rpcURLFlag         string
	chainIDFlag        uint64
	escrowContract     string
	commerceMode       string
	feedSource         string
	topicBackend       string
	escrowBackend      string
	multiaddrs         []string
	feedSourceCLI      string // basestation-tcp | replay | synthetic
	basestationTCPHost string
}

type registerOutcome struct {
	exitCode       int
	adapter        topic.TopicAdapter
	escrow         payment.EscrowAdapter
	escrowBinding  string
	sellerStdIn    topic.TopicRef
	sellerStdOut   topic.TopicRef
	descriptor     adsb.ServiceDescriptor
	childKey       keylib.NeuronPrivateKey
	agentURISha256 string
	topicBackend   string
	escrowBackend  string
}

// Compile-time references to silence unused-import warnings.
var (
	_ topic.TopicAdapter    = (*topic.MemoryTopicAdapter)(nil)
	_ payment.EscrowAdapter = (*payment.MemoryEscrow)(nil)
)

func runRegister(a registerArgs) registerOutcome {
	fail := func(code int) registerOutcome { return registerOutcome{exitCode: code} }

	if a.registryHex == "" {
		a.logger.Printf("adsb-seller: --mode=eip8004-registry requires --registry-address <0x...>")
		return fail(2)
	}
	registryAddr, err := keylib.EVMAddressFromHex(a.registryHex)
	if err != nil {
		a.logger.Printf("adsb-seller: invalid --registry-address %q: %v", a.registryHex, err)
		return fail(2)
	}

	chainID := a.chainIDFlag
	if chainID == 0 {
		if v := a.env["NEURON_CHAIN_ID"]; v != "" {
			n, perr := strconv.ParseUint(v, 10, 64)
			if perr != nil {
				a.logger.Printf("adsb-seller: NEURON_CHAIN_ID=%q: %v", v, perr)
				return fail(2)
			}
			chainID = n
		} else {
			chainID = defaultChainID
		}
	}

	rpc := a.rpcURLFlag
	if rpc == "" {
		if v := a.env["HEDERA_EVM_RPC"]; v != "" {
			rpc = v
		} else {
			rpc = defaultRPCURL
		}
	}

	commerceMode := a.commerceMode
	if commerceMode == "" {
		commerceMode = adsb.CommerceModeRegistrationOnly
	}

	childKey, err := keylib.NeuronPrivateKeyFromBytes(ethcrypto.FromECDSA(a.privKey))
	if err != nil {
		a.logger.Printf("adsb-seller: wrap signing key: %v", err)
		return fail(1)
	}

	var (
		adapter       topic.TopicAdapter
		escrow        payment.EscrowAdapter
		escrowBinding = "memory"
		topicConfig   map[string]map[string]any
		sellerStdIn   topic.TopicRef
		sellerStdOut  topic.TopicRef
	)
	if commerceMode == adsb.CommerceModeFull {
		ctx := context.Background()
		envLookup := func(key string) string { return a.env[key] }

		switch a.topicBackend {
		case "hcs":
			be, herr := adsb.NewHCSBackend(ctx, adsb.HCSBackendOptions{
				Role:            adsb.HCSRoleSeller,
				LookupEnv:       envLookup,
				TopicMemoPrefix: "adsb-" + childKey.PublicKey().EVMAddress().Hex()[2:10] + "-",
			})
			if herr != nil {
				a.logger.Printf("adsb-seller: %v", herr)
				return fail(1)
			}
			adapter = be.Adapter
			sellerStdIn = be.StdInRef
			sellerStdOut = be.StdOutRef
			topicConfig = map[string]map[string]any{
				"stdIn":  {"topicId": be.StdInRef.Locator()},
				"stdOut": {"topicId": be.StdOutRef.Locator()},
				"stdErr": {"topicId": be.StdErrRef.Locator()},
			}
			a.logger.Printf("[hcs] operator=%s topics: stdIn=%s stdOut=%s stdErr=%s",
				be.OperatorID, be.StdInRef.Locator(), be.StdOutRef.Locator(), be.StdErrRef.Locator())
		default: // "memory"
			adapter = a.deps.TopicAdapter
			if adapter == nil {
				adapter = topic.NewMemoryTopicAdapter()
			}
			topicSuffix := childKey.PublicKey().EVMAddress().Hex()
			var inRef, outRef, errRef topic.TopicRef
			inRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-stdin-" + topicSuffix})
			if err != nil {
				a.logger.Printf("adsb-seller: create stdIn topic: %v", err)
				return fail(1)
			}
			outRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-stdout-" + topicSuffix})
			if err != nil {
				a.logger.Printf("adsb-seller: create stdOut topic: %v", err)
				return fail(1)
			}
			errRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-stderr-" + topicSuffix})
			if err != nil {
				a.logger.Printf("adsb-seller: create stdErr topic: %v", err)
				return fail(1)
			}
			sellerStdIn = inRef
			sellerStdOut = outRef
			topicConfig = map[string]map[string]any{
				"stdIn":  {"topicId": inRef.Locator()},
				"stdOut": {"topicId": outRef.Locator()},
				"stdErr": {"topicId": errRef.Locator()},
			}
		}

		switch a.escrowBackend {
		case "evm":
			be, eerr := adsb.NewEVMBackend(ctx, adsb.EVMBackendOptions{
				LookupEnv:      envLookup,
				DefaultRPCURL:  defaultRPCURL,
				DefaultChainID: defaultChainID,
			})
			if eerr != nil {
				a.logger.Printf("adsb-seller: %v", eerr)
				return fail(1)
			}
			escrow = be.Escrow
			escrowBinding = be.EscrowBinding
			a.logger.Printf("[evm] rpc=%s chainId=%d escrowContract=%s tokenContract=%s operator=%s",
				be.RPCURL, be.ChainID, be.EscrowContract, be.TokenContract, be.OperatorAddr)
		default: // "memory"
			escrow = a.deps.EscrowAdapter
			if escrow == nil {
				escrow = payment.NewMemoryEscrow()
			}
		}
	}

	descriptor, err := adsb.BuildServiceDescriptor(adsb.DescriptorOptions{
		ChildKey:       &childKey,
		ChainID:        chainID,
		EscrowContract: a.escrowContract,
		FeedSource:     a.feedSource,
		CommerceMode:   commerceMode,
		ProfileID:      adsb.ProfileR1,
		TopicConfig:    topicConfig,
		Multiaddrs:     a.multiaddrs,
	})
	if err != nil {
		a.logger.Printf("adsb-seller: build descriptor: %v", err)
		return fail(1)
	}

	factory := a.deps.ContractFactory
	if factory == nil {
		factory = defaultContractFactory
	}

	ctx := context.Background()
	contract, err := factory(ctx, ContractFactoryOpts{
		RPCURL:          rpc,
		RegistryAddress: common.HexToAddress(registryAddr.Hex()),
		ChainID:         chainID,
		SignerKey:       a.privKey,
	})
	if err != nil {
		a.logger.Printf("adsb-seller: build registry contract: %v", err)
		return fail(1)
	}

	result, err := adsb.RegisterSeller(ctx, &childKey, descriptor, registryAddr, chainID, contract)
	if err != nil {
		a.logger.Printf("adsb-seller: register: %v", err)
		return fail(1)
	}

	tokenIDStr := "<nil>"
	if result.TokenID != nil {
		tokenIDStr = result.TokenID.String()
	}
	a.logger.Printf("[registry] mode=eip8004-registry outcome=%s sellerEVM=%s registry=%s chainId=%d tokenId=%s txHash=%s agentURISha256=%s",
		result.Outcome,
		result.SellerEVM.Hex(),
		result.RegistryAddress.Hex(),
		result.ChainID,
		tokenIDStr,
		result.TransactionHash,
		result.AgentURISha256,
	)
	a.logger.Printf("[registry] descriptor: name=%s version=%s pricing.unit=%s commerceMode=%s feedSource=%s profileID=%s",
		adsb.CommerceServiceName,
		adsb.CommerceServiceVersion,
		adsb.PricingUnit,
		descriptor.CommerceMode,
		descriptor.FeedSource,
		descriptor.ProfileID,
	)
	if a.feedSourceCLI == feedBaseStationTCP {
		a.logger.Printf("[seller] basestation-tcp source: upstream=%s (feedSourceConfig.upstream per audit Q-5)", a.basestationTCPHost)
	}
	if commerceMode == adsb.CommerceModeFull {
		a.logger.Printf("[seller] commerce-mode=full topics created: stdIn=%s stdOut=%s stdErr=%s — orchestrator will spawn after libp2p host is up",
			topicConfig["stdIn"]["topicId"], topicConfig["stdOut"]["topicId"], topicConfig["stdErr"]["topicId"])
		if len(a.multiaddrs) > 0 {
			a.logger.Printf("[seller] AgentURI carries %d multiaddrs for Stage-3B registry-only discovery: %v",
				len(a.multiaddrs), a.multiaddrs)
		} else {
			a.logger.Printf("[seller] WARNING: AgentURI has no multiaddrs — buyer must fall back to --seller-multiaddr")
		}
	}

	return registerOutcome{
		exitCode:       0,
		adapter:        adapter,
		escrow:         escrow,
		escrowBinding:  escrowBinding,
		sellerStdIn:    sellerStdIn,
		sellerStdOut:   sellerStdOut,
		descriptor:     descriptor,
		childKey:       childKey,
		agentURISha256: result.AgentURISha256,
		topicBackend:   a.topicBackend,
		escrowBackend:  a.escrowBackend,
	}
}

// defaultContractFactory dials the configured RPC and returns a real
// EVMRegistryContract.
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

// deriveFeedSource maps the CLI's --feed-source selector to the
// FR-A18 / FR-R15 feedSource enum value advertised in the heartbeat.
// Audit Q-5: basestation-tcp maps to "live" (existing four-label vocabulary;
// the upstream lineage moves into feedSourceConfig).
func deriveFeedSource(cliFeedSource string) string {
	switch cliFeedSource {
	case feedReplay:
		return adsb.FeedSourceReplay
	case feedSynthetic:
		return adsb.FeedSourceSynthetic
	case feedBaseStationTCP:
		return adsb.FeedSourceLive
	default:
		return adsb.FeedSourceLive
	}
}

type sourceParams struct {
	feedSource         string
	replayPath         string
	speedup            float64
	loop               bool
	synthAircraft      int
	synthFPS           int
	basestationTCPHost string
}

func chooseSource(p sourceParams) adsb.FeedSource {
	switch p.feedSource {
	case feedBaseStationTCP:
		return adsb.BaseStationTCPSource(p.basestationTCPHost)
	case feedReplay:
		return adsb.BaseStationReplaySource(p.replayPath, sbs.ReplayOptions{
			Speedup: p.speedup,
			Loop:    p.loop,
		})
	case feedSynthetic:
		return adsb.BaseStationSynthSource(sbs.SynthOptions{
			Aircraft: p.synthAircraft,
			Fps:      p.synthFPS,
		})
	default:
		return adsb.BaseStationSynthSource(sbs.SynthOptions{
			Aircraft: 1,
			Fps:      1,
		})
	}
}

// resolveKey parses --key-hex or generates an ephemeral secp256k1 key.
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
