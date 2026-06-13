package delivery

import (
	"crypto/ecdsa"
	"strings"
)

// ConnectionSetupResult holds the parsed result of processing a connectionSetup message.
// FR-D15: decrypt → validate → prepare for connect.
type ConnectionSetupResult struct {
	PeerID     string
	Multiaddrs []string
	Protocol   string
	NATStatus  string
}

// ProcessConnectionSetup processes a connectionSetup message by decrypting
// the multiaddrs and validating the result.
// FR-D15: (1) decrypt encryptedMultiaddrs, (2) validate multiaddr format,
// (3) return parsed fields for connect().
// FR-D16: protocol field follows libp2p path-like convention.
func ProcessConnectionSetup(
	peerID string,
	encryptedMultiaddrs string,
	protocol string,
	natStatus string,
	recipientPrivKey *ecdsa.PrivateKey,
) (*ConnectionSetupResult, error) {
	const op = "ProcessConnectionSetup"

	// Step 1: Decrypt encryptedMultiaddrs.
	multiaddrs, err := DecryptMultiaddrs(encryptedMultiaddrs, recipientPrivKey)
	if err != nil {
		return nil, err // Already a DeliveryError from ecies.go
	}

	// Step 2: Validate multiaddr format.
	for _, ma := range multiaddrs {
		if !isValidMultiaddrFormat(ma) {
			return nil, NewDeliveryError(ErrInvalidMultiaddr, op,
				"invalid multiaddr format: "+ma)
		}
	}

	if len(multiaddrs) == 0 {
		return nil, NewDeliveryError(ErrNoCompatibleTransport, op,
			"decrypted multiaddrs list is empty")
	}

	// Step 3: Validate protocol format. FR-D16.
	if !isValidProtocolID(protocol) {
		return nil, NewDeliveryError(ErrDialFailed, op,
			"invalid protocol ID format: "+protocol)
	}

	return &ConnectionSetupResult{
		PeerID:     peerID,
		Multiaddrs: multiaddrs,
		Protocol:   protocol,
		NATStatus:  natStatus,
	}, nil
}

// MaxConnectionSetupRetries is the recommended maximum number of connectionSetup
// retry attempts. FR-D17: SHOULD NOT exceed 3.
const MaxConnectionSetupRetries = 3

// RetryConnectionSetup attempts to process a connectionSetup and connect via
// the adapter, retrying up to maxRetries times if connect fails.
// FR-D17: If connect fails, the receiver MAY retry with updated multiaddrs.
func RetryConnectionSetup(
	adapter DeliveryAdapter,
	peerID string,
	encryptedMultiaddrs string,
	protocol string,
	natStatus string,
	recipientPrivKey *ecdsa.PrivateKey,
	maxRetries int,
) (*DeliveryChannel, error) {
	const op = "RetryConnectionSetup"

	if maxRetries <= 0 {
		maxRetries = 1
	}
	if maxRetries > MaxConnectionSetupRetries {
		maxRetries = MaxConnectionSetupRetries
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		setup, err := ProcessConnectionSetup(peerID, encryptedMultiaddrs, protocol, natStatus, recipientPrivKey)
		if err != nil {
			lastErr = err
			continue
		}

		channel, err := adapter.Connect(setup.PeerID, setup.Multiaddrs, setup.Protocol, nil)
		if err != nil {
			lastErr = err
			continue
		}

		return channel, nil
	}

	return nil, NewDeliveryError(ErrDialFailed, op,
		"all connection attempts failed after retries: "+lastErr.Error())
}

// isValidMultiaddrFormat performs basic format validation on a multiaddr string.
// Full validation requires the go-multiaddr library; this checks basic structure.
func isValidMultiaddrFormat(ma string) bool {
	if len(ma) == 0 {
		return false
	}
	// Multiaddrs start with /
	if ma[0] != '/' {
		return false
	}
	// Must have at least one protocol component (e.g., /ip4/...)
	parts := strings.Split(ma[1:], "/")
	return len(parts) >= 2
}

// isValidProtocolID validates a libp2p protocol ID format.
// FR-D16: path-like string with version (e.g., "/neuron/adsb/1.0.0").
func isValidProtocolID(protocol string) bool {
	if len(protocol) == 0 {
		return false
	}
	if protocol[0] != '/' {
		return false
	}
	// Must have at least two path segments.
	parts := strings.Split(protocol[1:], "/")
	return len(parts) >= 2
}
