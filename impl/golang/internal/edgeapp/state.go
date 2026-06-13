package edgeapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// EdgeStateSchemaVersion identifies the on-disk schema. Bump when adding
// breaking changes; the loader rejects any state with a different version
// and falls back to fresh-create semantics.
const EdgeStateSchemaVersion = 1

// EdgeState is the persistent state file written by RunSeller after topic
// creation and read on subsequent restarts to reuse the same HCS topic IDs.
//
// The file is opt-in via SellerConfig.StatePath. When the path is empty
// (the legacy default), every restart calls bus.CreateTopic three times,
// which yields fresh stdIn/stdOut/stdErr locators every run — wire-compatible
// with Phase C.2's behavior.
//
// When the path is set, RunSeller will:
//
//  1. Try to load the file. Missing file ⇒ run as before, then save state.
//  2. Verify the persisted Identity matches the running secp256k1 pubkey.
//     Mismatch ⇒ ignore the file, run as before, overwrite state.
//  3. Verify each persisted topic locator still resolves on the current
//     backend (bus.Resolve). Any failure ⇒ create a replacement, persist.
//  4. On graceful shutdown, persist the (possibly updated) state atomically.
//
// Atomic writes use os.Rename via a sibling tempfile so a crash mid-write
// never leaves a truncated state.json on disk.
type EdgeState struct {
	// SchemaVersion is the on-disk format version. Must equal
	// EdgeStateSchemaVersion or the loader rejects the file.
	SchemaVersion int `json:"schemaVersion"`

	// Identity binds the persisted topic IDs to a specific seller key.
	// On startup the loader verifies the running key matches; mismatch
	// ⇒ state is discarded (treat the seller as a fresh agent).
	Identity EdgeIdentity `json:"identity"`

	// Topics records the three locators the seller is reusing.
	Topics EdgeTopics `json:"topics"`

	// ProfileDescriptorHash is the keccak256 hex of the last spec-013
	// profile descriptor the seller published, or empty if the descriptor
	// has not been published yet. Used by post-D2 logic to skip re-publish
	// when the descriptor is unchanged across restarts.
	ProfileDescriptorHash string `json:"profileDescriptorHash,omitempty"`

	// UpdatedAt records when the file was last written, in RFC 3339 UTC.
	// Informational only — the loader does not gate on staleness.
	UpdatedAt string `json:"updatedAt"`
}

// EdgeIdentity binds the persisted state to a specific secp256k1 keypair.
// All three fields are derivable from the seller's private key; the file
// records them for post-mortem readability and for the loader's mismatch
// guard.
type EdgeIdentity struct {
	// EVMAddress is the EIP-55-checksummed address derived from the seller
	// pubkey. "0x" + 40 hex chars.
	EVMAddress string `json:"evmAddress"`

	// PublicKeyHex is the 65-byte uncompressed secp256k1 pubkey in hex
	// (matches SellerBootstrap.PublicKeyHex). The loader compares this
	// case-insensitively against the running key's hex.
	PublicKeyHex string `json:"publicKeyHex"`

	// PeerID is the libp2p multihash form ("12D3KooW…"). Recorded for
	// debugging; not used by the loader's identity guard.
	PeerID string `json:"peerID,omitempty"`
}

// EdgeTopics records the three persistent HCS (or other backend) topic
// locators. Locators are strings (e.g. "0.0.123456" for HCS) so the file
// is hand-readable and hand-editable. BackendKind is recorded so a state
// file written under one transport isn't accidentally reused under another.
type EdgeTopics struct {
	BackendKind   string `json:"backendKind"`
	StdInLocator  string `json:"stdInLocator"`
	StdOutLocator string `json:"stdOutLocator"`
	StdErrLocator string `json:"stdErrLocator"`
}

