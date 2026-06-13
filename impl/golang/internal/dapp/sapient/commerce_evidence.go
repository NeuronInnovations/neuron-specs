package sapient

import (
	"encoding/json"
	"fmt"
	"os"
)

// CommerceEvidence is the evidence-grade record of one SAPIENT rid commerce
// session (buyer or seller side). It never contains key material; every field
// is public-chain or wire-observable data. Per-backend honesty flags disclose
// exactly which layers were real vs in-process.
type CommerceEvidence struct {
	RequestID string `json:"requestId"`
	Role      string `json:"role"` // "buyer" | "seller"
	Service   string `json:"service"`
	Protocol  string `json:"protocol"`

	BuyerEVM      string `json:"buyerEVM,omitempty"`
	SellerEVM     string `json:"sellerEVM,omitempty"`
	SellerAgentID string `json:"sellerAgentId,omitempty"`
	SellerPeerID  string `json:"sellerPeerID,omitempty"`
	BuyerPeerID   string `json:"buyerPeerID,omitempty"`

	RegistryAddress string `json:"registryAddress,omitempty"`
	EscrowContract  string `json:"escrowContract,omitempty"`
	TokenContract   string `json:"tokenContract,omitempty"`
	ChainID         uint64 `json:"chainId,omitempty"`

	// Honesty flags: which backend each layer ran on.
	TopicBackend    string `json:"topicBackend"`              // "memory" | "hcs"
	EscrowBackend   string `json:"escrowBackend"`             // "memory" | "evm"
	RegistryBackend string `json:"registryBackend,omitempty"` // "memory" | "evm"

	// Topics maps channel → locator (sellerStdIn / buyerStdIn / …).
	Topics map[string]string `json:"topics,omitempty"`

	EscrowRef         string `json:"escrowRef,omitempty"`
	EscrowAvailable   string `json:"escrowAvailable,omitempty"` // seller's pre-stream gate observation
	MintTx            string `json:"mintTx,omitempty"`
	DepositTx         string `json:"depositTx,omitempty"`
	InvoiceAmount     string `json:"invoiceAmount,omitempty"`
	InvoiceCurrency   string `json:"invoiceCurrency,omitempty"`
	ReleaseRequestRef string `json:"releaseRequestRef,omitempty"`
	InvoiceAckAction  string `json:"invoiceAckAction,omitempty"`
	ApproveTx         string `json:"approveTx,omitempty"`
	ReleasedAmount    string `json:"releasedAmount,omitempty"`
	ReleaseRecipient  string `json:"releaseRecipient,omitempty"`
	EvidenceHash      string `json:"evidenceHash,omitempty"`
	SellerTokenDelta  string `json:"sellerTokenDelta,omitempty"` // seller-side release verification

	FrameCount  uint64 `json:"frameCount"`
	FinalState  string `json:"finalState,omitempty"`
	FinalAction string `json:"finalAction,omitempty"`
}

// WriteCommerceEvidence writes the record as indented JSON (0644 — it is
// public evidence, not a secret).
func WriteCommerceEvidence(path string, ev CommerceEvidence) error {
	b, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return fmt.Errorf("sapient.WriteCommerceEvidence: marshal: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("sapient.WriteCommerceEvidence: write %s: %w", path, err)
	}
	return nil
}

// ReadCommerceEvidence loads a record written by WriteCommerceEvidence.
func ReadCommerceEvidence(path string) (CommerceEvidence, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return CommerceEvidence{}, fmt.Errorf("sapient.ReadCommerceEvidence: read %s: %w", path, err)
	}
	var ev CommerceEvidence
	if err := json.Unmarshal(b, &ev); err != nil {
		return CommerceEvidence{}, fmt.Errorf("sapient.ReadCommerceEvidence: parse %s: %w", path, err)
	}
	return ev, nil
}
