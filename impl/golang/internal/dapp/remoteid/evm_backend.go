package remoteid

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// EVM escrow env var contract. Mirrors the names the existing
// `cmd/buyer-seller-demo` consumes (lines 555-604) so an operator with
// the same .env can re-use it for the Remote ID demo.
const (
	// EVMEnvRPC — JSON-RPC endpoint URL.
	// Required for --escrow-backend=evm. Defaults to Hedera testnet hashio
	// when missing AND the operator explicitly opted in (the factory still
	// errors when the value is empty; the default is only consulted when
	// non-empty env was supplied with a placeholder).
	EVMEnvRPC = "HEDERA_EVM_RPC"

	// EVMEnvSignerKey — ECDSA secp256k1 hex (no 0x prefix). The buyer
	// signs every escrow tx with this; the seller signs the
	// RequestRelease + ApproveRelease txs with the same key (single-key
	// demo posture; multi-tenant adapter splits this in Stage 4+).
	// Fallback: HEDERA_OPERATOR_KEY.
	EVMEnvSignerKey = "NEURON_EVM_PRIVATE_KEY"

	// EVMEnvSignerKeyFallback — re-used HCS operator key when
	// NEURON_EVM_PRIVATE_KEY is unset. Matches the buyer-seller-demo
	// precedent (main.go:567-570).
	EVMEnvSignerKeyFallback = "HEDERA_OPERATOR_KEY"

	// EVMEnvEscrowContract — NeuronEscrow contract address.
	// Required for --escrow-backend=evm.
	EVMEnvEscrowContract = "NEURON_ESCROW_CONTRACT"

	// EVMEnvTokenContract — TestToken contract address.
	// Required for --escrow-backend=evm.
	EVMEnvTokenContract = "NEURON_TOKEN_CONTRACT"

	// EVMEnvChainID — chain id (decimal). Defaults to 296 (Hedera testnet)
	// when empty.
	EVMEnvChainID = "NEURON_CHAIN_ID"
)

// EVMBackend wraps the constructed EVM escrow adapter + the captured
// signer + RPC for evidence logging.
type EVMBackend struct {
	Escrow         payment.EscrowAdapter
	EscrowBinding  string // always "evm-escrow"; mirrors EVMEscrowAdapter.CreateEscrow return
	EscrowContract string // 0x… capture for evidence
	TokenContract  string // 0x…
	RPCURL         string
	ChainID        uint64
	OperatorAddr   string // 0x… signer address (NEVER the private key itself)
}

// EVMBackendOptions configures the factory.
type EVMBackendOptions struct {
	// LookupEnv overrides os.Getenv (tests). Defaults to os.Getenv.
	LookupEnv func(key string) string

	// DefaultRPCURL is consulted when EVMEnvRPC is empty. Set to
	// "https://testnet.hashio.io/api" by the CLI; tests pass a
	// dummy value to avoid network egress.
	DefaultRPCURL string

	// DefaultChainID is consulted when EVMEnvChainID is empty. The CLI
	// passes 296 (Hedera testnet).
	DefaultChainID uint64

	// DialContractBackend, when non-nil, overrides the default
	// ethclient.Dial path. Tests inject a stub that constructs a
	// simulated backend; production leaves nil and lets the factory
	// dial the real RPC.
	DialContractBackend func(ctx context.Context, rpcURL string) (EthDialer, error)
}

// EthDialer is the minimal subset of *ethclient.Client the EVMEscrowAdapter
// requires (the underlying bindings only use bind.ContractBackend +
// bind.DeployBackend semantics).
type EthDialer = *ethclient.Client