// LoadEdgeState reads path and returns the parsed state. Missing file is
// not an error — returned (nil, nil) so callers can branch on
// `state == nil ⇒ fresh-create` without inspecting err.
//
// Schema-version mismatch is treated as "no state" — returns (nil, nil)
// with no error, so a future version bump degrades gracefully on hosts
// running an older binary that wrote a newer file. The newer binary is
// expected to handle older files via explicit migration, not via this
// path.
func LoadEdgeState(path string) (*EdgeState, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("edge state: read %s: %w", path, err)
	}
	var s EdgeState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("edge state: parse %s: %w", path, err)
	}
	if s.SchemaVersion != EdgeStateSchemaVersion {
		// Treat unknown / future schema as "no state". A real migration
		// path would deserialize via a versioned union, not via this
		// silent fallback.
		return nil, nil
	}
	return &s, nil
}

// SaveEdgeState writes s to path atomically: the bytes are written to
// a sibling temp file (path + ".tmp.<pid>"), then os.Rename'd onto path.
// On POSIX, rename within a single directory is atomic, so a reader
// observing path either sees the previous content or the new content,
// never a partial write. The temp file inherits the configured umask;
// callers that want a stricter mode should chmod path after the rename.
func SaveEdgeState(path string, s *EdgeState) error {
	if path == "" {
		return errors.New("edge state: empty path")
	}
	if s == nil {
		return errors.New("edge state: nil state")
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = EdgeStateSchemaVersion
	}
	if s.UpdatedAt == "" {
		s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("edge state: marshal: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".edge-state-*.tmp")
	if err != nil {
		return fmt.Errorf("edge state: create tmp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("edge state: write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("edge state: fsync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("edge state: close tmp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("edge state: rename: %w", err)
	}
	return nil
}

// MatchesIdentity reports whether s was written by an agent holding the
// secp256k1 key whose 65-byte uncompressed pubkey, hex-encoded without
// "0x", equals pubHex. Comparison is case-insensitive.
//
// A nil receiver returns false (no record ⇒ nothing to match against).
func (s *EdgeState) MatchesIdentity(pubHex string) bool {
	if s == nil {
		return false
	}
	a := strings.ToLower(strings.TrimPrefix(s.Identity.PublicKeyHex, "0x"))
	b := strings.ToLower(strings.TrimPrefix(pubHex, "0x"))
	return a != "" && a == b
}

// TopicRefs converts the persisted locators back into TopicRef structs.
// Returns the first error if any locator fails to parse — callers should
// fall back to fresh-create on error.
func (s *EdgeState) TopicRefs() (in, out, errRef topic.TopicRef, err error) {
	if s == nil {
		return topic.TopicRef{}, topic.TopicRef{}, topic.TopicRef{}, errors.New("edge state: nil")
	}
	kind := topic.BackendKind(s.Topics.BackendKind)
	if kind == "" {
		kind = topic.BackendHCS
	}
	if in, err = topic.NewTopicRef(kind, s.Topics.StdInLocator); err != nil {
		return topic.TopicRef{}, topic.TopicRef{}, topic.TopicRef{}, fmt.Errorf("stdIn: %w", err)
	}
	if out, err = topic.NewTopicRef(kind, s.Topics.StdOutLocator); err != nil {
		return topic.TopicRef{}, topic.TopicRef{}, topic.TopicRef{}, fmt.Errorf("stdOut: %w", err)
	}
	if errRef, err = topic.NewTopicRef(kind, s.Topics.StdErrLocator); err != nil {
		return topic.TopicRef{}, topic.TopicRef{}, topic.TopicRef{}, fmt.Errorf("stdErr: %w", err)
	}
	return in, out, errRef, nil
}

// BuildEdgeState constructs an EdgeState from the resolved identity +
// the live topic refs. Convenience for callers that already have these
// values in hand and want a one-liner save.
func BuildEdgeState(evmAddr, pubHex, peerID string, in, out, errRef topic.TopicRef) *EdgeState {
	return &EdgeState{
		SchemaVersion: EdgeStateSchemaVersion,
		Identity: EdgeIdentity{
			EVMAddress:   evmAddr,
			PublicKeyHex: pubHex,
			PeerID:       peerID,
		},
		Topics: EdgeTopics{
			BackendKind:   string(in.Transport()),
			StdInLocator:  in.Locator(),
			StdOutLocator: out.Locator(),
			StdErrLocator: errRef.Locator(),
		},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}
