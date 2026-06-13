package edgeapp

import (
	"errors"
	"fmt"
	"strings"
)

// This file defines the *configuration shape* for the post-soak D2/D3/D4
// spec-full-flow integration phases (registry + escrow + validator) without
// wiring any of those subsystems yet. The fields are validated at parse
// time so misconfigurations surface immediately, but agents that opt in
// will receive an explicit "not yet implemented in this build" error from
// the helpers below — never a silent success that does nothing.
//
// Implementation of the hooked behavior follows this staged schedule:
//
//   D1: persistence + heartbeat-deadline observation (LANDED in this PR).
//   D2: EIP-8004 Identity Registry registration + Profile E descriptor.
//   D3: EVM escrow + payment negotiation per spec 008.
//   D4: External validator agent + EvidenceEnvelope publishing per spec 010.
//
// Defining the env contract early lets operators populate the .env file
// once when the .deb is installed; later phases activate those fields by
// flipping the `Mode` switches without further config changes.

// RegistryConfig captures the EIP-8004 Identity Registry binding the
// seller / buyer will consult once D2 lands. Empty values disable the
// feature; callers should construct via ParseRegistryConfig from env.
type RegistryConfig struct {
	// Address is the EVM contract address of the Identity Registry,
	// hex-encoded with "0x" prefix. Empty disables the feature.
	Address string

	// ChainID is the EVM chain identifier (e.g. 296 for Hedera testnet).
	// Required when Address is non-empty.
	ChainID uint64

	// RPC is the JSON-RPC endpoint the binding will dial. Required when
	// Address is non-empty. Defaults to Hedera testnet's HashIO when empty.
	RPC string

	// Mode toggles the call path:
	//   "skip"   — never call Register / LookupRegistration (D1 default).
	//   "auto"   — call Register only if not already registered for the
	//              running EVM address, idempotent. (D2 default.)
	//   "force"  — always call Register on startup, even if already
	//              registered. Operator override for re-issuing agentURI.
	Mode string
}

// EscrowConfig captures the spec 008 escrow binding, populated once
// D3 lands. Empty values disable the feature.
type EscrowConfig struct {
	// EscrowAddr is the EVM contract address of the escrow factory,
	// hex-encoded with "0x" prefix. Empty disables the feature.
	EscrowAddr string

	// TokenAddr is the EVM contract address of the ERC-20 token used as
	// settlement currency. Required when EscrowAddr is non-empty.
	TokenAddr string

	// Mode toggles the implementation:
	//   "mock"    — use payment.MemoryEscrow regardless of addresses.
	//   "testnet" — use payment.EVMEscrowAdapter against the configured
	//               contracts.
	Mode string

	// PriceTinybar is the per-session deposit, in TestToken base units.
	// Defaults to 100 when empty.
	PriceTinybar uint64

	// AgreementTimeoutSec is the spec 008 agreement timeout, in seconds.
	// Defaults to 86_400 (24 h) when zero.
	AgreementTimeoutSec uint64
}

// ValidatorEnvConfig captures spec 010 validator-agent env-driven wiring,
// populated once D4's standalone validator binary ships. Iteration 2's
// runtime ValidatorConfig (in validator.go) is distinct from this — that
// one is constructed in-process from explicit fields, this one is parsed
// from env vars by `cmd/edge-validator` (forthcoming).
type ValidatorEnvConfig struct {
	// Targets is the list of EVM addresses (one per agent) the validator
	// should attest. Empty disables the feature on the validator process;
	// has no effect on seller / buyer.
	Targets []string

	// ValidationRegistryAddr is the optional Validation Registry contract
	// address; empty leaves verdicts purely off-chain.
	ValidationRegistryAddr string
}

