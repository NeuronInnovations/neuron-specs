package remoteid

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
)

// Default values for BasestationConfig — exported so tests can reference
// the same constants without duplication.
const (
	// DefaultBasestationSourceLabel is the source-label stamped onto
	// emitted DecodedFrame.Source when the operator does not override
	// it. The default is intentionally explicit ("synthetic") so the
	// FID display can render a "SYN" badge by substring match, and so
	// downstream evidence audits never confuse this feed with the real
	// DroneScout DS400 hardware.
	DefaultBasestationSourceLabel = "basestation-tcp-synthetic"

	// DefaultBasestationPairingTTL bounds how long the operator-side
	// cache holds the last-seen FE record before treating subsequent
	// FF MSG,3 emissions as "operator absent". 30 s matches the
	// bridge's effective Operator-ID broadcast cadence with a safety
	// margin.
	DefaultBasestationPairingTTL = 30 * time.Second
)

// Unit-conversion constants. Imperial → SI metric for the SBS dialect's
// AltFeet / SpdKnots / VrtFpm fields.
const (
	feetToMeters    = 0.3048
	knotsToMps      = 0.514444 // exactly 1852/3600 m/s, rounded to 6dp
	fpmToMps        = feetToMeters / 60.0
)

// BasestationConfig parameterises RunBasestation.
type BasestationConfig struct {
	// HostPort is the TCP "host:port" of the bridge's BaseStation
	// listener — e.g. "127.0.0.1:30003".
	HostPort string

	// SourceLabel is the value stamped onto each emitted
	// DecodedFrame.Source. Empty → DefaultBasestationSourceLabel.
	SourceLabel string

	// PairingTTL bounds the operator-cache lifetime. Zero →
	// DefaultBasestationPairingTTL.
	PairingTTL time.Duration

	// nowFunc is an internal test seam — production code never sets
	// it. nil → time.Now. The cache uses nowFunc() for TTL math so
	// tests can deterministically expire the operator entry.
	nowFunc func() time.Time
}

// RunBasestation is the FeedSource that consumes the BaseStation
// dialect emitted by the BlueMark neuron-rid-bridge and produces
// canonical DecodedFrame records.
//
// Topology: the bridge splits each drone broadcast into multiple SBS
// records keyed by distinct ICAO prefixes — FF* for the drone, FE* for
// its operator. Because the wire loses the original MAC binding, we
// maintain a tiny per-process pairing cache:
//
//   - Drone MSG,1     → updates the drone-side cache (Callsign).
//   - Drone MSG,3     → triggers a DecodedFrame emission for the drone;
//                       enriched with the last-known operator FE
//                       record if its TTL has not lapsed.
//   - Drone MSG,4     → updates the drone-side velocity cache.
//   - Operator MSG,1  → updates the operator-side cache (ID).
//   - Operator MSG,2  → updates the operator-side position cache.
//
// Unit conversions (per FR-R05 canonical SI units):
//   - AltFeet → meters    (×0.3048)
//   - SpdKnots → m/s      (×0.514444)
//   - VrtFpm → m/s        (×0.00508)
//
// Source-label stamping: DecodedFrame.Source = cfg.SourceLabel (or
// DefaultBasestationSourceLabel). The label is the FID badge driver
// — substring-match against "synth" / "synthetic" lights the SYN
// marker.
//
// On ctx-cancel: drains pending pump work, returns ctx.Err(), no
// goroutine leak.
//
// TODO(multi-drone): the pairing cache is degenerate for >1 drone
// because the bridge does not expose the MAC ↔ ICAO binding on the
// wire. Single-drone correctness is sufficient for the reference demo
// vertical slice; multi-drone fan-out requires richer correlation
// hints from the bridge (see plan §"Open Risks").
func RunBasestation(ctx context.Context, cfg BasestationConfig, out chan<- DecodedFrame) error {
	if cfg.HostPort == "" {
		return errors.New("feeds/remoteid: RunBasestation requires a host:port")
	}
	srcLabel := cfg.SourceLabel
	if srcLabel == "" {
		srcLabel = DefaultBasestationSourceLabel
	}
	ttl := cfg.PairingTTL
	if ttl <= 0 {
		ttl = DefaultBasestationPairingTTL
	}
	now := cfg.nowFunc
	if now == nil {
		now = time.Now
	}

	// Internal TCP pump goroutine. The pump owns its own context so we
	// can shut it down deterministically when the outer ctx cancels;
	// the buffered records channel hands records to the cache loop.
	pumpCtx, pumpCancel := context.WithCancel(ctx)
	defer pumpCancel()

	records := make(chan sbs.RIDSBSRecord, 64)
	pumpErr := make(chan error, 1)
	go func() {
		// RunRIDBaseStationTCP does NOT close out; the cache loop
		// exits via ctx-cancel or this goroutine's exit. We close
		// records ourselves after the pump returns so the cache loop
		// can drain and exit.
		err := sbs.RunRIDBaseStationTCP(pumpCtx, cfg.HostPort, records)
		close(records)
		pumpErr <- err
	}()

	cache := &basestationCache{
		ttl: ttl,
		now: now,
	}

	for {
		select {
		case <-ctx.Done():
			pumpCancel()
			// Drain remaining records so the pump goroutine does not
			// block on `case out <- *rec:`. We then wait for the
			// pump to exit (it does so when the upstream connection
			// closes — typically because (a) the operator process is
			// exiting and the kernel closes the socket, or (b) the
			// upstream bridge tears down on its own).
			//
			// In test scenarios, the test's server-side Close()
			// (which closes accepted conns) is what makes the pump's
			// bufio.Scanner.Scan() return EOF and the pump exit;
			// tests therefore call server.Close() before waiting on
			// the RunBasestation goroutine's exit. See
			// basestation_source_test.go for the convention.
			drainAndWait(records, pumpErr)
			return ctx.Err()
		case rec, ok := <-records:
			if !ok {
				// Pump exited (ctx-cancelled inside dial, or a
				// terminal error). Surface its error verbatim.
				err := <-pumpErr
				if err == nil {
					return nil
				}
				return err
			}
			if frame, emit := cache.observe(rec, srcLabel); emit {
				select {
				case <-ctx.Done():
					pumpCancel()
					drainAndWait(records, pumpErr)
					return ctx.Err()
				case out <- frame:
				}
			}
		}
	}
}

