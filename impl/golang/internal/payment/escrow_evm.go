package payment

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment/bindings"
)

// EVMEscrowAdapter implements EscrowAdapter by calling a deployed NeuronEscrow
// contract and its associated ERC-20 token via go-ethereum generated bindings.
//
// The adapter wraps the pull-pattern (approveRelease + withdraw) into a single
// ApproveRelease call, matching the EscrowAdapter interface semantics.
type EVMEscrowAdapter struct {
	escrow       *bindings.NeuronEscrow
	token        *bindings.TestToken
	client       *ethclient.Client
	auth         *bind.TransactOpts
	contractAddr common.Address
	tokenAddr    common.Address
}

// NewEVMEscrowAdapter creates an adapter backed by deployed NeuronEscrow and
// TestToken contracts.
func NewEVMEscrowAdapter(
	client *ethclient.Client,
	escrowAddr common.Address,
	tokenAddr common.Address,
	auth *bind.TransactOpts,
) (*EVMEscrowAdapter, error) {
	escrowContract, err := bindings.NewNeuronEscrow(escrowAddr, client)
	if err != nil {
		return nil, fmt.Errorf("payment.NewEVMEscrowAdapter: bind escrow at %s: %w", escrowAddr.Hex(), err)
	}

	tokenContract, err := bindings.NewTestToken(tokenAddr, client)
	if err != nil {
		return nil, fmt.Errorf("payment.NewEVMEscrowAdapter: bind token at %s: %w", tokenAddr.Hex(), err)
	}

	return &EVMEscrowAdapter{
		escrow:       escrowContract,
		token:        tokenContract,
		client:       client,
		auth:         auth,
		contractAddr: escrowAddr,
		tokenAddr:    tokenAddr,
	}, nil
}

// CreateEscrow creates a new escrow instance on-chain. FR-P17.
func (e *EVMEscrowAdapter) CreateEscrow(
	ctx context.Context,
	buyer, seller string,
	arbiter *string,
	currency string,
	threshold uint64,
	agreementHash [32]byte,
	timeout uint64,
) (EscrowRef, error) {
	buyerAddr := common.HexToAddress(buyer)
	sellerAddr := common.HexToAddress(seller)

	var arbiterAddr common.Address
	if arbiter != nil {
		arbiterAddr = common.HexToAddress(*arbiter)
	}

	opts := e.txOpts(ctx)
	tx, err := e.escrow.CreateEscrow(opts,
		buyerAddr, sellerAddr, arbiterAddr,
		e.tokenAddr, // Use the configured ERC-20 token
		threshold,
		agreementHash,
		timeout,
	)
	if err != nil {
		return EscrowRef{}, fmt.Errorf("payment.CreateEscrow: %w", err)
	}

	receipt, err := bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return EscrowRef{}, fmt.Errorf("payment.CreateEscrow: wait mined: %w", err)
	}

	// Parse EscrowCreated event to get the escrow ID.
	for _, log := range receipt.Logs {
		event, err := e.escrow.ParseEscrowCreated(*log)
		if err == nil {
			locator := fmt.Sprintf("%s:%s", e.contractAddr.Hex(), event.EscrowId.String())
			return EscrowRef{
				Binding: "evm-escrow",
				Locator: locator,
			}, nil
		}
	}

	return EscrowRef{}, fmt.Errorf("payment.CreateEscrow: EscrowCreated event not found")
}

