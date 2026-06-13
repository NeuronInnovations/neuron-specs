package browserprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
)

// LoadedAsset is a small in-memory wrapper around a file to be sent on the
// data stream. Mirrors TS LoadedAsset in impl/typescript/src/server-demo/file-send.ts.
type LoadedAsset struct {
	Metadata FileMetadata
	Bytes    []byte
}

// LoadAsset reads path from disk, computes SHA-256, and builds a LoadedAsset.
//
// Tamper hook (parity with TS file-send.ts:23-33): if the NEURON_TAMPER env
// var is set to "hash", one byte of the payload is flipped AFTER the SHA-256
// is recorded. The browser reassembles the altered bytes, computes a
// different hash, and aborts with NEURON-BROWSER-082. This is a defensive
// test knob used for the Stage-3 integrity demo and should NEVER be enabled
// in production.
func LoadAsset(path, contentType string) (*LoadedAsset, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load asset %s: %w", path, err)
	}
	if len(bytes) == 0 {
		return nil, fmt.Errorf("asset %s is empty", path)
	}

	sum := sha256.Sum256(bytes)
	shaHex := hex.EncodeToString(sum[:])

	if os.Getenv("NEURON_TAMPER") == "hash" {
		idx := len(bytes) / 2
		bytes[idx] ^= 0x01
		fmt.Fprintf(os.Stderr,
			"[wt-seller] NEURON_TAMPER=hash: flipped payload byte at index %d; declared sha256 unchanged\n",
			idx)
	}

	return &LoadedAsset{
		Metadata: FileMetadata{
			Filename:    filepath.Base(path),
			SizeBytes:   len(bytes),
			ContentType: contentType,
			Sha256Hex:   shaHex,
		},
		Bytes: bytes,
	}, nil
}

// SendAsset writes one length-prefixed frame for the metadata (frame 0)
// followed by one or more frames carrying raw data chunks of up to
// delivery.MaxFrameSize bytes each.
//
// Wire format exactly matches impl/typescript/src/server-demo/file-send.ts:46-56
// and impl/typescript/src/browser-client/file-receive.ts:57-93:
//
//	frame 0 = UTF-8 JSON(metadata)   // {filename, sizeBytes, contentType, sha256Hex}
//	frame 1..N = bytes[offset..offset+≤4 MiB]
//
// No explicit terminator — the receiver detects completion when the sum of
// chunk lengths equals metadata.sizeBytes.
func SendAsset(w io.Writer, asset *LoadedAsset) error {
	fw := delivery.NewFrameWriter(w)

	metaJSON, err := json.Marshal(asset.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := fw.WriteFrame(metaJSON); err != nil {
		return fmt.Errorf("write frame 0: %w", err)
	}

	for offset := 0; offset < len(asset.Bytes); offset += delivery.MaxFrameSize {
		end := min(offset+delivery.MaxFrameSize, len(asset.Bytes))
		if err := fw.WriteFrame(asset.Bytes[offset:end]); err != nil {
			return fmt.Errorf("write data frame at offset %d: %w", offset, err)
		}
	}
	return nil
}
