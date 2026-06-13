package delivery

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T015: ConnectionSetup Processing Tests ---

func TestProcessConnectionSetup_Success(t *testing.T) {
	// FR-D15: decrypt → validate → return parsed fields.
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	multiaddrs := []string{"/ip4/192.168.1.1/udp/4001/quic-v1"}
	encrypted, err := EncryptMultiaddrs(multiaddrs, &key.PublicKey)
	require.NoError(t, err)

	result, err := ProcessConnectionSetup(
		"12D3KooWTest", encrypted, "/neuron/adsb/1.0.0", "public", key,
	)
	require.NoError(t, err)
	assert.Equal(t, "12D3KooWTest", result.PeerID)
	assert.Equal(t, multiaddrs, result.Multiaddrs)
	assert.Equal(t, "/neuron/adsb/1.0.0", result.Protocol)
	assert.Equal(t, "public", result.NATStatus)
}

func TestProcessConnectionSetup_InvalidMultiaddr(t *testing.T) {
	// FR-D15: invalid multiaddr → InvalidMultiaddr error.
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Encrypt invalid multiaddrs.
	encrypted, err := EncryptMultiaddrs([]string{"not-a-multiaddr"}, &key.PublicKey)
	require.NoError(t, err)

	_, err = ProcessConnectionSetup(
		"12D3KooWTest", encrypted, "/neuron/adsb/1.0.0", "public", key,
	)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrInvalidMultiaddr, de.Kind())
}

func TestProcessConnectionSetup_EmptyMultiaddrs(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	encrypted, err := EncryptMultiaddrs([]string{}, &key.PublicKey)
	require.NoError(t, err)

	_, err = ProcessConnectionSetup(
		"12D3KooWTest", encrypted, "/neuron/adsb/1.0.0", "public", key,
	)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrNoCompatibleTransport, de.Kind())
}

func TestProcessConnectionSetup_InvalidProtocol(t *testing.T) {
	// FR-D16: invalid protocol ID.
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	encrypted, err := EncryptMultiaddrs([]string{"/ip4/1.2.3.4/udp/4001/quic-v1"}, &key.PublicKey)
	require.NoError(t, err)

	_, err = ProcessConnectionSetup(
		"12D3KooWTest", encrypted, "invalid-no-slash", "public", key,
	)
	require.Error(t, err)
}

func TestProcessConnectionSetup_DecryptionFails(t *testing.T) {
	// FR-D14: wrong key → encryption failure.
	key1, err := crypto.GenerateKey()
	require.NoError(t, err)
	key2, err := crypto.GenerateKey()
	require.NoError(t, err)

	encrypted, err := EncryptMultiaddrs([]string{"/ip4/1.2.3.4/udp/4001/quic-v1"}, &key1.PublicKey)
	require.NoError(t, err)

	_, err = ProcessConnectionSetup(
		"12D3KooWTest", encrypted, "/neuron/adsb/1.0.0", "public", key2,
	)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrConnectionSetupEncryptionFailed, de.Kind())
}

func TestIsValidMultiaddrFormat(t *testing.T) {
	assert.True(t, isValidMultiaddrFormat("/ip4/1.2.3.4/udp/4001"))
	assert.True(t, isValidMultiaddrFormat("/ip4/1.2.3.4/udp/4001/quic-v1"))
	assert.False(t, isValidMultiaddrFormat(""))
	assert.False(t, isValidMultiaddrFormat("no-slash"))
	assert.False(t, isValidMultiaddrFormat("/single"))
}

func TestIsValidProtocolID(t *testing.T) {
	assert.True(t, isValidProtocolID("/neuron/adsb/1.0.0"))
	assert.True(t, isValidProtocolID("/my-app/v1"))
	assert.False(t, isValidProtocolID(""))
	assert.False(t, isValidProtocolID("no-slash"))
	assert.False(t, isValidProtocolID("/single"))
}
