package adsb

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

// EVM escrow env var contract. Mirrors internal/dapp/remoteid env var names
// so an operator with the same .env can run both DApps without re-keying.
const (
	EVMEnvRPC               = "HEDERA_EVM_RPC"
	EVMEnvSignerKey         = "NEURON_EVM_PRIVATE_KEY"
	EVMEnvSignerKeyFallback = "HEDERA_OPERATOR_KEY"
	EVMEnvEscrowContract    = "NEURON_ESCROW_CONTRACT"
	EVMEnvTokenContract     = "NEURON_TOKEN_CONTRACT"
	EVMEnvChainID           = "NEURON_CHAIN_ID"
)

// EVMBackend wraps the constructed EVM escrow adapter + captured signer +
// RPC for evidence logging.
type EVMBackend struct {
	Escrow         payment.EscrowAdapter
	EscrowBinding  string
	EscrowContract string
	TokenContract  string
	RPCURL         string
	ChainID        uint64
	OperatorAddr   string
}

// EVMBackendOptions configures the factory.
type EVMBackendOptions struct {
	LookupEnv           func(key string) string
	DefaultRPCURL       string
	DefaultChainID      uint64
	DialContractBackend func(ctx context.Context, rpcURL string) (EthDialer, error)
}

// EthDialer is the minimal *ethclient.Client subset EVMEscrowAdapter requires.
type EthDialer = *ethclient.Client

// NewEVMBackend constructs the EVM escrow adapter from env vars. Mirrors
// internal/dapp/remoteid.NewEVMBackend with adsb-namespaced error messages.
// The signer private key is NEVER logged or echoed; only the derived public
// EVM address is exposed for evidence.
func NewEVMBackend(ctx context.Context, opts EVMBackendOptions) (*EVMBackend, error) {
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.Getenv
	}
	missing := missingEVMEnvVars(lookup)
	if len(missing) > 0 {
		return nil, fmt.Errorf("adsb.NewEVMBackend: missing env %v — refusing to fall back to memory; set these before --escrow-backend=evm", missing)
	}

	rpcURL := lookup(EVMEnvRPC)
	if rpcURL == "" {
		rpcURL = opts.DefaultRPCURL
	}
	if rpcURL == "" {
		return nil, fmt.Errorf("adsb.NewEVMBackend: no RPC URL — set %s or pass DefaultRPCURL", EVMEnvRPC)
	}

	chainID := opts.DefaultChainID
	if v := lookup(EVMEnvChainID); v != "" {
		parsed := new(big.Int)
		if _, ok := parsed.SetString(v, 10); !ok {
			return nil, fmt.Errorf("adsb.NewEVMBackend: parse %s=%q as integer", EVMEnvChainID, v)
		}
		if !parsed.IsUint64() {
			return nil, fmt.Errorf("adsb.NewEVMBackend: %s=%q does not fit uint64", EVMEnvChainID, v)
		}
		chainID = parsed.Uint64()
	}
	if chainID == 0 {
		chainID = 296
	}

	signerHex := lookup(EVMEnvSignerKey)
	if signerHex == "" {
		signerHex = lookup(EVMEnvSignerKeyFallback)
	}
	if signerHex == "" {
		return nil, fmt.Errorf("adsb.NewEVMBackend: missing env %s (and fallback %s) — refusing to fall back to memory", EVMEnvSignerKey, EVMEnvSignerKeyFallback)
	}
	signerKey, err := ethcrypto.HexToECDSA(strings.TrimPrefix(signerHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("adsb.NewEVMBackend: parse env %s as ECDSA hex: invalid format", EVMEnvSignerKey)
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
		return nil, fmt.Errorf("adsb.NewEVMBackend: dial %s: %w", rpcURL, err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(signerKey, new(big.Int).SetUint64(chainID))
	if err != nil {
		return nil, fmt.Errorf("adsb.NewEVMBackend: build transactor: %w", err)
	}

	escrowAdapter, err := payment.NewEVMEscrowAdapter(client, escrowAddr, tokenAddr, auth)
	if err != nil {
		return nil, fmt.Errorf("adsb.NewEVMBackend: %w", err)
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
	if lookup(EVMEnvSignerKey) == "" && lookup(EVMEnvSignerKeyFallback) == "" {
		missing = append(missing, EVMEnvSignerKey+" (or "+EVMEnvSignerKeyFallback+")")
	}
	return missing
}

// MissingEVMEnvVars is the exported view; CLIs call this to bail with exit 2.
func MissingEVMEnvVars(lookup func(key string) string) []string {
	return missingEVMEnvVars(lookup)
}

// ErrEVMConfigMissing is returned when --escrow-backend=evm is requested
// without the required env vars.
var ErrEVMConfigMissing = errors.New("adsb: EVM escrow backend config missing")

// ErrEVMZeroPricing is returned when --escrow-backend=evm is requested
// with `--pricing-amount=0`.
var ErrEVMZeroPricing = errors.New("adsb: --escrow-backend=evm forbids --pricing-amount=0; set a non-zero amount before re-running")

// ValidatePricingForEVM returns ErrEVMZeroPricing when amount is "0", "",
// or any non-decimal "0"-equivalent.
func ValidatePricingForEVM(amount string) error {
	amount = strings.TrimSpace(amount)
	if amount == "" || amount == "0" {
		return ErrEVMZeroPricing
	}
	parsed := new(big.Int)
	if _, ok := parsed.SetString(amount, 10); !ok {
		return fmt.Errorf("adsb.ValidatePricingForEVM: amount %q is not a base-10 integer", amount)
	}
	if parsed.Sign() <= 0 {
		return ErrEVMZeroPricing
	}
	return nil
}

// Compile-time assertion that topic.TopicAdapter still exists.
var _ topic.TopicAdapter
