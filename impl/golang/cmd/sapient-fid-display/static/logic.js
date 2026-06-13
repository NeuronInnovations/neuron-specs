// Neuron SAPIENT FID — pure display logic (no DOM, no Leaflet).
//
// Everything here is deterministic and unit-tested via `node --test`
// (scripts/validation/ui-logic-test.sh); app.js consumes it as window.FIDLogic.
// Keep this file dependency-free and side-effect-free.

(function (root) {
  'use strict';

  // trackKeyOf mirrors the server's trackKey: identity is source-scoped
  // ("nodeId|uid"); legacy frames without a nodeId keep the bare uid.
  function trackKeyOf(snap) {
    return (snap.nodeId ? snap.nodeId + '|' : '') + snap.uid;
  }

  // markerKind: the modality split. kind==="adsb" is an aircraft; everything
  // else (legacy/rid) renders as a drone. Interpretation lives HERE, never in
  // the buyer.
  function markerKind(snap) {
    return snap && snap.kind === 'adsb' ? 'aircraft' : 'drone';
  }

  function hasOperator(snap) {
    return !!(snap && snap.rid
      && typeof snap.rid.operatorLat === 'number'
      && typeof snap.rid.operatorLon === 'number');
  }

  // adsbSourceClass maps the bridge's single-letter feed source (adsb.source)
  // + provenance to the filter vocabulary. Honest best-effort:
  //   relayed     — adsb.provenance === "relayed" (takes precedence)
  //   adsb        — A (1090ES) / F (piAware)
  //   mlat        — M (multilateration)
  //   flarm-ogn   — L / O / S / D (the 868 MHz FLARM/OGN family)
  //   uat         — detected via rf.frequencyHz ≈ 978 MHz when present
  //   other       — anything else; the raw letter is always surfaced.
  function adsbSourceClass(snap) {
    const adsb = snap && snap.adsb;
    if (!adsb) return { cls: 'other', raw: '' };
    if (adsb.provenance === 'relayed') return { cls: 'relayed', raw: adsb.source || '' };
    const freq = snap.rf && typeof snap.rf.frequencyHz === 'number' ? snap.rf.frequencyHz : null;
    if (freq !== null && Math.abs(freq - 978e6) < 1e6) return { cls: 'uat', raw: adsb.source || '' };
    switch (adsb.source) {
      case 'A': case 'F': return { cls: 'adsb', raw: adsb.source };
      case 'M': return { cls: 'mlat', raw: adsb.source };
      case 'L': case 'O': case 'S': case 'D': return { cls: 'flarm-ogn', raw: adsb.source };
      default: return { cls: 'other', raw: adsb.source || '' };
    }
  }

  // labelForCounts: source/modality-aware global label.
  //   mixed → "live tracks"; aircraft-only → "live aircraft";
  //   drones-only → "live drones"; none → "live tracks".
  function labelForCounts(counts) {
    const aircraft = (counts && counts.aircraft) || 0;
    const drones = (counts && counts.drones) || 0;
    if (aircraft > 0 && drones > 0) return 'live tracks';
    if (aircraft > 0) return 'live aircraft';
    if (drones > 0) return 'live drones';
    return 'live tracks';
  }

  function isStale(lastSeenMs, nowMs, staleAfterMs) {
    if (!lastSeenMs) return true;
    return (nowMs - lastSeenMs) > staleAfterMs;
  }

  // trackAgeMs corrects for client/server clock skew: skewMs is
  // (clientNowAtFetch - serverNowAtFetch) from /state.json's "now".
  function trackAgeMs(lastSeenMs, clientNowMs, skewMs) {
    return clientNowMs - (skewMs || 0) - lastSeenMs;
  }

  // passesFilters: combinable checkbox groups. Each group's set holds the
  // CHECKED values; a value must be in its group's set to pass (AND across
  // groups, OR within a group). The ADS-B SOURCE group constrains only
  // aircraft — drones are unaffected by it.
  //   filters = { modality:Set, source:Set(nodeIds), status:Set, adsbSource:Set }
  //   ctx     = { stale:bool }  (precomputed staleness for the snap)
  function passesFilters(snap, filters, ctx) {
    const kind = markerKind(snap);
    const modalityValue = kind === 'aircraft' ? 'aircraft' : 'drones';
    if (!filters.modality.has(modalityValue)) return false;
    if (!filters.source.has(snap.nodeId || '')) return false;

    const stale = !!(ctx && ctx.stale);
    const liveOk = filters.status.has('live') && !stale;
    const staleOk = filters.status.has('stale') && stale;
    if (!liveOk && !staleOk) return false;
    if (snap.agent) {
      const simOk = filters.status.has('simulated') && snap.agent.simulated;
      const chainOk = filters.status.has('onchain') && !snap.agent.simulated;
      if (!simOk && !chainOk) return false;
    }

    if (kind === 'aircraft') {
      const src = adsbSourceClass(snap);
      if (!filters.adsbSource.has(src.cls)) return false;
    }
    return true;
  }

  // sourceStatusLabel: card pill text + honest sub-label.
  function sourceStatusLabel(source) {
    const main = { live: 'LIVE', stale: 'STALE', offline: 'OFFLINE', unknown: 'UNKNOWN' }[source.status] || 'UNKNOWN';
    let sub = '';
    if (source.awaitingFirstMessage) sub = 'connected · awaiting first message';
    else if (source.status === 'stale' && source.sessionConnected) sub = 'connected · no recent messages';
    else if (source.status === 'offline') sub = 'no buyer session';
    else if (source.status === 'unknown') sub = 'no session feed · no recent tracks';
    return { main: main, sub: sub };
  }

  // recenterStates: the two explicit per-modality recenter controls. A
  // modality with zero LIVE tracks is disabled and shows its honest 0 —
  // the UI never implies drones are live when only aircraft are.
  function recenterStates(counts) {
    const aircraft = (counts && counts.liveAircraft) || 0;
    const drones = (counts && counts.liveDrones) || 0;
    return [
      { kind: 'aircraft', label: 'live aircraft', count: aircraft, enabled: aircraft > 0 },
      { kind: 'drone', label: 'live drones', count: drones, enabled: drones > 0 },
    ];
  }

  // focusForKind: client-side per-modality focus bounds. Includes ONLY
  // live (non-stale, skew-corrected) tracks of the requested markerKind
  // with a usable position — recentering on aircraft never moves the view
  // to drones and vice versa. Returns null when no such track exists.
  function focusForKind(snaps, kind, nowMs, skewMs, staleAfterMs) {
    let count = 0, sumLat = 0, sumLon = 0;
    let minLat = Infinity, minLon = Infinity, maxLat = -Infinity, maxLon = -Infinity;
    for (const snap of snaps || []) {
      if (!snap || markerKind(snap) !== kind) continue;
      const p = snap.position;
      if (!p || typeof p.lat !== 'number' || typeof p.lon !== 'number') continue;
      const lastMs = snap.lastSeen ? Date.parse(snap.lastSeen) : 0;
      if (!lastMs || trackAgeMs(lastMs, nowMs, skewMs) > staleAfterMs) continue;
      count++;
      sumLat += p.lat; sumLon += p.lon;
      if (p.lat < minLat) minLat = p.lat;
      if (p.lat > maxLat) maxLat = p.lat;
      if (p.lon < minLon) minLon = p.lon;
      if (p.lon > maxLon) maxLon = p.lon;
    }
    if (count === 0) return null;
    return { count, lat: sumLat / count, lon: sumLon / count, minLat, minLon, maxLat, maxLon };
  }

  // classLine: the SINGLE class/confidence formatter — the same payload must
  // produce the same text in the display and the explorer. The confidence is
  // per-frame and source-dependent (e.g. an MLAT frame classifies lower than
  // an ADS-B frame with a self-declared category); a missing confidence is an
  // explicit honest state, never silently hidden or defaulted.
  function classLine(classification) {
    if (!classification) return null;
    const type = classification.type || '—';
    const conf = typeof classification.confidence === 'number' && classification.confidence > 0
      ? 'conf ' + (classification.confidence * 100).toFixed(0) + '%'
      : 'conf not provided';
    return type + ' · ' + conf;
  }

  // shortId: bounded middle-ellipsis for long IDs/EVMs/hashes — output never
  // exceeds head + tail + 1 chars, so monospace values cannot blow a layout.
  function shortId(s, head, tail) {
    const h = head == null ? 10 : head;
    const t = tail == null ? 6 : tail;
    if (!s) return '—';
    return s.length <= h + t + 2 ? s : s.slice(0, h) + '…' + s.slice(-t);
  }

  const api = {
    trackKeyOf, markerKind, hasOperator, adsbSourceClass,
    labelForCounts, isStale, trackAgeMs, passesFilters, sourceStatusLabel,
    recenterStates, focusForKind, classLine, shortId,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  root.FIDLogic = api;
})(typeof window !== 'undefined' ? window : globalThis);
