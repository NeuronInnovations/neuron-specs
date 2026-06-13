package adsb

import "github.com/neuron-sdk/neuron-go-sdk/internal/payment"

// Protocol IDs advertised by an ADS-B BaseStation seller per the audit
// (BaseStation fast-fusion audit §5 Option B) and the
// NormalizedTrack contract (docs/normalized-track-contract.md §9).
//
// Note that this package's seller advertises the BaseStation decoded-track
// stream only. The existing /jetvision/raw/1.0.0 BEAST path is owned by the
// legacy edge-seller (internal/edgeapp.AdsbStreamCatalog); they run as
// independent EIP-8004 agents per the audit Risk-3 mitigation.
const (
	// ProtocolBaseStation is the canonical-JSON NormalizedTrack stream.
	// Schema reference: docs/normalized-track-contract.md.
	ProtocolBaseStation = "/jetvision/basestation/1.0.0"
)

// SchemaURL points at the NormalizedTrack contract document. Once Spec 019
// lands this URL will move to specs.neuron.network/dapp/normalized-track/.
const SchemaURL = "https://specs.neuron.network/contracts/normalized-track.md"

// CatalogOptions configures the optional stream entries. v1 currently
// advertises only the basestation stream; future optional streams (filtered,
// status) follow the remoteid catalog pattern when they land.
type CatalogOptions struct {
	// (reserved for future use)
}

// DefaultCatalogOptions returns the conservative v1 default — basestation-only.
func DefaultCatalogOptions() CatalogOptions {
	return CatalogOptions{}
}

// BuildAdsbBasestationStreamCatalog returns the streams[] entries the
// BaseStation ADS-B seller advertises in its ConnectionSetup (008 FR-P33a).
// Always includes the mandatory basestation entry.
func BuildAdsbBasestationStreamCatalog(opts CatalogOptions) []payment.StreamCatalogEntry {
	_ = opts // reserved
	return []payment.StreamCatalogEntry{
		{
			Name:       "basestation",
			ProtocolID: ProtocolBaseStation,
			Direction:  payment.StreamDirectionSeller,
			Schema:     SchemaURL,
		},
	}
}
