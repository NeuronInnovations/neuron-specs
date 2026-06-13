package edgeapp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagAdsbAggregatedFrame_PropagatesIdentity(t *testing.T) {
	t.Parallel()

	af := AggregatedFrame{
		SellerEVM:    "0xabcdef0123456789abcdef0123456789abcdef01",
		SellerName:   "jv-london",
		SellerPeerID: "12D3KooWBuyerProbe",
		Frame: feeds.FeedFrame{
			Raw:                  []byte{0x8d, 0xab, 0xcd, 0xef},
			SecondsSinceMidnight: 100,
			Nanoseconds:          500,
		},
		Meta:       feeds.ModeSMeta{DF: 17, ICAO: "abcdef"},
		ReceivedAt: time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
	}

	tf := TagAdsbAggregatedFrame(af)

	assert.Equal(t, "adsb", tf.Source)
	assert.Equal(t, "aircraft", tf.Type)
	assert.Equal(t, "12D3KooWBuyerProbe", tf.SellerPeerID)
	assert.WithinDuration(t, time.Now().UTC(), tf.ReceivedAt, 5*time.Second)
	assert.Equal(t, af, tf.Frame, "inner AggregatedFrame must round-trip verbatim")
}

func TestTaggedAdsbFrame_JSONShape(t *testing.T) {
	t.Parallel()

	af := AggregatedFrame{
		SellerEVM:    "0xabcdef0123456789abcdef0123456789abcdef01",
		SellerName:   "jv-london",
		SellerPeerID: "12D3KooW",
		Frame: feeds.FeedFrame{
			Raw:                  []byte{0x01, 0x02},
			SecondsSinceMidnight: 10,
			Nanoseconds:          0,
		},
		Meta: feeds.ModeSMeta{DF: 17, ICAO: "abc123"},
	}

	tf := TaggedAdsbFrame{
		Source:       "adsb",
		Type:         "aircraft",
		SellerPeerID: "12D3KooW",
		ReceivedAt:   time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
		Frame:        af,
	}

	data, err := json.Marshal(tf)
	require.NoError(t, err)
	body := string(data)

	// The display contract requires source/type/sellerPeerID/receivedAt
	// at the top level and the inner frame inside "frame".
	assert.Contains(t, body, `"source":"adsb"`)
	assert.Contains(t, body, `"type":"aircraft"`)
	assert.Contains(t, body, `"sellerPeerID":"12D3KooW"`)
	assert.Contains(t, body, `"frame":{`)
	assert.Contains(t, body, `"sellerEVM":"0xabcdef0123456789abcdef0123456789abcdef01"`)
	assert.Contains(t, body, `"meta":{"DF":17,"ICAO":"abc123"`)
}
