package payment

// TrustCheckResult holds the result of a pre-engagement trust check. FR-P30.
// All checks are SHOULD-level — warnings, not errors.
type TrustCheckResult struct {
	LivenessOK     bool
	RegistrationOK bool
	ReputationOK   bool
	ValidationOK   bool
	Warnings       []string
}

// AllPassed returns true if all trust checks passed.
func (r TrustCheckResult) AllPassed() bool {
	return r.LivenessOK && r.RegistrationOK && r.ReputationOK && r.ValidationOK
}

// PreEngagementCheck performs SHOULD-level trust verification before funding
// an escrow. FR-P30: liveness, identity registration, reputation, validation.
//
// This function accepts check results as parameters rather than importing
// health/registry/reputation packages directly — avoiding circular imports
// and keeping the trust module a pure decision layer.
func PreEngagementCheck(
	livenessAlive bool,
	registered bool,
	reputationPositive bool,
	validationPassed bool,
) TrustCheckResult {
	result := TrustCheckResult{
		LivenessOK:     livenessAlive,
		RegistrationOK: registered,
		ReputationOK:   reputationPositive,
		ValidationOK:   validationPassed,
	}

	if !livenessAlive {
		result.Warnings = append(result.Warnings,
			"seller LivenessState is not ALIVE — engagement risk (FR-P30)")
	}
	if !registered {
		result.Warnings = append(result.Warnings,
			"seller not found in Identity Registry — engagement risk (FR-P30)")
	}
	if !reputationPositive {
		result.Warnings = append(result.Warnings,
			"seller reputation is negative or unknown — engagement risk (FR-P30)")
	}
	if !validationPassed {
		result.Warnings = append(result.Warnings,
			"seller validation status is not passed — engagement risk (FR-P30)")
	}

	return result
}
