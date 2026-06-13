package account

import "fmt"

// FR-014: Actionable error messages on validation failure
// ValidationError represents a structured validation failure for account construction
// or mutation. Each error pinpoints the offending field, a machine-readable rule code,
// and a human-readable actionable message.
//
// Rule codes follow the pattern "V-{TYPE}-{NN}" where TYPE is PARENT, CHILD, or SHARED,
// and NN is a zero-padded sequence number (e.g. "V-PARENT-01", "V-CHILD-03").
type ValidationError struct {
	Field    string // the field that failed validation, e.g. "publicKey", "did"
	RuleCode string // machine-readable rule code, e.g. "V-PARENT-01"
	Message  string // human-readable, actionable description of the failure
}

// Error implements the error interface, returning a formatted string that includes
// the rule code, field name, and message in the form "[RuleCode] Field: Message".
func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.RuleCode, e.Field, e.Message)
}

// Validate checks ALL validation rules for this account's type and returns every
// violation found. It does not short-circuit on the first error; callers receive
// the complete list of problems so they can be displayed or logged together.
//
// Returns an empty slice when the account satisfies all rules for its type.
// Returns a single-element slice with a generic error for Unspecified (zero-value)
// account types.
func (a *NeuronAccount) Validate() []ValidationError {
	switch a.accountType {
	case Parent:
		return a.validateParent()
	case Child:
		return a.validateChild()
	case Shared:
		return a.validateShared()
	default:
		return []ValidationError{
			{
				Field:    "accountType",
				RuleCode: "V-TYPE-00",
				Message:  "account type is unspecified or unknown; must be Parent, Child, or Shared",
			},
		}
	}
}

// validateParent checks rules V-PARENT-01 through V-PARENT-05.
func (a *NeuronAccount) validateParent() []ValidationError {
	var errs []ValidationError

	// V-PARENT-01: Must have DID (did != nil && did.identifier != "")
	if a.did == nil || a.did.identifier == "" {
		errs = append(errs, ValidationError{
			Field:    "did",
			RuleCode: "V-PARENT-01",
			Message:  "Parent account must have a DID",
		})
	}

	// V-PARENT-02: Must have single NeuronPublicKey (publicKey != nil && !isZeroPublicKey)
	if a.publicKey == nil || isZeroPublicKey(*a.publicKey) {
		errs = append(errs, ValidationError{
			Field:    "publicKey",
			RuleCode: "V-PARENT-02",
			Message:  "Parent account must have a single NeuronPublicKey",
		})
	}

	// V-PARENT-03: Must have currency symbol (currencySymbol != "")
	if a.currencySymbol == "" {
		errs = append(errs, ValidationError{
			Field:    "currencySymbol",
			RuleCode: "V-PARENT-03",
			Message:  "Parent account must have a currency symbol",
		})
	}

	// V-PARENT-04: Must NOT have parent reference (parentPubKey == nil)
	if a.parentPubKey != nil {
		errs = append(errs, ValidationError{
			Field:    "parentPubKey",
			RuleCode: "V-PARENT-04",
			Message:  "Parent account must not have a parent public key reference",
		})
	}

	// V-PARENT-05: Must NOT have multisig key (multisigKey == nil)
	if a.multisigKey != nil {
		errs = append(errs, ValidationError{
			Field:    "multisigKey",
			RuleCode: "V-PARENT-05",
			Message:  "Parent account must not have a multisig key",
		})
	}

	return errs
}

// validateChild checks rules V-CHILD-01 through V-CHILD-05.
func (a *NeuronAccount) validateChild() []ValidationError {
	var errs []ValidationError

	// V-CHILD-01: Must have parent NeuronPublicKey ref (parentPubKey != nil && !isZeroPublicKey)
	if a.parentPubKey == nil || isZeroPublicKey(*a.parentPubKey) {
		errs = append(errs, ValidationError{
			Field:    "parentPubKey",
			RuleCode: "V-CHILD-01",
			Message:  "Child account must reference a parent NeuronPublicKey",
		})
	}

	// V-CHILD-02: Must have single NeuronPublicKey (publicKey != nil && !isZeroPublicKey)
	if a.publicKey == nil || isZeroPublicKey(*a.publicKey) {
		errs = append(errs, ValidationError{
			Field:    "publicKey",
			RuleCode: "V-CHILD-02",
			Message:  "Child account must have a single NeuronPublicKey",
		})
	}

	// V-CHILD-03: Must have registry binding (registryBinding != nil)
	if a.registryBinding == nil {
		errs = append(errs, ValidationError{
			Field:    "registryBinding",
			RuleCode: "V-CHILD-03",
			Message:  "Child account must have a registry binding",
		})
	}

	// V-CHILD-04: Must NOT have DID (did == nil)
	if a.did != nil {
		errs = append(errs, ValidationError{
			Field:    "did",
			RuleCode: "V-CHILD-04",
			Message:  "Child account must not have a DID",
		})
	}

	// V-CHILD-05: Must NOT have multisig key (multisigKey == nil)
	if a.multisigKey != nil {
		errs = append(errs, ValidationError{
			Field:    "multisigKey",
			RuleCode: "V-CHILD-05",
			Message:  "Child account must not have a multisig key",
		})
	}

	return errs
}

// validateShared checks rules V-SHARED-01 through V-SHARED-04.
func (a *NeuronAccount) validateShared() []ValidationError {
	var errs []ValidationError

	// V-SHARED-01: Must have MultisigKey with threshold (multisigKey != nil)
	if a.multisigKey == nil {
		errs = append(errs, ValidationError{
			Field:    "multisigKey",
			RuleCode: "V-SHARED-01",
			Message:  "Shared account must have a MultisigKey with threshold",
		})
	}

	// V-SHARED-02: Must NOT have DID (did == nil)
	if a.did != nil {
		errs = append(errs, ValidationError{
			Field:    "did",
			RuleCode: "V-SHARED-02",
			Message:  "Shared account must not have a DID",
		})
	}

	// V-SHARED-03: Must NOT have parent reference (parentPubKey == nil)
	if a.parentPubKey != nil {
		errs = append(errs, ValidationError{
			Field:    "parentPubKey",
			RuleCode: "V-SHARED-03",
			Message:  "Shared account must not have a parent public key reference",
		})
	}

	// V-SHARED-04: Must NOT have single publicKey (publicKey == nil)
	if a.publicKey != nil {
		errs = append(errs, ValidationError{
			Field:    "publicKey",
			RuleCode: "V-SHARED-04",
			Message:  "Shared account must not have a single public key",
		})
	}

	return errs
}
