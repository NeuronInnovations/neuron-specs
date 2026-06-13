package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileMetadata is sent as the first frame (JSON-encoded) of a file transfer.
// It describes the file being transferred and carries the SHA256 digest for
// integrity verification on the receiver side.
type FileMetadata struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	SHA256      string `json:"sha256"`
}

// FileTransferResult is returned after a successful file transfer (send or receive).
type FileTransferResult struct {
	Filename   string
	Size       int64
	SHA256     string
	FrameCount int
}

// contentTypeFromExt maps common image extensions to MIME types.
// Falls back to "application/octet-stream" for unknown extensions.
func contentTypeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

// SendFile reads a file from disk and sends it over the delivery channel.
//
// Protocol: frame 0 = JSON-encoded FileMetadata, frames 1..N = raw file data
// chunks of at most MaxFrameSize bytes each.
//
// The function computes a SHA256 digest of the file contents so the receiver
// can verify integrity after reassembly.
func SendFile(adapter DeliveryAdapter, channel *DeliveryChannel, filePath string) (*FileTransferResult, error) {
	const op = "SendFile"

	// Read the entire file.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, NewDeliveryError(ErrStreamError, op,
			fmt.Sprintf("failed to read file: %s", err.Error()))
	}

	fileSize := int64(len(data))

	// Compute SHA256 digest.
	hash := sha256.Sum256(data)
	hexDigest := hex.EncodeToString(hash[:])

	// Build metadata.
	ext := filepath.Ext(filePath)
	meta := FileMetadata{
		Filename:    filepath.Base(filePath),
		Size:        fileSize,
		ContentType: contentTypeFromExt(ext),
		SHA256:      hexDigest,
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, NewDeliveryError(ErrStreamError, op,
			fmt.Sprintf("failed to marshal metadata: %s", err.Error()))
	}

	// Frame 0: send metadata.
	if _, err := adapter.Send(channel, metaJSON); err != nil {
		return nil, NewDeliveryError(ErrStreamError, op,
			fmt.Sprintf("failed to send metadata frame: %s", err.Error()))
	}

	// Frames 1..N: send data chunks.
	frameCount := 0
	for offset := 0; offset < len(data); offset += MaxFrameSize {
		end := offset + MaxFrameSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]

		if _, err := adapter.Send(channel, chunk); err != nil {
			return nil, NewDeliveryError(ErrStreamError, op,
				fmt.Sprintf("failed to send data frame %d: %s", frameCount+1, err.Error()))
		}
		frameCount++
	}

	// For a zero-byte file, no data frames are sent (frameCount stays 0).

	return &FileTransferResult{
		Filename:   meta.Filename,
		Size:       fileSize,
		SHA256:     hexDigest,
		FrameCount: frameCount,
	}, nil
}

// ReceiveFile receives a file over the delivery channel and writes it to outputDir.
//
// Protocol: reads a JSON metadata frame first, then data frames until the
// declared file size is reached. After reassembly the SHA256 digest is verified
// against the metadata. On mismatch a DeliveryError with ErrStreamError is returned.
func ReceiveFile(adapter DeliveryAdapter, channel *DeliveryChannel, outputDir string) (*FileTransferResult, error) {
	const op = "ReceiveFile"

	// Frame 0: receive metadata.
	metaFrame, err := adapter.Receive(channel)
	if err != nil {
		return nil, NewDeliveryError(ErrStreamError, op,
			fmt.Sprintf("failed to receive metadata frame: %s", err.Error()))
	}

	var meta FileMetadata
	if err := json.Unmarshal(metaFrame.Data, &meta); err != nil {
		return nil, NewDeliveryError(ErrStreamError, op,
			fmt.Sprintf("failed to parse metadata JSON: %s", err.Error()))
	}

	// Receive data frames until we have enough bytes.
	var received []byte
	frameCount := 0
	for int64(len(received)) < meta.Size {
		frame, err := adapter.Receive(channel)
		if err != nil {
			return nil, NewDeliveryError(ErrStreamError, op,
				fmt.Sprintf("failed to receive data frame %d: %s", frameCount+1, err.Error()))
		}
		received = append(received, frame.Data...)
		frameCount++
	}

	// Truncate to declared size in case the last frame carried trailing bytes.
	if int64(len(received)) > meta.Size {
		received = received[:meta.Size]
	}

	// Verify SHA256 integrity.
	hash := sha256.Sum256(received)
	hexDigest := hex.EncodeToString(hash[:])
	if hexDigest != meta.SHA256 {
		return nil, NewDeliveryError(ErrStreamError, op,
			fmt.Sprintf("SHA256 mismatch: expected %s, got %s", meta.SHA256, hexDigest))
	}

	// Sanitize the filename to prevent path traversal.
	safeName := filepath.Base(meta.Filename)
	outPath := filepath.Join(outputDir, safeName)

	if err := os.WriteFile(outPath, received, 0o644); err != nil {
		return nil, NewDeliveryError(ErrStreamError, op,
			fmt.Sprintf("failed to write file: %s", err.Error()))
	}

	return &FileTransferResult{
		Filename:   safeName,
		Size:       meta.Size,
		SHA256:     hexDigest,
		FrameCount: frameCount,
	}, nil
}
