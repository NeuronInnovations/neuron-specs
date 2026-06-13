package delivery

import (
	"errors"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FR-D-stream-direction: the three accepted values are seller-initiates /
// buyer-initiates / either, sourced from payment.StreamDirection* constants.

func TestValidateStreamDirection_AcceptsCanonicalValues(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{
		payment.StreamDirectionSeller,
		payment.StreamDirectionBuyer,
		payment.StreamDirectionEither,
	} {
		assert.NoError(t, ValidateStreamDirection(dir), "direction %q should be accepted", dir)
	}
}

func TestValidateStreamDirection_RejectsUnknown(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{
		"",
		"seller",
		"both",
		"server-initiates",
		"any",
	} {
		err := ValidateStreamDirection(dir)
		require.Error(t, err, "direction %q should be rejected", dir)
		var de *DeliveryError
		require.True(t, errors.As(err, &de), "error should be DeliveryError, got %T", err)
		assert.Equal(t, ErrUnknownStreamDirection, de.Kind())
	}
}

func TestIsOpenAllowed_SellerInitiates(t *testing.T) {
	t.Parallel()
	assert.True(t, IsOpenAllowed(payment.StreamDirectionSeller, RoleSeller))
	assert.False(t, IsOpenAllowed(payment.StreamDirectionSeller, RoleBuyer))
}

func TestIsOpenAllowed_BuyerInitiates(t *testing.T) {
	t.Parallel()
	assert.False(t, IsOpenAllowed(payment.StreamDirectionBuyer, RoleSeller))
	assert.True(t, IsOpenAllowed(payment.StreamDirectionBuyer, RoleBuyer))
}

func TestIsOpenAllowed_Either(t *testing.T) {
	t.Parallel()
	assert.True(t, IsOpenAllowed(payment.StreamDirectionEither, RoleSeller))
	assert.True(t, IsOpenAllowed(payment.StreamDirectionEither, RoleBuyer))
}

func TestIsOpenAllowed_UnknownDirectionRejected(t *testing.T) {
	t.Parallel()
	assert.False(t, IsOpenAllowed("garbage", RoleSeller))
	assert.False(t, IsOpenAllowed("garbage", RoleBuyer))
}

func TestCheckOpenAllowed_SellerInitiatesByBuyerFails(t *testing.T) {
	t.Parallel()
	err := CheckOpenAllowed(payment.StreamDirectionSeller, RoleBuyer, "/jetvision/raw/1.0.0")
	require.Error(t, err)
	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrStreamDirectionViolation, de.Kind())
	assert.Contains(t, err.Error(), "/jetvision/raw/1.0.0")
	assert.Contains(t, err.Error(), "buyer")
}

func TestCheckOpenAllowed_BuyerInitiatesBySellerFails(t *testing.T) {
	t.Parallel()
	err := CheckOpenAllowed(payment.StreamDirectionBuyer, RoleSeller, "/jetvision/status/1.0.0")
	require.Error(t, err)
	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrStreamDirectionViolation, de.Kind())
}

func TestCheckOpenAllowed_EitherBothPass(t *testing.T) {
	t.Parallel()
	assert.NoError(t, CheckOpenAllowed(payment.StreamDirectionEither, RoleSeller, "/x/y/1.0.0"))
	assert.NoError(t, CheckOpenAllowed(payment.StreamDirectionEither, RoleBuyer, "/x/y/1.0.0"))
}

func TestCheckOpenAllowed_UnknownDirectionSurfacesValidationError(t *testing.T) {
	t.Parallel()
	err := CheckOpenAllowed("not-a-direction", RoleSeller, "/x/y/1.0.0")
	require.Error(t, err)
	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrUnknownStreamDirection, de.Kind())
}

func TestLegacyStreamDirection_IsSellerInitiates(t *testing.T) {
	t.Parallel()
	// Pre-2026-05-08 implementations advertised a single-string `protocol`
	// field with no direction at all; the back-compat default is
	// "seller-initiates" (the JV-box ADS-B baseline behavior).
	assert.Equal(t, payment.StreamDirectionSeller, LegacyStreamDirection())
}

func TestCheckCatalogEntry_AcceptsValid(t *testing.T) {
	t.Parallel()
	entry := payment.StreamCatalogEntry{
		Name:       "raw",
		ProtocolID: "/jetvision/raw/1.0.0",
		Direction:  payment.StreamDirectionSeller,
	}
	assert.NoError(t, CheckCatalogEntry(entry))
}

func TestCheckCatalogEntry_RejectsEmptyDirection(t *testing.T) {
	t.Parallel()
	entry := payment.StreamCatalogEntry{
		Name:       "raw",
		ProtocolID: "/jetvision/raw/1.0.0",
		Direction:  "",
	}
	err := CheckCatalogEntry(entry)
	require.Error(t, err)
	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrUnknownStreamDirection, de.Kind())
	assert.Contains(t, err.Error(), "raw")
}

func TestCheckCatalog_AcceptsMixedDirections(t *testing.T) {
	t.Parallel()
	cat := []payment.StreamCatalogEntry{
		{Name: "raw", ProtocolID: "/jetvision/raw/1.0.0", Direction: payment.StreamDirectionSeller},
		{Name: "filtered", ProtocolID: "/jetvision/filtered/*", Direction: payment.StreamDirectionSeller},
		{Name: "status", ProtocolID: "/jetvision/status/1.0.0", Direction: payment.StreamDirectionBuyer},
		{Name: "control", ProtocolID: "/jetvision/control/1.0.0", Direction: payment.StreamDirectionEither},
	}
	assert.NoError(t, CheckCatalog(cat))
}

func TestCheckCatalog_RejectsAnyInvalidEntry(t *testing.T) {
	t.Parallel()
	cat := []payment.StreamCatalogEntry{
		{Name: "raw", ProtocolID: "/jetvision/raw/1.0.0", Direction: payment.StreamDirectionSeller},
		{Name: "weird", ProtocolID: "/x/y/1.0.0", Direction: "garbage"},
	}
	err := CheckCatalog(cat)
	require.Error(t, err)
}

func TestCheckCatalog_EmptyIsValid(t *testing.T) {
	t.Parallel()
	// The legacy single-protocol path applies; no entries to validate.
	assert.NoError(t, CheckCatalog(nil))
	assert.NoError(t, CheckCatalog([]payment.StreamCatalogEntry{}))
}
