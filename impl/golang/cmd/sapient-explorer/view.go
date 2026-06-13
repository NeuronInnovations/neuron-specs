package main

import (
	"encoding/json"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
)

// AgentSummary is one row in the registry list (/agents.json). It is a
// whitelisted projection of sapient.AgentEvidence — never the raw record — so no
// field a future evidence schema might add (e.g. a secret) can leak through the
// API. The server emits empty strings; the UI substitutes "—".
type AgentSummary struct {
	AgentID    string `json:"agentId"`
	SellerEVM  string `json:"sellerEVM"`
	PeerID     string `json:"peerID"`
	NodeID     string `json:"nodeId"`
	Service    string `json:"service"`
	Protocol   string `json:"protocol"`
	Simulated  bool   `json:"simulated"`
	ChainID    uint64 `json:"chainId"`
	Outcome    string `json:"outcome"`
	FeedSource string `json:"feedSource,omitempty"`
	SourceFile string `json:"sourceFile,omitempty"`
}

// ProvenanceView is the honest SIM-vs-ON-CHAIN provenance block. It never
// invents chain proof: a SIM record's on-disk transactionHash is the placeholder
// "0xtxhash_register", so any placeholder hash is suppressed.
type ProvenanceView struct {
	Mode            string `json:"mode"` // "SIM" | "ON-CHAIN"
	ChainID         uint64 `json:"chainId"`
	RegistryAddress string `json:"registryAddress,omitempty"`
	ContractAddress string `json:"contractAddress,omitempty"` // ON-CHAIN only (alias of registry)
	TokenID         string `json:"tokenId,omitempty"`         // = agentId
	TransactionHash string `json:"transactionHash,omitempty"` // suppressed when SIM/placeholder
	Outcome         string `json:"outcome,omitempty"`
	Source          string `json:"source"` // "local-evidence-file" (MVP)
}

// SensorModelView is the card's capability extension surfaced for display —
// generic over the extension key (neuron.rid/1, neuron.adsb/1, …) since the
// multi-source registry holds both modalities.
type SensorModelView struct {
	NodeID       string   `json:"nodeId,omitempty"`
	Wire         string   `json:"wire,omitempty"`
	Schema       string   `json:"schema,omitempty"`
	SchemaSha256 string   `json:"schemaSha256,omitempty"`
	SensorModels []string `json:"sensorModels,omitempty"`
	// Multi-source additions (omitempty — absent on legacy renders).
	ExtensionID  string   `json:"extensionId,omitempty"`
	Modality     string   `json:"modality,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// AgentCardDetail is the full detail for one agent (/agents/{id}.json). Card is
// the byte-exact agentURI (load-bearing for the SHA-256), passed through verbatim.
type AgentCardDetail struct {
	AgentSummary
	RegistryAddress string           `json:"registryAddress"`
	AgentURISha256  string           `json:"agentURISha256"`
	Provenance      ProvenanceView   `json:"provenance"`
	Sensor          *SensorModelView `json:"sensor"`
	Wire            string           `json:"wire,omitempty"`
	Card            json.RawMessage  `json:"card"`
}

// simPlaceholderTxHash is the synthetic transactionHash the in-memory registry
// contract writes for a simulated mint. It is not a real receipt and must never
// be surfaced as one.
const simPlaceholderTxHash = "0xtxhash_register"

func summaryFromEvidence(ev sapient.AgentEvidence, sourceFile string) AgentSummary {
	return AgentSummary{
		AgentID:    ev.AgentID,
		SellerEVM:  ev.SellerEVM,
		PeerID:     ev.PeerID,
		NodeID:     ev.NodeID,
		Service:    ev.Service,
		Protocol:   ev.Protocol,
		Simulated:  ev.Simulated,
		ChainID:    ev.ChainID,
		Outcome:    ev.Outcome,
		FeedSource: ev.FeedSource,
		SourceFile: sourceFile,
	}
}

func provenanceFromEvidence(ev sapient.AgentEvidence) ProvenanceView {
	mode := "ON-CHAIN"
	if ev.Simulated {
		mode = "SIM"
	}
	pv := ProvenanceView{
		Mode:            mode,
		ChainID:         ev.ChainID,
		RegistryAddress: ev.RegistryAddress,
		TokenID:         ev.AgentID,
		Outcome:         ev.Outcome,
		Source:          "local-evidence-file",
	}
	// Honest provenance: only show a tx hash / contract for a real on-chain
	// registration, and never the SIM placeholder hash.
	if !ev.Simulated {
		pv.ContractAddress = ev.RegistryAddress
		if ev.TransactionHash != "" && ev.TransactionHash != simPlaceholderTxHash {
			pv.TransactionHash = ev.TransactionHash
		}
	}
	return pv
}

// extractSensorModel lifts the card's capability extension via the shared
// generic parser (sapient.ParseCardMeta) — works for neuron.rid/1,
// neuron.adsb/1, and any future neuron.* key. Returns nil for cards without
// an extension or unparseable input — the caller renders "—" rather than
// fabricating fields.
func extractSensorModel(card json.RawMessage) *SensorModelView {
	if len(card) == 0 {
		return nil
	}
	meta := sapient.ParseCardMeta(card)
	if meta.ExtensionID == "" {
		return nil
	}
	return &SensorModelView{
		NodeID:       meta.NodeID,
		Wire:         meta.Wire,
		Schema:       meta.Schema,
		SchemaSha256: meta.SchemaSha256,
		SensorModels: meta.SensorModels,
		ExtensionID:  meta.ExtensionID,
		Modality:     meta.Modality,
		Capabilities: meta.Capabilities,
	}
}

func detailFromEvidence(ev sapient.AgentEvidence, sourceFile string) AgentCardDetail {
	sensor := extractSensorModel(ev.AgentURI)
	wire := ""
	if sensor != nil {
		wire = sensor.Wire
	}
	// Pass the card through byte-for-byte (load-bearing for agentURISha256).
	// Substitute JSON null when absent so the field is always valid JSON.
	card := ev.AgentURI
	if len(card) == 0 {
		card = json.RawMessage("null")
	}
	return AgentCardDetail{
		AgentSummary:    summaryFromEvidence(ev, sourceFile),
		RegistryAddress: ev.RegistryAddress,
		AgentURISha256:  ev.AgentURISha256,
		Provenance:      provenanceFromEvidence(ev),
		Sensor:          sensor,
		Wire:            wire,
		Card:            card,
	}
}
