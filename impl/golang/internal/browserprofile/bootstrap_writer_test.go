package browserprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteIP4Host(t *testing.T) {
	cases := []struct {
		name, in, host, want string
		wantErr              bool
	}{
		{
			name: "loopback to public IP",
			in:   "/ip4/127.0.0.1/udp/4443/quic-v1/webtransport/certhash/uEiA",
			host: "203.0.113.10",
			want: "/ip4/203.0.113.10/udp/4443/quic-v1/webtransport/certhash/uEiA",
		},
		{
			name: "preserves trailing certhashes",
			in:   "/ip4/0.0.0.0/udp/4443/quic-v1/webtransport/certhash/uEiA/certhash/uEiB",
			host: "203.0.113.42",
			want: "/ip4/203.0.113.42/udp/4443/quic-v1/webtransport/certhash/uEiA/certhash/uEiB",
		},
		{
			name:    "rejects non-ip4 multiaddr",
			in:      "/dns4/example.com/udp/4443/quic-v1/webtransport",
			host:    "10.0.0.1",
			wantErr: true,
		},
		{
			name:    "rejects malformed multiaddr with no component after host",
			in:      "/ip4/127.0.0.1",
			host:    "10.0.0.1",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := substituteIP4Host(tc.in, tc.host)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAppendPeerID(t *testing.T) {
	const pid = "12D3KooWEShe5uWFxUyoL89fyVajW8aUQBGYRFT9JL19GpUy1H3M"
	t.Run("appends when absent", func(t *testing.T) {
		got := appendPeerID("/ip4/1.2.3.4/udp/4443/quic-v1/webtransport", pid)
		assert.Equal(t, "/ip4/1.2.3.4/udp/4443/quic-v1/webtransport/p2p/"+pid, got)
	})
	t.Run("no-op when present", func(t *testing.T) {
		in := "/ip4/1.2.3.4/udp/4443/quic-v1/webtransport/p2p/" + pid
		assert.Equal(t, in, appendPeerID(in, pid))
	})
}

func TestWriteBootstrap_AtomicAndPrettyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bootstrap-wt.json")

	b := BootstrapWT{
		Version:                 BootstrapVersion,
		SellerEVMAddress:        "0x5533527cF40444AC0c7e26490C6e02Fbddb97B21",
		SellerPeerID:            "12D3KooWEShe5uWFxUyoL89fyVajW8aUQBGYRFT9JL19GpUy1H3M",
		SellerWTMultiaddr:       "/ip4/127.0.0.1/udp/4443/quic-v1/webtransport/certhash/uEiA/p2p/12D3KooWEShe5uWFxUyoL89fyVajW8aUQBGYRFT9JL19GpUy1H3M",
		ControlStreamProtocolID: "/neuron/browser-profile/control/1.0.0",
		DataStreamProtocolID:    "/neuron/browser-profile/data/1.0.0",
		EchoProtocolID:          EchoProtocolID,
	}

	require.NoError(t, WriteBootstrap(path, b))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	// Pretty JSON: contains newlines between fields, trailing newline.
	assert.True(t, strings.HasSuffix(string(raw), "\n"))
	assert.Contains(t, string(raw), "\n  \"sellerPeerID\"")

	var parsed BootstrapWT
	require.NoError(t, json.Unmarshal(raw, &parsed))
	assert.Equal(t, b, parsed)

	// No temp files left behind.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp-")
	}
}

func TestWriteBootstrap_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bootstrap-wt.json")

	require.NoError(t, os.WriteFile(path, []byte("stale"), 0o644))

	fresh := BootstrapWT{
		Version:                 BootstrapVersion,
		SellerEVMAddress:        "0x5533527cF40444AC0c7e26490C6e02Fbddb97B21",
		SellerPeerID:            "12D3KooWEShe5uWFxUyoL89fyVajW8aUQBGYRFT9JL19GpUy1H3M",
		SellerWTMultiaddr:       "/ip4/203.0.113.10/udp/4443/quic-v1/webtransport/p2p/12D3KooWEShe5uWFxUyoL89fyVajW8aUQBGYRFT9JL19GpUy1H3M",
		ControlStreamProtocolID: "/neuron/browser-profile/control/1.0.0",
		DataStreamProtocolID:    "/neuron/browser-profile/data/1.0.0",
		EchoProtocolID:          EchoProtocolID,
	}
	require.NoError(t, WriteBootstrap(path, fresh))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "stale")

	var parsed BootstrapWT
	require.NoError(t, json.Unmarshal(raw, &parsed))
	assert.Equal(t, fresh, parsed)
}
