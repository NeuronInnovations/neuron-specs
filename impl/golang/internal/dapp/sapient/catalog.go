package sapient

import "github.com/neuron-sdk/neuron-go-sdk/internal/payment"

// Protocol IDs advertised by a SAPIENT ASM (seller) per spec 015 FR-S11.
const (
	// ProtocolDetection — the assembled SAPIENT DetectionReport stream
	// (SapientMessage protobuf, 4-byte BE length-framed). Seller-initiates.
	// Mandatory; FR-S11.
	ProtocolDetection = "/sapient/detection/2.0.0"

	// ProtocolDetectionRaw — the position-relaxed / raw service (015 FR-S35).
	// RESERVED in Phase 1: declared as a constant for forward reference but NOT
	// served — no handler, not advertised in the catalog.
	ProtocolDetectionRaw = "/sapient/detection-raw/2.0.0"
)

// SchemaURL is the URI placeholder for the SAPIENT DetectionReport schema (the
// BSI Flex 335 v2.0 protobuf). The authoritative wire definition is the vendored
// DSTL proto under sapientpb/proto/; this URL is the eventual hosting target.
const SchemaURL = "https://specs.neuron.network/profile/sapient/v2/detection-report.md"

// CatalogOptions configures optional stream entries. Phase 1 advertises only the
// mandatory assembled detection stream; the reserved raw stream
// (ProtocolDetectionRaw) is not advertised until it is served.
type CatalogOptions struct {
	// (reserved for future optional entries, e.g. the raw service)
}

// DefaultCatalogOptions returns the conservative Phase-1 default — detection-only.
func DefaultCatalogOptions() CatalogOptions { return CatalogOptions{} }

// BuildSapientStreamCatalog returns the streams[] entries the SAPIENT seller
// advertises in its ConnectionSetup (008 FR-P33a). Phase 1 always returns exactly
// the mandatory assembled detection stream; the raw stream is reserved (FR-S35)
// and intentionally absent until a handler exists.
func BuildSapientStreamCatalog(opts CatalogOptions) []payment.StreamCatalogEntry {
	_ = opts // reserved
	return []payment.StreamCatalogEntry{
		{
			Name:       "detection",
			ProtocolID: ProtocolDetection,
			Direction:  payment.StreamDirectionSeller,
			Schema:     SchemaURL,
		},
	}
}
