package sapient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// AgentEvidence is the local, file-based evidence record a SAPIENT seller writes
// after registering its Agent Card, and the sapient-agent-explorer reads back.
// It is a flattened, human-readable projection of RegisterResult plus the full
// card (AgentURI) verbatim. FR-S20a: the agent-card envelope IS the agentURI
// registration file; this record carries that file together with the agent ID
// and the identity binding for downstream verification.
type AgentEvidence struct {
	// AgentID is the EIP-8004 tokenId (the seller's agent ID). A small counter
	// when minted on the in-memory contract (Simulated=true).
	AgentID string `json:"agentId"`

	// SellerEVM / NodeID / PeerID are the bound identity triple (FR-S94): the
	// token owner, the identity-derived SAPIENT node_id, and the libp2p PeerID
	// the seller dials from.
	SellerEVM string `json:"sellerEVM"`
	NodeID    string `json:"nodeId"`
	PeerID    string `json:"peerID"`

	// Service / Protocol are the advertised "rid" service and the
	// /sapient/detection/2.0.0 stream protocol.
	Service  string `json:"service"`
	Protocol string `json:"protocol"`

	RegistryAddress string `json:"registryAddress"`
	ChainID         uint64 `json:"chainId"`
	Outcome         string `json:"outcome"`
	AgentURISha256  string `json:"agentURISha256"`
	TransactionHash string `json:"transactionHash"`

	// Simulated is true when the registration was minted on the in-process
	// MemoryRegistryContract (local evidence mode) rather than a real chain.
	Simulated bool `json:"simulated"`

	// FeedSource is the seller's advertised feed provenance at registration
	// time (live|replay|synthetic|placeholder, 017 FR-R-E02). Optional —
	// omitted by records written before this field existed. Display layers
	// surface it; it is runtime provenance, not part of the on-chain card.
	FeedSource string `json:"feedSource,omitempty"`

	// AgentURI is the canonical card JSON verbatim (the EIP-8004 agentURI).
	AgentURI json.RawMessage `json:"agentURI"`
}

// EvidenceFromResult projects a RegisterResult into an AgentEvidence record
// for the default RID service. simulated reports whether the backing contract
// was the in-memory one.
func EvidenceFromResult(res RegisterResult, simulated bool) AgentEvidence {
	return EvidenceFromResultProfile(res, simulated, CommerceServiceName)
}

// EvidenceFromResultProfile is EvidenceFromResult with an explicit advertised
// service name (e.g. the JetVision seller's "jetvision-adsb-sapient").
func EvidenceFromResultProfile(res RegisterResult, simulated bool, serviceName string) AgentEvidence {
	agentID := ""
	if res.TokenID != nil {
		agentID = res.TokenID.String()
	}
	return AgentEvidence{
		AgentID:         agentID,
		SellerEVM:       res.SellerEVM.Hex(),
		NodeID:          res.NodeID,
		PeerID:          res.PeerID,
		Service:         serviceName,
		Protocol:        ProtocolDetection,
		RegistryAddress: res.RegistryAddress.Hex(),
		ChainID:         res.ChainID,
		Outcome:         res.Outcome.String(),
		AgentURISha256:  res.AgentURISha256,
		TransactionHash: res.TransactionHash,
		Simulated:       simulated,
		AgentURI:        json.RawMessage(res.AgentURIJSON),
	}
}

// WriteEvidence writes ev to path as indented JSON (trailing newline).
func WriteEvidence(path string, ev AgentEvidence) error {
	b, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return fmt.Errorf("sapient.WriteEvidence: marshal: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("sapient.WriteEvidence: write %s: %w", path, err)
	}
	return nil
}

// ReadEvidence parses one AgentEvidence JSON file.
func ReadEvidence(path string) (AgentEvidence, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return AgentEvidence{}, fmt.Errorf("sapient.ReadEvidence: %w", err)
	}
	var ev AgentEvidence
	if err := json.Unmarshal(b, &ev); err != nil {
		return AgentEvidence{}, fmt.Errorf("sapient.ReadEvidence: parse %s: %w", path, err)
	}
	return ev, nil
}

// LoadEvidenceDir reads every *.json evidence file in dir, sorted by filename.
// Files that fail to parse are reported as an error (no silent skipping).
func LoadEvidenceDir(dir string) ([]AgentEvidence, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("sapient.LoadEvidenceDir: glob %s: %w", dir, err)
	}
	sort.Strings(matches)
	out := make([]AgentEvidence, 0, len(matches))
	for _, m := range matches {
		ev, rerr := ReadEvidence(m)
		if rerr != nil {
			return nil, rerr
		}
		out = append(out, ev)
	}
	return out, nil
}
