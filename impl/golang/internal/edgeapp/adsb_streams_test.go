package edgeapp

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FR-A02 (016 ADS-B DApp): the seller MUST advertise a streams[] catalog
// containing at minimum the raw stream. Phase 4 wires this on the buyer
// side of the Profile-E reverse-connect flow — the buyer is the dialee
// and advertises the catalog inside its ReverseConnectionSetup.

func TestAdsbStreamCatalog_RawProtocolPresent(t *testing.T) {
	t.Parallel()

	cat := AdsbStreamCatalog()
	require.Len(t, cat, 1, "Phase 4 advertises exactly one stream entry; filtered+status defer to Phase 6")

	entry := cat[0]
	assert.Equal(t, "raw", entry.Name)
	assert.Equal(t, AdsbProtocolRaw, entry.ProtocolID)
	assert.Equal(t, "/jetvision/raw/1.0.0", entry.ProtocolID, "spec-016 FR-A02 canonical ID")
	assert.Equal(t, payment.StreamDirectionSeller, entry.Direction, "ADS-B raw stream is seller-initiated")
}

func TestAdsbProtocolRaw_DiffersFromDefaultProtocol(t *testing.T) {
	t.Parallel()
	// Sanity: the Phase-4 alias MUST be a different string from
	// DefaultProtocol, otherwise the alias HandleIncoming registration
	// would conflict with the legacy registration.
	assert.NotEqual(t, DefaultProtocol, AdsbProtocolRaw)
}
