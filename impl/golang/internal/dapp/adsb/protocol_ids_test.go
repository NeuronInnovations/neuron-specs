package adsb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Protocol-ID rename guard (chore/rename-basestation-protocols): the ADS-B
// decoded-track stream is published under the JetVision provider path. Buyer
// and seller both key off ProtocolBaseStation, so this pins the exact handshake
// string and fails if it ever reverts to the pre-rename /adsb/ concept path.
func TestProtocolBaseStation_IsJetVisionPath(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/jetvision/basestation/1.0.0", ProtocolBaseStation)
	assert.NotEqual(t, "/adsb/basestation/1.0.0", ProtocolBaseStation,
		"must not revert to the pre-rename ADS-B concept path")
}
