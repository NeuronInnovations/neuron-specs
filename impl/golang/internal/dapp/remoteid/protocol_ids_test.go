package remoteid

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Protocol-ID rename guard (chore/rename-basestation-protocols): the Remote ID
// provider streams are published under the DS240 provider path. These constants
// are the buyer/seller handshake strings, so pin each one and fail if any
// reverts to a pre-rename /remoteid/ concept path.
func TestRemoteIDProtocolIDs_AreDS240Paths(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/ds240/raw/1.0.0", ProtocolRaw)
	assert.Equal(t, "/ds240/filtered/*", ProtocolFilteredPattern)
	assert.Equal(t, "/ds240/status/1.0.0", ProtocolStatus)
	assert.Equal(t, "/ds240/basestation/1.0.0", ProtocolBasestation)

	for _, p := range []string{ProtocolRaw, ProtocolFilteredPattern, ProtocolStatus, ProtocolBasestation} {
		assert.Falsef(t, strings.HasPrefix(p, "/remoteid/"), "%q must not revert to the pre-rename Remote ID concept path", p)
	}
}