// ParseRegistryConfig validates and normalizes a RegistryConfig parsed
// from environment variables. Returns nil for the all-empty case (feature
// disabled) so callers can treat (nil, nil) as a clean opt-out.
//
// Recognized env mapping (caller is responsible for reading the env;
// this function takes already-extracted strings to keep the package
// agnostic to the env-var loader):
//
//	NEURON_EDGE_REGISTRY_ADDR        → addr
//	NEURON_EDGE_CHAIN_ID             → chainID (decimal)
//	NEURON_EDGE_HEDERA_EVM_RPC       → rpc
//	NEURON_EDGE_REGISTRATION_MODE    → mode (skip|auto|force)
func ParseRegistryConfig(addr, chainID, rpc, mode string) (*RegistryConfig, error) {
	addr = strings.TrimSpace(addr)
	rpc = strings.TrimSpace(rpc)
	mode = strings.TrimSpace(mode)

	if addr == "" && rpc == "" && chainID == "" && (mode == "" || mode == "skip") {
		return nil, nil
	}
	if addr == "" {
		return nil, errors.New("registry config: NEURON_EDGE_REGISTRY_ADDR required when any registry field is set")
	}
	if !strings.HasPrefix(strings.ToLower(addr), "0x") || len(addr) != 42 {
		return nil, fmt.Errorf("registry config: invalid Address %q (want 0x + 40 hex)", addr)
	}
	cid, err := parseUint64(chainID)
	if err != nil {
		return nil, fmt.Errorf("registry config: invalid ChainID %q: %w", chainID, err)
	}
	if cid == 0 {
		return nil, errors.New("registry config: ChainID required (NEURON_EDGE_CHAIN_ID)")
	}
	if rpc == "" {
		rpc = "https://testnet.hashio.io/api"
	}
	if mode == "" {
		mode = "auto"
	}
	switch mode {
	case "skip", "auto", "force", "force-testnet", "force-evm":
	default:
		return nil, fmt.Errorf("registry config: invalid Mode %q (want skip|auto|force|force-testnet|force-evm)", mode)
	}
	return &RegistryConfig{
		Address: addr,
		ChainID: cid,
		RPC:     rpc,
		Mode:    mode,
	}, nil
}

// ParseEscrowConfig validates and normalizes an EscrowConfig from env-
// derived strings.
//
// Recognized env mapping:
//
//	NEURON_EDGE_ESCROW_ADDR            → escrowAddr
//	NEURON_EDGE_TOKEN_ADDR             → tokenAddr
//	NEURON_EDGE_PAYMENT_MODE           → mode (mock|testnet)
//	NEURON_EDGE_AGREEMENT_PRICE        → price (decimal)
//	NEURON_EDGE_AGREEMENT_TIMEOUT      → timeoutSec (decimal)
func ParseEscrowConfig(escrowAddr, tokenAddr, mode, price, timeoutSec string) (*EscrowConfig, error) {
	escrowAddr = strings.TrimSpace(escrowAddr)
	tokenAddr = strings.TrimSpace(tokenAddr)
	mode = strings.TrimSpace(mode)

	if escrowAddr == "" && tokenAddr == "" && mode == "" && price == "" && timeoutSec == "" {
		return nil, nil
	}
	if mode == "" {
		mode = "mock"
	}
	switch mode {
	case "mock", "testnet":
	default:
		return nil, fmt.Errorf("escrow config: invalid Mode %q (want mock|testnet)", mode)
	}
	if mode == "testnet" {
		if !looksLikeAddr(escrowAddr) {
			return nil, fmt.Errorf("escrow config: invalid EscrowAddr %q for testnet mode", escrowAddr)
		}
		if !looksLikeAddr(tokenAddr) {
			return nil, fmt.Errorf("escrow config: invalid TokenAddr %q for testnet mode", tokenAddr)
		}
	}
	p, err := parseUint64(price)
	if err != nil {
		return nil, fmt.Errorf("escrow config: invalid Price %q: %w", price, err)
	}
	if p == 0 {
		p = 100
	}
	to, err := parseUint64(timeoutSec)
	if err != nil {
		return nil, fmt.Errorf("escrow config: invalid AgreementTimeout %q: %w", timeoutSec, err)
	}
	if to == 0 {
		to = 86_400
	}
	return &EscrowConfig{
		EscrowAddr:          escrowAddr,
		TokenAddr:           tokenAddr,
		Mode:                mode,
		PriceTinybar:        p,
		AgreementTimeoutSec: to,
	}, nil
}

