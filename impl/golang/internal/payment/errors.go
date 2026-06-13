package payment

import "fmt"

// PaymentErrorKind identifies the category of a payment protocol error.
// FR-P32: NEURON-PAYMENT-* domain following 006 error taxonomy.
type PaymentErrorKind string

const (
	// ErrInvalidServiceOffering — neuron-commerce missing MUST fields. FR-P01.
	ErrInvalidServiceOffering PaymentErrorKind = "InvalidServiceOffering"

	// ErrNegotiationFailed — serviceResponse indicates rejection. FR-P08.
	ErrNegotiationFailed PaymentErrorKind = "NegotiationFailed"

	// ErrNegotiationExpired — negotiationDeadline elapsed without response. FR-P07a.
	ErrNegotiationExpired PaymentErrorKind = "NegotiationExpired"

	// ErrVersionMismatch — payload version major >= 2. FR-P12a.
	ErrVersionMismatch PaymentErrorKind = "VersionMismatch"

	// ErrEscrowCreationFailed — createEscrow failed. FR-P17.
	ErrEscrowCreationFailed PaymentErrorKind = "EscrowCreationFailed"

	// ErrInsufficientBalance — requestRelease amount > available. FR-P25a.
	ErrInsufficientBalance PaymentErrorKind = "InsufficientBalance"

	// ErrInvoiceValidationFailed — invoice doesn't match agreed terms. FR-P10.
	ErrInvoiceValidationFailed PaymentErrorKind = "InvoiceValidationFailed"

	// ErrReleaseNotAuthorized — release approval rejected. FR-P21.
	ErrReleaseNotAuthorized PaymentErrorKind = "ReleaseNotAuthorized"

	// ErrRefundNotEligible — claimRefund before timeout. FR-P22.
	ErrRefundNotEligible PaymentErrorKind = "RefundNotEligible"

	// ErrBindingUnavailable — settlement binding not found. FR-P02.
	ErrBindingUnavailable PaymentErrorKind = "BindingUnavailable"

	// ErrTimeoutNotElapsed — refund attempted pre-timeout. FR-P22.
	ErrTimeoutNotElapsed PaymentErrorKind = "TimeoutNotElapsed"

	// ErrInvalidEscrowRef — escrow reference invalid. FR-P23.
	ErrInvalidEscrowRef PaymentErrorKind = "InvalidEscrowRef"

	// ErrUnsupportedDeliveryMode — delivery.mode not recognized. FR-P01a.
	ErrUnsupportedDeliveryMode PaymentErrorKind = "UnsupportedDeliveryMode"

	// ErrInvalidDeliveryRef — delivery cross-reference broken. FR-P01b.
	ErrInvalidDeliveryRef PaymentErrorKind = "InvalidDeliveryRef"

	// ErrConnectionSetupRequired — P2P mode but no connectionSetup. FR-P35.
	ErrConnectionSetupRequired PaymentErrorKind = "ConnectionSetupRequired"

	// ErrConnectionSetupEncryptionFailed — encryption/decryption failed. FR-P34.
	ErrConnectionSetupEncryptionFailed PaymentErrorKind = "ConnectionSetupEncryptionFailed"
)

// PaymentError is the structured error type for Spec 008.
// Format: "payment.Operation: [Kind] descriptive message"
// FR-P32: NEURON-PAYMENT-* domain.
type PaymentError struct {
	kind      PaymentErrorKind
	operation string
	message   string
	cause     error
}

// NewPaymentError creates a PaymentError with the given kind, operation, and message.
func NewPaymentError(kind PaymentErrorKind, operation, message string) *PaymentError {
	return &PaymentError{kind: kind, operation: operation, message: message}
}

// WrapPaymentError creates a PaymentError that wraps an underlying cause.
func WrapPaymentError(kind PaymentErrorKind, operation string, cause error) *PaymentError {
	return &PaymentError{kind: kind, operation: operation, message: cause.Error(), cause: cause}
}

// Error implements the error interface.
func (e *PaymentError) Error() string {
	return fmt.Sprintf("payment.%s: [%s] %s", e.operation, e.kind, e.message)
}

// Kind returns the error category.
func (e *PaymentError) Kind() PaymentErrorKind {
	return e.kind
}

// Operation returns the operation that produced this error.
func (e *PaymentError) Operation() string {
	return e.operation
}

// Unwrap returns the underlying cause, if any.
func (e *PaymentError) Unwrap() error {
	return e.cause
}

// Is supports errors.Is matching by kind.
func (e *PaymentError) Is(target error) bool {
	if t, ok := target.(*PaymentError); ok {
		return e.kind == t.kind
	}
	return false
}
