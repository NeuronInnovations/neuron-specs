package keylib

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T014: Matching function tests.
// FR-016, SEC-004, SC-005, SC-007

func TestPrivateKey_MatchesPublicKey(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	t.Run("true for matching key", func(t *testing.T) {
		pub := key.PublicKey()
		assert.True(t, key.MatchesPublicKey(pub))
	})

	t.Run("false for non-matching key", func(t *testing.T) {
		otherKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		otherNeuron, err := NeuronPrivateKeyFromBlockchainKey(otherKey)
		require.NoError(t, err)

		assert.False(t, key.MatchesPublicKey(otherNeuron.PublicKey()))
	})
}

func TestPrivateKey_MatchesEVMAddress(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	t.Run("true for matching address", func(t *testing.T) {
		addr := key.PublicKey().EVMAddress()
		assert.True(t, key.MatchesEVMAddress(addr))
	})

	t.Run("false for non-matching address", func(t *testing.T) {
		otherKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		otherNeuron, err := NeuronPrivateKeyFromBlockchainKey(otherKey)
		require.NoError(t, err)

		assert.False(t, key.MatchesEVMAddress(otherNeuron.PublicKey().EVMAddress()))
	})
}

func TestPublicKey_MatchesPeerID(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)
	pub := key.PublicKey()

	t.Run("true for matching PeerID", func(t *testing.T) {
		pid, err := pub.PeerID()
		require.NoError(t, err)
		assert.True(t, pub.MatchesPeerID(pid))
	})

	t.Run("false for non-matching PeerID", func(t *testing.T) {
		otherKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		otherNeuron, err := NeuronPrivateKeyFromBlockchainKey(otherKey)
		require.NoError(t, err)
		otherPID, err := otherNeuron.PublicKey().PeerID()
		require.NoError(t, err)

		assert.False(t, pub.MatchesPeerID(otherPID))
	})
}

func TestPublicKey_MatchesEVMAddress(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)
	pub := key.PublicKey()

	t.Run("true for matching address", func(t *testing.T) {
		addr := pub.EVMAddress()
		assert.True(t, pub.MatchesEVMAddress(addr))
	})

	t.Run("false for non-matching address", func(t *testing.T) {
		otherKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		otherNeuron, err := NeuronPrivateKeyFromBlockchainKey(otherKey)
		require.NoError(t, err)

		assert.False(t, pub.MatchesEVMAddress(otherNeuron.PublicKey().EVMAddress()))
	})
}

func TestMatching_NoFalseNegatives(t *testing.T) {
	// Generate 10 keys and verify all matching functions return true for same-key pairs.
	for i := 0; i < 10; i++ {
		ecKey, err := crypto.GenerateKey()
		require.NoError(t, err)

		key, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
		require.NoError(t, err)

		pub := key.PublicKey()
		addr := pub.EVMAddress()
		pid, err := pub.PeerID()
		require.NoError(t, err)

		assert.True(t, key.MatchesPublicKey(pub), "key %d: MatchesPublicKey false negative", i)
		assert.True(t, key.MatchesEVMAddress(addr), "key %d: MatchesEVMAddress false negative", i)
		assert.True(t, pub.MatchesPeerID(pid), "key %d: MatchesPeerID false negative", i)
		assert.True(t, pub.MatchesEVMAddress(addr), "key %d: pub.MatchesEVMAddress false negative", i)
	}
}

func TestMatching_NoFalsePositives(t *testing.T) {
	// Generate two different keys and verify cross-key matches all return false.
	key1, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	ecKey2, err := crypto.GenerateKey()
	require.NoError(t, err)
	key2, err := NeuronPrivateKeyFromBlockchainKey(ecKey2)
	require.NoError(t, err)

	pub1 := key1.PublicKey()
	pub2 := key2.PublicKey()
	addr1 := pub1.EVMAddress()
	addr2 := pub2.EVMAddress()
	pid1, err := pub1.PeerID()
	require.NoError(t, err)
	pid2, err := pub2.PeerID()
	require.NoError(t, err)

	// Cross-key: all must be false.
	assert.False(t, key1.MatchesPublicKey(pub2))
	assert.False(t, key2.MatchesPublicKey(pub1))
	assert.False(t, key1.MatchesEVMAddress(addr2))
	assert.False(t, key2.MatchesEVMAddress(addr1))
	assert.False(t, pub1.MatchesPeerID(pid2))
	assert.False(t, pub2.MatchesPeerID(pid1))
	assert.False(t, pub1.MatchesEVMAddress(addr2))
	assert.False(t, pub2.MatchesEVMAddress(addr1))
}

