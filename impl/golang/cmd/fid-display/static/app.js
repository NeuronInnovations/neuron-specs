// Neuron FID Display — Phase 1 client.
//
// Renders the multistream-buyer's fused TaggedFrame stream onto a
// Leaflet map. State arrives via two paths:
//   • initial /state.json snapshot (so a fresh page is never blank),
//   • live /events SSE stream (snapshot frames at connect, then
//     update frames as the buyer applies new frames).
//
// Phase 1 scope (per ~/.claude/plans/plan-and-implement-a-synchronous-goblet.md):
//   • file split out of inline index.html (no behavioural regressions);
//   • dark theme + locked palette (sky / amber / fuchsia / green-system);
//   • per-source aggregation cards in the left rail;
//   • client-side 1 Hz freshness ticker (fresh ≤ 5 s, warm ≤ 30 s, stale > 30 s);
//   • CARTO Voyager Dark tiles with OSM raster fallback on tileerror;
//   • upgraded marker shapes (drone circle + heading chevron, operator
//     diamond, ADS-B silhouette + heading vector).
//
// Schema and SSE wire format are unchanged — see fid-display-contract.md.

(async function () {
  'use strict';

  // ─── Config ────────────────────────────────────────────────────

  // Freshness thresholds (ms). Synthetic seller emits at ~2 Hz, JV
  // ADS-B at ~1 Hz; 5 s is ≥ 2× nominal interval. 30 s catches a real
  // network blip. Beyond that the data is interactively dead even if
  // the server hasn't evicted yet.
  const FRESH_MS = 5_000;
  const WARM_MS  = 30_000;

  // Tile providers — CARTO primary, OSM raster fallback on tileerror.
  const TILE_PRIMARY = {
    url: 'https://{s}.basemaps.cartocdn.com/rastertiles/voyager_labels_under/{z}/{x}/{y}{r}.png',
    options: {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
      subdomains: 'abcd',
      maxZoom: 19,
    },
  };
  const TILE_FALLBACK = {
    url: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
    options: {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
      maxZoom: 19,
    },
  };

  // ─── Initial map config ───────────────────────────────────────

  let cfg = { lat: 51.4775, lon: -0.4614, zoom: 13 };
  try {
    const r = await fetch('/config.json');
    if (r.ok) cfg = await r.json();
  } catch (e) { /* use defaults */ }

  const map = L.map('map', { zoomControl: true, attributionControl: true })
    .setView([cfg.lat, cfg.lon], cfg.zoom);

  // ─── Tile layer with fallback ──────────────────────────────────

  let tileLayer = L.tileLayer(TILE_PRIMARY.url, TILE_PRIMARY.options);
  let tileFallbackActive = false;
  tileLayer.on('tileerror', () => {
    if (tileFallbackActive) return;
    tileFallbackActive = true;
    console.warn('[fid] CARTO tiles unreachable; falling back to OSM raster');
    map.removeLayer(tileLayer);
    tileLayer = L.tileLayer(TILE_FALLBACK.url, TILE_FALLBACK.options);
    tileLayer.addTo(map);
  });
  tileLayer.addTo(map);

  // ─── Region-jump buttons ──────────────────────────────────────

  // Preserves the pre-redesign jump targets: London = real ADS-B
  // cluster (Heathrow approach), Land's End = synthetic RID orbit.
  document.getElementById('jumpLondon').addEventListener('click', () => {
    map.setView([51.4775, -0.4614], 9);
  });
  document.getElementById('jumpLandsEnd').addEventListener('click', () => {
    map.setView([50.1027, -5.6705], 14);
  });

  // Reset view → /config.json bounds.
  document.getElementById('resetView').addEventListener('click', () => {
    map.setView([cfg.lat, cfg.lon], cfg.zoom);
  });

  // ─── Marker store + per-source aggregates ─────────────────────

  // markers : Map<key, { marker, lastSeen, kind, sellerPeerID, synthetic }>
  const markers = new Map();

  function isSynthetic(frameSource) {
    const fs = (frameSource || '').toLowerCase();
    return fs.indexOf('synth') >= 0;
  }

  // Source A = ADS-B real (normalizedTracks).
  // Source B = RID synthetic (drones + operators).
  function sourceForKind(kind) {
    if (kind === 'normalized-track' || kind === 'aircraft') return 'A';
    if (kind === 'drone' || kind === 'operator') return 'B';
    return 'unknown';
  }

  // ─── Marker rendering ─────────────────────────────────────────

  function shortenPID(pid) {
    if (!pid) return '—';
    if (pid.length <= 14) return pid;
    return pid.slice(0, 8) + '…' + pid.slice(-4);
  }

  function escapeHTML(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
    }[c]));
  }

  function markerHTML(kind, opts) {
    const synClass = opts.synthetic ? ' synthetic' : '';
    const synBadge = opts.synthetic && opts.synLabel
      ? `<div class="syn-badge" aria-label="synthetic">${escapeHTML(opts.synLabel)}</div>`
      : '';
    const ariaLabel = opts.synthetic
      ? `synthetic ${kind} marker`
      : `${kind} marker`;
    if (kind === 'drone') {
      // Heading chevron only when we have a track value.
      const heading = (typeof opts.track === 'number' && !Number.isNaN(opts.track))
        ? `<div class="heading" style="transform: rotate(${opts.track}deg)"></div>`
        : '';
      return `<div class="marker drone${synClass}" role="img" aria-label="${ariaLabel}">
        ${heading}<div class="shape"></div>${synBadge}
      </div>`;
    }
    if (kind === 'operator') {
      return `<div class="marker operator${synClass}" role="img" aria-label="${ariaLabel}">
        <div class="shape"></div>${synBadge}
      </div>`;
    }
    if (kind === 'normalized-track') {
      // Only rotate to a decoded heading. When heading is unknown
      // (hasHeading=false) keep a neutral north-up silhouette and omit the
      // heading vector, rather than implying a true bearing of 0°.
      const hasHdg = opts.hasHeading && typeof opts.headingDeg === 'number' && !Number.isNaN(opts.headingDeg);
      const hdg = hasHdg ? opts.headingDeg : 0;
      const vector = hasHdg
        ? `<div class="heading-vector" style="transform: rotate(${hdg}deg)"></div>`
        : '';
      const silhouette = `<svg class="shape" viewBox="0 0 18 18" aria-hidden="true"
        style="transform: rotate(${hdg}deg)">
        <use href="/icons.svg#aircraft"></use>
      </svg>`;
      return `<div class="marker normalized-track" role="img" aria-label="${ariaLabel}">
        ${vector}${silhouette}
      </div>`;
    }
    // aircraft (legacy bucket)
    return `<div class="marker aircraft" role="img" aria-label="${ariaLabel}">
      <div class="shape"></div>
    </div>`;
  }

  function iconForKind(kind, opts) {
    return L.divIcon({
      className: '',
      iconSize: [18, 18],
      iconAnchor: [9, 9],
      popupAnchor: [0, -10],
      html: markerHTML(kind, opts || {}),
    });
  }

  // ─── Popup builders ───────────────────────────────────────────

  function popupShell(headHTML, bodyRows, footHTML) {
    const body = bodyRows.map(([k, v, cls]) => `
      <div class="row">
        <span class="k">${escapeHTML(k)}</span>
        <span class="v${cls ? ' ' + cls : ''}">${v}</span>
      </div>`).join('');
    return `<div class="popup">
      <div class="pop-head">${headHTML}</div>
      <div class="pop-body">${body}</div>
      <div class="pop-foot">${footHTML}</div>
    </div>`;
  }

  function ageString(lastSeen) {
    const age = Math.max(0, Date.now() - new Date(lastSeen).getTime());
    if (age < 1000) return 'just now';
    if (age < 60_000) return `${(age / 1000).toFixed(1)} s ago`;
    if (age < 3_600_000) return `${Math.floor(age / 60_000)} min ago`;
    return `${Math.floor(age / 3_600_000)} h ago`;
  }

  function timeOf(lastSeen) {
    return new Date(lastSeen).toLocaleTimeString();
  }

  function dronePopup(snap) {
    const synth = isSynthetic(snap.frameSource);
    const speedKmh = (snap.speedHorizontal * 3.6).toFixed(1);
    const head = `
      <span class="id">${escapeHTML(snap.droneId)}</span>
      <span class="pill ${synth ? 'synthetic' : 'real'}">${synth ? 'SYN' : 'REAL'}</span>`;
    const rows = [
      ['position', `${snap.lat.toFixed(5)}, ${snap.lon.toFixed(5)}`],
      ['altitude', `${snap.alt.toFixed(0)} m (${escapeHTML(snap.fix || '—')})`],
      ['speed',    `${speedKmh} km/h · track ${snap.track.toFixed(1)}°`],
      ['feed',     escapeHTML(snap.frameSource || '—')],
      ['regulator', escapeHTML(snap.regulatorVariant || '—')],
    ];
    const foot = `
      <span>seller <span class="seller">${escapeHTML(shortenPID(snap.sellerPeerID))}</span></span>
      <span>last seen ${escapeHTML(timeOf(snap.lastSeen))}</span>`;
    return popupShell(head, rows, foot);
  }

  function operatorPopup(snap) {
    const synth = isSynthetic(snap.frameSource);
    const head = `
      <span class="id">${escapeHTML(snap.operatorId)} <span style="color:var(--text-muted);font-weight:400">(pilot)</span></span>
      <span class="pill ${synth ? 'synthetic' : 'real'}">${synth ? 'SYN' : 'REAL'}</span>`;
    const rows = [
      ['position', `${snap.lat.toFixed(5)}, ${snap.lon.toFixed(5)}`],
      ['id type',  escapeHTML(snap.operatorIdType || '—')],
      ['feed',     escapeHTML(snap.frameSource || '—')],
    ];
    if (snap.droneId) rows.push(['paired drone', escapeHTML(snap.droneId)]);
    const foot = `
      <span>seller <span class="seller">${escapeHTML(shortenPID(snap.sellerPeerID))}</span></span>
      <span>last seen ${escapeHTML(timeOf(snap.lastSeen))}</span>`;
    return popupShell(head, rows, foot);
  }

  function normalizedTrackPopup(snap) {
    // Render "—" for velocity that was never decoded (hasGroundSpeed /
    // hasHeading false). Only show a number — including a genuine 0.0 — when
    // the flag is true. This is the UI half of the "speed 0.0 km/h" fix:
    // unknown velocity must read as unknown, not as a stationary aircraft.
    const speedStr = snap.hasGroundSpeed
      ? `${(snap.groundSpeedMps * 3.6).toFixed(1)} km/h`
      : '—';
    const headingStr = snap.hasHeading
      ? `${snap.headingDeg.toFixed(1)}°`
      : '—';
    const velocityRow = ['speed', `${speedStr} · heading ${headingStr}`];
    const head = `
      <span class="id">${escapeHTML(snap.entityID)}${snap.callsign ? ' · ' + escapeHTML(snap.callsign) : ''}</span>
      <span class="pill ${snap.fakePosition ? 'synthetic' : 'real'}">${snap.fakePosition ? 'SYN' : 'REAL'}</span>`;
    const rows = snap.fakePosition
      ? [
          ['position', 'synthetic (fakePosition=true)', 'placeholder'],
          velocityRow,
          ['squawk',   escapeHTML(snap.squawk || '—')],
          ['frames',   String(snap.frameCount)],
        ]
      : [
          ['position', `${snap.lat.toFixed(5)}, ${snap.lon.toFixed(5)}`],
          ['altitude', `${snap.altitudeM.toFixed(0)} m`],
          velocityRow,
          ['squawk',   escapeHTML(snap.squawk || '—')],
          ['frames',   String(snap.frameCount)],
        ];
    const foot = `
      <span>seller <span class="seller">${escapeHTML(shortenPID(snap.sellerPeerID))}</span></span>
      <span>last seen ${escapeHTML(timeOf(snap.lastSeen))}</span>`;
    return popupShell(head, rows, foot);
  }

  function aircraftPopup(snap) {
    const head = `
      <span class="id">${escapeHTML(snap.icao)}</span>
      <span class="pill synthetic">PLACEHOLDER</span>`;
    const rows = [
      ['source',   `${escapeHTML(snap.source)} · DF ${snap.df}`],
      ['position', 'placeholder (ICAO hash)', 'placeholder'],
      ['frames',   String(snap.frameCount)],
    ];
    const foot = `
      <span>seller <span class="seller">${escapeHTML(shortenPID(snap.sellerPeerID))} · ${escapeHTML(snap.sellerName || '—')}</span></span>
      <span>last seen ${escapeHTML(timeOf(snap.lastSeen))}</span>`;
    return popupShell(head, rows, foot);
  }

  // ─── Marker placement ─────────────────────────────────────────

  function placeMarker(key, lat, lon, kind, popup, lastSeen, iconOpts, sellerPeerID) {
    if (typeof lat !== 'number' || typeof lon !== 'number') return;
    const existing = markers.get(key);
    if (existing) {
      existing.marker.setLatLng([lat, lon]);
      existing.marker.setPopupContent(popup);
      existing.marker.setIcon(iconForKind(kind, iconOpts));
      existing.lastSeen = lastSeen;
      existing.synthetic = !!iconOpts.synthetic;
      existing.sellerPeerID = sellerPeerID || existing.sellerPeerID;
    } else {
      const m = L.marker([lat, lon], { icon: iconForKind(kind, iconOpts) })
        .bindPopup(popup)
        .addTo(map);
      markers.set(key, {
        marker:       m,
        lastSeen:     lastSeen,
        kind:         kind,
        synthetic:    !!iconOpts.synthetic,
        sellerPeerID: sellerPeerID || '',
      });
    }
    refreshAggregates();
  }

  function applyDrone(snap) {
    const synth = isSynthetic(snap.frameSource);
    placeMarker(
      'drone:' + snap.droneId, snap.lat, snap.lon, 'drone',
      dronePopup(snap), snap.lastSeen,
      { synthetic: synth, synLabel: 'SYN DRONE', track: snap.track },
      snap.sellerPeerID,
    );
  }

  function applyOperator(snap) {
    const synth = isSynthetic(snap.frameSource);
    placeMarker(
      'operator:' + snap.operatorId, snap.lat, snap.lon, 'operator',
      operatorPopup(snap), snap.lastSeen,
      { synthetic: synth, synLabel: 'SYN PILOT' },
      snap.sellerPeerID,
    );
  }

  function applyNormalizedTrack(snap) {
    placeMarker(
      'normalized-track:' + snap.entityID, snap.lat, snap.lon, 'normalized-track',
      normalizedTrackPopup(snap), snap.lastSeen,
      { synthetic: !!snap.fakePosition, headingDeg: snap.headingDeg, hasHeading: !!snap.hasHeading },
      snap.sellerPeerID,
    );
  }

  function applyAircraft(snap) {
    placeMarker(
      'aircraft:' + snap.icao, snap.lat, snap.lon, 'aircraft',
      aircraftPopup(snap), snap.lastSeen,
      { synthetic: true, synLabel: 'PLACEHOLDER' },
      snap.sellerPeerID,
    );
  }

  function dispatchSSE(ev) {
    if (!ev || !ev.kind) return;
    switch (ev.kind) {
      case 'drone':
        if (ev.drone)    applyDrone(ev.drone);
        if (ev.operator) applyOperator(ev.operator);
        break;
      case 'aircraft':
        if (ev.aircraft) applyAircraft(ev.aircraft);
        break;
      case 'normalized-track':
        if (ev.normalizedTrack) applyNormalizedTrack(ev.normalizedTrack);
        break;
    }
  }

  // ─── Status rail aggregates ───────────────────────────────────

  const railRefs = {
    sourceA: {
      count:      document.querySelector('#card-source-a .count'),
      freshness:  document.querySelector('#card-source-a .freshness'),
      age:        document.querySelector('#card-source-a .freshness .age'),
      seller:     document.querySelector('#card-source-a .seller'),
    },
    sourceB: {
      droneCount: document.querySelector('#card-source-b .count.drones'),
      pilotCount: document.querySelector('#card-source-b .count.pilots'),
      freshness:  document.querySelector('#card-source-b .freshness'),
      age:        document.querySelector('#card-source-b .freshness .age'),
      seller:     document.querySelector('#card-source-b .seller'),
    },
    legacy: {
      count: document.querySelector('#card-legacy .count'),
    },
    pass: {
      card:    document.getElementById('card-pass'),
      verdict: document.querySelector('#card-pass .verdict'),
      detail:  document.querySelector('#card-pass .detail'),
    },
  };

  function freshnessState(maxLastSeen, count) {
    if (count === 0 || !maxLastSeen) return 'idle';
    const age = Date.now() - maxLastSeen;
    if (age <= FRESH_MS) return 'fresh';
    if (age <= WARM_MS)  return 'warm';
    return 'stale';
  }

  function ageLabel(maxLastSeen, count) {
    if (count === 0 || !maxLastSeen) return 'no data';
    const age = Math.max(0, Date.now() - maxLastSeen);
    if (age < 1000) return 'fresh · just now';
    if (age < 60_000) return `${(age / 1000).toFixed(1)} s since last frame`;
    if (age < 3_600_000) return `${Math.floor(age / 60_000)} min since last frame`;
    return `${Math.floor(age / 3_600_000)} h since last frame`;
  }

  // Walk all markers once, partition them per kind, return aggregates.
  function computeAggregates() {
    const agg = {
      drones:           { count: 0, maxLastSeen: 0, sellers: new Set() },
      operators:        { count: 0, maxLastSeen: 0, sellers: new Set() },
      normalizedTracks: { count: 0, maxLastSeen: 0, sellers: new Set() },
      aircraft:         { count: 0, maxLastSeen: 0, sellers: new Set() },
    };
    for (const m of markers.values()) {
      let bucket;
      switch (m.kind) {
        case 'drone':            bucket = agg.drones; break;
        case 'operator':         bucket = agg.operators; break;
        case 'normalized-track': bucket = agg.normalizedTracks; break;
        case 'aircraft':         bucket = agg.aircraft; break;
        default: continue;
      }
      bucket.count += 1;
      const t = new Date(m.lastSeen).getTime();
      if (t > bucket.maxLastSeen) bucket.maxLastSeen = t;
      if (m.sellerPeerID) bucket.sellers.add(m.sellerPeerID);
    }
    return agg;
  }

  function paintRail() {
    const agg = computeAggregates();

    // Source A — ADS-B real (normalizedTracks).
    railRefs.sourceA.count.textContent = String(agg.normalizedTracks.count);
    {
      const state = freshnessState(agg.normalizedTracks.maxLastSeen, agg.normalizedTracks.count);
      railRefs.sourceA.freshness.dataset.state = state;
      railRefs.sourceA.age.textContent = ageLabel(agg.normalizedTracks.maxLastSeen, agg.normalizedTracks.count);
      const sellers = [...agg.normalizedTracks.sellers];
      railRefs.sourceA.seller.textContent = sellers.length === 0
        ? 'seller: —'
        : sellers.length === 1
          ? 'seller: ' + shortenPID(sellers[0])
          : `sellers: ${sellers.length} distinct`;
    }

    // Source B — RID synthetic (drones + operators).
    railRefs.sourceB.droneCount.textContent = String(agg.drones.count);
    railRefs.sourceB.pilotCount.textContent = String(agg.operators.count);
    {
      const maxLS = Math.max(agg.drones.maxLastSeen, agg.operators.maxLastSeen);
      const count = agg.drones.count + agg.operators.count;
      const state = freshnessState(maxLS, count);
      railRefs.sourceB.freshness.dataset.state = state;
      railRefs.sourceB.age.textContent = ageLabel(maxLS, count);
      const sellers = new Set([...agg.drones.sellers, ...agg.operators.sellers]);
      railRefs.sourceB.seller.textContent = sellers.size === 0
        ? 'seller: —'
        : sellers.size === 1
          ? 'seller: ' + shortenPID([...sellers][0])
          : `sellers: ${sellers.size} distinct`;
    }

    // Legacy aircraft bucket.
    railRefs.legacy.count.textContent = String(agg.aircraft.count);

    // PASS gate — verbatim rule from scripts/tevv/demo-check.sh:92.
    // drones >= 1 AND operators >= 1 AND normalizedTracks >= 1.
    const dronesOK    = agg.drones.count            >= 1;
    const operatorsOK = agg.operators.count         >= 1;
    const tracksOK    = agg.normalizedTracks.count  >= 1;
    const metCount = [dronesOK, operatorsOK, tracksOK].filter(Boolean).length;
    let verdict, detail, state;
    if (metCount === 3)      { verdict = 'PASS';     state = 'fresh'; }
    else if (metCount === 0) { verdict = 'NO DATA';  state = 'stale'; }
    else                     { verdict = 'PARTIAL';  state = 'warm';  }
    detail = `drones ${dronesOK ? '✓' : '·'} ` +
             `operators ${operatorsOK ? '✓' : '·'} ` +
             `normalizedTracks ${tracksOK ? '✓' : '·'}`;
    railRefs.pass.verdict.textContent = verdict;
    railRefs.pass.detail.textContent  = detail;
    railRefs.pass.card.dataset.state  = state;
  }

  let rafPending = false;
  function refreshAggregates() {
    if (rafPending) return;
    rafPending = true;
    requestAnimationFrame(() => {
      rafPending = false;
      paintRail();
    });
  }

  // 1 Hz freshness ticker so "X s since last frame" advances even when
  // no new updates arrive.
  setInterval(paintRail, 1000);

  // ─── Connection badge ─────────────────────────────────────────

  const connBadge = document.getElementById('connBadge');
  function setConn(state, label) {
    connBadge.dataset.state = state;
    connBadge.textContent = label;
  }

  // ─── SSE wiring ───────────────────────────────────────────────

  function connectSSE() {
    setConn('connecting', 'connecting');
    const es = new EventSource('/events');
    es.addEventListener('open', () => setConn('live', 'live'));
    es.addEventListener('error', () => setConn('reconnecting', 'reconnecting'));
    es.addEventListener('snapshot', (e) => {
      try { dispatchSSE(JSON.parse(e.data)); } catch (err) { console.error(err); }
    });
    es.addEventListener('update', (e) => {
      try { dispatchSSE(JSON.parse(e.data)); } catch (err) { console.error(err); }
    });
  }

  // Seed from /state.json so a fresh page is never blank.
  try {
    const r = await fetch('/state.json');
    if (r.ok) {
      const body = await r.json();
      if (Array.isArray(body.drones))            body.drones.forEach(applyDrone);
      if (Array.isArray(body.aircraft))          body.aircraft.forEach(applyAircraft);
      if (Array.isArray(body.normalizedTracks))  body.normalizedTracks.forEach(applyNormalizedTrack);
      if (Array.isArray(body.operators))         body.operators.forEach(applyOperator);
    }
  } catch (e) { /* ignore */ }

  paintRail();
  connectSSE();
})();
