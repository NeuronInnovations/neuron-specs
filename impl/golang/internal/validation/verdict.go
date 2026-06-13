package validation

import "fmt"

// Verdict represents a three-outcome validation result. FR-V04.
type Verdict string

const (
	// VerdictCompliant indicates the subject agent is compliant with the assessed spec.
	VerdictCompliant Verdict = "compliant"
	// VerdictNonCompliant indicates the subject agent violates the assessed spec.
	VerdictNonCompliant Verdict = "non-compliant"
	// VerdictInconclusive indicates the validator could not reach a definitive conclusion.
	VerdictInconclusive Verdict = "inconclusive"
)

// RegistryCode represents the on-chain verdict code in the Validation Registry. FR-V08.
type RegistryCode uint8

const (
	// CodePending is the initial state before a verdict is rendered.
	CodePending RegistryCode = 0
	// CodePass maps to VerdictCompliant.
	CodePass RegistryCode = 1
	// CodeFail maps to VerdictNonCompliant.
	CodeFail RegistryCode = 2
	// CodeInconclusive maps to VerdictInconclusive.
	CodeInconclusive RegistryCode = 3
)

// IsValidVerdict returns true if v is one of the three allowed verdict values. FR-V04.
func IsValidVerdict(v Verdict) bool {
	switch v {
	case VerdictCompliant, VerdictNonCompliant, VerdictInconclusive:
		return true
	default:
		return false
	}
}

// VerdictToCode maps a Verdict string to its on-chain RegistryCode. FR-V08.
func VerdictToCode(v Verdict) (RegistryCode, error) {
	switch v {
	case VerdictCompliant:
		return CodePass, nil
	case VerdictNonCompliant:
		return CodeFail, nil
	case VerdictInconclusive:
		return CodeInconclusive, nil
	default:
		return 0, NewValidationError(ErrInvalidVerdict,
			fmt.Sprintf("unknown verdict %q; expected compliant, non-compliant, or inconclusive", v))
	}
}

// CodeToVerdict maps an on-chain RegistryCode to a Verdict string. FR-V08.
// CodePending (0) has no corresponding verdict and returns an error.
func CodeToVerdict(c RegistryCode) (Verdict, error) {
	switch c {
	case CodePass:
		return VerdictCompliant, nil
	case CodeFail:
		return VerdictNonCompliant, nil
	case CodeInconclusive:
		return VerdictInconclusive, nil
	default:
		return "", NewValidationError(ErrInvalidVerdict,
			fmt.Sprintf("registry code %d has no verdict mapping (0=PENDING has no verdict)", c))
	}
}
