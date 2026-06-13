package delivery

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T011: ECIES Encrypt/Decrypt Tests ---

func TestECIES_RoundTrip(t *testing.T) {
	// FR-D11, SC-D02: Encrypt + decrypt round-trip recovers original multiaddrs.
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	multiaddrs := []string{
		"/ip4/192.168.1.1/udp/4001/quic-v1",
		"/ip4/10.0.0.1/tcp/4002",
	}

	encrypted, err := EncryptMultiaddrs(multiaddrs, &key.PublicKey)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// Verify it's valid base64.
	_, err = base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)

	// Decrypt with matching key.
	decrypted, err := DecryptMultiaddrs(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, multiaddrs, decrypted)
}

func TestECIES_WrongKey(t *testing.T) {
	// FR-D14, SC-D02: Wrong key → ConnectionSetupEncryptionFailed.
	key1, err := crypto.GenerateKey()
	require.NoError(t, err)
	key2, err := crypto.GenerateKey()
	require.NoError(t, err)

	multiaddrs := []string{"/ip4/1.2.3.4/udp/4001/quic-v1"}

	encrypted, err := EncryptMultiaddrs(multiaddrs, &key1.PublicKey)
	require.NoError(t, err)

	// Decrypt with different key must fail.
	_, err = DecryptMultiaddrs(encrypted, key2)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrConnectionSetupEncryptionFailed, de.Kind())
}

func TestECIES_Randomized(t *testing.T) {
	// FR-D13: Same input → different ciphertext (ephemeral key per call).
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	multiaddrs := []string{"/ip4/1.2.3.4/udp/4001/quic-v1"}

	enc1, err := EncryptMultiaddrs(multiaddrs, &key.PublicKey)
	require.NoError(t, err)

	enc2, err := EncryptMultiaddrs(multiaddrs, &key.PublicKey)
	require.NoError(t, err)

	assert.NotEqual(t, enc1, enc2, "two encryptions of same data must produce different ciphertexts")

	// Both must decrypt correctly.
	dec1, err := DecryptMultiaddrs(enc1, key)
	require.NoError(t, err)
	assert.Equal(t, multiaddrs, dec1)

	dec2, err := DecryptMultiaddrs(enc2, key)
	require.NoError(t, err)
	assert.Equal(t, multiaddrs, dec2)
}

func TestECIES_ValidBase64(t *testing.T) {
	// FR-D12: Output is valid base64 per 006 FR-W03.
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	encrypted, err := EncryptMultiaddrs([]string{"/ip4/1.2.3.4/udp/4001/quic-v1"}, &key.PublicKey)
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(encrypted)
	require.NoError(t, err)
	// Minimum: 33 (ephPub) + 12 (nonce) + 16 (tag) = 61 bytes + ciphertext
	assert.GreaterOrEqual(t, len(raw), 61)
}

// --- T012: ECIES Edge Cases ---

func TestECIES_EmptyMultiaddrArray(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	encrypted, err := EncryptMultiaddrs([]string{}, &key.PublicKey)
	require.NoError(t, err)

	decrypted, err := DecryptMultiaddrs(encrypted, key)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestECIES_SingleMultiaddr(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	multiaddrs := []string{"/ip4/127.0.0.1/udp/4001/quic-v1"}

	encrypted, err := EncryptMultiaddrs(multiaddrs, &key.PublicKey)
	require.NoError(t, err)

	decrypted, err := DecryptMultiaddrs(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, multiaddrs, decrypted)
}

func TestECIES_LargeMultiaddrList(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	multiaddrs := make([]string, 50)
	for i := range multiaddrs {
		multiaddrs[i] = "/ip4/10.0.0.1/udp/4001/quic-v1"
	}

	encrypted, err := EncryptMultiaddrs(multiaddrs, &key.PublicKey)
	require.NoError(t, err)

	decrypted, err := DecryptMultiaddrs(encrypted, key)
	require.NoError(t, err)
	assert.Len(t, decrypted, 50)
}

func TestECIES_MalformedBase64(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	_, err = DecryptMultiaddrs("not-valid-base64!!!", key)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrConnectionSetupEncryptionFailed, de.Kind())
}

func TestECIES_TruncatedCiphertext(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Too short (< 61 bytes minimum).
	short := base64.StdEncoding.EncodeToString(make([]byte, 30))
	_, err = DecryptMultiaddrs(short, key)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrConnectionSetupEncryptionFailed, de.Kind())
}

func TestECIES_NilPublicKey(t *testing.T) {
	_, err := EncryptMultiaddrs([]string{"/ip4/1.2.3.4/udp/4001"}, nil)
	require.Error(t, err)
}

func TestECIES_NilPrivateKey(t *testing.T) {
	_, err := DecryptMultiaddrs("base64data==", nil)
	require.Error(t, err)
}
