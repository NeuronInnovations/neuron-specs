package edgeapp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRegistryConfig_AllEmpty(t *testing.T) {
	c, err := ParseRegistryConfig("", "", "", "")
	require.NoError(t, err)
	assert.Nil(t, c, "all-empty input ⇒ feature disabled, no config object")

	// "skip" mode with nothing else is also disabled.
	c, err = ParseRegistryConfig("", "", "", "skip")
	require.NoError(t, err)
	assert.Nil(t, c)
}

func TestParseRegistryConfig_HappyPath(t *testing.T) {
	c, err := ParseRegistryConfig(
		"0x5d9b1fe5eb02173205aee8dc4f72db15bfb5f73c",
		"296", "", "auto",
	)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "0x5d9b1fe5eb02173205aee8dc4f72db15bfb5f73c", c.Address)
	assert.Equal(t, uint64(296), c.ChainID)
	assert.Equal(t, "https://testnet.hashio.io/api", c.RPC, "default RPC populated when omitted")
	assert.Equal(t, "auto", c.Mode)
}

func TestParseRegistryConfig_RejectsBadAddress(t *testing.T) {
	_, err := ParseRegistryConfig("0xdeadbeef", "296", "", "auto")
	require.Error(t, err)
}

func TestParseRegistryConfig_RejectsMissingChainID(t *testing.T) {
	_, err := ParseRegistryConfig(
		"0x5d9b1fe5eb02173205aee8dc4f72db15bfb5f73c", "", "", "auto",
	)
	require.Error(t, err, "address without chainID ⇒ error")
}

func TestParseRegistryConfig_RejectsBadMode(t *testing.T) {
	_, err := ParseRegistryConfig(
		"0x5d9b1fe5eb02173205aee8dc4f72db15bfb5f73c", "296", "", "shenanigans",
	)
	require.Error(t, err)
}

func TestParseEscrowConfig_AllEmpty(t *testing.T) {
	c, err := ParseEscrowConfig("", "", "", "", "")
	require.NoError(t, err)
	assert.Nil(t, c)
}

func TestParseEscrowConfig_MockMode_MinimalConfig(t *testing.T) {
	c, err := ParseEscrowConfig("", "", "mock", "", "")
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "mock", c.Mode)
	assert.Equal(t, uint64(100), c.PriceTinybar, "default price filled in")
	assert.Equal(t, uint64(86_400), c.AgreementTimeoutSec, "default 24h timeout")
}

func TestParseEscrowConfig_TestnetRequiresAddresses(t *testing.T) {
	_, err := ParseEscrowConfig("", "", "testnet", "", "")
	require.Error(t, err, "testnet mode without contracts ⇒ error")
}

func TestParseEscrowConfig_TestnetHappyPath(t *testing.T) {
	c, err := ParseEscrowConfig(
		"0xe34659d94bffab0ed329cc521b53ef90c85a8cd5",
		"0xea0c760f3b25da8082b1cf3d269fab4561dccc3c",
		"testnet", "500", "3600",
	)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "testnet", c.Mode)
	assert.Equal(t, uint64(500), c.PriceTinybar)
	assert.Equal(t, uint64(3600), c.AgreementTimeoutSec)
}

func TestParseEscrowConfig_RejectsBadMode(t *testing.T) {
	_, err := ParseEscrowConfig("", "", "vapor", "", "")
	require.Error(t, err)
}

func TestRegistryConfig_EnsureRegisteredForceTestnetSurfacesNotImplemented(t *testing.T) {
	// Iteration 2 narrows the "force-testnet" / "force-evm" modes to
	// surface ErrFeatureNotImplemented (no testnet calls until iteration 3).
	// The iteration-1 expectation that `auto` errors is replaced by the
	// new contract: `auto` is a no-op shim; callers consume SelectAdapter.
	c, err := ParseRegistryConfig(
		"0x5d9b1fe5eb02173205aee8dc4f72db15bfb5f73c", "296", "", "auto",
	)
	require.NoError(t, err)
	c.Mode = "force-testnet" // overrides for the test scenario
	err = c.EnsureRegistered("0xabc")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFeatureNotImplemented))
}

func TestRegistryConfig_SkipModeIsNoop(t *testing.T) {
	c := &RegistryConfig{Mode: "skip"}
	assert.NoError(t, c.EnsureRegistered("0xabc"))

	var nilCfg *RegistryConfig
	assert.NoError(t, nilCfg.EnsureRegistered("0xabc"))
}

func TestEscrowConfig_MockModeIsNoop(t *testing.T) {
	c := &EscrowConfig{Mode: "mock"}
	assert.NoError(t, c.MakeEscrow())

	var nilCfg *EscrowConfig
	assert.NoError(t, nilCfg.MakeEscrow())
}

func TestEscrowConfig_TestnetSurfacesNotImplemented(t *testing.T) {
	c := &EscrowConfig{Mode: "testnet"}
	err := c.MakeEscrow()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFeatureNotImplemented))
}

func TestRegistryConfig_SelectAdapter(t *testing.T) {
	t.Run("nil-receiver-returns-nil", func(t *testing.T) {
		var c *RegistryConfig
		assert.Nil(t, c.SelectAdapter(nil))
	})
	t.Run("skip-mode-returns-nil", func(t *testing.T) {
		c := &RegistryConfig{Mode: "skip"}
		assert.Nil(t, c.SelectAdapter(nil))
	})
	t.Run("auto-mode-returns-MemoryRegistry", func(t *testing.T) {
		c := &RegistryConfig{Mode: "auto"}
		ad := c.SelectAdapter(nil)
		require.NotNil(t, ad)
		_, ok := ad.(*MemoryRegistry)
		assert.True(t, ok, "auto mode should return *MemoryRegistry")
	})
	t.Run("auto-mode-with-mem-uses-passed-instance", func(t *testing.T) {
		c := &RegistryConfig{Mode: "auto"}
		mem := NewMemoryRegistry()
		ad := c.SelectAdapter(mem)
		assert.Same(t, mem, ad, "passed mem should be reused")
	})
	t.Run("force-testnet-returns-disabled", func(t *testing.T) {
		c := &RegistryConfig{Mode: "force-testnet"}
		ad := c.SelectAdapter(nil)
		require.NotNil(t, ad)
		// Disabled adapter errors on every call.
		_, _, err := ad.Register(t.Context(), "0xabc", "{}")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrFeatureNotImplemented))
	})
	t.Run("unknown-mode-returns-disabled", func(t *testing.T) {
		c := &RegistryConfig{Mode: "shenanigans"}
		ad := c.SelectAdapter(nil)
		require.NotNil(t, ad)
		_, _, err := ad.Register(t.Context(), "0xabc", "{}")
		assert.True(t, errors.Is(err, ErrFeatureNotImplemented))
	})
}

func TestRegistryConfig_AutoModeIsNoop(t *testing.T) {
	c := &RegistryConfig{Mode: "auto"}
	assert.NoError(t, c.EnsureRegistered("0xabc"),
		"auto mode is now a no-op at the EnsureRegistered shim level — caller invokes registration.go's EnsureRegistered directly with the SelectAdapter result")
}
