package delivery

import (
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// Role identifies which side of the agreement is acting. The payment-layer
// type already names these "seller-initiates" / "buyer-initiates" in
// payment.StreamDirectionSeller / payment.StreamDirectionBuyer; Role
// names the actor in the symmetric way (Seller, Buyer) to keep call sites
// readable.
type Role string

const (
	// RoleSeller — the producer side of the agreement.
	RoleSeller Role = "seller"

	// RoleBuyer — the consumer side of the agreement.
	RoleBuyer Role = "buyer"
)

// ValidateStreamDirection returns an error if direction is not one of the
// three values defined by 008 FR-P33a / 009 FR-D-stream-direction. Empty
// strings are NOT accepted — back-compat for the legacy single-string
// `protocol` field (which carries no direction at all) is handled by
// LegacyStreamDirection, not by this validator.
func ValidateStreamDirection(direction string) error {
	switch direction {
	case payment.StreamDirectionSeller,
		payment.StreamDirectionBuyer,
		payment.StreamDirectionEither:
		return nil
	default:
		return NewDeliveryError(ErrUnknownStreamDirection, "ValidateStreamDirection",
			"unknown stream direction: "+direction+
				` (want "seller-initiates" | "buyer-initiates" | "either")`)
	}
}

// LegacyStreamDirection is the implicit direction assigned to streams
// advertised via the legacy single-string `protocol` field on
// ConnectionSetup (FR-P33a back-compat). The legacy convention was
// "seller dials, seller opens stream", so legacy streams default to
// seller-initiates.
func LegacyStreamDirection() string {
	return payment.StreamDirectionSeller
}

// IsOpenAllowed reports whether the given role MAY call OpenStream for
// a stream whose declared direction is the given value. Per
// FR-D-stream-direction:
//
//   - direction = "seller-initiates" → only RoleSeller may open.
//   - direction = "buyer-initiates"  → only RoleBuyer may open.
//   - direction = "either"           → both roles may open.
//
// Returns false for unknown direction values. Callers SHOULD call
// ValidateStreamDirection first to surface a typed error for unknowns.
func IsOpenAllowed(direction string, opener Role) bool {
	switch direction {
	case payment.StreamDirectionSeller:
		return opener == RoleSeller
	case payment.StreamDirectionBuyer:
		return opener == RoleBuyer
	case payment.StreamDirectionEither:
		return opener == RoleSeller || opener == RoleBuyer
	default:
		return false
	}
}

// CheckOpenAllowed combines ValidateStreamDirection with IsOpenAllowed
// and returns a StreamDirectionViolation error if the opener is not
// permitted. Returns the underlying validation error if the direction
// value itself is unknown. Returns nil on permitted opens.
//
// Intended for the libp2p adapter's NewStream call site once the full
// adapter integration lands (Phase 6 or later). In Phase 1 this helper
// is exposed for unit-level validation against the wire-format payload
// before full adapter wiring.
func CheckOpenAllowed(direction string, opener Role, protocolID string) error {
	if err := ValidateStreamDirection(direction); err != nil {
		return err
	}
	if !IsOpenAllowed(direction, opener) {
		return NewDeliveryError(ErrStreamDirectionViolation, "CheckOpenAllowed",
			"role "+string(opener)+" cannot open protocol "+protocolID+
				" (direction="+direction+")")
	}
	return nil
}

// CheckCatalogEntry validates a single StreamCatalogEntry from a
// ConnectionSetup.Streams[] array per FR-P33a + FR-D-stream-direction.
// Returns nil when the entry's Direction field is one of the three
// allowed values; returns a typed ErrUnknownStreamDirection otherwise.
// Empty Direction values are rejected (the spec requires direction to be
// present on every entry).
func CheckCatalogEntry(entry payment.StreamCatalogEntry) error {
	if entry.Direction == "" {
		return NewDeliveryError(ErrUnknownStreamDirection, "CheckCatalogEntry",
			"stream catalog entry "+entry.Name+" has empty direction")
	}
	return ValidateStreamDirection(entry.Direction)
}

// CheckCatalog validates every entry in a streams[] catalog and returns
// the first error encountered. Returns nil on a fully valid catalog or
// when streams is empty (the legacy single-protocol path applies, with
// LegacyStreamDirection() supplying the implicit default).
func CheckCatalog(streams []payment.StreamCatalogEntry) error {
	for _, entry := range streams {
		if err := CheckCatalogEntry(entry); err != nil {
			return err
		}
	}
	return nil
}
