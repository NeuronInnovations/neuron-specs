package edgeapp

import "github.com/neuron-sdk/neuron-go-sdk/internal/payment"

// Phase-4 ADS-B streams[] advertisement (reference MVP plan §1 Phase 4).
//
// This is a minimal forward-compat shim:
//
//   - The legacy libp2p stream protocol ID for ADS-B remains DefaultProtocol
//     ("/neuron/edge-feed/1.0.0"). All existing sellers dial that.
//   - Phase 4 ALSO advertises the spec-016 canonical protocol ID
//     "/jetvision/raw/1.0.0" inside the streams[] catalog of the buyer's
//     ReverseConnectionSetup (008 FR-P33a).
//   - The buyer registers HandleIncoming for BOTH protocol IDs so the
//     advertisement is truthful — a future seller that prefers the
//     spec-016 path will succeed when it dials "/jetvision/raw/1.0.0", and
//     legacy sellers continue to use "/neuron/edge-feed/1.0.0".
//
// This is NOT the Phase 6A full ADS-B DApp package migration. The BEAST
// pipeline in seller.go and the connection-manager logic in
// aggregated.go remain untouched. Only the buyer's outbound advertisement
// and incoming-stream alias change.

// AdsbProtocolRaw is the spec-016 FR-A02 canonical protocol ID for the
// ADS-B raw stream. Advertised in streams[] alongside DefaultProtocol.
const AdsbProtocolRaw = "/jetvision/raw/1.0.0"

// AdsbStreamCatalog returns the streams[] catalog the ADS-B buyer
// advertises in its ReverseConnectionSetup. Phase 4 advertises a single
// entry (the raw stream) — filtered + status streams ship in Phase 6
// alongside the full DApp migration.
func AdsbStreamCatalog() []payment.StreamCatalogEntry {
	return []payment.StreamCatalogEntry{
		{
			Name:       "raw",
			ProtocolID: AdsbProtocolRaw,
			Direction:  payment.StreamDirectionSeller,
		},
	}
}
