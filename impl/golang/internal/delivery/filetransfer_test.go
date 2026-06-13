package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock adapter: uses a buffered channel of frames for in-process send/receive.
// ---------------------------------------------------------------------------

type mockDeliveryAdapter struct {
	frames chan []byte
}

func newMockDeliveryAdapter(bufSize int) *mockDeliveryAdapter {
	return &mockDeliveryAdapter{
		frames: make(chan []byte, bufSize),
	}
}

func (m *mockDeliveryAdapter) Connect(_ string, _ []string, _ string, _ *ConnectOptions) (*DeliveryChannel, error) {
	return &DeliveryChannel{
		ID:        "mock-channel",
		PeerID:    "mock-peer",
		Protocol:  "/neuron/file/1.0.0",
		Transport: "mock",
	}, nil
}

func (m *mockDeliveryAdapter) Send(_ *DeliveryChannel, data []byte) (*SendResult, error) {
	// Copy to avoid aliasing caller's slice.
	cp := make([]byte, len(data))
	copy(cp, data)
	m.frames <- cp
	return &SendResult{BytesSent: len(data)}, nil
}

func (m *mockDeliveryAdapter) Receive(_ *DeliveryChannel) (*DataFrame, error) {
	data := <-m.frames
	return &DataFrame{
		Data:       data,
		ReceivedAt: time.Now(),
	}, nil
}

func (m *mockDeliveryAdapter) Disconnect(_ *DeliveryChannel) error {
	return nil
}

func (m *mockDeliveryAdapter) GetStatus(_ *DeliveryChannel) ChannelStatus {
	return ChannelStatus{State: StateConnected, Transport: "mock"}
}

// ---------------------------------------------------------------------------
// tamperingAdapter wraps a mockDeliveryAdapter and flips a byte on every
// data frame (but not the metadata frame) to simulate corruption.
// ---------------------------------------------------------------------------

type tamperingAdapter struct {
	inner     *mockDeliveryAdapter
	metaSent  bool // tracks whether metadata has already been sent
	metaRecvd bool // tracks whether metadata has already been received
}

func (t *tamperingAdapter) Connect(peerID string, addrs []string, proto string, opts *ConnectOptions) (*DeliveryChannel, error) {
	return t.inner.Connect(peerID, addrs, proto, opts)
}

func (t *tamperingAdapter) Send(ch *DeliveryChannel, data []byte) (*SendResult, error) {
	if !t.metaSent {
		t.metaSent = true
		return t.inner.Send(ch, data)
	}
	// Tamper: flip the first byte of every data frame.
	cp := make([]byte, len(data))
	copy(cp, data)
	if len(cp) > 0 {
		cp[0] ^= 0xFF
	}
	return t.inner.Send(ch, cp)
}

func (t *tamperingAdapter) Receive(ch *DeliveryChannel) (*DataFrame, error) {
	frame, err := t.inner.Receive(ch)
	if err != nil {
		return nil, err
	}
	if !t.metaRecvd {
		t.metaRecvd = true
	}
	return frame, nil
}

func (t *tamperingAdapter) Disconnect(ch *DeliveryChannel) error {
	return t.inner.Disconnect(ch)
}

func (t *tamperingAdapter) GetStatus(ch *DeliveryChannel) ChannelStatus {
	return t.inner.GetStatus(ch)
}

// ---------------------------------------------------------------------------
// Helper: create a temp file with the given content and extension.
// ---------------------------------------------------------------------------

func createTempFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o644))
	return path
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSendReceiveFile_RoundTrip(t *testing.T) {
	// Arrange: create a fake JPEG file.
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// JPEG files start with FF D8 FF; add some body bytes.
	jpegContent := make([]byte, 1024)
	jpegContent[0] = 0xFF
	jpegContent[1] = 0xD8
	jpegContent[2] = 0xFF
	for i := 3; i < len(jpegContent); i++ {
		jpegContent[i] = byte(i % 256)
	}

	srcPath := createTempFile(t, srcDir, "photo.jpg", jpegContent)

	adapter := newMockDeliveryAdapter(64)
	ch := &DeliveryChannel{ID: "ch-1", PeerID: "peer-1", Protocol: "/neuron/file/1.0.0", Transport: "mock"}

	// Act: send and receive concurrently.
	var sendResult *FileTransferResult
	var sendErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		sendResult, sendErr = SendFile(adapter, ch, srcPath)
	}()

	recvResult, recvErr := ReceiveFile(adapter, ch, dstDir)
	<-done

	// Assert: no errors.
	require.NoError(t, sendErr, "SendFile should succeed")
	require.NoError(t, recvErr, "ReceiveFile should succeed")

	// Verify results match.
	assert.Equal(t, "photo.jpg", sendResult.Filename)
	assert.Equal(t, "photo.jpg", recvResult.Filename)
	assert.Equal(t, int64(1024), sendResult.Size)
	assert.Equal(t, int64(1024), recvResult.Size)
	assert.Equal(t, sendResult.SHA256, recvResult.SHA256)
	assert.Equal(t, 1, sendResult.FrameCount, "1024 bytes fits in one frame")
	assert.Equal(t, 1, recvResult.FrameCount)

	// Verify SHA256.
	expectedHash := sha256.Sum256(jpegContent)
	assert.Equal(t, hex.EncodeToString(expectedHash[:]), recvResult.SHA256)

	// Verify written file content matches original.
	received, err := os.ReadFile(filepath.Join(dstDir, "photo.jpg"))
	require.NoError(t, err)
	assert.Equal(t, jpegContent, received)
}