// drainAndWait pulls all remaining records from the channel so the
// pump's `case out <- *rec:` can complete, then waits for the pump's
// final error to land on pumpErr. This is the orderly-shutdown
// counterpart to the ctx-done branch in the main loop.
func drainAndWait(records <-chan sbs.RIDSBSRecord, pumpErr <-chan error) {
	for {
		select {
		case _, ok := <-records:
			if !ok {
				<-pumpErr
				return
			}
		case <-pumpErr:
			// Pump returned its error before close(records) was
			// observed; drain any leftover records and return.
			for range records {
			}
			return
		}
	}
}

// basestationCache is the single-drone / single-operator pairing
// cache. It is *not* goroutine-safe — the cache is owned by the
// RunBasestation loop, which is the only writer / reader.
type basestationCache struct {
	ttl time.Duration
	now func() time.Time

	// drone-side state
	droneICAO     string
	droneCallsign string

	droneSpdMps     float64
	droneSpdSet     bool
	droneTrkDeg     float64
	droneTrkSet     bool
	droneVrtMps     float64
	droneVrtSet     bool

	// operator-side state. operatorObservedAt anchors the TTL.
	operatorICAO       string
	operatorID         string
	operatorLat        float64
	operatorLatSet     bool
	operatorLon        float64
	operatorLonSet     bool
	operatorObservedAt time.Time
}