// --- T024: Cross-Format Verification Tests ---

func TestEVMAddressMatchesPeerID(t *testing.T) {
	t.Run("true when EVMAddress and PeerID from same public key", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		addr := pub.EVMAddress()
		pid, err := pub.PeerID()
		require.NoError(t, err)

		assert.True(t, EVMAddressMatchesPeerID(addr, pid, pub),
			"EVMAddressMatchesPeerID must return true for same-key EVMAddress and PeerID")
	})

	t.Run("false when EVMAddress from key A and PeerID from key B with pubkey B", func(t *testing.T) {
		// Key A: Hardhat #0.
		keyA, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)
		addrA := keyA.PublicKey().EVMAddress()

		// Key B: randomly generated.
		ecKeyB, err := crypto.GenerateKey()
		require.NoError(t, err)
		keyB, err := NeuronPrivateKeyFromBlockchainKey(ecKeyB)
		require.NoError(t, err)
		pubB := keyB.PublicKey()
		pidB, err := pubB.PeerID()
		require.NoError(t, err)

		// addrA (from key A) + pidB (from key B) + pubB (from key B):
		// PeerID matches pubB, but EVMAddress does NOT match pubB. Must return false.
		assert.False(t, EVMAddressMatchesPeerID(addrA, pidB, pubB),
			"EVMAddressMatchesPeerID must return false when EVMAddress is from a different key")
	})

	t.Run("false when EVMAddress from key A and PeerID from key B with pubkey A", func(t *testing.T) {
		// Key A: Hardhat #0.
		keyA, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)
		pubA := keyA.PublicKey()
		addrA := pubA.EVMAddress()

		// Key B: randomly generated.
		ecKeyB, err := crypto.GenerateKey()
		require.NoError(t, err)
		keyB, err := NeuronPrivateKeyFromBlockchainKey(ecKeyB)
		require.NoError(t, err)
		pidB, err := keyB.PublicKey().PeerID()
		require.NoError(t, err)

		// addrA (from key A) + pidB (from key B) + pubA (from key A):
		// EVMAddress matches pubA, but PeerID does NOT match pubA. Must return false.
		assert.False(t, EVMAddressMatchesPeerID(addrA, pidB, pubA),
			"EVMAddressMatchesPeerID must return false when PeerID is from a different key")
	})

	t.Run("true for multiple randomly generated keys", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			ecKey, err := crypto.GenerateKey()
			require.NoError(t, err)

			key, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
			require.NoError(t, err)

			pub := key.PublicKey()
			addr := pub.EVMAddress()
			pid, err := pub.PeerID()
			require.NoError(t, err)

			assert.True(t, EVMAddressMatchesPeerID(addr, pid, pub),
				"key %d: EVMAddressMatchesPeerID false negative", i)
		}
	})
}

// --- T025: Full Derivation Chain Determinism Test ---

