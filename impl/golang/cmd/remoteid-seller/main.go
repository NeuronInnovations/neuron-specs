// Command remoteid-seller is the reference MVP Remote ID seller.
//
// It runs a libp2p host that serves the `/ds240/raw/1.0.0` stream
// protocol per spec 017 FR-R02. Two operation modes are supported via the
// `--mode` flag:
//
//   - `fixture-direct` (default) — Profile F (013 FR-F-01..F-06). No
//     registration, no commerce. The seller prints its libp2p multiaddrs
//     to stdout; a buyer dials them with `--seller=<multiaddr>`. Suitable
//     for CI smoke runs and the Phase-2 vertical-slice demo. Explicitly
//     NOT the reference demo path.
//   - `eip8004-registry` — Level R1 of the reference demo. The seller registers
//     its AgentURI in an EIP-8004 Identity Registry (003 / 007) before
//     opening its listen socket. A buyer in the same mode looks the seller
//     up by EVM address through `internal/registry.LookupRegistration` and
//     dials the cross-checked multiaddr. Commerce (008) is intentionally
//     not engaged in R1; the heartbeat-advertised `commerceMode` is
//     `registration-only` per FR-P58.
//
// Frame source remains one of: `--replay <path>`, `--synth`, or
// `--ds400-transport=<udp|tcp|http> --ds400-address=<addr>` (Phase-3 stub).
//
// Flags (registry-mode additions in bold):
//
//	--mode <fixture-direct|eip8004-registry>   operation mode (default fixture-direct)
//	--listen <multiaddr>                        libp2p listen address (default: /ip4/0.0.0.0/udp/0/quic-v1)
//	--replay <path>                              fixture JSON file path; mutually exclusive with --synth
//	--synth                                      use the synthetic-orbit source
//	--synth-drones <int>                         synthetic drone count (default 2)
//	--synth-fps <int>                            aggregate frame rate across all drones (default 2)
//	--speedup <float>                            replay speedup factor (default 1.0)
//	--loop                                       restart replay from beginning on EOF
//	--key-hex <hex>                              32-byte hex secp256k1 private key; defaults to a process-ephemeral key
//	**--registry-address <0x...>                 EIP-8004 Identity Registry contract address (registry mode)**
//	**--rpc-url <url>                            EVM JSON-RPC endpoint (registry mode; defaults to env HEDERA_EVM_RPC)**
//	**--chain-id <uint>                          EVM chain id (registry mode; defaults to 296 Hedera testnet)**
//	**--escrow-contract <0x...>                  optional escrow contract address; embedded in commerce service settlement.config**
//	**--commerce-mode <full|registration-only|data-only>  FR-P58 disclosure (default registration-only)**
//	**--feed-source <live|replay|synthetic|placeholder>   FR-R15 disclosure (default auto-derived from --replay / --synth / --ds400)**
//
// On startup the binary prints the libp2p PeerID and listen multiaddrs
// (after multiaddr filtering per 009 FR-D11a) so the operator can hand
// the seller's address to a buyer. In registry mode the binary ALSO
// prints the on-chain evidence lines (registry contract, tokenId,
// transaction hash, agentURI SHA-256) for TEVV capture per
// the registry evidence template.
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

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	remoteidsrc "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

const (
	modeFixtureDirect   = "fixture-direct"
	modeEIP8004Registry = "eip8004-registry"

	defaultChainID = uint64(296) // Hedera testnet
	defaultRPCURL  = "https://testnet.hashio.io/api"
)

// Deps holds dependency injection seams so tests can drive run() with a
// memory-backed registry contract without bringing up a real EVM RPC.
// All fields are optional: production uses defaults that hit Hedera EVM.
type Deps struct {
	// ContractFactory builds the RegistryContract for registry-backed
	// mode. If nil, the default factory dials the configured RPC and
	// constructs a real *registry.EVMRegistryContract.
	ContractFactory func(ctx context.Context, opts ContractFactoryOpts) (registry.RegistryContract, error)

	// TopicAdapter is the bus the seller uses for 008 negotiation when
	// --commerce-mode=full. When nil, a fresh MemoryTopicAdapter is
	// created (Stage 2 demo default). Stage 2b will swap in an HCS
	// adapter via this seam.
	TopicAdapter topic.TopicAdapter

	// EscrowAdapter is the seller-side escrow used to RequestRelease /
	// ApproveRelease during the Stage 2 full-commerce settlement phase.
	// When nil, a fresh MemoryEscrow is created.
	EscrowAdapter payment.EscrowAdapter

	// SkipServe, when true, causes run() to return after registry
	// registration without entering the libp2p serve loop. Tests set
	// this so they can assert registration evidence without keeping
	// goroutines alive.
	SkipServe bool

	// SignalCh, when non-nil, replaces the default SIGINT/SIGTERM
	// notifier. Tests pass a channel they control to shut down the
	// serve loop deterministically.
	SignalCh <-chan os.Signal

	// HeartbeatInterval, when > 0, overrides the heartbeat publisher's
	// default 5s cadence. Tests pass a short interval (e.g. 50ms) so
	// the first heartbeat lands fast enough to assert on. Stage 3C.
	HeartbeatInterval time.Duration
}

