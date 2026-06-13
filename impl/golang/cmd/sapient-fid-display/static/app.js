// Neuron SAPIENT FID — multi-source client for the sapient-track stream.
//
// Tracks arrive via the initial /state.json snapshot and the /events SSE
// stream; per-source session health arrives in /state.json's additive
// `sources` block (built server-side from the buyer's /sessions + EIP-8004
// evidence). Pure logic lives in logic.js (window.FIDLogic, unit-tested).
//
// Honest labels (docs/tevv/sapient-display-ui-runbook.md):
//   LIVE/OFFLINE/STALE  — from the buyer's /sessions; never implied
//   EIP-8004 SIM        — agent.simulated=true (in-memory registry)
//   heartbeat           — "not published" (file audit-lane; nothing observed)
//   commerce            — "off (advertisement-only)" from the card
//   SAPIENT protobuf    — the canonical wire; this display is a projection.

(async function () {
  'use strict';

  const FRESH_MS = 5_000;
  const WARM_MS = 30_000;
  const FL = window.FIDLogic;

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

  let cfg = { lat: 50.1027, lon: -5.6705, zoom: 13 };
  try {
    const r = await fetch('/config.json');
    if (r.ok) cfg = await r.json();
  } catch (e) { /* defaults */ }

  const map = L.map('map', { zoomControl: true, attributionControl: true })
    .setView([cfg.lat, cfg.lon], cfg.zoom);

  let tileLayer = L.tileLayer(TILE_PRIMARY.url, TILE_PRIMARY.options);
  let tileFallbackActive = false;
  tileLayer.on('tileerror', () => {
    if (tileFallbackActive) return;
    tileFallbackActive = true;
    map.removeLayer(tileLayer);
    tileLayer = L.tileLayer(TILE_FALLBACK.url, TILE_FALLBACK.options);
    tileLayer.addTo(map);
  });
  tileLayer.addTo(map);

  // ─── Auto-focus ───────────────────────────────────────────────
  // /state.json carries a server-computed `focus` block (drone-first; an
  // aircraft cluster never hijacks the viewport while a drone is live).
  // Applied on first load and once when the first positioned tracks arrive —
  // never after the operator pans/zooms; the ⌖ control re-applies on demand.
  let userMoved = false;
  let programmaticMove = false;
  let focusedOnce = false;
  let firstTrackTimer = null;
  map.on('dragstart zoomstart', () => { if (!programmaticMove) userMoved = true; });
  map.on('moveend zoomend', () => { programmaticMove = false; });

  function applyFocus(focus, force) {
    if (!focus || typeof focus.lat !== 'number') return;
    if (userMoved && !force) return;
    programmaticMove = true;
    focusedOnce = true;
    const degenerate = (focus.maxLat - focus.minLat) < 1e-6 && (focus.maxLon - focus.minLon) < 1e-6;
    if (focus.count <= 1 || degenerate) {
      map.setView([focus.lat, focus.lon], 15);
    } else {
      map.fitBounds(
        [[focus.minLat, focus.minLon], [focus.maxLat, focus.maxLon]],
        { padding: [48, 48], maxZoom: 16 },
      );
    }
  }

  async function refocus(force) {
    try {
      const r = await fetch('/state.json');
      if (r.ok) applyFocus((await r.json()).focus, force);
    } catch (e) { /* keep current view */ }
  }

  // Explicit per-modality recenter controls — operator actions, so they
  // override the don't-fight-the-operator guard. One button per modality
  // ("live aircraft" / "live drones"); a modality with zero live tracks is
  // disabled with its honest 0 — the control never implies drones are live
  // when only aircraft are. Bounds come from live tracks of that modality
  // only (logic.js focusForKind) — no cross-modality mixing.
  const recenterBtns = new Map(); // kind -> <a>
  const RecenterControl = L.Control.extend({
    options: { position: 'topleft' },
    onAdd() {
      const div = L.DomUtil.create('div', 'recenter-ctl');
      FL.recenterStates({}).forEach((st) => {
        const a = document.createElement('a');
        a.href = '#';
        a.setAttribute('role', 'button');
        a.dataset.kind = st.kind;
        a.innerHTML = `⌖ ${st.label} <span class="ct">0</span>`;
        div.appendChild(a);
        recenterBtns.set(st.kind, a);
      });
      L.DomEvent.on(div, 'click', (e) => {
        L.DomEvent.stop(e);
        const a = e.target && e.target.closest ? e.target.closest('a') : null;
        if (!a || a.classList.contains('disabled')) return;
        recenterOnKind(a.dataset.kind);
      });
      return div;
    },
  });
  map.addControl(new RecenterControl());

  function recenterOnKind(kind) {
    const f = FL.focusForKind(tracks.values(), kind, Date.now(), clockSkewMs, staleAfterMs);
    if (f) applyFocus(f, true);
  }

  function updateRecenterButtons(counts) {
    FL.recenterStates(counts).forEach((st) => {
      const a = recenterBtns.get(st.kind);
      if (!a) return;
      const badge = a.querySelector('.ct');
      if (badge && badge.textContent !== String(st.count)) badge.textContent = String(st.count);
      a.classList.toggle('disabled', !st.enabled);
      a.setAttribute('aria-disabled', st.enabled ? 'false' : 'true');
      a.title = st.enabled ? `Recenter on ${st.label} (${st.count})` : `no ${st.label} right now`;
      a.setAttribute('aria-label', a.title);
    });
  }

  // ─── Stores ───────────────────────────────────────────────────
  // tracks: Map<trackKey, snap>; markers: Map<key, L.Marker> where key is
  // "track:<trackKey>", "op:<trackKey>", or "sensor:<id>". trackKey is
  // "nodeId|uid" — identity is source-scoped (multi-source rule).
  const tracks = new Map();
  const markers = new Map();
  const sensors = new Map(); // sensorKey -> SensorView
  let sources = [];          // SourceView[] from /state.json
  let sourceByNode = new Map();
  let staleAfterMs = 60_000;
  let clockSkewMs = 0;       // clientNow - serverNow (corrects track ages)
  let sessionsMeta = { configured: false };
  let selectedKey = null;    // marker key of the selected entity

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
    }[c]));
  }
  const short = (s, head = 8, tail = 4) => FL.shortId(s, head, tail);
  function timeOf(ts) { return new Date(ts).toLocaleTimeString(); }
  function fmtAge(ms) {
    if (ms == null || !isFinite(ms)) return '—';
    if (ms < 1000) return 'just now';
    if (ms < 60_000) return (ms / 1000).toFixed(1) + ' s ago';
    return Math.floor(ms / 60_000) + ' min ago';
  }
  function trackAge(snap) {
    return FL.trackAgeMs(new Date(snap.lastSeen).getTime(), Date.now(), clockSkewMs);
  }
  function isTrackStale(snap) {
    // Prefer the server's verdict (state.json tracks carry `stale`); fall
    // back to client-side age for SSE-updated snapshots.
    if (typeof snap.stale === 'boolean' && snap.ageMs != null) {
      // Server verdict was at serialization time; re-evaluate from age.
      return trackAge(snap) > staleAfterMs;
    }
    return trackAge(snap) > staleAfterMs;
  }

  // ─── Filters ──────────────────────────────────────────────────
  const filterGroupsEl = document.getElementById('filterGroups');
  const sourceFiltersEl = document.getElementById('sourceFilters');

  function readFilters() {
    const groups = { modality: new Set(), source: new Set(), status: new Set(), adsbSource: new Set() };
    filterGroupsEl.querySelectorAll('.filter-group').forEach((g) => {
      const name = g.dataset.group;
      g.querySelectorAll('input[type="checkbox"]:checked').forEach((cb) => {
        groups[name].add(cb.dataset.filter);
      });
    });
    return groups;
  }

  function rebuildSourceFilters() {
    const known = new Set();
    sources.forEach((s) => known.add(s.nodeId || ''));
    tracks.forEach((snap) => known.add(snap.nodeId || ''));
    const existing = new Set();
    sourceFiltersEl.querySelectorAll('input').forEach((cb) => existing.add(cb.dataset.filter));
    if (known.size === 0) return;
    const ph = sourceFiltersEl.querySelector('.placeholder');
    if (ph) ph.remove();
    known.forEach((nodeId) => {
      if (existing.has(nodeId)) return;
      const src = sourceByNode.get(nodeId);
      const label = document.createElement('label');
      const cb = document.createElement('input');
      cb.type = 'checkbox';
      cb.checked = true;
      cb.dataset.filter = nodeId;
      cb.addEventListener('change', applyFilters);
      label.appendChild(cb);
      label.appendChild(document.createTextNode(src ? src.name : (nodeId ? 'source ' + short(nodeId, 8, 0) : 'no node id')));
      sourceFiltersEl.appendChild(label);
    });
  }

  function applyFilters() {
    const filters = readFilters();
    tracks.forEach((snap, key) => {
      const stale = isTrackStale(snap);
      const passes = FL.passesFilters(snap, filters, { stale });
      setMarkerVisible('track:' + key, passes);
      const opVisible = passes && filters.modality.has('operators') && FL.hasOperator(snap);
      setMarkerVisible('op:' + key, opVisible);
    });
    sensors.forEach((v, key) => {
      const visible = filters.modality.has('sensors')
        && (v.nodeId ? filters.source.has(v.nodeId) || !sourceByNode.has(v.nodeId) : true);
      setMarkerVisible('sensor:' + key, visible);
    });
  }

  function setMarkerVisible(key, visible) {
    const m = markers.get(key);
    if (!m) return;
    const onMap = map.hasLayer(m);
    if (visible && !onMap) m.addTo(map);
    if (!visible && onMap) map.removeLayer(m);
  }

  filterGroupsEl.querySelectorAll('input[type="checkbox"]').forEach((cb) => {
    cb.addEventListener('change', applyFilters);
  });

  // ─── Markers ──────────────────────────────────────────────────

  // Marker hitboxes are larger than the visible glyph (30px container vs a
  // ~20-26px shape) for comfortable clicking; rotation lives on the INNER
  // element so the hitbox and the selected ring stay axis-aligned under
  // heading rotation.
  function trackIcon(snap) {
    const hasHdg = snap.velocity && typeof snap.velocity.trackDeg === 'number';
    if (FL.markerKind(snap) === 'aircraft') {
      // Aircraft glyph: plane silhouette rotated to the true track.
      const rot = hasHdg ? snap.velocity.trackDeg : 0;
      return L.divIcon({
        className: '', iconSize: [30, 30], iconAnchor: [15, 15],
        html: `<div class="marker sapient-aircraft" role="img" aria-label="ADS-B aircraft"><svg viewBox="0 0 24 24" width="26" height="26" style="transform: rotate(${rot}deg)"><path d="M12 2 L13.5 9 L21 12 L13.5 13.5 L13 19 L15.5 21 L15.5 22 L12 21 L8.5 22 L8.5 21 L11 19 L10.5 13.5 L3 12 L10.5 9 Z"/></svg></div>`,
      });
    }
    const heading = hasHdg
      ? `<div class="heading" style="transform: rotate(${snap.velocity.trackDeg}deg)"></div>`
      : '';
    return L.divIcon({
      className: '', iconSize: [30, 30], iconAnchor: [15, 15],
      html: `<div class="marker sapient-track" role="img" aria-label="RID drone track">${heading}<div class="shape"></div></div>`,
    });
  }

  function operatorIcon() {
    return L.divIcon({
      className: '', iconSize: [26, 26], iconAnchor: [13, 13],
      html: `<div class="marker sapient-operator" role="img" aria-label="operator"><div class="shape"></div></div>`,
    });
  }

  function sensorIcon(v) {
    const est = v.source === 'estimated' ? ' estimated' : '';
    return L.divIcon({
      className: '', iconSize: [22, 22], iconAnchor: [11, 11],
      html: `<div class="marker sapient-sensor${est}" role="img" aria-label="sensor location"><div class="shape"></div></div>`,
    });
  }

  // markerEl returns the marker's root .marker div (recreated on setIcon).
  function markerEl(m) {
    const el = m.getElement ? m.getElement() : null;
    return el ? el.querySelector('.marker') : null;
  }

  function restyleMarker(key) {
    const m = markers.get(key);
    if (!m) return;
    const el = markerEl(m);
    if (!el) return;
    const isSelected = key === selectedKey;
    el.classList.toggle('selected', isSelected);
    // Selected marker rises above overlapping neighbors so its ring shows.
    m.setZIndexOffset(isSelected ? 1000 : 0);
    if (key.startsWith('track:')) {
      const snap = tracks.get(key.slice('track:'.length));
      if (snap) el.classList.toggle('stale', isTrackStale(snap));
    }
  }

  // upsertMarker creates or moves a marker. No popups — clicking selects the
  // entity and opens the detail drawer. Updates pulse briefly.
  function upsertMarker(key, lat, lon, icon, pulse) {
    let m = markers.get(key);
    if (m) {
      m.setLatLng([lat, lon]);
      m.setIcon(icon); // recreates the element — restyle below
    } else {
      m = L.marker([lat, lon], { icon, keyboard: false });
      m.on('click', (e) => {
        L.DomEvent.stop(e.originalEvent || e);
        select(key);
      });
      m.addTo(map);
      markers.set(key, m);
    }
    restyleMarker(key);
    if (pulse) {
      const el = markerEl(m);
      if (el) {
        el.classList.add('pulse');
        el.addEventListener('animationend', () => el.classList.remove('pulse'), { once: true });
      }
    }
  }

  function removeMarker(key) {
    const m = markers.get(key);
    if (!m) return;
    map.removeLayer(m);
    markers.delete(key);
  }

  // ─── Detail drawer (replaces Leaflet popups) ──────────────────
  const drawerEl = document.getElementById('drawer');
  const drawerBody = document.getElementById('drawerBody');

  function pills(snap) {
    const out = [];
    if (snap.kind === 'adsb') {
      out.push('<span class="pill adsb" title="JetVision Air!Squitter ADS-B/MLAT/FLARM source (neuron.adsb/1)">ADS-B</span>');
    }
    if (snap.cot && snap.cot.demoProfile && snap.cot.affiliation === 'friendly') {
      out.push('<span class="pill friendly" title="demo CoT display profile, not an assessment">FRIENDLY · demo</span>');
    }
    if (snap.agent) {
      out.push(snap.agent.simulated
        ? '<span class="pill sim" title="EIP-8004 registration on the local in-memory registry">EIP-8004 SIM</span>'
        : '<span class="pill onchain">EIP-8004 ON-CHAIN</span>');
    }
    out.push('<span class="pill wire" title="canonical wire; this display is a projection">SAPIENT protobuf</span>');
    if (snap.feedSource) out.push(`<span class="pill feed">feed: ${esc(snap.feedSource)}</span>`);
    return out.join('');
  }

  function row(k, v, mono) {
    return `<div class="row"><span class="k">${esc(k)}</span><span class="v${mono ? ' mono' : ''}">${v}</span></div>`;
  }
  function sect(label) { return `<div class="sect">${esc(label)}</div>`; }

  function sourceSection(nodeId) {
    const src = sourceByNode.get(nodeId || '');
    const parts = [sect('SOURCE')];
    if (!src) {
      parts.push(row('source', nodeId ? `unregistered (${esc(short(nodeId, 13, 6))})` : '—', true));
      return parts.join('');
    }
    const st = FL.sourceStatusLabel(src);
    parts.push(row('source', esc(src.name)));
    parts.push(row('service', esc(src.service || '—')));
    parts.push(row('node_id', esc(short(src.nodeId, 13, 6)), true));
    parts.push(row('seller peer', esc(short(src.peerID, 10, 6)), true));
    parts.push(row('session', `${st.main}${st.sub ? ' · ' + esc(st.sub) : ''}`));
    parts.push(row('messages', src.messageCount != null ? String(src.messageCount) : '—'));
    if (src.messageRate != null) parts.push(row('rate', src.messageRate.toFixed(1) + ' msg/s'));
    return parts.join('');
  }

  function agentSection(agent) {
    if (!agent) return sect('EIP-8004') + row('agent', 'no evidence loaded for this source');
    const parts = [sect('EIP-8004')];
    parts.push(row('agent id', esc(agent.agentId || '—')));
    parts.push(row('seller EVM', esc(short(agent.sellerEVM, 10, 6)), true));
    parts.push(row('registry', agent.simulated ? 'in-memory (SIM)' : 'on-chain'));
    if (agent.protocol) parts.push(row('protocol', esc(agent.protocol), true));
    const src = sourceByNode.get(agent.nodeId || '');
    if (src && src.agentURISha256) parts.push(row('card sha256', esc(short(src.agentURISha256, 10, 6)), true));
    if (src) parts.push(row('heartbeat', esc(src.heartbeat ? src.heartbeat.status : '—')));
    return parts.join('');
  }

  function renderTrackDrawer(snap) {
    const parts = [];
    const title = (snap.adsb && (snap.adsb.callsign || snap.adsb.registration)) || snap.uid;
    parts.push(`<div class="pop-head"><span class="id">${esc(title)}</span>${pills(snap)}</div>`);

    parts.push(sect('TRACK'));
    parts.push(row('uid', esc(snap.uid), true));
    // classLine is the shared formatter (logic.js) — the explorer renders the
    // same payload through the same function, so the two UIs can never show
    // different class/confidence text for the same frame.
    const cl = FL.classLine(snap.classification);
    if (cl) parts.push(row('class', esc(cl)));
    if (snap.position) {
      parts.push(row('position', `${snap.position.lat.toFixed(5)}, ${snap.position.lon.toFixed(5)}`));
      parts.push(row('altitude', `${snap.position.alt.toFixed(0)} m`));
    }
    // Honest unknown: omitted velocity renders '—', never a fabricated 0.
    const speed = snap.velocity
      ? `${(snap.velocity.speedMps * 3.6).toFixed(1)} km/h · track ${snap.velocity.trackDeg.toFixed(1)}°`
      : '—';
    parts.push(row('speed', speed));
    parts.push(row('last seen', `${esc(timeOf(snap.lastSeen))} <span class="age" data-age-for="${esc(FL.trackKeyOf(snap))}">(${esc(fmtAge(trackAge(snap)))})</span>`));

    parts.push(sourceSection(snap.nodeId));
    parts.push(agentSection(snap.agent));

    parts.push(sect('PAYLOAD'));
    if (snap.adsb) {
      parts.push(row('icao24', esc(snap.adsb.icao24 || '—'), true));
      if (snap.adsb.callsign) parts.push(row('callsign', esc(snap.adsb.callsign), true));
      if (snap.adsb.registration) parts.push(row('registration', esc(snap.adsb.registration), true));
      if (snap.adsb.typeCode) parts.push(row('type', esc(snap.adsb.typeCode)));
      if (snap.adsb.operator) parts.push(row('operator', esc(snap.adsb.operator)));
      if (snap.adsb.originIcao || snap.adsb.destIcao) {
        parts.push(row('route', `${esc(snap.adsb.originIcao || '?')} → ${esc(snap.adsb.destIcao || '?')}`));
      }
      const srcCls = FL.adsbSourceClass(snap);
      parts.push(row('adsb source', `${esc(snap.adsb.source || '—')} (${esc(srcCls.cls)})${snap.adsb.provenance ? ' · ' + esc(snap.adsb.provenance) : ''}`));
      if (snap.adsb.emitterCategory) parts.push(row('category', esc(snap.adsb.emitterCategory)));
      if (snap.adsb.squawk) parts.push(row('squawk', esc(snap.adsb.squawk), true));
      if (snap.adsb.emergency) parts.push(row('emergency', esc(snap.adsb.emergency)));
      if (typeof snap.adsb.baroAltFt === 'number') parts.push(row('baro alt', `${snap.adsb.baroAltFt.toFixed(0)} ft`));
      if (typeof snap.adsb.signalDbm === 'number') parts.push(row('signal', `${snap.adsb.signalDbm.toFixed(0)} dBm`));
    } else {
      if (snap.rid) {
        if (snap.rid.serial) parts.push(row('serial', esc(snap.rid.serial), true));
        if (snap.rid.uasId) parts.push(row('uas id', esc(snap.rid.uasId), true));
        if (snap.rid.idType) parts.push(row('id type', esc(snap.rid.idType)));
        if (snap.rid.uaType) parts.push(row('ua type', esc(snap.rid.uaType)));
        if (snap.rid.status) parts.push(row('status', esc(snap.rid.status)));
        if (snap.rid.macAddress) parts.push(row('mac', esc(snap.rid.macAddress), true));
        if (snap.rid.operatorId) parts.push(row('operator', esc(snap.rid.operatorId)));
      }
      if (snap.rf) {
        const rssi = typeof snap.rf.rssiDbm === 'number' ? `${snap.rf.rssiDbm.toFixed(0)} dBm` : '—';
        const freq = typeof snap.rf.frequencyHz === 'number' ? `${(snap.rf.frequencyHz / 1e9).toFixed(3)} GHz` : '—';
        parts.push(row('rssi', rssi));
        parts.push(row('frequency', freq));
        if (snap.rf.transport) parts.push(row('transport', esc(snap.rf.transport)));
        if (snap.rf.channel) parts.push(row('channel', esc(snap.rf.channel)));
      }
      if (snap.cot) {
        parts.push(row('cot type', esc(snap.cot.type || '—'), true));
        const aff = esc(snap.cot.affiliation || '—') + (snap.cot.demoProfile ? ' (demo profile)' : '');
        parts.push(row('cot affiliation', aff));
      }
    }

    parts.push(sect('DEBUG'));
    parts.push(row('track key', esc(FL.trackKeyOf(snap)), true));
    parts.push(row('object_id', esc(snap.objectId || '—'), true));
    parts.push(row('frames', String(snap.frameCount)));
    parts.push(row('received at', esc(snap.lastSeen)));
    return parts.join('');
  }

  function renderOperatorDrawer(snap) {
    const parts = [];
    parts.push(`<div class="pop-head"><span class="id">${esc(snap.rid.operatorId || snap.uid + '-OP')} <span class="muted">(operator)</span></span>${pills(snap)}</div>`);
    parts.push(sect('OPERATOR'));
    parts.push(row('position', `${snap.rid.operatorLat.toFixed(5)}, ${snap.rid.operatorLon.toFixed(5)}`));
    if (typeof snap.rid.operatorAltM === 'number') parts.push(row('altitude', `${snap.rid.operatorAltM.toFixed(0)} m`));
    if (snap.rid.operatorIdType) parts.push(row('id type', esc(snap.rid.operatorIdType)));
    parts.push(row('paired track', esc(snap.uid), true));
    parts.push(sourceSection(snap.nodeId));
    parts.push(agentSection(snap.agent));
    parts.push(sect('DEBUG'));
    parts.push(row('track key', esc(FL.trackKeyOf(snap)), true));
    parts.push(row('frames', String(snap.frameCount)));
    return parts.join('');
  }

  function renderSensorDrawer(v) {
    const parts = [];
    parts.push(`<div class="pop-head"><span class="id">${esc(v.label || v.sensorId || 'sensor')}</span><span class="pill src">SENSOR</span></div>`);
    parts.push(sect('SENSOR'));
    parts.push(row('position', `${v.lat.toFixed(5)}, ${v.lon.toFixed(5)}`));
    if (typeof v.altM === 'number') parts.push(row('altitude', `${v.altM.toFixed(0)} m`));
    parts.push(row('provenance', esc(v.source)));
    if (v.confidence) parts.push(row('confidence', esc(v.confidence)));
    if (v.nodeId) parts.push(row('node_id', esc(short(v.nodeId, 13, 6)), true));
    if (v.nodeId) parts.push(sourceSection(v.nodeId));
    parts.push(`<p class="honest-note">sensor positions are operator-configured, never inferred from track data</p>`);
    return parts.join('');
  }

  function renderDrawer() {
    if (!selectedKey) return;
    if (selectedKey.startsWith('track:')) {
      const snap = tracks.get(selectedKey.slice('track:'.length));
      if (snap) drawerBody.innerHTML = renderTrackDrawer(snap);
    } else if (selectedKey.startsWith('op:')) {
      const snap = tracks.get(selectedKey.slice('op:'.length));
      if (snap && snap.rid) drawerBody.innerHTML = renderOperatorDrawer(snap);
    } else if (selectedKey.startsWith('sensor:')) {
      const v = sensors.get(selectedKey.slice('sensor:'.length));
      if (v) drawerBody.innerHTML = renderSensorDrawer(v);
    }
  }

  function select(key) {
    const prev = selectedKey;
    selectedKey = key;
    if (prev) restyleMarker(prev);
    restyleMarker(key);
    renderDrawer();
    drawerEl.classList.remove('hidden');
  }

  function clearSelection() {
    const prev = selectedKey;
    selectedKey = null;
    if (prev) restyleMarker(prev);
    drawerEl.classList.add('hidden');
  }

  document.getElementById('drawerClose').addEventListener('click', clearSelection);
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') clearSelection(); });
  map.on('click', clearSelection);

  // ─── Apply tracks ─────────────────────────────────────────────

  function applyTrack(snap, fromUpdate) {
    if (!snap || !snap.uid) return;
    const key = FL.trackKeyOf(snap);
    tracks.set(key, snap);

    const filters = readFilters();
    const stale = isTrackStale(snap);
    if (snap.position) {
      upsertMarker('track:' + key, snap.position.lat, snap.position.lon, trackIcon(snap), !!fromUpdate);
      setMarkerVisible('track:' + key, FL.passesFilters(snap, filters, { stale }));
    }
    if (FL.hasOperator(snap)) {
      upsertMarker('op:' + key, snap.rid.operatorLat, snap.rid.operatorLon, operatorIcon(), false);
      setMarkerVisible('op:' + key, filters.modality.has('operators') && FL.passesFilters(snap, filters, { stale }));
    }
    if (selectedKey === 'track:' + key || selectedKey === 'op:' + key) renderDrawer();

    // Map opened before any data: settle 1 s after the first positioned track,
    // then fit once via the server-computed focus (unless the operator moved).
    if (!focusedOnce && !userMoved && snap.position && !firstTrackTimer) {
      firstTrackTimer = setTimeout(() => { if (!focusedOnce) refocus(false); }, 1000);
    }
    refreshRail();
  }

  // Drop markers for tracks the server evicted (state.json is authoritative
  // on each poll; SSE has no delete event).
  function reconcileTracks(serverTracks) {
    const seen = new Set();
    serverTracks.forEach((snap) => seen.add(FL.trackKeyOf(snap)));
    Array.from(tracks.keys()).forEach((key) => {
      if (seen.has(key)) return;
      tracks.delete(key);
      removeMarker('track:' + key);
      removeMarker('op:' + key);
      if (selectedKey === 'track:' + key || selectedKey === 'op:' + key) clearSelection();
    });
  }

  // ─── Sensors layer ────────────────────────────────────────────

  function sensorKey(v) {
    return v.sensorId || v.nodeId || v.peerID || v.agentId || `${v.lat},${v.lon}`;
  }

  async function loadSensorsLayer() {
    try {
      const r = await fetch('/sensors.json');
      if (!r.ok) return;
      const body = await r.json();
      (body.sensors || []).forEach((v) => {
        const key = sensorKey(v);
        sensors.set(key, v);
        upsertMarker('sensor:' + key, v.lat, v.lon, sensorIcon(v), false);
      });
      applyFilters();
      refreshRail();
    } catch (e) { /* layer absent */ }
  }

  // ─── Rail painters ────────────────────────────────────────────

  const refs = {
    sourcesList: document.getElementById('sourcesList'),
    payloads: document.getElementById('payloadCounts'),
    freshness: document.querySelector('#card-payloads .freshness'),
    age: document.querySelector('#card-payloads .freshness .age'),
    staleThreshold: document.getElementById('staleThreshold'),
    profileBadges: document.getElementById('profileBadges'),
    eipGrid: document.getElementById('eipGrid'),
    conn: document.getElementById('connBadge'),
  };

  function payloadCounts() {
    let aircraft = 0, drones = 0, operators = 0, stale = 0;
    let liveAircraft = 0, liveDrones = 0;
    tracks.forEach((snap) => {
      const kind = FL.markerKind(snap);
      const isStale = isTrackStale(snap);
      if (kind === 'aircraft') aircraft++; else drones++;
      if (FL.hasOperator(snap)) operators++;
      if (isStale) stale++;
      else if (kind === 'aircraft') liveAircraft++;
      else liveDrones++;
    });
    return { aircraft, drones, operators, stale, sensors: sensors.size, liveAircraft, liveDrones };
  }

  function setCount(k, v) {
    const el = refs.payloads.querySelector(`[data-k="${k}"]`);
    if (el) el.textContent = String(v);
  }

  function paintPayloads() {
    const counts = payloadCounts();
    setCount('aircraft', counts.aircraft);
    setCount('drones', counts.drones);
    setCount('operators', counts.operators);
    setCount('sensors', counts.sensors);
    setCount('stale', counts.stale);
    updateRecenterButtons(counts);

    let maxLastSeen = 0;
    tracks.forEach((snap) => {
      const t = new Date(snap.lastSeen).getTime();
      if (t > maxLastSeen) maxLastSeen = t;
    });
    const age = maxLastSeen ? FL.trackAgeMs(maxLastSeen, Date.now(), clockSkewMs) : Infinity;
    const state = tracks.size === 0 ? 'idle' : age <= FRESH_MS ? 'fresh' : age <= WARM_MS ? 'warm' : 'stale';
    refs.freshness.dataset.state = state;
    refs.age.textContent = tracks.size === 0 ? 'no data'
      : age < 1000 ? 'fresh · just now'
      : age < 60_000 ? `${(age / 1000).toFixed(1)} s since last frame`
      : `${Math.floor(age / 60_000)} min since last frame`;
    refs.staleThreshold.textContent = `stale after ${Math.round(staleAfterMs / 1000)} s`;

    // Stale restyle sweep + live drawer age refresh.
    tracks.forEach((snap, key) => restyleMarker('track:' + key));
    if (selectedKey) {
      const ageEl = drawerBody.querySelector('.age[data-age-for]');
      if (ageEl) {
        const snap = tracks.get(ageEl.dataset.ageFor);
        if (snap) ageEl.textContent = `(${fmtAge(trackAge(snap))})`;
      }
    }

    // Profile badges: honest aggregate labels.
    const pillsOut = [];
    let anyFriendlyDemo = false, anySim = false, anyChain = false, wire = '';
    tracks.forEach((snap) => {
      if (snap.cot && snap.cot.demoProfile && snap.cot.affiliation === 'friendly') anyFriendlyDemo = true;
      if (snap.agent) { if (snap.agent.simulated) anySim = true; else anyChain = true; }
      if (snap.wire) wire = snap.wire;
    });
    if (anyFriendlyDemo) pillsOut.push('<span class="pill friendly" title="demo CoT display profile, not an assessment">FRIENDLY · demo CoT profile</span>');
    if (anyChain) pillsOut.push('<span class="pill onchain">EIP-8004 ON-CHAIN</span>');
    if (anySim) pillsOut.push('<span class="pill sim" title="in-memory EIP-8004 registry">EIP-8004 SIM</span>');
    if (wire) pillsOut.push(`<span class="pill wire">${esc(wire)}</span>`);
    refs.profileBadges.innerHTML = pillsOut.join('');
  }

  function paintSources() {
    if (!sources.length) {
      refs.sourcesList.innerHTML = sessionsMeta.configured
        ? '<p class="placeholder">no source evidence configured</p>'
        : '<p class="placeholder">no source evidence · no session feed</p>';
      return;
    }
    const cards = sources.map((s) => {
      const st = FL.sourceStatusLabel(s);
      const rate = s.messageRate != null ? `${s.messageRate.toFixed(1)}/s` : '—';
      const last = s.lastSeen ? fmtAge(FL.trackAgeMs(new Date(s.lastSeen).getTime(), Date.now(), clockSkewMs)) : '—';
      const chainPill = s.unregistered ? ''
        : (s.simulated
          ? '<span class="pill sim">SIM</span>'
          : '<span class="pill onchain">ON-CHAIN</span>');
      const caps = (s.status === 'offline' && s.capabilities && s.capabilities.length)
        ? `<div class="caps">${s.capabilities.map((c) => `<span class="pill cap">${esc(c)}</span>`).join('')}</div>`
        : '';
      const sensorNote = (s.status === 'offline' && !hasSensorFor(s.nodeId))
        ? '<p class="honest-note">sensor location: not provided</p>' : '';
      return `<div class="source-card" data-node="${esc(s.nodeId || '')}">
        <div class="source-head">
          <span class="source-name">${esc(s.name)}</span>
          <span class="status-pill ${esc(s.status)}">${esc(st.main)}</span>
        </div>
        ${st.sub ? `<p class="source-sub">${esc(st.sub)}</p>` : ''}
        <div class="source-meta">
          <span>agent ${esc(s.agentId || '—')}</span>${chainPill}
          <span class="mono">${esc(s.service || '')}</span>
        </div>
        <div class="source-stats">
          <span title="message rate over 60s">${esc(rate)}</span>
          <span title="tracks currently held">${s.trackCounts ? s.trackCounts.total : 0} trk</span>
          <span title="last seen">${esc(last)}</span>
        </div>
        ${caps}${sensorNote}
      </div>`;
    });
    refs.sourcesList.innerHTML = cards.join('');
  }

  function hasSensorFor(nodeId) {
    if (!nodeId) return false;
    let found = false;
    sensors.forEach((v) => { if (v.nodeId === nodeId) found = true; });
    return found;
  }

  function paintEIP() {
    const set = (k, v) => {
      const dd = refs.eipGrid.querySelector(`dd[data-k="${k}"]`);
      if (dd) dd.textContent = v || '—';
    };
    if (!sources.length) { set('registry', '—'); set('heartbeat', '—'); set('commerce', '—'); return; }
    const real = sources.filter((s) => !s.unregistered);
    const onChain = real.filter((s) => !s.simulated).length;
    const sim = real.length - onChain;
    const regParts = [];
    if (onChain) regParts.push(`${onChain} on-chain`);
    if (sim) regParts.push(`${sim} SIM`);
    set('registry', regParts.join(' · ') || '—');
    set('heartbeat', 'not published (file audit-lane)');
    set('commerce', 'off (advertisement-only)');
  }

  // ─── State polling (sources + skew + eviction reconcile) ─────

  async function pollState() {
    try {
      const r = await fetch('/state.json');
      if (!r.ok) return;
      const body = await r.json();
      if (body.now) clockSkewMs = Date.now() - new Date(body.now).getTime();
      if (typeof body.staleAfterMs === 'number') staleAfterMs = body.staleAfterMs;
      if (body.sessions) sessionsMeta = body.sessions;
      if (Array.isArray(body.sources)) {
        sources = body.sources;
        sourceByNode = new Map(sources.map((s) => [s.nodeId || '', s]));
      }
      if (Array.isArray(body.tracks)) {
        body.tracks.forEach((snap) => applyTrack(snap, false));
        reconcileTracks(body.tracks);
      }
      rebuildSourceFilters();
      paintSources();
      paintEIP();
      applyFilters();
    } catch (e) { /* keep last known */ }
  }

  let rafPending = false;
  function refreshRail() {
    if (rafPending) return;
    rafPending = true;
    requestAnimationFrame(() => { rafPending = false; paintPayloads(); });
  }
  setInterval(paintPayloads, 1000);
  setInterval(pollState, 5000);

  // ─── SSE ──────────────────────────────────────────────────────

  function setConn(state, label) {
    refs.conn.dataset.state = state;
    refs.conn.textContent = label;
  }

  function connectSSE() {
    setConn('connecting', 'connecting');
    const es = new EventSource('/events');
    es.addEventListener('open', () => setConn('live', 'live'));
    es.addEventListener('error', () => setConn('reconnecting', 'reconnecting'));
    const onEvent = (e) => {
      try {
        const ev = JSON.parse(e.data);
        if (ev && ev.kind === 'sapient-track' && ev.track) applyTrack(ev.track, true);
      } catch (err) { console.error(err); }
    };
    es.addEventListener('snapshot', onEvent);
    es.addEventListener('update', onEvent);
  }

  // Seed from /state.json so a fresh page is never blank — and open on the
  // server-computed focus when one is suggested.
  try {
    const r = await fetch('/state.json');
    if (r.ok) {
      const body = await r.json();
      if (body.now) clockSkewMs = Date.now() - new Date(body.now).getTime();
      if (typeof body.staleAfterMs === 'number') staleAfterMs = body.staleAfterMs;
      if (body.sessions) sessionsMeta = body.sessions;
      if (Array.isArray(body.sources)) {
        sources = body.sources;
        sourceByNode = new Map(sources.map((s) => [s.nodeId || '', s]));
      }
      if (Array.isArray(body.tracks)) body.tracks.forEach((snap) => applyTrack(snap, false));
      applyFocus(body.focus, false);
    }
  } catch (e) { /* ignore */ }

  rebuildSourceFilters();
  paintSources();
  paintEIP();
  paintPayloads();
  loadSensorsLayer();
  connectSSE();

  // Deep link: ?select=<uid|nodeId|uid|sensorKey> opens the detail drawer on
  // load (demo/share links; also exercised by screenshot automation).
  const wanted = new URLSearchParams(location.search).get('select');
  if (wanted) {
    const trySelect = (attempt) => {
      for (const key of tracks.keys()) {
        if (key === wanted || key.endsWith('|' + wanted) || key === wanted.split('|').pop()) {
          select('track:' + key);
          // A share link points at THIS track — bring it into view.
          const snap = tracks.get(key);
          if (snap && snap.position && !userMoved) {
            programmaticMove = true;
            focusedOnce = true;
            map.setView([snap.position.lat, snap.position.lon], 12);
          }
          return;
        }
      }
      for (const key of sensors.keys()) {
        if (key === wanted) { select('sensor:' + key); return; }
      }
      if (attempt < 20) setTimeout(() => trySelect(attempt + 1), 500);
    };
    trySelect(0);
  }
})();