func TestFullDerivationChainDeterminism(t *testing.T) {
	t.Run("all derivations are deterministic", func(t *testing.T) {
		// Generate a NeuronPrivateKey.
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		// First derivation pass.
		pub1 := key.PublicKey()
		addr1 := pub1.EVMAddress()
		pid1, err := pub1.PeerID()
		require.NoError(t, err)
		did1 := pub1.DIDKey()

		// Second derivation pass.
		pub2 := key.PublicKey()
		addr2 := pub2.EVMAddress()
		pid2, err := pub2.PeerID()
		require.NoError(t, err)
		did2 := pub2.DIDKey()

		// All pairs must be equal.
		assert.Equal(t, pub1.Bytes(), pub2.Bytes(), "public key not deterministic")
		assert.Equal(t, addr1.Bytes(), addr2.Bytes(), "EVMAddress not deterministic")
		assert.Equal(t, pid1.String(), pid2.String(), "PeerID not deterministic")
		assert.Equal(t, did1, did2, "DID:key not deterministic")
	})

	t.Run("matching functions confirm same-key identity", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		addr := pub.EVMAddress()
		pid, err := pub.PeerID()
		require.NoError(t, err)

		// All matching functions must return true for same-key values.
		assert.True(t, key.MatchesPublicKey(pub), "MatchesPublicKey failed for same key")
		assert.True(t, key.MatchesEVMAddress(addr), "PrivateKey.MatchesEVMAddress failed for same key")
		assert.True(t, pub.MatchesPeerID(pid), "MatchesPeerID failed for same key")
		assert.True(t, pub.MatchesEVMAddress(addr), "PublicKey.MatchesEVMAddress failed for same key")
		assert.True(t, EVMAddressMatchesPeerID(addr, pid, pub),
			"EVMAddressMatchesPeerID failed for same key")
	})

	t.Run("second key cross-matches all return false", func(t *testing.T) {
		// Key 1: Hardhat #0.
		key1, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub1 := key1.PublicKey()
		addr1 := pub1.EVMAddress()
		pid1, err := pub1.PeerID()
		require.NoError(t, err)

		// Key 2: randomly generated.
		ecKey2, err := crypto.GenerateKey()
		require.NoError(t, err)
		key2, err := NeuronPrivateKeyFromBlockchainKey(ecKey2)
		require.NoError(t, err)

		pub2 := key2.PublicKey()
		addr2 := pub2.EVMAddress()
		pid2, err := pub2.PeerID()
		require.NoError(t, err)

		// Cross-key matching: all must be false.
		assert.False(t, key1.MatchesPublicKey(pub2), "cross-key MatchesPublicKey should be false")
		assert.False(t, key2.MatchesPublicKey(pub1), "cross-key MatchesPublicKey should be false (reverse)")
		assert.False(t, key1.MatchesEVMAddress(addr2), "cross-key PrivateKey.MatchesEVMAddress should be false")
		assert.False(t, key2.MatchesEVMAddress(addr1), "cross-key PrivateKey.MatchesEVMAddress should be false (reverse)")
		assert.False(t, pub1.MatchesPeerID(pid2), "cross-key MatchesPeerID should be false")
		assert.False(t, pub2.MatchesPeerID(pid1), "cross-key MatchesPeerID should be false (reverse)")
		assert.False(t, pub1.MatchesEVMAddress(addr2), "cross-key PublicKey.MatchesEVMAddress should be false")
		assert.False(t, pub2.MatchesEVMAddress(addr1), "cross-key PublicKey.MatchesEVMAddress should be false (reverse)")
		assert.False(t, EVMAddressMatchesPeerID(addr1, pid2, pub2),
			"cross-key EVMAddressMatchesPeerID should be false (addr1 vs pid2/pub2)")
		assert.False(t, EVMAddressMatchesPeerID(addr2, pid1, pub1),
			"cross-key EVMAddressMatchesPeerID should be false (addr2 vs pid1/pub1)")
		assert.False(t, EVMAddressMatchesPeerID(addr1, pid2, pub1),
			"cross-key EVMAddressMatchesPeerID should be false (addr1 vs pid2/pub1)")
	})

	t.Run("determinism holds across random keys", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			ecKey, err := crypto.GenerateKey()
			require.NoError(t, err)

			key, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
			require.NoError(t, err)

			// Two full derivation passes.
			pub1 := key.PublicKey()
			addr1 := pub1.EVMAddress()
			pid1, err := pub1.PeerID()
			require.NoError(t, err)
			did1 := pub1.DIDKey()

			pub2 := key.PublicKey()
			addr2 := pub2.EVMAddress()
			pid2, err := pub2.PeerID()
			require.NoError(t, err)
			did2 := pub2.DIDKey()

			assert.Equal(t, pub1.Bytes(), pub2.Bytes(), "key %d: pub not deterministic", i)
			assert.Equal(t, addr1.Bytes(), addr2.Bytes(), "key %d: addr not deterministic", i)
			assert.Equal(t, pid1.String(), pid2.String(), "key %d: pid not deterministic", i)
			assert.Equal(t, did1, did2, "key %d: did not deterministic", i)
		}
	})
}
