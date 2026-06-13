// edge-validator is the standalone spec-010 validator agent for the
// reverse-connect demo. It subscribes to a seller's stdIn + a buyer's
// stdIn, captures the 5-message protocol transcript per agreement
// (serviceRequest → serviceResponse → escrowCreated → invoice →
// invoiceAck), and publishes two EvidenceEnvelopes per completed
// agreement: one against spec 008 (payment), one against spec 009
// (delivery, currently inconclusive — iteration 4 has no
// stream-bytes hook).
//
// Run modes:
//
//	--mode=mock     in-memory bus; only useful for in-process tests
//	                where the seller, buyer, and validator share the
//	                same MemoryBus. The standalone binary has no way
//	                to attach to another process's MemoryBus, so this
//	                mode mostly exists for symmetry with edge-seller.
//	--mode=testnet  real Hedera testnet HCS via topic.NewTestnetClientFromEnv
//	                + RealHCSClient. Required env vars: HEDERA_OPERATOR_ID,
//	                HEDERA_OPERATOR_KEY.
//
// Required env / flags (in addition to mode):
//
//	NEURON_EDGE_VALIDATOR_PRIVATE_KEY   secp256k1 hex (32 bytes, no 0x prefix)
//	                                    (separate from operator key — the
//	                                    validator's signing identity).
//	NEURON_EDGE_VALIDATOR_STDOUT        validator's own stdOut topic locator
//	                                    (e.g. 0.0.X). Validator publishes
//	                                    EvidenceEnvelopes here. Required.
//	NEURON_EDGE_VALIDATOR_AGENT_ID      validator's EIP-8004 tokenID as a
//	                                    decimal UnsignedInt256 string.
//	                                    Default "1" (mock).
//	NEURON_EDGE_VALIDATOR_SUBJECT_ID    subject (seller) tokenID, decimal.
//	                                    Default "2" (mock).
//	NEURON_EDGE_SELLER_STDIN            seller's stdIn topic locator. Required.
//	NEURON_EDGE_BUYER_STDIN             buyer's stdIn topic locator. Required.
//	NEURON_EDGE_VALIDATOR_EVIDENCE_URI  prefix for the evidenceURI field
//	                                    (default "memory://"). Use a real
//	                                    HTTPS / IPFS prefix in production.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/neuron-sdk/neuron-go-sdk/internal/edgeapp"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

func main() {
	mode := flag.String("mode", "testnet", "Bus mode: testnet (real HCS) or mock (in-memory)")
	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)

	priv, err := loadValidatorKey()
	if err != nil {
		logger.Fatalf("load validator private key: %v", err)
	}
	pub := priv.PublicKey()
	logger.Printf("validator identity: evm=%s", pub.EVMAddress().Hex())

	bus, err := makeBus(*mode, logger)
	if err != nil {
		logger.Fatalf("build bus (%s): %v", *mode, err)
	}

	validatorStdOut, err := resolveOrCreateValidatorStdOut(bus, logger)
	if err != nil {
		logger.Fatalf("validator stdOut: %v", err)
	}
	sellerStdIn, err := mustParseTopicRef("NEURON_EDGE_SELLER_STDIN", bus.SupportedTransport())
	if err != nil {
		logger.Fatalf("seller stdIn: %v", err)
	}
	buyerStdIn, err := mustParseTopicRef("NEURON_EDGE_BUYER_STDIN", bus.SupportedTransport())
	if err != nil {
		logger.Fatalf("buyer stdIn: %v", err)
	}

	cfg := edgeapp.ValidatorConfig{
		Bus:               bus,
		Key:               &priv,
		ValidatorStdOut:   validatorStdOut,
		ValidatorAgentID:  envOr("NEURON_EDGE_VALIDATOR_AGENT_ID", "1"),
		SubjectAgentID:    envOr("NEURON_EDGE_VALIDATOR_SUBJECT_ID", "2"),
		SellerStdIn:       sellerStdIn,
		BuyerStdIn:        buyerStdIn,
		EvidenceURIPrefix: envOr("NEURON_EDGE_VALIDATOR_EVIDENCE_URI", "memory://"),
		// Replay defaults to true — the standalone validator is a third-party
		// witness that typically joins after agreement messages are already
		// on-topic. Set NEURON_EDGE_VALIDATOR_REPLAY=false to opt out (e.g.
		// for a long-running validator that should only see live messages).
		Replay: envOr("NEURON_EDGE_VALIDATOR_REPLAY", "true") != "false",
		Logger: logger,
	}
	v, err := edgeapp.NewValidator(cfg)
	if err != nil {
		logger.Fatalf("validator: %v", err)
	}

	logger.Printf("validator: subscribing to seller.stdIn=%s buyer.stdIn=%s; publishing on %s",
		sellerStdIn.Locator(), buyerStdIn.Locator(), validatorStdOut.Locator())
	logger.Printf("validator: validatorAgentID=%s subjectAgentID=%s evidenceURIPrefix=%s",
		cfg.ValidatorAgentID, cfg.SubjectAgentID, cfg.EvidenceURIPrefix)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := v.Run(ctx); err != nil && err != context.Canceled {
		logger.Fatalf("validator: %v", err)
	}
	logger.Printf("validator: graceful shutdown")
}

