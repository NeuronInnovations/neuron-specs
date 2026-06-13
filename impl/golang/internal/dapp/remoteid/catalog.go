package remoteid

import "github.com/neuron-sdk/neuron-go-sdk/internal/payment"

// Protocol IDs advertised by a Remote ID seller per 017 FR-R02 / FR-R03.
const (
	// ProtocolRaw — the raw RemoteIdFrame stream. Mandatory; FR-R02.
	ProtocolRaw = "/ds240/raw/1.0.0"

	// ProtocolFilteredPattern — geofence-radius filter wildcard.
	// Wildcard expansion: `/ds240/filtered/<radius_meters>` where
	// <radius_meters> is a positive integer ≤ 50000. FR-R03/FR-R04.
	// Phase 2 advertises the pattern but does not serve filtered streams
	// yet; filter handler lands in Phase 6 with the rest of the DApp's
	// Phase 6 buildout per the reference MVP plan.
	ProtocolFilteredPattern = "/ds240/filtered/*"

	// ProtocolStatus — buyer-initiated status query. FR-R03.
	// Phase 2 advertises the entry but does not serve it; the
	// buyer-initiated direction requires the stream-direction split
	// (Phase 1 P1.6) which is validation-only today.
	ProtocolStatus = "/ds240/status/1.0.0"

	// ProtocolBasestation — companion data stream carrying the SAME
	// RemoteIdFrame canonical-JSON shape as ProtocolRaw, but advertised
	// under a distinct protocol-id so a buyer can opt in to the
	// BaseStation-bridge-fed feed (cmd/remoteid-seller's
	// --basestation-tcp-host source) without dialing the generic raw
	// stream. The two streams emit byte-for-byte identical frames; the
	// distinct protocol-id is purely a discovery / negotiation hint.
	// See the simulated-DS240 seller design notes.
	ProtocolBasestation = "/ds240/basestation/1.0.0"
)

// SchemaURL is the URI placeholder for the RemoteIdFrame schema reference.
// Phase 2 ships the schema as part of `specs/017-remote-id-dapp/contracts/`
// (contract authoring deferred); the URL is the eventual hosting target.
const SchemaURL = "https://specs.neuron.network/dapp/remote-id/v1/raw-frame.md"

// BuildRemoteIDStreamCatalog returns the seller's advertised stream catalog
// per 017 FR-R02 / FR-R03. The catalog ALWAYS includes the mandatory raw
// stream (`/ds240/raw/1.0.0`); the optional filtered + status streams
// are included when their feature flags say so.
//
// Phase 2 of the reference MVP advertises ONLY the raw stream by default; the
// other two entries are advertised as forward-compatibility hints so that
// buyer-side selection logic exercises the multi-entry catalog code path
// even though only `raw` has a working handler today. Operators that want
// to suppress the filtered/status entries (e.g., to avoid confusing demo
// audiences) pass IncludeFiltered=false, IncludeStatus=false.
type CatalogOptions struct {
	// IncludeFiltered advertises the `/ds240/filtered/*` wildcard
	// entry. Phase 2: false. Phase 6: true.
	IncludeFiltered bool

	// IncludeStatus advertises the `/ds240/status/1.0.0`
	// buyer-initiated entry. Phase 2: false. Phase 6: true.
	IncludeStatus bool

	// IncludeBasestation advertises the `/ds240/basestation/1.0.0`
	// companion data-stream entry (same RemoteIdFrame schema as raw).
	// Default false — operators serving the VPS-1 BaseStation-fed
	// seller opt in via the seller CLI's
	// --advertise-basestation-protocol flag. Plan §"Step 4 — Stream
	// catalog".
	IncludeBasestation bool
}

// DefaultCatalogOptions returns the conservative Phase-2 default —
// raw-only — suitable for the reference MVP fixture vertical slice.
func DefaultCatalogOptions() CatalogOptions {
	return CatalogOptions{
		IncludeFiltered: false,
		IncludeStatus:   false,
	}
}

// BuildRemoteIDStreamCatalog returns the streams[] entries the seller
// advertises in its ConnectionSetup (008 FR-P33a). The returned slice
// satisfies 017 FR-R02 (raw is always present) and, depending on opts,
// FR-R03 (filtered + status as forward-compat advertisements).
func BuildRemoteIDStreamCatalog(opts CatalogOptions) []payment.StreamCatalogEntry {
	entries := []payment.StreamCatalogEntry{
		{
			Name:       "raw",
			ProtocolID: ProtocolRaw,
			Direction:  payment.StreamDirectionSeller,
			Schema:     SchemaURL,
		},
	}
	if opts.IncludeFiltered {
		entries = append(entries, payment.StreamCatalogEntry{
			Name:       "filtered",
			ProtocolID: ProtocolFilteredPattern,
			Direction:  payment.StreamDirectionSeller,
		})
	}
	if opts.IncludeStatus {
		entries = append(entries, payment.StreamCatalogEntry{
			Name:       "status",
			ProtocolID: ProtocolStatus,
			Direction:  payment.StreamDirectionBuyer,
		})
	}
	if opts.IncludeBasestation {
		entries = append(entries, payment.StreamCatalogEntry{
			Name:       "basestation",
			ProtocolID: ProtocolBasestation,
			Direction:  payment.StreamDirectionSeller,
			Schema:     SchemaURL, // identical RemoteIdFrame schema as raw
		})
	}
	return entries
}