func TestSendReceiveFile_LargeFile(t *testing.T) {
	// Arrange: create a file larger than MaxFrameSize to exercise multi-frame chunking.
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// 6 MiB file => should split into 2 frames (4 MiB + 2 MiB).
	largeContent := make([]byte, 6*1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 251) // prime modulus for variety
	}

	srcPath := createTempFile(t, srcDir, "large.png", largeContent)

	// Buffer must hold metadata frame + 2 data frames.
	adapter := newMockDeliveryAdapter(64)
	ch := &DeliveryChannel{ID: "ch-2", PeerID: "peer-2", Protocol: "/neuron/file/1.0.0", Transport: "mock"}

	var sendResult *FileTransferResult
	var sendErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		sendResult, sendErr = SendFile(adapter, ch, srcPath)
	}()

	recvResult, recvErr := ReceiveFile(adapter, ch, dstDir)
	<-done

	require.NoError(t, sendErr)
	require.NoError(t, recvErr)

	assert.Equal(t, "large.png", recvResult.Filename)
	assert.Equal(t, int64(6*1024*1024), recvResult.Size)
	assert.Equal(t, 2, sendResult.FrameCount, "6 MiB / 4 MiB = 2 data frames")
	assert.Equal(t, 2, recvResult.FrameCount)
	assert.Equal(t, sendResult.SHA256, recvResult.SHA256)

	// Verify file integrity.
	received, err := os.ReadFile(filepath.Join(dstDir, "large.png"))
	require.NoError(t, err)
	assert.Equal(t, largeContent, received)
}

func TestReceiveFile_SHA256Mismatch(t *testing.T) {
	// Arrange: use a tampering adapter that corrupts data frames.
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	content := []byte("this is some test data for integrity checking")
	srcPath := createTempFile(t, srcDir, "test.jpg", content)

	mock := newMockDeliveryAdapter(64)
	tamper := &tamperingAdapter{inner: mock}
	ch := &DeliveryChannel{ID: "ch-3", PeerID: "peer-3", Protocol: "/neuron/file/1.0.0", Transport: "mock"}

	var sendErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Sender uses the tampering adapter which corrupts data frames on Send.
		_, sendErr = SendFile(tamper, ch, srcPath)
	}()

	// Receiver uses the plain mock adapter (reads whatever was written).
	_, recvErr := ReceiveFile(mock, ch, dstDir)
	<-done

	require.NoError(t, sendErr, "SendFile itself should succeed (tampering is transparent to sender)")
	require.Error(t, recvErr, "ReceiveFile should detect SHA256 mismatch")
	assert.Contains(t, recvErr.Error(), "SHA256 mismatch")

	// Verify it is a DeliveryError.
	var delErr *DeliveryError
	assert.ErrorAs(t, recvErr, &delErr)
	assert.Equal(t, ErrStreamError, delErr.Kind())
}

func TestSendFile_EmptyFile(t *testing.T) {
	// Arrange: create an empty file.
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := createTempFile(t, srcDir, "empty.dat", []byte{})

	adapter := newMockDeliveryAdapter(64)
	ch := &DeliveryChannel{ID: "ch-4", PeerID: "peer-4", Protocol: "/neuron/file/1.0.0", Transport: "mock"}

	var sendResult *FileTransferResult
	var sendErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		sendResult, sendErr = SendFile(adapter, ch, srcPath)
	}()

	recvResult, recvErr := ReceiveFile(adapter, ch, dstDir)
	<-done

	require.NoError(t, sendErr)
	require.NoError(t, recvErr)

	assert.Equal(t, "empty.dat", sendResult.Filename)
	assert.Equal(t, int64(0), sendResult.Size)
	assert.Equal(t, 0, sendResult.FrameCount, "empty file has zero data frames")
	assert.Equal(t, "empty.dat", recvResult.Filename)
	assert.Equal(t, int64(0), recvResult.Size)
	assert.Equal(t, 0, recvResult.FrameCount)

	// SHA256 of empty input.
	emptyHash := sha256.Sum256([]byte{})
	assert.Equal(t, hex.EncodeToString(emptyHash[:]), recvResult.SHA256)

	// Verify the written file is indeed empty.
	received, err := os.ReadFile(filepath.Join(dstDir, "empty.dat"))
	require.NoError(t, err)
	assert.Empty(t, received)

	// Content type should be application/octet-stream for .dat.
	assert.Equal(t, "application/octet-stream", contentTypeFromExt(".dat"))
}
