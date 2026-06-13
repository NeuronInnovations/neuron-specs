package health

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Spec 006 FR-W02 explicitly lists `nextHeartbeatDeadline` as an UnsignedInt64
// field that MUST be encoded as a JSON string. Per FR-A09 the canonical JSON
// of HeartbeatPayload feeds the signing pre-image, so any regression here
// would silently break cross-language signature verification.

func TestHeartbeatPayload_FR_W02_DeadlineIsQuotedString(t *testing.T) {
	// Use a deadline value above 2^53 so any number-form regression would
	// corrupt the value when read by a JavaScript SDK.
	deadline := uint64(1700000000000000000)
	p := BuildHeartbeatPayload(deadline, RoleBuyer)

	data, err := json.Marshal(p)
	require.NoError(t, err)
	jsonStr := string(data)

	assert.Contains(t, jsonStr, `"nextHeartbeatDeadline":"1700000000000000000"`,
		"FR-W02: nextHeartbeatDeadline must be a quoted decimal string")
	assert.NotContains(t, jsonStr, `"nextHeartbeatDeadline":1700000000000000000`,
		"FR-W02 regression: nextHeartbeatDeadline must not be a JSON number")
}

func TestHeartbeatPayload_FR_W02_ShutdownSentinelQuoted(t *testing.T) {
	// The shutdown sentinel (deadline == 0) must also be quoted.
	p := BuildHeartbeatPayload(ShutdownSentinel, RoleBuyer)
	p.NextHeartbeatDeadline = ShutdownSentinel

	data, err := json.Marshal(p)
	require.NoError(t, err)
	jsonStr := string(data)

	assert.Contains(t, jsonStr, `"nextHeartbeatDeadline":"0"`,
		"FR-W02: shutdown sentinel must serialize as quoted string \"0\"")
	assert.NotContains(t, jsonStr, `"nextHeartbeatDeadline":0`,
		"FR-W02 regression: shutdown sentinel must not be unquoted 0")
}

func TestHeartbeatPayload_FR_W02_RoundTripQuotedForm(t *testing.T) {
	original := BuildHeartbeatPayload(1700000000000000000, RoleSeller)

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored HeartbeatPayload
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Equal(t, original.NextHeartbeatDeadline, restored.NextHeartbeatDeadline,
		"round-trip through canonical (quoted) form must preserve deadline value")
}

func TestHeartbeatPayload_FR_W02_AcceptsLegacyNumberForm(t *testing.T) {
	// Liberal in what we accept: a heartbeat previously serialized with
	// nextHeartbeatDeadline as a JSON number (e.g. an older in-memory
	// artifact) must still parse cleanly.
	legacyJSON := []byte(`{"type":"heartbeat","version":"1.0.0",` +
		`"nextHeartbeatDeadline":1700000000000000000,` +
		`"role":"buyer"}`)

	var p HeartbeatPayload
	require.NoError(t, json.Unmarshal(legacyJSON, &p))
	assert.Equal(t, uint64(1700000000000000000), p.NextHeartbeatDeadline)
	assert.Equal(t, RoleBuyer, p.Role)
}
