package remoteid

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCatalog_DefaultIsRawOnly(t *testing.T) {
	t.Parallel()
	entries := BuildRemoteIDStreamCatalog(DefaultCatalogOptions())
	require.Len(t, entries, 1)
	assert.Equal(t, "raw", entries[0].Name)
	assert.Equal(t, ProtocolRaw, entries[0].ProtocolID)
	assert.Equal(t, payment.StreamDirectionSeller, entries[0].Direction)
	assert.Equal(t, SchemaURL, entries[0].Schema)
}

func TestBuildCatalog_IncludeFiltered(t *testing.T) {
	t.Parallel()
	entries := BuildRemoteIDStreamCatalog(CatalogOptions{IncludeFiltered: true})
	require.Len(t, entries, 2)
	assert.Equal(t, "raw", entries[0].Name)
	assert.Equal(t, "filtered", entries[1].Name)
	assert.Equal(t, ProtocolFilteredPattern, entries[1].ProtocolID)
	assert.Equal(t, payment.StreamDirectionSeller, entries[1].Direction)
}

func TestBuildCatalog_IncludeStatus(t *testing.T) {
	t.Parallel()
	entries := BuildRemoteIDStreamCatalog(CatalogOptions{IncludeStatus: true})
	require.Len(t, entries, 2)
	assert.Equal(t, "status", entries[1].Name)
	assert.Equal(t, ProtocolStatus, entries[1].ProtocolID)
	// buyer-initiated per FR-R03
	assert.Equal(t, payment.StreamDirectionBuyer, entries[1].Direction)
}

func TestBuildCatalog_AllEntries(t *testing.T) {
	t.Parallel()
	entries := BuildRemoteIDStreamCatalog(CatalogOptions{IncludeFiltered: true, IncludeStatus: true})
	require.Len(t, entries, 3)
	assert.Equal(t, "raw", entries[0].Name)
	assert.Equal(t, "filtered", entries[1].Name)
	assert.Equal(t, "status", entries[2].Name)
}

// FR-R02: the raw stream MUST always be present regardless of options.
func TestBuildCatalog_RawAlwaysIncluded(t *testing.T) {
	t.Parallel()
	for _, opts := range []CatalogOptions{
		{IncludeFiltered: false, IncludeStatus: false},
		{IncludeFiltered: true, IncludeStatus: false},
		{IncludeFiltered: false, IncludeStatus: true},
		{IncludeFiltered: true, IncludeStatus: true},
		{IncludeBasestation: true},
		{IncludeFiltered: true, IncludeStatus: true, IncludeBasestation: true},
	} {
		entries := BuildRemoteIDStreamCatalog(opts)
		require.NotEmpty(t, entries)
		assert.Equal(t, ProtocolRaw, entries[0].ProtocolID, "raw MUST be first and present per FR-R02")
	}
}

// TestBuildCatalog_DefaultStillRawOnly asserts the additive
// IncludeBasestation option does NOT change the default behaviour —
// existing callers continue to see one entry.
func TestBuildCatalog_DefaultStillRawOnly(t *testing.T) {
	t.Parallel()
	entries := BuildRemoteIDStreamCatalog(DefaultCatalogOptions())
	require.Len(t, entries, 1, "default catalog must remain raw-only after IncludeBasestation was introduced")
	assert.Equal(t, ProtocolRaw, entries[0].ProtocolID)
}

// TestBuildCatalog_IncludeBasestation covers the new flag in isolation.
func TestBuildCatalog_IncludeBasestation(t *testing.T) {
	t.Parallel()
	entries := BuildRemoteIDStreamCatalog(CatalogOptions{IncludeBasestation: true})
	require.Len(t, entries, 2, "raw + basestation = 2 entries")
	assert.Equal(t, "raw", entries[0].Name)
	assert.Equal(t, "basestation", entries[1].Name)
	assert.Equal(t, ProtocolBasestation, entries[1].ProtocolID)
	assert.Equal(t, payment.StreamDirectionSeller, entries[1].Direction)
	assert.Equal(t, SchemaURL, entries[1].Schema, "basestation reuses the raw schema verbatim")
}

// TestBuildCatalog_AllFourEntries covers the combined-flags ordering:
// raw → filtered → status → basestation, in the order BuildCatalog
// appends them.
func TestBuildCatalog_AllFourEntries(t *testing.T) {
	t.Parallel()
	entries := BuildRemoteIDStreamCatalog(CatalogOptions{
		IncludeFiltered:    true,
		IncludeStatus:      true,
		IncludeBasestation: true,
	})
	require.Len(t, entries, 4)
	assert.Equal(t, "raw", entries[0].Name)
	assert.Equal(t, "filtered", entries[1].Name)
	assert.Equal(t, "status", entries[2].Name)
	assert.Equal(t, "basestation", entries[3].Name)
}
