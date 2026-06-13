package topic

import (
	"crypto/subtle"
	"strings"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

// FR-T10: Validate TopicMessage integrity (verify signature, match senderAddress)
// ValidateTopicMessage verifies the integrity of a TopicMessage by:
//  1. Rebuilding the canonical signing input from timestamp, sequenceNumber, and payload.
//  2. Recovering the signer's public key from the signature over the signing input.
//  3. Deriving the EVM address from the recovered public key.
//  4. Comparing the recovered EVM address with msg.senderAddress using constant-time comparison.
//
// Returns nil if the message is valid. Returns:
//   - ErrInvalidSignature if the signature is empty or recovery fails.
//   - ErrSenderMismatch if the recovered address does not match senderAddress.
func ValidateTopicMessage(msg TopicMessage) error {
	if len(msg.signature) == 0 {
		return NewTopicError(ErrInvalidSignature, "signature is empty")
	}

	sig, err := keylib.SignatureFromBytes(msg.signature)
	if err != nil {
		return WrapTopicError(ErrInvalidSignature, "failed to parse signature bytes", err)
	}

	signingInput := TopicMessageSigningInput(msg.timestamp, msg.sequenceNumber, msg.payload)

	recoveredPub, err := sig.RecoverPublicKey(signingInput)
	if err != nil {
		return WrapTopicError(ErrInvalidSignature, "failed to recover public key from signature", err)
	}

	recoveredAddr := recoveredPub.EVMAddress().Hex()

	// Constant-time comparison of EIP-55 checksummed addresses.
	// Both are produced by EVMAddress.Hex(), so case matches.
	// Normalize to lowercase for robust comparison in case the stored senderAddress
	// uses a different case convention.
	if subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(recoveredAddr)),
		[]byte(strings.ToLower(msg.senderAddress)),
	) != 1 {
		return NewTopicError(ErrSenderMismatch,
			"recovered signer address does not match senderAddress")
	}

	return nil
}