// ContractFactoryOpts is what the CLI hands to the contract factory once
// it has parsed flags + env.
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

// run is the testable entry point. It parses args/env, dispatches to
// fixture-direct or registry-backed mode, and returns an exit code.
func run(args []string, env map[string]string, stdout, stderr io.Writer, deps Deps) int {
	fs := flag.NewFlagSet("remoteid-seller", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		mode = fs.String("mode", modeFixtureDirect, "operation mode: fixture-direct | eip8004-registry")
		// 0.0.0.0 binds all interfaces; delivery.FilterPublicMultiaddrs prunes
		// loopback/Docker/RFC1918 before any ConnectionSetup is built (HCS 1024-byte
		// limit). Operators MAY narrow to a specific IP at deploy time.
		listen      = fs.String("listen", "/ip4/0.0.0.0/udp/0/quic-v1", "libp2p listen multiaddr")
		replayPath  = fs.String("replay", "", "fixture JSON file path; mutually exclusive with --synth and --ds400")
		useSynth    = fs.Bool("synth", false, "use synthetic-orbit source")
		synthDrones = fs.Int("synth-drones", 2, "synthetic drone count (with --synth)")
		synthFPS    = fs.Int("synth-fps", 2, "aggregate frame rate across all drones (with --synth)")
		speedup     = fs.Float64("speedup", 1.0, "replay speedup factor (with --replay)")
		loop        = fs.Bool("loop", false, "restart replay from beginning on EOF (with --replay)")
		keyHex      = fs.String("key-hex", "", "32-byte hex secp256k1 private key; defaults to ephemeral")

		ds400Transport = fs.String("ds400-transport", "", "DS-400 transport: udp|tcp|http (Phase-3 stub)")
		ds400Address   = fs.String("ds400-address", "", "DS-400 endpoint address (host:port or URL)")
		ds400Source    = fs.String("ds400-source", "", "override DecodedFrame.Source label")

		// DroneScout MQTT source (Stage D, 2026-05-14). --mqtt-url
		// presence selects the source — mutually exclusive with
		// --replay / --synth / --ds400-transport. Password is read from
		// env (--mqtt-password-env names the variable) so it never
		// appears on the command line or in logs.
		mqttURL         = fs.String("mqtt-url", "", "DroneScout MQTT broker URL (e.g., tcp://broker:1883, ssl://broker:8883); selects DroneScout MQTT source")
		mqttTopic       = fs.String("mqtt-topic", "#", "MQTT topic filter (default '#' — all topics)")
		mqttClientID    = fs.String("mqtt-client-id", "", "MQTT client ID (default: auto-generated random)")
		mqttUsername    = fs.String("mqtt-username", "", "MQTT username (optional)")
		mqttPasswordEnv = fs.String("mqtt-password-env", "", "Env-var NAME holding the MQTT password; NEVER the password itself")
		mqttCompression = fs.String("mqtt-compression", "none", "DroneScout payload compression: 'none' only (Stage C-lite)")
		mqttTLS         = fs.Bool("mqtt-tls", false, "Metadata only — paho infers TLS from URL scheme; this flag is for operator-side disclosure")
		mqttSensorModel = fs.String("mqtt-sensor-model", "dronescout-ds240", "Sensor model tag stamped onto DecodedFrame.Source")

		registryAddrFlag   = fs.String("registry-address", "", "EIP-8004 Identity Registry contract address (registry mode)")
		rpcURL             = fs.String("rpc-url", "", "EVM JSON-RPC endpoint (registry mode; defaults to env HEDERA_EVM_RPC)")
		chainIDFlag        = fs.Uint64("chain-id", 0, "EVM chain id (registry mode; defaults to 296)")
		escrowContractFlag = fs.String("escrow-contract", "", "optional escrow contract address; embedded in commerce settlement.config")
		commerceMode       = fs.String("commerce-mode", "", "FR-P58 disclosure: full|registration-only|data-only (default registration-only in registry mode)")
		feedSourceFlag     = fs.String("feed-source", "", "FR-R15 disclosure: live|replay|synthetic|placeholder (default auto-derived)")
		topicBackend       = fs.String("topic-backend", "memory", "Stage-2b topic adapter: memory (in-process). Stage 3A adds 'hcs'. No silent fallback — see runbook.")
		escrowBackend      = fs.String("escrow-backend", "memory", "Stage-2b escrow adapter: memory (in-process). Stage 3A adds 'evm'. No silent fallback.")

		// VPS-1 "fake DroneScout DS240" BaseStation TCP source. Reads
		// the BlueMark neuron-rid-bridge's SBS export (MSG,1/3/4 for
		// FF-prefixed drones, MSG,1/2 for FE-prefixed operators) and
		// adapts it to the canonical DecodedFrame shape via the
		// single-drone pairing cache in
		// internal/feeds/remoteid.RunBasestation.
		basestationHost           = fs.String("basestation-tcp-host", "", "BaseStation TCP host:port (e.g. 127.0.0.1:30003) — selects the VPS-1 fake-DS240 BaseStation source; mutually exclusive with --replay / --synth / --ds400 / --mqtt-url")
		basestationSourceLabel    = fs.String("basestation-source-label", "", "Override the DecodedFrame.Source label stamped onto frames from the BaseStation source (default 'basestation-tcp-synthetic')")
		advertiseBasestationProto = fs.Bool("advertise-basestation-protocol", false, "When --basestation-tcp-host is set, also advertise /ds240/basestation/1.0.0 alongside /ds240/raw/1.0.0 (default: true when --basestation-tcp-host is set, false otherwise)")
	)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "remoteid-seller: parse flags:", err)
		return 2
	}

	logger := log.New(stderr, "", log.LstdFlags|log.Lmicroseconds)

	// Source selection — shared between both modes.
	chosen := 0
	if *replayPath != "" {
		chosen++
	}
	if *useSynth {
		chosen++
	}
	if *ds400Transport != "" || *ds400Address != "" {
		chosen++
	}
	if *mqttURL != "" {
		chosen++
	}
	if *basestationHost != "" {
		chosen++
	}
	if chosen == 0 {
		logger.Printf("remoteid-seller: pick exactly one source: --replay <path> OR --synth OR --ds400-transport=<udp|tcp|http> --ds400-address=<addr> OR --mqtt-url=<url> OR --basestation-tcp-host=<host:port>")
		return 2
	}
	if chosen > 1 {
		logger.Printf("remoteid-seller: --replay, --synth, --ds400-*, --mqtt-url, and --basestation-tcp-host are mutually exclusive")
		return 2
	}
	if (*ds400Transport != "" && *ds400Address == "") || (*ds400Transport == "" && *ds400Address != "") {
		logger.Printf("remoteid-seller: --ds400-transport and --ds400-address must both be set")
		return 2
	}

	// Auto-derive --advertise-basestation-protocol default: true when
	// --basestation-tcp-host is set AND the operator did not pass the
	// flag explicitly (false in all other cases). The Go flag package
	// does not expose "was the flag explicitly set" without
	// fs.Visit walking; the simplest equivalent is to flip the flag
	// here when basestationHost is set, recognising that an operator
	// who explicitly typed `--advertise-basestation-protocol=false`
	// loses their override. We accept that — operators who want to
	// suppress basestation advertisement should not set
	// --basestation-tcp-host at all, or use the cmd-test
	// invocation that drives Deps directly.
	if *basestationHost != "" && !*advertiseBasestationProto {
		explicitlySet := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "advertise-basestation-protocol" {
				explicitlySet = true
			}
		})
		if !explicitlySet {
			*advertiseBasestationProto = true
		}
	}

	// Mode validation.
	switch *mode {
	case modeFixtureDirect, modeEIP8004Registry:
		// ok
	default:
		logger.Printf("remoteid-seller: unknown --mode=%q (expected fixture-direct or eip8004-registry)", *mode)
		return 2
	}

	// Backend selection (Stage 2b: memory; Stage 3A: hcs + evm).
	switch *topicBackend {
	case "memory", "hcs":
		// ok — actual construction happens in runRegister.
	default:
		logger.Printf("remoteid-seller: unknown --topic-backend=%q (expected: memory | hcs)", *topicBackend)
		return 2
	}
	switch *escrowBackend {
	case "memory", "evm":
		// ok — actual construction happens in runRegister.
	default:
		logger.Printf("remoteid-seller: unknown --escrow-backend=%q (expected: memory | evm)", *escrowBackend)
		return 2
	}

	// Stage 3A pre-flight: explicit no-silent-fallback checks for the
	// real backends. Each check produces the exit-2 message the operator
	// needs to fix the .env BEFORE any other initialization.
	envLookup := func(key string) string { return env[key] }
	if *topicBackend == "hcs" {
		if missing := remoteid.MissingHCSEnvVars(envLookup); len(missing) > 0 {
			logger.Printf("remoteid-seller: --topic-backend=hcs requires env %v — refusing to fall back to memory; see runbook §4 'HCS + EVM testnet'", missing)
			return 2
		}
	}
	if *escrowBackend == "evm" {
		if missing := remoteid.MissingEVMEnvVars(envLookup); len(missing) > 0 {
			logger.Printf("remoteid-seller: --escrow-backend=evm requires env %v — refusing to fall back to memory; see runbook §4 'HCS + EVM testnet'", missing)
			return 2
		}
	}

	privKey, err := resolveKey(*keyHex)
	if err != nil {
		logger.Printf("remoteid-seller: resolve key: %v", err)
		return 2
	}

	feedSource := *feedSourceFlag
	if feedSource == "" {
		feedSource = deriveFeedSource(*replayPath, *useSynth, *ds400Transport, *mqttURL, *basestationHost)
	}

	// Stage 3B: when --mode=eip8004-registry + --commerce-mode=full and
	// we WILL serve (i.e. not SkipServe), build the libp2p host BEFORE
	// register so the descriptor's NeuronP2PExchangeService can carry
	// the seller's live listen multiaddrs. The buyer reads those out
	// of the AgentURI and dials without --seller-multiaddr.
	//
	// Stage 1/2 path (SkipServe or commerce-mode=registration-only)
	// keeps the historical ordering: register first, host second. That
	// path leaves Multiaddrs empty on the descriptor.
	var (
		host            host.Host
		hostBuiltEarly  bool
		earlyMultiaddrs []string
	)
	preBuildHost := *mode == modeEIP8004Registry &&
		*commerceMode == remoteid.CommerceModeFull &&
		!deps.SkipServe
	if preBuildHost {
		host, err = delivery.NewLibp2pHost(privKey, *listen)
		if err != nil {
			logger.Printf("remoteid-seller: build host: %v", err)
			return 2
		}
		hostBuiltEarly = true
		filtered := delivery.FilterMultiaddrs(host.Addrs())
		if len(filtered) == 0 {
			// Fall back to the unfiltered set when filtering would yield
			// zero addrs (loopback-only test hosts). For a real testnet
			// run with non-loopback --listen, filtering preserves the
			// useful subset.
			filtered = host.Addrs()
		}
		earlyMultiaddrs = make([]string, len(filtered))
		for i, ma := range filtered {
			earlyMultiaddrs[i] = ma.String()
		}
		if len(earlyMultiaddrs) == 0 {
			logger.Printf("remoteid-seller: --commerce-mode=full requires at least one libp2p listen multiaddr (host.Addrs() returned 0). Set --listen=/ip4/<addr>/udp/<port>/quic-v1 explicitly.")
			_ = host.Close()
			return 2
		}
		logger.Printf("[seller] libp2p host ready peerID=%s multiaddrs=%d", host.ID(), len(earlyMultiaddrs))
	}

	// Register BEFORE binding the libp2p socket (Stage 1/2 path) or
	// AFTER (Stage 3B path with pre-built host). Either way, runRegister
	// only depends on the secp256k1 key + (optionally) the multiaddrs.
	var regOutcome registerOutcome
	if *mode == modeEIP8004Registry {
		regOutcome = runRegister(registerArgs{
			env:            env,
			logger:         logger,
			deps:           deps,
			privKey:        privKey,
			registryHex:    *registryAddrFlag,
			rpcURLFlag:     *rpcURL,
			chainIDFlag:    *chainIDFlag,
			escrowContract: *escrowContractFlag,
			commerceMode:   *commerceMode,
			feedSource:     feedSource,
			topicBackend:   *topicBackend,
			escrowBackend:  *escrowBackend,
			multiaddrs:     earlyMultiaddrs,
		})
		_ = regOutcome.escrowBinding // referenced via the session goroutine below
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
			logger.Printf("remoteid-seller: build host: %v", err)
			return 2
		}
	}
	defer host.Close()

	source := chooseSource(sourceParams{
		replayPath:             *replayPath,
		useSynth:               *useSynth,
		speedup:                *speedup,
		loop:                   *loop,
		synthDrones:            *synthDrones,
		synthFPS:               *synthFPS,
		ds400Transport:         *ds400Transport,
		ds400Address:           *ds400Address,
		ds400Source:            *ds400Source,
		mqttURL:                *mqttURL,
		mqttTopic:              *mqttTopic,
		mqttClientID:           *mqttClientID,
		mqttUsername:           *mqttUsername,
		mqttPasswordEnv:        *mqttPasswordEnv,
		mqttCompression:        *mqttCompression,
		mqttTLS:                *mqttTLS,
		mqttSensorModel:        *mqttSensorModel,
		basestationHost:        *basestationHost,
		basestationSourceLabel: *basestationSourceLabel,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sellerCfg := remoteid.SellerConfig{
		Host:   host,
		Source: source,
		Logger: logger,
	}
	if *advertiseBasestationProto {
		// Multi-protocol: both raw and basestation. The first entry
		// becomes SellerRunningContext.Protocol for back-compat.
		sellerCfg.ProtocolIDs = []string{remoteid.ProtocolRaw, remoteid.ProtocolBasestation}
		logger.Printf("[seller] advertising 2 stream protocols: %v", sellerCfg.ProtocolIDs)
	}

	running, err := remoteid.Start(ctx, sellerCfg)
	if err != nil {
		logger.Printf("remoteid-seller: start: %v", err)
		return 1
	}

	logger.Printf("ready: peerID=%s mode=%s", host.ID(), *mode)
	advertise := delivery.FilterMultiaddrs(host.Addrs())
	if len(advertise) == 0 {
		advertise = host.Addrs()
	}
	for _, a := range advertise {
		fmt.Fprintf(stdout, "%s/p2p/%s\n", a, host.ID())
	}

	// Stage 3C: start the heartbeat publisher after the libp2p host is
	// up so we know the seller's PeerID. Lifetime is the seller PROCESS
	// (outer ctx), not the per-buyer session — a buyer observing
	// liveness keeps receiving signals after one commerce session ends.
	// Only enabled when commerce-mode=full (which is when stdOut topic
	// exists); registration-only mode has no publish target.
	var heartbeatLoop *remoteid.HeartbeatLoop
	if regOutcome.adapter != nil && regOutcome.sellerStdOut.Locator() != "" {
		ck := regOutcome.childKey
		hbLoop, hbErr := remoteid.StartHeartbeatLoop(ctx, remoteid.HeartbeatLoopOptions{
			Key:            &ck,
			StdOutRef:      regOutcome.sellerStdOut,
			Adapter:        regOutcome.adapter,
			Descriptor:     regOutcome.descriptor,
			Interval:       deps.HeartbeatInterval,
			Logger:         logger,
			SellerEVM:      ck.PublicKey().EVMAddress().Hex(),
			SellerPeerID:   host.ID().String(),
			ServiceName:    remoteid.CommerceServiceName,
			TopicBackend:   regOutcome.topicBackend,
			EscrowBackend:  regOutcome.escrowBackend,
			AgentURISha256: regOutcome.agentURISha256,
			// Stage 3C ships DegradedFunc as a stub returning false. Stage
			// 3D will wire real signals (frame-source errors, libp2p
			// stream death, escrow timeout proximity).
			DegradedFunc: func() bool { return false },
		})
		if hbErr != nil {
			logger.Printf("remoteid-seller: start heartbeat loop: %v", hbErr)
			running.Cancel()
			<-running.Done
			return 1
		}
		heartbeatLoop = hbLoop
		logger.Printf("[heartbeat] loop started interval=%v sellerEVM=%s sellerPeerID=%s topicBackend=%s escrowBackend=%s",
			func() time.Duration {
				if deps.HeartbeatInterval > 0 {
					return deps.HeartbeatInterval
				}
				return 5 * time.Second
			}(),
			ck.PublicKey().EVMAddress().Hex(), host.ID().String(),
			regOutcome.topicBackend, regOutcome.escrowBackend)
	}

	// Stage 2b: when commerce-mode=full, drive the per-buyer commerce
	// session in a goroutine. The session subscribes to the seller's
	// stdIn topic, accepts one ServiceRequest, drives the lifecycle to
	// COMPLETED, then exits. The goroutine's result lands on sessionCh
	// so the main loop can surface it on shutdown.
	sessionCh := make(chan remoteid.SellerSessionResult, 1)
	// Stage 3C: sessionDone lets run() wait for the session goroutine's
	// log writes to flush before returning. Otherwise tests under -race
	// can observe the goroutine still writing to logger.Output after
	// run() returns (the goroutine writes its `[seller-session] error`
	// line during ctx-cancel teardown, racing the test's stderr read).
	sessionDone := make(chan struct{})
	hasSessionGoroutine := false
	if regOutcome.adapter != nil {
		hasSessionGoroutine = true
		ck := regOutcome.childKey // capture by value
		go func() {
			defer close(sessionDone)
			result, err := remoteid.RunSellerCLISession(ctx, remoteid.SellerCLIOptions{
				Key:           &ck,
				Adapter:       regOutcome.adapter,
				SellerStdIn:   regOutcome.sellerStdIn,
				Descriptor:    regOutcome.descriptor,
				Host:          host,
				Escrow:        regOutcome.escrow,
				EscrowBinding: regOutcome.escrowBinding,
				Mode:          remoteid.CommerceModeFull,
				Logger:        logger,
				FrameSummary: func() (uint64, uint64, uint64) {
					// Stage 2b/3A: frame counts in evidenceHash are
					// placeholder. Live counter wires in Stage 4+.
					return 0, 0, 0
				},
			})
			if err != nil {
				logger.Printf("[seller-session] error: %v", err)
			}
			// Best-effort send; the main select may have already moved on
			// via SIGINT, in which case nobody's reading sessionCh — but
			// the channel is buffered (cap=1), so this never blocks.
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
	// Stage 3C: cancel the outer ctx so the heartbeat loop (bound to it)
	// can exit, then wait for its Done. running.Cancel only ends the
	// libp2p serve loop; the heartbeat ticker watches the outer ctx.
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

// registerArgs bundles the inputs for the registry-mode branch.
type registerArgs struct {
	env            map[string]string
	logger         *log.Logger
	deps           Deps
	privKey        *ecdsa.PrivateKey
	registryHex    string
	rpcURLFlag     string
	chainIDFlag    uint64
	escrowContract string
	commerceMode   string
	feedSource     string
	topicBackend   string
	escrowBackend  string
	// multiaddrs (Stage 3B) is the seller's filtered libp2p listen
	// multiaddr list. Embedded in the descriptor's
	// NeuronP2PExchangeService so registry-backed buyers can dial
	// without out-of-band coordination. Empty in Stage 1/2 paths.
	multiaddrs []string
}

// registerOutcome carries the registration outputs the run() loop needs
// to wire the Stage-2b/3A commerce-mode=full orchestrator goroutine after
// the libp2p host is up. exitCode != 0 means the caller should return
// immediately. When commerceMode != "full" all session-related fields
// are zero.
type registerOutcome struct {
	exitCode      int
	adapter       topic.TopicAdapter
	escrow        payment.EscrowAdapter
	escrowBinding string // "memory" or "evm-escrow"
	sellerStdIn   topic.TopicRef
	sellerStdOut  topic.TopicRef // Stage 3C: heartbeat publish target
	descriptor    remoteid.ServiceDescriptor
	childKey      keylib.NeuronPrivateKey
	// agentURISha256 is the SHA-256 of the registered AgentURI JSON.
	// Surfaced via the heartbeat's Capabilities.Operational so buyers
	// can cross-check (Stage 3C, FR-R21 defence-in-depth).
	agentURISha256 string
	// topicBackend / escrowBackend echo the operator's CLI flag choices
	// onto the heartbeat's Operational disclosure block.
	topicBackend  string
	escrowBackend string
}

// Compile-time references to silence unused-import warnings when the
// Stage 2 orchestrator hooks land. The packages are imported because
// Deps fields reference their types.
var (
	_ topic.TopicAdapter    = (*topic.MemoryTopicAdapter)(nil)
	_ payment.EscrowAdapter = (*payment.MemoryEscrow)(nil)
)

// runRegister handles the eip8004-registry mode pre-serve registration
// step. Returns an outcome carrying the registration artefacts (adapter,
// escrow, topic refs, descriptor, child key) when commerce-mode=full, so
// the caller can spawn the Stage-2b session goroutine after the libp2p
// host is up. exitCode != 0 means the caller should propagate it.
func runRegister(a registerArgs) registerOutcome {
	fail := func(code int) registerOutcome { return registerOutcome{exitCode: code} }

	if a.registryHex == "" {
		a.logger.Printf("remoteid-seller: --mode=eip8004-registry requires --registry-address <0x...>")
		return fail(2)
	}
	registryAddr, err := keylib.EVMAddressFromHex(a.registryHex)
	if err != nil {
		a.logger.Printf("remoteid-seller: invalid --registry-address %q: %v", a.registryHex, err)
		return fail(2)
	}

	chainID := a.chainIDFlag
	if chainID == 0 {
		if v := a.env["NEURON_CHAIN_ID"]; v != "" {
			n, perr := strconv.ParseUint(v, 10, 64)
			if perr != nil {
				a.logger.Printf("remoteid-seller: NEURON_CHAIN_ID=%q: %v", v, perr)
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
		commerceMode = remoteid.CommerceModeRegistrationOnly
	}

	childKey, err := keylib.NeuronPrivateKeyFromBytes(ethcrypto.FromECDSA(a.privKey))
	if err != nil {
		a.logger.Printf("remoteid-seller: wrap signing key: %v", err)
		return fail(1)
	}

	// Stage 2b/3A: when commerce-mode=full, build the topic adapter +
	// escrow + 3 topics BEFORE the descriptor so the AgentURI carries
	// resolvable topic IDs for the buyer.
	var (
		adapter       topic.TopicAdapter
		escrow        payment.EscrowAdapter
		escrowBinding = "memory"
		topicConfig   map[string]map[string]any
		sellerStdIn   topic.TopicRef
		sellerStdOut  topic.TopicRef // Stage 3C: heartbeat publish target
	)
	if commerceMode == remoteid.CommerceModeFull {
		ctx := context.Background()
		envLookup := func(key string) string { return a.env[key] }
		// --- topic adapter ---
		switch a.topicBackend {
		case "hcs":
			be, herr := remoteid.NewHCSBackend(ctx, remoteid.HCSBackendOptions{
				Role:            remoteid.HCSRoleSeller,
				LookupEnv:       envLookup,
				TopicMemoPrefix: "remoteid-" + childKey.PublicKey().EVMAddress().Hex()[2:10] + "-",
			})
			if herr != nil {
				a.logger.Printf("remoteid-seller: %v", herr)
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
			inRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-stdin-" + topicSuffix})
			if err != nil {
				a.logger.Printf("remoteid-seller: create stdIn topic: %v", err)
				return fail(1)
			}
			outRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-stdout-" + topicSuffix})
			if err != nil {
				a.logger.Printf("remoteid-seller: create stdOut topic: %v", err)
				return fail(1)
			}
			errRef, err = adapter.CreateTopic(topic.CreateTopicOpts{Memo: "remoteid-stderr-" + topicSuffix})
			if err != nil {
				a.logger.Printf("remoteid-seller: create stdErr topic: %v", err)
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
		// --- escrow adapter ---
		switch a.escrowBackend {
		case "evm":
			be, eerr := remoteid.NewEVMBackend(ctx, remoteid.EVMBackendOptions{
				LookupEnv:      envLookup,
				DefaultRPCURL:  defaultRPCURL,
				DefaultChainID: defaultChainID,
			})
			if eerr != nil {
				a.logger.Printf("remoteid-seller: %v", eerr)
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

	descriptor, err := remoteid.BuildServiceDescriptor(remoteid.DescriptorOptions{
		ChildKey:       &childKey,
		ChainID:        chainID,
		EscrowContract: a.escrowContract,
		FeedSource:     a.feedSource,
		CommerceMode:   commerceMode,
		ProfileID:      remoteid.ProfileR1,
		TopicConfig:    topicConfig,
		Multiaddrs:     a.multiaddrs,
	})
	if err != nil {
		a.logger.Printf("remoteid-seller: build descriptor: %v", err)
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
		a.logger.Printf("remoteid-seller: build registry contract: %v", err)
		return fail(1)
	}

	result, err := remoteid.RegisterSeller(ctx, &childKey, descriptor, registryAddr, chainID, contract)
	if err != nil {
		a.logger.Printf("remoteid-seller: register: %v", err)
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
		remoteid.CommerceServiceName,
		remoteid.CommerceServiceVersion,
		remoteid.PricingUnit,
		descriptor.CommerceMode,
		descriptor.FeedSource,
		descriptor.ProfileID,
	)
	if commerceMode == remoteid.CommerceModeFull {
		a.logger.Printf("[seller] commerce-mode=full topics created: stdIn=%s stdOut=%s stdErr=%s — orchestrator will spawn after libp2p host is up",
			topicConfig["stdIn"]["topicId"], topicConfig["stdOut"]["topicId"], topicConfig["stdErr"]["topicId"])
		if len(a.multiaddrs) > 0 {
			a.logger.Printf("[seller] AgentURI carries %d multiaddrs for Stage-3B registry-only discovery: %v",
				len(a.multiaddrs), a.multiaddrs)
		} else {
			a.logger.Printf("[seller] WARNING: AgentURI has no multiaddrs — buyer must fall back to --seller-multiaddr (use Stage 3B by passing --listen for non-loopback addrs)")
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

// deriveFeedSource maps the CLI's source-selection flags to the FR-R14
// feedSource enum. Live is reserved for the DS-400 source today.
//
// The DroneScout MQTT source intentionally maps to "replay" by default,
// not "live": the source layer cannot tell whether the broker on the
// other end is a real ds240 sensor or an in-process broker echoing
// fixtures. An operator who has independently verified a real-sensor
// run opts in to the live claim via `--feed-source=live`. See
// `internal/feeds/remoteid/dronescout_mqtt_source.go` for the
// source-side discipline.
//
// The VPS-1 BaseStation TCP source maps to "replay" for the same
// reason — the BlueMark neuron-rid-bridge is a Python simulator
// pretending to be a DroneScout DS240, never a real sensor. An
// operator with a verified real-DS240 swap-in passes
// `--feed-source=live` to override.
func deriveFeedSource(replayPath string, useSynth bool, ds400Transport string, mqttURL string, basestationHost string) string {
	switch {
	case replayPath != "":
		return remoteid.FeedSourceReplay
	case useSynth:
		return remoteid.FeedSourceSynthetic
	case ds400Transport != "":
		return remoteid.FeedSourceLive
	case mqttURL != "":
		return remoteid.FeedSourceReplay
	case basestationHost != "":
		return remoteid.FeedSourceReplay
	default:
		return remoteid.FeedSourceLive
	}
}

type sourceParams struct {
	replayPath     string
	useSynth       bool
	speedup        float64
	loop           bool
	synthDrones    int
	synthFPS       int
	ds400Transport string
	ds400Address   string
	ds400Source    string

	// DroneScout MQTT (Stage D). When mqttURL != "", chooseSource
	// returns a FeedSource that calls RunDroneScoutMQTT.
	mqttURL         string
	mqttTopic       string
	mqttClientID    string
	mqttUsername    string
	mqttPasswordEnv string
	mqttCompression string
	mqttTLS         bool
	mqttSensorModel string

	// VPS-1 fake-DS240 BaseStation source (reference demo, plan §"Step 6").
	// When basestationHost != "", chooseSource returns a FeedSource
	// that calls RunBasestation.
	basestationHost        string
	basestationSourceLabel string
}

func chooseSource(p sourceParams) remoteid.FeedSource {
	switch {
	case p.basestationHost != "":
		cfg := remoteidsrc.BasestationConfig{
			HostPort:    p.basestationHost,
			SourceLabel: p.basestationSourceLabel,
		}
		return func(ctx context.Context, out chan<- remoteidsrc.DecodedFrame) error {
			return remoteidsrc.RunBasestation(ctx, cfg, out)
		}
	case p.mqttURL != "":
		cfg := remoteidsrc.DroneScoutMQTTConfig{
			URL:         p.mqttURL,
			Topic:       p.mqttTopic,
			ClientID:    p.mqttClientID,
			Username:    p.mqttUsername,
			PasswordEnv: p.mqttPasswordEnv,
			Compression: p.mqttCompression,
			TLS:         p.mqttTLS,
			SensorModel: p.mqttSensorModel,
			// OnReady deliberately nil in the CLI path — it's a
			// test-only synchronisation hook.
		}
		return func(ctx context.Context, out chan<- remoteidsrc.DecodedFrame) error {
			return remoteidsrc.RunDroneScoutMQTT(ctx, cfg, out)
		}
	case p.ds400Transport != "":
		cfg := remoteidsrc.DS400Config{
			Transport: remoteidsrc.DS400Transport(p.ds400Transport),
			Address:   p.ds400Address,
			Source:    p.ds400Source,
		}
		return func(ctx context.Context, out chan<- remoteidsrc.DecodedFrame) error {
			return remoteidsrc.RunDS400(ctx, cfg, out)
		}
	case p.useSynth:
		return remoteid.SynthSource(remoteidsrc.SynthOptions{
			FPS:        p.synthFPS,
			DroneCount: p.synthDrones,
		})
	default:
		return remoteid.ReplaySource(p.replayPath, remoteidsrc.ReplayOptions{
			Speedup: p.speedup,
			Loop:    p.loop,
		})
	}
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
