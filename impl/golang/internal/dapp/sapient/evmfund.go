package sapient

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	paybindings "github.com/neuron-sdk/neuron-go-sdk/internal/payment/bindings"
)

// ComputeShortfall returns how much must be minted so balance covers needed
// (0 when already sufficient). Pure helper — unit-tested without a chain.
func ComputeShortfall(balance, needed *big.Int) *big.Int {
	if balance.Cmp(needed) >= 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Sub(needed, balance)
}

// EnsureTokenBalance makes sure the signer's TestToken (NTT) balance covers
// `amount`, minting the shortfall when necessary. The TestToken is the
// unrestricted-mint DEMO token deployed next to the escrow — this is a
// demo-funding convenience, prominently logged and recorded in evidence (a
// production token obviously cannot be minted by its buyer). Returns the mint
// tx hash, or "" when no mint was needed.
func EnsureTokenBalance(ctx context.Context, rpcURL string, tokenAddr common.Address, chainID uint64, signer *ecdsa.PrivateKey, amount *big.Int, logger *log.Logger) (string, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return "", fmt.Errorf("sapient.EnsureTokenBalance: dial %s: %w", rpcURL, err)
	}
	defer client.Close()

	token, err := paybindings.NewTestToken(tokenAddr, client)
	if err != nil {
		return "", fmt.Errorf("sapient.EnsureTokenBalance: bind token %s: %w", tokenAddr.Hex(), err)
	}
	owner := ethcrypto.PubkeyToAddress(signer.PublicKey)
	balance, err := token.BalanceOf(&bind.CallOpts{Context: ctx}, owner)
	if err != nil {
		return "", fmt.Errorf("sapient.EnsureTokenBalance: balanceOf: %w", err)
	}
	shortfall := ComputeShortfall(balance, amount)
	if shortfall.Sign() == 0 {
		if logger != nil {
			logger.Printf("[escrow] token balance %s covers %s — no mint needed", balance, amount)
		}
		return "", nil
	}

	auth, err := bind.NewKeyedTransactorWithChainID(signer, new(big.Int).SetUint64(chainID))
	if err != nil {
		return "", fmt.Errorf("sapient.EnsureTokenBalance: build transactor: %w", err)
	}
	auth.Context = ctx
	tx, err := token.Mint(auth, owner, shortfall)
	if err != nil {
		return "", fmt.Errorf("sapient.EnsureTokenBalance: mint: %w", err)
	}
	if _, err := bind.WaitMined(ctx, client, tx); err != nil {
		return "", fmt.Errorf("sapient.EnsureTokenBalance: wait mined: %w", err)
	}
	if logger != nil {
		logger.Printf("[escrow] DEMO TestToken mint: balance=%s + minted=%s covers required=%s tx=%s",
			balance, shortfall, amount, tx.Hash().Hex())
	}
	return tx.Hash().Hex(), nil
}

// TokenBalanceProbe returns a closure reading the signer-owner's TestToken
// balance — wired into SellerCommerceOptions.TokenBalance so the seller can
// verify the release actually landed in its account.
func TokenBalanceProbe(rpcURL string, tokenAddr common.Address, owner common.Address) func(ctx context.Context) (*big.Int, error) {
	return func(ctx context.Context) (*big.Int, error) {
		client, err := ethclient.DialContext(ctx, rpcURL)
		if err != nil {
			return nil, fmt.Errorf("sapient.TokenBalanceProbe: dial %s: %w", rpcURL, err)
		}
		defer client.Close()
		token, err := paybindings.NewTestToken(tokenAddr, client)
		if err != nil {
			return nil, fmt.Errorf("sapient.TokenBalanceProbe: bind token: %w", err)
		}
		return token.BalanceOf(&bind.CallOpts{Context: ctx}, owner)
	}
}