// Deposit adds funds to the escrow. FR-P18.
// Handles ERC-20 approve + deposit in a single call.
func (e *EVMEscrowAdapter) Deposit(ctx context.Context, escrowRef EscrowRef, amount string) (DepositResult, error) {
	escrowId, err := e.parseLocator(escrowRef)
	if err != nil {
		return DepositResult{}, err
	}

	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok || amt.Sign() <= 0 {
		return DepositResult{}, NewPaymentError(ErrEscrowCreationFailed, "Deposit",
			fmt.Sprintf("invalid deposit amount %q", amount))
	}

	// Step 1: Approve the escrow contract to spend tokens.
	opts := e.txOpts(ctx)
	approveTx, err := e.token.Approve(opts, e.contractAddr, amt)
	if err != nil {
		return DepositResult{}, fmt.Errorf("payment.Deposit: approve: %w", err)
	}
	_, err = bind.WaitMined(ctx, e.client, approveTx)
	if err != nil {
		return DepositResult{}, fmt.Errorf("payment.Deposit: approve wait: %w", err)
	}

	// Step 2: Call deposit on the escrow contract.
	opts = e.txOpts(ctx)
	depositTx, err := e.escrow.Deposit(opts, escrowId, amt)
	if err != nil {
		return DepositResult{}, fmt.Errorf("payment.Deposit: deposit: %w", err)
	}
	_, err = bind.WaitMined(ctx, e.client, depositTx)
	if err != nil {
		return DepositResult{}, fmt.Errorf("payment.Deposit: deposit wait: %w", err)
	}

	// Query new balance.
	balance, err := e.escrow.GetBalance(&bind.CallOpts{Context: ctx}, escrowId)
	if err != nil {
		return DepositResult{}, fmt.Errorf("payment.Deposit: getBalance: %w", err)
	}

	return DepositResult{
		TransactionRef: depositTx.Hash().Hex(),
		NewBalance:     balance.String(),
	}, nil
}

// GetBalance queries the current escrow balance. FR-P19.
func (e *EVMEscrowAdapter) GetBalance(ctx context.Context, escrowRef EscrowRef) (Balance, error) {
	escrowId, err := e.parseLocator(escrowRef)
	if err != nil {
		return Balance{}, err
	}

	available, err := e.escrow.GetBalance(&bind.CallOpts{Context: ctx}, escrowId)
	if err != nil {
		return Balance{}, fmt.Errorf("payment.GetBalance: %w", err)
	}

	return Balance{
		Available:  available.String(),
		Currency:   "NTT", // TestToken symbol
		LastSynced: 0,     // EVM doesn't provide sync timestamp in the call
	}, nil
}

// RequestRelease records a pending release request. FR-P20, FR-P25a.
func (e *EVMEscrowAdapter) RequestRelease(
	ctx context.Context,
	escrowRef EscrowRef,
	amount string,
	recipient string,
	evidenceHash [32]byte,
) (ReleaseRequestRef, error) {
	escrowId, err := e.parseLocator(escrowRef)
	if err != nil {
		return ReleaseRequestRef{}, err
	}

	amt, ok := new(big.Int).SetString(amount, 10)
	if !ok || amt.Sign() <= 0 {
		return ReleaseRequestRef{}, NewPaymentError(ErrInsufficientBalance, "RequestRelease",
			fmt.Sprintf("invalid release amount %q", amount))
	}

	recipientAddr := common.HexToAddress(recipient)

	opts := e.txOpts(ctx)
	tx, err := e.escrow.RequestRelease(opts, escrowId, amt, recipientAddr, evidenceHash)
	if err != nil {
		return ReleaseRequestRef{}, fmt.Errorf("payment.RequestRelease: %w", err)
	}

	receipt, err := bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return ReleaseRequestRef{}, fmt.Errorf("payment.RequestRelease: wait mined: %w", err)
	}

	// Parse ReleaseRequested event to get the release ID.
	for _, log := range receipt.Logs {
		event, err := e.escrow.ParseReleaseRequested(*log)
		if err == nil {
			locator := fmt.Sprintf("%s:%s:%s", e.contractAddr.Hex(), escrowId.String(), event.ReleaseId.String())
			return ReleaseRequestRef{
				Binding: "evm-escrow",
				Locator: locator,
			}, nil
		}
	}

	return ReleaseRequestRef{}, fmt.Errorf("payment.RequestRelease: ReleaseRequested event not found")
}

