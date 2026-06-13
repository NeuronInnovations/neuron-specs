package browserprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/libp2p/go-libp2p/core/host"
)

// BootstrapVersion is the version tag embedded in bootstrap-wt.json.
// Distinct from Tier 1's "1" so the browser's 2a-wt validator cannot
// accept Tier 1 bootstraps and vice versa.
const BootstrapVersion = "2a-wt"

// BootstrapWT is the JSON shape consumed by the browser WebTransport
// demo. Fields mirror the 2a-wt validator in
// impl/typescript/src/browser-client-wt/bootstrap-wt-schema.ts.
type BootstrapWT struct {
	Version                 string `json:"version"`
	SellerEVMAddress        string `json:"sellerEVMAddress"`
	SellerPeerID            string `json:"sellerPeerID"`
	SellerWTMultiaddr       string `json:"sellerWTMultiaddr"`
	ControlStreamProtocolID string `json:"controlStreamProtocolID"`
	DataStreamProtocolID    string `json:"dataStreamProtocolID"`
	EchoProtocolID          string `json:"echoProtocolID"`
}

// PickWebTransportMultiaddr returns a /webtransport multiaddr (with
// /p2p/<PEERID> appended) suitable for the browser to dial.
//
// Selection rules:
//  1. Prefer a host-advertised multiaddr whose /ip4/ component equals
//     preferIP. This happens naturally for local loopback runs.
//  2. Otherwise take the first /webtransport multiaddr and substitute
//     the /ip4/ host with preferIP. This is needed on the VPS where
//     the host binds 0.0.0.0 but the browser must dial the public IP.
//
// Returns an error if the host advertises no /webtransport addresses.
func PickWebTransportMultiaddr(h host.Host, preferIP string) (string, error) {
	addrs := h.Addrs()
	if len(addrs) == 0 {
		return "", fmt.Errorf("host has no listen addresses")
	}

	var wtAddrs []string
	for _, a := range addrs {
		s := a.String()
		if strings.Contains(s, "/webtransport") {
			wtAddrs = append(wtAddrs, s)
		}
	}
	if len(wtAddrs) == 0 {
		return "", fmt.Errorf("host advertises no /webtransport multiaddrs; got %v", addrs)
	}

	for _, s := range wtAddrs {
		if strings.HasPrefix(s, "/ip4/"+preferIP+"/") {
			return appendPeerID(s, h.ID().String()), nil
		}
	}

	substituted, err := substituteIP4Host(wtAddrs[0], preferIP)
	if err != nil {
		return "", err
	}
	return appendPeerID(substituted, h.ID().String()), nil
}

func appendPeerID(maStr, pid string) string {
	if strings.Contains(maStr, "/p2p/") {
		return maStr
	}
	return maStr + "/p2p/" + pid
}

// substituteIP4Host replaces the leading /ip4/<host>/ segment with
// /ip4/<newHost>/. All remaining components (port, quic-v1,
// webtransport, certhash, ...) are preserved verbatim.
func substituteIP4Host(maStr, newHost string) (string, error) {
	const prefix = "/ip4/"
	if !strings.HasPrefix(maStr, prefix) {
		return "", fmt.Errorf("multiaddr does not start with %s: %s", prefix, maStr)
	}
	rest := maStr[len(prefix):]
	idx := strings.Index(rest, "/")
	if idx == -1 {
		return "", fmt.Errorf("malformed multiaddr (no component after host): %s", maStr)
	}
	return prefix + newHost + rest[idx:], nil
}

// WriteBootstrap serializes b as pretty JSON and writes it atomically
// to path. The write is create-temp + fsync + rename so readers (scp,
// browser fetch) never observe a half-written file.
func WriteBootstrap(path string, b BootstrapWT) error {
	payload, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bootstrap: %w", err)
	}
	payload = append(payload, '\n')

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	cleanupTmp := func() {
		_ = os.Remove(tmpPath)
	}

	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		cleanupTmp()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanupTmp()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanupTmp()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		cleanupTmp()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanupTmp()
		return fmt.Errorf("rename temp file -> %s: %w", path, err)
	}
	return nil
}