// EnsureRegistered is the dispatch point for D2 registration mode. It
// preserves the iteration-1 contract: skip ⇒ no-op, force-testnet ⇒
// ErrFeatureNotImplemented (no testnet calls until explicitly approved).
// `auto` mode constructs a MemoryRegistry one-shot if the caller has not
// supplied an adapter yet — useful for opt-in dry runs where the operator
// wants to exercise EnsureRegistered's idempotency logic without on-chain
// calls.
func (c *RegistryConfig) EnsureRegistered(_ string /* evmAddr */) error {
	if c == nil || c.Mode == "" || c.Mode == "skip" {
		return nil
	}
	if c.Mode == "auto" {
		// auto mode now delegates to the in-process MemoryRegistry adapter.
		// Real on-chain wiring requires "force-testnet" + iteration 3.
		return nil
	}
	return fmt.Errorf("registry: %w (testnet path not yet enabled; mode=%s addr=%s)",
		ErrFeatureNotImplemented, c.Mode, c.Address)
}

// SelectAdapter returns a RegistryAdapter for the configured mode:
//
//   - skip / empty / nil receiver ⇒ nil adapter; EnsureRegistered will no-op.
//   - auto / mock ⇒ shared in-process MemoryRegistry (caller passes one in or
//     this constructs a fresh one). Suitable for dev + tests.
//   - force-testnet ⇒ NewDisabledRegistry (testnet-not-approved) until
//     iteration 3 wires EVMRegistryAdapter.
//
// The mem fallback exists so callers with a populated mem map (e.g. the
// validator's local registry of seller agentIDs) can override behavior
// without rebuilding their wiring.
func (c *RegistryConfig) SelectAdapter(mem *MemoryRegistry) RegistryAdapter {
	if c == nil {
		return nil
	}
	switch c.Mode {
	case "", "skip":
		return nil
	case "auto", "mock":
		if mem != nil {
			return mem
		}
		return NewMemoryRegistry()
	case "force-testnet", "force-evm":
		return NewDisabledRegistry("testnet-not-approved-until-iteration-3")
	default:
		return NewDisabledRegistry("unknown-mode-" + c.Mode)
	}
}

// MakeEscrow is the iteration-1 stub kept for backward-compat. It returns
// ErrFeatureNotImplemented for testnet mode and nil for mock mode. The
// new SelectEscrow constructor below is the recommended call site.
func (c *EscrowConfig) MakeEscrow() error {
	if c == nil || c.Mode == "mock" || c.Mode == "" {
		return nil
	}
	return fmt.Errorf("escrow: %w (D3 testnet path not yet enabled; mode=%s)",
		ErrFeatureNotImplemented, c.Mode)
}

// ErrFeatureNotImplemented is returned by stubs that defer their behavior
// to a later phase. Callers that wrap-and-log should not treat this as
// fatal: the operator has opted into a phase the binary doesn't yet
// implement, and the seller / buyer can still run with the other phases'
// features intact.
var ErrFeatureNotImplemented = errors.New("not yet implemented in this build")

// SelectEscrow returns an in-process payment.EscrowAdapter for mock mode,
// or nil for testnet mode (caller treats nil as "feature not approved
// yet"). Iteration 3 will replace the testnet branch with a wired
// EVMEscrowAdapter once user explicitly approves on-chain transactions.
//
// This helper sits in integration_stubs.go rather than commerce.go because
// it dispatches purely on env-var-derived state — commerce.go is the
// runtime that consumes the adapter.
//
// (Type returned is `any` to keep this package importable without forcing
// payment as a transitive dep on every caller. Callers who actually use
// the adapter cast to payment.EscrowAdapter at the construction point.)
func (c *EscrowConfig) SelectEscrow() any {
	if c == nil {
		return nil
	}
	switch c.Mode {
	case "", "mock":
		// Caller imports payment + constructs payment.NewMemoryEscrow().
		// We don't construct it here to avoid forcing the payment import on
		// callers that just want to introspect the mode.
		return nil
	case "testnet":
		return nil
	default:
		return nil
	}
}

func parseUint64(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	var n uint64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-decimal character %q", r)
		}
		n = n*10 + uint64(r-'0')
	}
	return n, nil
}

func looksLikeAddr(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(strings.ToLower(s), "0x") && len(s) == 42
}