// observe ingests one RIDSBSRecord. Returns (frame, true) for drone
// MSG,3 records (the position-bearing event); otherwise updates the
// cache and returns (DecodedFrame{}, false).
func (c *basestationCache) observe(rec sbs.RIDSBSRecord, sourceLabel string) (DecodedFrame, bool) {
	isOperator := strings.HasPrefix(rec.ICAO, "FE")
	isDrone := strings.HasPrefix(rec.ICAO, "FF")

	if isOperator {
		switch rec.MSGType {
		case 1:
			// Operator identity — record the ID + ICAO + observation
			// time so the TTL math has a reference instant.
			c.operatorICAO = rec.ICAO
			if rec.Callsign != "" {
				c.operatorID = rec.Callsign
			}
			c.operatorObservedAt = c.now().UTC()
		case 2:
			// Operator ground position — record lat/lon + refresh the
			// observation timestamp so a steady stream of MSG,2 keeps
			// the operator-cache alive across the pairing TTL.
			c.operatorICAO = rec.ICAO
			if rec.LatSet {
				c.operatorLat = rec.Lat
				c.operatorLatSet = true
			}
			if rec.LonSet {
				c.operatorLon = rec.Lon
				c.operatorLonSet = true
			}
			c.operatorObservedAt = c.now().UTC()
		}
		return DecodedFrame{}, false
	}

	if !isDrone {
		// Records with neither FF* nor FE* prefixes are out-of-band
		// for the bridge dialect; drop silently.
		return DecodedFrame{}, false
	}

	c.droneICAO = rec.ICAO
	switch rec.MSGType {
	case 1:
		if rec.Callsign != "" {
			c.droneCallsign = rec.Callsign
		}
	case 4:
		// Velocity — record speed/track/vertical-rate, converting to SI.
		if rec.SpdSet {
			c.droneSpdMps = rec.SpdKnots * knotsToMps
			c.droneSpdSet = true
		}
		if rec.TrkSet {
			c.droneTrkDeg = rec.TrkDeg
			c.droneTrkSet = true
		}
		if rec.VrtSet {
			c.droneVrtMps = rec.VrtFpm * fpmToMps
			c.droneVrtSet = true
		}
	case 3:
		// Drone airborne position — emit one DecodedFrame.
		return c.buildFrame(rec, sourceLabel), true
	}
	return DecodedFrame{}, false
}

// buildFrame turns a drone MSG,3 record into a canonical DecodedFrame,
// merging cached identity / velocity / operator state.
func (c *basestationCache) buildFrame(rec sbs.RIDSBSRecord, sourceLabel string) DecodedFrame {
	now := c.now().UTC()
	frame := DecodedFrame{
		Type:        "remote-id-frame",
		Version:     "1.0.0",
		ObservedAt:  now,
		Source:      sourceLabel,
		DroneID:     c.droneCallsign,
		DroneIDType: "serial",
	}
	// Fallback: if we never saw a MSG,1 identity, use the ICAO as a
	// best-effort identifier so downstream consumers always have a
	// stable droneId.
	if frame.DroneID == "" {
		frame.DroneID = rec.ICAO
	}

	if rec.LatSet && rec.LonSet {
		pos := &Position{
			Lat: rec.Lat,
			Lon: rec.Lon,
			Fix: "3D",
		}
		if rec.AltSet {
			pos.Alt = rec.AltFeet * feetToMeters
		}
		frame.Position = pos
	}

	// Velocity — prefer carry-over from a prior MSG,4 update if any of
	// the velocity components were set, or use the MSG,3-embedded
	// track/speed if present.
	hasInlineSpd := rec.SpdSet
	hasInlineTrk := rec.TrkSet
	hasInlineVrt := rec.VrtSet
	if c.droneSpdSet || c.droneTrkSet || c.droneVrtSet || hasInlineSpd || hasInlineTrk || hasInlineVrt {
		vel := &Velocity{}
		switch {
		case hasInlineSpd:
			vel.SpeedHorizontal = rec.SpdKnots * knotsToMps
		case c.droneSpdSet:
			vel.SpeedHorizontal = c.droneSpdMps
		}
		switch {
		case hasInlineTrk:
			vel.Track = rec.TrkDeg
		case c.droneTrkSet:
			vel.Track = c.droneTrkDeg
		}
		switch {
		case hasInlineVrt:
			vel.SpeedVertical = rec.VrtFpm * fpmToMps
		case c.droneVrtSet:
			vel.SpeedVertical = c.droneVrtMps
		}
		frame.Velocity = vel
	}

	// Operator enrichment: only attach when an operator was seen
	// within the TTL window.
	if c.operatorICAO != "" {
		if now.Sub(c.operatorObservedAt) <= c.ttl {
			op := &Operator{
				IDType: "caa",
				ID:     c.operatorID,
			}
			if op.ID == "" {
				op.ID = c.operatorICAO
			}
			if c.operatorLatSet && c.operatorLonSet {
				op.Position = &Position{
					Lat: c.operatorLat,
					Lon: c.operatorLon,
					Fix: "2D",
				}
			}
			frame.Operator = op
		}
	}

	return frame
}