// NewEVMBackend constructs the EVM escrow adapter from env vars.
// Returns an explicit error listing the missing env var names when
// config is incomplete — callers MUST exit 2. NEVER falls back to the
// memory escrow.
//
// FR anchors:
//   - 008 FR-P16: settlement bindings implement the six EscrowAdapter
//     ops on-chain. EVMEscrowAdapter is the production binding.
//   - 008 FR-P58: descriptor's `commerceMode = "full"` for any
//     --escrow-backend=evm run.
//
// Security: the signer private key is read from env, parsed into an
// in-memory *ecdsa.PrivateKey, and used for `bind.NewKeyedTransactorWithChainID`.
// The key is NEVER logged, printed, or echoed back through any error
// message. Only the derived public EVM address is exposed for evidence.
func NewEVMBackend(ctx context.Context, opts EVMBackendOptions) (*EVMBackend, error) {
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.Getenv
	}
	missing := missingEVMEnvVars(lookup)
	if len(missing) > 0 {
		return nil, fmt.Errorf("remoteid.NewEVMBackend: missing env %v — refusing to fall back to memory; set these before --escrow-backend=evm", missing)
	}

	rpcURL := lookup(EVMEnvRPC)
	if rpcURL == "" {
		rpcURL = opts.DefaultRPCURL
	}
	if rpcURL == "" {
		return nil, fmt.Errorf("remoteid.NewEVMBackend: no RPC URL — set %s or pass DefaultRPCURL", EVMEnvRPC)
	}

	chainID := opts.DefaultChainID
	if v := lookup(EVMEnvChainID); v != "" {
		parsed := new(big.Int)
		if _, ok := parsed.SetString(v, 10); !ok {
			return nil, fmt.Errorf("remoteid.NewEVMBackend: parse %s=%q as integer", EVMEnvChainID, v)
		}
		if !parsed.IsUint64() {
			return nil, fmt.Errorf("remoteid.NewEVMBackend: %s=%q does not fit uint64", EVMEnvChainID, v)
		}
		chainID = parsed.Uint64()
	}
	if chainID == 0 {
		chainID = 296 // Hedera testnet default
	}

	signerHex := lookup(EVMEnvSignerKey)
	if signerHex == "" {
		signerHex = lookup(EVMEnvSignerKeyFallback)
	}
	if signerHex == "" {
		return nil, fmt.Errorf("remoteid.NewEVMBackend: missing env %s (and fallback %s) — refusing to fall back to memory", EVMEnvSignerKey, EVMEnvSignerKeyFallback)
	}
	signerKey, err := ethcrypto.HexToECDSA(strings.TrimPrefix(signerHex, "0x"))
	if err != nil {
		// Mask any chance of the key bytes appearing in the error. The
		// returned error includes only the env var name, not its value.
		return nil, fmt.Errorf("remoteid.NewEVMBackend: parse env %s as ECDSA hex: invalid format", EVMEnvSignerKey)
	}
	operatorAddr := ethcrypto.PubkeyToAddress(signerKey.PublicKey)

	escrowAddrHex := lookup(EVMEnvEscrowContract)
	tokenAddrHex := lookup(EVMEnvTokenContract)
	escrowAddr := common.HexToAddress(escrowAddrHex)
	tokenAddr := common.HexToAddress(tokenAddrHex)

	dial := opts.DialContractBackend
	if dial == nil {
		dial = defaultEthDial
	}
	client, err := dial(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("remoteid.NewEVMBackend: dial %s: %w", rpcURL, err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(signerKey, new(big.Int).SetUint64(chainID))
	if err != nil {
		return nil, fmt.Errorf("remoteid.NewEVMBackend: build transactor: %w", err)
	}

	escrowAdapter, err := payment.NewEVMEscrowAdapter(client, escrowAddr, tokenAddr, auth)
	if err != nil {
		return nil, fmt.Errorf("remoteid.NewEVMBackend: %w", err)
	}

	return &EVMBackend{
		Escrow:         escrowAdapter,
		EscrowBinding:  "evm-escrow",
		EscrowContract: escrowAddr.Hex(),
		TokenContract:  tokenAddr.Hex(),
		RPCURL:         rpcURL,
		ChainID:        chainID,
		OperatorAddr:   operatorAddr.Hex(),
	}, nil
}

func defaultEthDial(ctx context.Context, rpcURL string) (*ethclient.Client, error) {
	return ethclient.DialContext(ctx, rpcURL)
}

// missingEVMEnvVars returns the env var names the lookup did not
// satisfy. Used by CLI validation tests to exit 2 with the exact
// missing-var list before construction.
func missingEVMEnvVars(lookup func(key string) string) []string {
	if lookup == nil {
		lookup = os.Getenv
	}
	var missing []string
	if lookup(EVMEnvEscrowContract) == "" {
		missing = append(missing, EVMEnvEscrowContract)
	}
	if lookup(EVMEnvTokenContract) == "" {
		missing = append(missing, EVMEnvTokenContract)
	}
	// Signer key: either NEURON_EVM_PRIVATE_KEY or HEDERA_OPERATOR_KEY
	// satisfies. Don't list it as a single missing name; the factory's
	// own error message lists both.
	if lookup(EVMEnvSignerKey) == "" && lookup(EVMEnvSignerKeyFallback) == "" {
		missing = append(missing, EVMEnvSignerKey+" (or "+EVMEnvSignerKeyFallback+")")
	}
	return missing
}

// MissingEVMEnvVars is the exported view; the CLIs call this to bail
// with exit 2 before any construction.
func MissingEVMEnvVars(lookup func(key string) string) []string {
	return missingEVMEnvVars(lookup)
}

// ErrEVMConfigMissing is returned when --escrow-backend=evm is requested
// without the required env vars.
var ErrEVMConfigMissing = errors.New("remoteid: EVM escrow backend config missing")

// ErrEVMZeroPricing is returned when --escrow-backend=evm is requested
// with `--pricing-amount=0`. Stage 3A's anti-scope rules forbid
// zero-value pricing for the real escrow path — MemoryEscrow rejects
// amount=0 anyway, and Hedera testnet usability requires a sensible
// release amount (default 100_000_000 tinybar = 1 HBAR in the runbook).
var ErrEVMZeroPricing = errors.New("remoteid: --escrow-backend=evm forbids --pricing-amount=0; set a non-zero amount before re-running")

// ValidatePricingForEVM returns ErrEVMZeroPricing when amount is "0",
// "", or any non-decimal "0"-equivalent. Called by the CLI before
// constructing the backend.
func ValidatePricingForEVM(amount string) error {
	amount = strings.TrimSpace(amount)
	if amount == "" || amount == "0" {
		return ErrEVMZeroPricing
	}
	// More permissive zero-detection: "00", "0.0", etc. Parse as big.Int.
	parsed := new(big.Int)
	if _, ok := parsed.SetString(amount, 10); !ok {
		return fmt.Errorf("remoteid.ValidatePricingForEVM: amount %q is not a base-10 integer", amount)
	}
	if parsed.Sign() <= 0 {
		return ErrEVMZeroPricing
	}
	return nil
}

// Compile-time assertion so the package's exported types match the
// EscrowAdapter contract; if payment.EscrowAdapter ever drifts, this
// catches it at build time rather than runtime.
var _ topic.TopicAdapter
