package keylib

import "fmt"

// ErrorKind enumerates the categories of errors in the keylib package.
// FR-008: All error types MUST provide descriptive messages.
type ErrorKind int

const (
	ErrInvalidFormat      ErrorKind = iota + 1 // Input format not recognized
	ErrInvalidLength                           // Input has wrong byte length
	ErrInvalidHex                              // Non-hex characters in input (with position)
	ErrInvalidKey                              // Key fails secp256k1 curve validation
	ErrZeroValue                               // All-zero key material
	ErrKeyMismatch                             // Key relationship verification failed
	ErrEncryption                              // Encrypt/decrypt failure
	ErrMnemonic                                // Invalid mnemonic (bad checksum, etc.)
	ErrDerivation                              // BIP-44 derivation failure
	ErrUnsupportedKeyType                      // Operation not supported for key type (e.g. Ed25519)
	ErrSDKError                                // Wrapped blockchain SDK error
)

var errorKindNames = map[ErrorKind]string{
	ErrInvalidFormat:      "InvalidFormat",
	ErrInvalidLength:      "InvalidLength",
	ErrInvalidHex:         "InvalidHex",
	ErrInvalidKey:         "InvalidKey",
	ErrZeroValue:          "ZeroValue",
	ErrKeyMismatch:        "KeyMismatch",
	ErrEncryption:         "Encryption",
	ErrMnemonic:           "Mnemonic",
	ErrDerivation:         "Derivation",
	ErrUnsupportedKeyType: "UnsupportedKeyType",
	ErrSDKError:           "SDKError",
}

// String returns the name of the error kind.
func (k ErrorKind) String() string {
	if name, ok := errorKindNames[k]; ok {
		return name
	}
	return fmt.Sprintf("ErrorKind(%d)", int(k))
}

// KeyError is the structured error type for all keylib operations.
// It includes the error kind, the operation that failed, and a descriptive message.
// Error messages MUST NOT contain private key material (SEC-003, SEC-005).
type KeyError struct {
	kind      ErrorKind
	operation string
	message   string
	wrapped   error // non-nil only for ErrSDKError
}

// NewKeyError creates a new KeyError with the given kind, operation, and message.
func NewKeyError(kind ErrorKind, operation, message string) *KeyError {
	return &KeyError{
		kind:      kind,
		operation: operation,
		message:   message,
	}
}

// NewSDKError creates a KeyError of kind ErrSDKError that wraps an underlying error.
// FR-008a: SDKError wraps underlying blockchain SDK errors.
func NewSDKError(operation string, err error) *KeyError {
	return &KeyError{
		kind:      ErrSDKError,
		operation: operation,
		message:   err.Error(),
		wrapped:   err,
	}
}

// Error returns a descriptive error message including kind, operation, and context.
func (e *KeyError) Error() string {
	return fmt.Sprintf("keylib.%s: [%s] %s", e.operation, e.kind.String(), e.message)
}

// Kind returns the error kind.
func (e *KeyError) Kind() ErrorKind {
	return e.kind
}

// Operation returns the operation name where the error occurred.
func (e *KeyError) Operation() string {
	return e.operation
}

// Unwrap returns the underlying error for ErrSDKError, or nil.
func (e *KeyError) Unwrap() error {
	return e.wrapped
}

// Is supports errors.Is comparison by error kind.
func (e *KeyError) Is(target error) bool {
	if t, ok := target.(*KeyError); ok {
		return e.kind == t.kind
	}
	return false
}