// ApproveRelease authorizes fund transfer and triggers withdrawal. FR-P21.
// Wraps the contract's approveRelease + withdraw into a single operation.
func (e *EVMEscrowAdapter) ApproveRelease(
	ctx context.Context,
	escrowRef EscrowRef,
	releaseRef ReleaseRequestRef,
) (ReleaseResult, error) {
	escrowId, err := e.parseLocator(escrowRef)
	if err != nil {
		return ReleaseResult{}, err
	}

	releaseId, err := e.parseReleaseLocator(releaseRef)
	if err != nil {
		return ReleaseResult{}, err
	}

	// Step 1: Approve the release.
	opts := e.txOpts(ctx)
	approveTx, err := e.escrow.ApproveRelease(opts, escrowId, releaseId)
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("payment.ApproveRelease: approve: %w", err)
	}
	_, err = bind.WaitMined(ctx, e.client, approveTx)
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("payment.ApproveRelease: approve wait: %w", err)
	}

	// Step 2: Withdraw funds (pull pattern).
	opts = e.txOpts(ctx)
	withdrawTx, err := e.escrow.Withdraw(opts, escrowId, releaseId)
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("payment.ApproveRelease: withdraw: %w", err)
	}

	receipt, err := bind.WaitMined(ctx, e.client, withdrawTx)
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("payment.ApproveRelease: withdraw wait: %w", err)
	}

	// Parse Withdrawn event.
	for _, log := range receipt.Logs {
		event, err := e.escrow.ParseWithdrawn(*log)
		if err == nil {
			return ReleaseResult{
				TransactionRef: withdrawTx.Hash().Hex(),
				Released:       event.Amount.String(),
				Recipient:      event.Recipient.Hex(),
			}, nil
		}
	}

	return ReleaseResult{
		TransactionRef: withdrawTx.Hash().Hex(),
	}, nil
}

// ClaimRefund returns funds to buyer after timeout. FR-P22, FR-P25b.
func (e *EVMEscrowAdapter) ClaimRefund(ctx context.Context, escrowRef EscrowRef) error {
	escrowId, err := e.parseLocator(escrowRef)
	if err != nil {
		return err
	}

	opts := e.txOpts(ctx)
	tx, err := e.escrow.ClaimRefund(opts, escrowId)
	if err != nil {
		return fmt.Errorf("payment.ClaimRefund: %w", err)
	}

	_, err = bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return fmt.Errorf("payment.ClaimRefund: wait mined: %w", err)
	}

	return nil
}

// parseLocator extracts the escrow ID from an EscrowRef locator.
// Format: "<contractAddr>:<escrowId>"
func (e *EVMEscrowAdapter) parseLocator(ref EscrowRef) (*big.Int, error) {
	if ref.Binding != "evm-escrow" {
		return nil, NewPaymentError(ErrInvalidEscrowRef, "parseLocator",
			fmt.Sprintf("expected binding \"evm-escrow\", got %q", ref.Binding))
	}

	parts := strings.SplitN(ref.Locator, ":", 2)
	if len(parts) != 2 {
		return nil, NewPaymentError(ErrInvalidEscrowRef, "parseLocator",
			fmt.Sprintf("invalid locator format %q, expected \"addr:id\"", ref.Locator))
	}

	id, ok := new(big.Int).SetString(parts[1], 10)
	if !ok {
		return nil, NewPaymentError(ErrInvalidEscrowRef, "parseLocator",
			fmt.Sprintf("invalid escrow ID %q", parts[1]))
	}

	return id, nil
}

// parseReleaseLocator extracts the escrow ID and release ID from a ReleaseRequestRef.
// Format: "<contractAddr>:<escrowId>:<releaseId>"
func (e *EVMEscrowAdapter) parseReleaseLocator(ref ReleaseRequestRef) (*big.Int, error) {
	if ref.Binding != "evm-escrow" {
		return nil, NewPaymentError(ErrInvalidEscrowRef, "parseReleaseLocator",
			fmt.Sprintf("expected binding \"evm-escrow\", got %q", ref.Binding))
	}

	parts := strings.Split(ref.Locator, ":")
	if len(parts) != 3 {
		return nil, NewPaymentError(ErrInvalidEscrowRef, "parseReleaseLocator",
			fmt.Sprintf("invalid release locator %q, expected \"addr:escrowId:releaseId\"", ref.Locator))
	}

	releaseId, ok := new(big.Int).SetString(parts[2], 10)
	if !ok {
		return nil, NewPaymentError(ErrInvalidEscrowRef, "parseReleaseLocator",
			fmt.Sprintf("invalid release ID %q", parts[2]))
	}

	return releaseId, nil
}

// txOpts creates a copy of the base TransactOpts with the given context.
func (e *EVMEscrowAdapter) txOpts(ctx context.Context) *bind.TransactOpts {
	return &bind.TransactOpts{
		From:     e.auth.From,
		Signer:   e.auth.Signer,
		GasLimit: e.auth.GasLimit,
		Context:  ctx,
	}
}

// Compile-time assertion: EVMEscrowAdapter implements EscrowAdapter.
var _ EscrowAdapter = (*EVMEscrowAdapter)(nil)
