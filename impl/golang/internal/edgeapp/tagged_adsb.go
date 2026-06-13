package edgeapp

import "time"

// TaggedAdsbFrame is the v2-tagged envelope for an ADS-B AggregatedFrame
// per docs/fid-display-contract.md.
//
// Phase-5 ships the tagged envelope as the dual-stream display contract:
// the fused buyer / fid-display sees both ADS-B and Remote ID frames
// wrapped in a uniform {source, type, sellerPeerID, receivedAt, frame}
// envelope. ADS-B sets source="adsb", type="aircraft"; Remote ID sets
// source="remote-id", type="drone".
//
// The legacy AggregatedFrame envelope (cmd/edge-buyer's default output)
// is preserved unchanged for back-compat with existing ATC tooling and
// the production FID display. The tagged envelope is OPT-IN via the
// BuyerConfig.OnTaggedAdsb hook (or, equivalently, the cmd/edge-buyer
// --tagged-output flag).
type TaggedAdsbFrame struct {
	Source       string          `json:"source"`       // always "adsb"
	Type         string          `json:"type"`         // always "aircraft"
	SellerPeerID string          `json:"sellerPeerID"` // copied from inner AggregatedFrame
	ReceivedAt   time.Time       `json:"receivedAt"`   // buyer-side wall-clock at envelope time
	Frame        AggregatedFrame `json:"frame"`        // the legacy ADS-B payload
}

// TagAdsbAggregatedFrame wraps an AggregatedFrame in the v2-tagged
// envelope. The envelope's ReceivedAt is filled from time.Now().UTC()
// at the call site; SellerPeerID is propagated from the inner frame so
// the display can route by source seller without parsing the inner
// frame.
func TagAdsbAggregatedFrame(af AggregatedFrame) TaggedAdsbFrame {
	return TaggedAdsbFrame{
		Source:       "adsb",
		Type:         "aircraft",
		SellerPeerID: af.SellerPeerID,
		ReceivedAt:   time.Now().UTC(),
		Frame:        af,
	}
}
