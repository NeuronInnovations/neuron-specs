package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
)

func TestSummaryFromEvidence(t *testing.T) {
	t.Parallel()
	ev := sapient.AgentEvidence{
		AgentID: "7", SellerEVM: "0xabc", PeerID: "16UiuPeer", NodeID: "node-7",
		Service: "rid", Protocol: "/sapient/detection/2.0.0",
		Simulated: true, ChainID: 0, Outcome: "minted", FeedSource: "live",
	}
	got := summaryFromEvidence(ev, "7.json")
	require.Equal(t, "7", got.AgentID)
	require.Equal(t, "0xabc", got.SellerEVM)
	require.Equal(t, "16UiuPeer", got.PeerID)
	require.Equal(t, "node-7", got.NodeID)
	require.Equal(t, "rid", got.Service)
	require.Equal(t, "/sapient/detection/2.0.0", got.Protocol)
	require.True(t, got.Simulated)
	require.Equal(t, uint64(0), got.ChainID)
	require.Equal(t, "minted", got.Outcome)
	require.Equal(t, "live", got.FeedSource)
	require.Equal(t, "7.json", got.SourceFile)
}

func TestProvenanceFromEvidence_SimHidesPlaceholderTxHash(t *testing.T) {
	t.Parallel()
	ev := sapient.AgentEvidence{
		AgentID: "1", Simulated: true, ChainID: 0,
		RegistryAddress: "0x0000000000000000000000000000000000008004",
		TransactionHash: "0xtxhash_register", Outcome: "minted",
	}
	pv := provenanceFromEvidence(ev)
	require.Equal(t, "SIM", pv.Mode)
	require.Equal(t, uint64(0), pv.ChainID)
	require.Empty(t, pv.TransactionHash, "SIM placeholder tx hash must be hidden")
	require.Empty(t, pv.ContractAddress, "no on-chain contract in SIM")
	require.Equal(t, "1", pv.TokenID)
	require.Equal(t, "local-evidence-file", pv.Source)
}

func TestProvenanceFromEvidence_OnChainShowsTxHash(t *testing.T) {
	t.Parallel()
	ev := sapient.AgentEvidence{
		AgentID: "1", Simulated: false, ChainID: 296,
		RegistryAddress: "0x1111222233abc", TransactionHash: "0xdeadbeef", Outcome: "minted",
	}
	pv := provenanceFromEvidence(ev)
	require.Equal(t, "ON-CHAIN", pv.Mode)
	require.Equal(t, uint64(296), pv.ChainID)
	require.Equal(t, "0xdeadbeef", pv.TransactionHash)
	require.Equal(t, "0x1111222233abc", pv.ContractAddress)
	require.Equal(t, "0x1111222233abc", pv.RegistryAddress)
}

func TestProvenanceFromEvidence_OnChainSuppressesPlaceholderHash(t *testing.T) {
	t.Parallel()
	// Defensive: even if an on-chain record carried the placeholder, never show it.
	ev := sapient.AgentEvidence{
		AgentID: "2", Simulated: false, ChainID: 296,
		TransactionHash: "0xtxhash_register",
	}
	pv := provenanceFromEvidence(ev)
	require.Equal(t, "ON-CHAIN", pv.Mode)
	require.Empty(t, pv.TransactionHash, "placeholder hash is never a real receipt")
}

func TestExtractSensorModel_Present(t *testing.T) {
	t.Parallel()
	card := json.RawMessage(`{"services":[
		{"type":"neuron-topic","name":"sapient-stdin","config":{}},
		{"type":"neuron-topic","name":"sapient-stdout","config":{"neuron.rid/1":{"nodeId":"n1","wire":"BSI Flex 335 v2.0 protobuf","schema":"https://x","schemaSha256":"deadbeef","sensorModels":["DroneScout DS240","DroneScout DS-400"]}}}
	]}`)
	sm := extractSensorModel(card)
	require.NotNil(t, sm)
	require.Equal(t, "n1", sm.NodeID)
	require.Equal(t, "BSI Flex 335 v2.0 protobuf", sm.Wire)
	require.Equal(t, "https://x", sm.Schema)
	require.Equal(t, "deadbeef", sm.SchemaSha256)
	require.Equal(t, []string{"DroneScout DS240", "DroneScout DS-400"}, sm.SensorModels)
}

func TestExtractSensorModel_AbsentOrInvalid(t *testing.T) {
	t.Parallel()
	require.Nil(t, extractSensorModel(json.RawMessage(`{"services":[{"type":"neuron-topic","name":"sapient-stdout","config":{}}]}`)),
		"no neuron.rid/1 extension")
	require.Nil(t, extractSensorModel(json.RawMessage(`{"services":[]}`)), "no stdout service")
	require.Nil(t, extractSensorModel(nil), "nil card")
	require.Nil(t, extractSensorModel(json.RawMessage(`not json`)), "invalid json")
}

func TestDetailFromEvidence_CardByteFidelity(t *testing.T) {
	t.Parallel()
	card := json.RawMessage(`{"services":[{"type":"neuron-topic","name":"sapient-stdout","config":{"neuron.rid/1":{"wire":"BSI Flex 335 v2.0 protobuf"}}}]}`)
	ev := sapient.AgentEvidence{
		AgentID: "3", SellerEVM: "0xabc", Simulated: true,
		RegistryAddress: "0x8004", AgentURISha256: "abcd", AgentURI: card,
	}
	d := detailFromEvidence(ev, "3.json")
	require.Equal(t, "3", d.AgentID, "embedded summary")
	require.Equal(t, "3.json", d.SourceFile)
	require.Equal(t, "SIM", d.Provenance.Mode, "provenance composed")
	require.Equal(t, "abcd", d.AgentURISha256)
	require.NotNil(t, d.Sensor)
	require.Equal(t, "BSI Flex 335 v2.0 protobuf", d.Wire, "wire lifted from extension")
	require.Equal(t, []byte(card), []byte(d.Card), "card passed through byte-for-byte")
}

func TestDetailFromEvidence_NilCardIsJSONNull(t *testing.T) {
	t.Parallel()
	ev := sapient.AgentEvidence{AgentID: "4", Simulated: true}
	d := detailFromEvidence(ev, "4.json")
	require.Nil(t, d.Sensor, "no extension when card absent")
	require.Empty(t, d.Wire)
	// Card must be valid JSON (null), never empty bytes that break json.Marshal.
	require.True(t, json.Valid(d.Card), "card field is always valid JSON")
}
