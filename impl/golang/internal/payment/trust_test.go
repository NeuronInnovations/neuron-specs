package payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- T039: Trust Gating Tests ---

func TestPreEngagementCheck_AllPass(t *testing.T) {
	// FR-P30: All checks pass.
	result := PreEngagementCheck(true, true, true, true)
	assert.True(t, result.AllPassed())
	assert.Empty(t, result.Warnings)
	assert.True(t, result.LivenessOK)
	assert.True(t, result.RegistrationOK)
	assert.True(t, result.ReputationOK)
	assert.True(t, result.ValidationOK)
}

func TestPreEngagementCheck_SellerDead(t *testing.T) {
	// FR-P30: Seller not alive — warning, not error.
	result := PreEngagementCheck(false, true, true, true)
	assert.False(t, result.AllPassed())
	assert.False(t, result.LivenessOK)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "LivenessState")
}

func TestPreEngagementCheck_NotRegistered(t *testing.T) {
	result := PreEngagementCheck(true, false, true, true)
	assert.False(t, result.AllPassed())
	assert.False(t, result.RegistrationOK)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "Identity Registry")
}

func TestPreEngagementCheck_MultipleWarnings(t *testing.T) {
	result := PreEngagementCheck(false, false, false, false)
	assert.False(t, result.AllPassed())
	assert.Len(t, result.Warnings, 4)
}

func TestPreEngagementCheck_ReputationOnly(t *testing.T) {
	result := PreEngagementCheck(true, true, false, true)
	assert.False(t, result.AllPassed())
	assert.True(t, result.LivenessOK)
	assert.True(t, result.RegistrationOK)
	assert.False(t, result.ReputationOK)
}