// loadValidatorKey reads NEURON_EDGE_VALIDATOR_PRIVATE_KEY (preferred)
// or falls back to NEURON_EDGE_PRIVATE_KEY for symmetry with edge-seller
// / edge-buyer when an operator wants to share signing keys.
func loadValidatorKey() (keylib.NeuronPrivateKey, error) {
	hexKey := strings.TrimSpace(strings.TrimPrefix(os.Getenv("NEURON_EDGE_VALIDATOR_PRIVATE_KEY"), "0x"))
	if hexKey == "" {
		hexKey = strings.TrimSpace(strings.TrimPrefix(os.Getenv("NEURON_EDGE_PRIVATE_KEY"), "0x"))
	}
	if hexKey == "" {
		return keylib.NeuronPrivateKey{}, fmt.Errorf("NEURON_EDGE_VALIDATOR_PRIVATE_KEY (or NEURON_EDGE_PRIVATE_KEY) required")
	}
	bs, err := hex.DecodeString(hexKey)
	if err != nil {
		return keylib.NeuronPrivateKey{}, fmt.Errorf("decode validator key: %w", err)
	}
	return keylib.NeuronPrivateKeyFromBytes(bs)
}

func makeBus(mode string, logger *log.Logger) (topic.TopicAdapter, error) {
	switch mode {
	case "mock":
		logger.Printf("WARNING: --mode=mock — validator will subscribe to a fresh in-process MemoryBus; standalone use is rarely useful in this mode")
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

func mustParseTopicRef(envKey string, transport topic.BackendKind) (topic.TopicRef, error) {
	loc := strings.TrimSpace(os.Getenv(envKey))
	if loc == "" {
		return topic.TopicRef{}, fmt.Errorf("%s required (e.g. 0.0.123456)", envKey)
	}
	return topic.NewTopicRef(transport, loc)
}

// resolveOrCreateValidatorStdOut returns the stdOut topic ref the validator
// will publish evidence envelopes to. If NEURON_EDGE_VALIDATOR_STDOUT is set,
// the locator is parsed; otherwise the validator auto-creates a fresh topic
// on the configured bus and logs the locator so the operator can capture it
// for cross-process consumers.
//
// Auto-create mirrors edge-seller's startup behavior — the validator is the
// publisher of its own stdOut, so creating it on first launch is the natural
// shape. Operators who want a stable locator across restarts should either
// pre-create the topic + set the env var, or persist the locator from the
// log line on first run.
func resolveOrCreateValidatorStdOut(bus topic.TopicAdapter, logger *log.Logger) (topic.TopicRef, error) {
	if loc := strings.TrimSpace(os.Getenv("NEURON_EDGE_VALIDATOR_STDOUT")); loc != "" {
		return topic.NewTopicRef(bus.SupportedTransport(), loc)
	}
	ref, err := bus.CreateTopic(topic.CreateTopicOpts{
		Transport: bus.SupportedTransport(),
		Memo:      "edge-validator-stdOut",
	})
	if err != nil {
		return topic.TopicRef{}, fmt.Errorf("auto-create validator stdOut: %w", err)
	}
	logger.Printf("validator stdOut: AUTO-CREATED %s — set NEURON_EDGE_VALIDATOR_STDOUT to this for stable across restarts",
		ref.Locator())
	return ref, nil
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
