// Neuron Agent Explorer — tabbed tactical SA console.
//
// Two views over the SAPIENT demo:
//   • Tactical Map  — live drone tracks + operators, consumed read-only by the
//     server proxying sapient-fid-display (/tracks.json + /events). Degrades to
//     an "offline" banner when that feed is down.
//   • Agent Registry — registered agents from local evidence (/agents.json,
//     /agents/{id}.json): Agent Card, SIM vs ON-CHAIN provenance, sensor model,
//     wire format, and the verbatim agentURI card.
//
// Honest labels (never overclaim): FRIENDLY = demo CoT profile; SIM = in-memory
// EIP-8004 registry (placeholder tx hash suppressed server-side); SAPIENT
// protobuf = canonical wire, this display is a projection; missing fields → "—".

(function () {
  'use strict';

  const FRESH_MS = 5_000;
  const WARM_MS = 30_000;
  // Shared pure logic (markerKind / classLine / focusForKind / shortId …) —
  // byte-identical copy of the display's logic.js, so both UIs render the
  // same payload identically. Pinned by TestMapLayout_SharedLogicByteIdentical.
  const FL = window.FIDLogic;

  const TILE_PRIMARY = {
    url: 'https://{s}.basemaps.cartocdn.com/rastertiles/voyager_labels_under/{z}/{x}/{y}{r}.png',
    options: {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
      subdomains: 'abcd', maxZoom: 19,
    },
  };
  const TILE_FALLBACK = {
    url: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
    options: { attribution: '&copy; OpenStreetMap contributors', maxZoom: 19 },
  };

  // ─── Helpers ──────────────────────────────────────────────────
  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
    }[c]));
  }
  function dash(v) { return (v === undefined || v === null || v === '') ? '—' : v; }
  const short = (s, head = 10, tail = 6) => FL.shortId(s, head, tail);
  function timeOf(ts) { try { return new Date(ts).toLocaleTimeString(); } catch (e) { return '—'; } }
  function kv(k, v, mono) { return `<div class="kv"><span class="k">${esc(k)}</span><span class="v${mono ? ' mono' : ''}">${v}</span></div>`; }
  const $ = (sel) => document.querySelector(sel);

  // ─── State ────────────────────────────────────────────────────
  let cfg = { lat: 50.1027, lon: -5.6705, zoom: 13, fidDisplayUp: false };
  let map, tileLayer, tileFallbackActive = false;
  const tracks = new Map();           // uid -> snap
  const markers = new Map();          // key -> { marker, kind, uid }
  let sensors = [];                   // SensorView[] (operator-provided)
  const sensorMarkers = new Map();    // sensorKey -> { marker, view }
  let agents = [];                    // AgentSummary[]
  const detailCache = new Map();      // agentId -> AgentCardDetail
  let selectedAgentId = null;
  let selectedKey = null;             // marker key of the selected map entity
  let currentView = 'map';
  const filters = { drones: true, operators: true, stale: true, friendly: true, sim: true, onchain: true, sensors: true };

  // Auto-focus guards (forked from sapient-fid-display).
  let userMoved = false, programmaticMove = false, focusedOnce = false, firstTrackTimer = null;

  // ─── Boot ─────────────────────────────────────────────────────
  async function boot() {
    try {
      const r = await fetch('/config.json');
      if (r.ok) cfg = Object.assign(cfg, await r.json());
    } catch (e) { /* defaults */ }

    initMap();
    initTabs();
    initSidebar();
    initFilters();
    initDrawer();
    initRegistryNav();
    setFidBadge(cfg.fidDisplayUp);

    await loadAgents();
    await pollTracks();          // initial seed (or offline banner)
    await loadSensors();         // operator-provided sensor layer (off if --sensors unset)
    connectSSE();
    setInterval(paintLive, 1000);
    setInterval(pollTracks, 5000); // backstop refresh + offline/fid state
    applyHashRoute();
    window.addEventListener('hashchange', applyHashRoute);
    initOnboarding();          // welcome card (first run) + "how this works" tour

    // Deep link: ?select=<uid|nodeId|uid> opens the track drawer on load
    // (demo/share links; also exercised by screenshot automation).
    const wanted = new URLSearchParams(location.search).get('select');
    if (wanted) {
      const trySelect = (attempt) => {
        for (const key of tracks.keys()) {
          if (key === wanted || key.endsWith('|' + wanted) || key === wanted.split('|').pop()) {
            openTrackDrawer(key);
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
        if (attempt < 20) setTimeout(() => trySelect(attempt + 1), 500);
      };
      trySelect(0);
    }
  }

  // ─── Map ──────────────────────────────────────────────────────
  function initMap() {
    map = L.map('map', { zoomControl: true, attributionControl: true }).setView([cfg.lat, cfg.lon], cfg.zoom);
    tileLayer = L.tileLayer(TILE_PRIMARY.url, TILE_PRIMARY.options);
    tileLayer.on('tileerror', () => {
      if (tileFallbackActive) return;
      tileFallbackActive = true;
      map.removeLayer(tileLayer);
      tileLayer = L.tileLayer(TILE_FALLBACK.url, TILE_FALLBACK.options).addTo(map);
    });
    tileLayer.addTo(map);

    map.on('dragstart zoomstart', () => { if (!programmaticMove) userMoved = true; });
    map.on('moveend zoomend', () => { programmaticMove = false; });
    map.on('click', closeDrawer);

    // Per-modality recenter controls (mirrors sapient-fid-display): one
    // button per modality, disabled with an honest 0 when that modality has
    // no live tracks — never implying drones are live when only aircraft are.
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
  }

  const recenterBtns = new Map(); // kind -> <a>

  // Client-side per-modality focus: bounds from live tracks of that modality
  // only (shared logic.js focusForKind) — no cross-modality mixing.
  function recenterOnKind(kind) {
    const f = FL.focusForKind(tracks.values(), kind, Date.now(), 0, WARM_MS);
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

  function applyFocus(focus, force) {
    if (!focus || typeof focus.lat !== 'number') return;
    if (userMoved && !force) return;
    programmaticMove = true; focusedOnce = true;
    const degenerate = (focus.maxLat - focus.minLat) < 1e-6 && (focus.maxLon - focus.minLon) < 1e-6;
    if (focus.count <= 1 || degenerate) {
      map.setView([focus.lat, focus.lon], 15);
    } else {
      map.fitBounds([[focus.minLat, focus.minLon], [focus.maxLat, focus.maxLon]], { padding: [48, 48], maxZoom: 16 });
    }
  }
  async function refocus(force) {
    try {
      const r = await fetch('/tracks.json');
      if (r.ok) { const b = await r.json(); applyFocus(b.focus, force); }
    } catch (e) { /* keep view */ }
  }

  // Marker hitboxes are larger than the visible glyph (30px container vs a
  // ~20-26px shape) for comfortable clicking; rotation lives on the INNER
  // element so the hitbox and selected ring stay axis-aligned. Ported from
  // sapient-fid-display — an aircraft never renders as a drone dot.
  function trackIcon(snap) {
    const hasHdg = snap.velocity && typeof snap.velocity.trackDeg === 'number';
    if (FL.markerKind(snap) === 'aircraft') {
      const rot = hasHdg ? snap.velocity.trackDeg : 0;
      return L.divIcon({ className: '', iconSize: [30, 30], iconAnchor: [15, 15], html: `<div class="marker sapient-aircraft" role="img" aria-label="ADS-B aircraft"><svg viewBox="0 0 24 24" width="26" height="26" style="transform: rotate(${rot}deg)"><path d="M12 2 L13.5 9 L21 12 L13.5 13.5 L13 19 L15.5 21 L15.5 22 L12 21 L8.5 22 L8.5 21 L11 19 L10.5 13.5 L3 12 L10.5 9 Z"/></svg></div>` });
    }
    const heading = hasHdg ? `<div class="heading" style="transform: rotate(${snap.velocity.trackDeg}deg)"></div>` : '';
    return L.divIcon({ className: '', iconSize: [30, 30], iconAnchor: [15, 15], html: `<div class="marker sapient-track" role="img" aria-label="RID drone track">${heading}<div class="shape"></div></div>` });
  }
  function operatorIcon() {
    return L.divIcon({ className: '', iconSize: [26, 26], iconAnchor: [13, 13], html: `<div class="marker sapient-operator" role="img" aria-label="operator"><div class="shape"></div></div>` });
  }

  // markerEl returns the entry's root .marker div (recreated on setIcon).
  function markerEl(e) {
    const el = e.marker.getElement ? e.marker.getElement() : null;
    return el ? el.querySelector('.marker') : null;
  }

  // restyleMarker re-applies selection ring + stale fade after setIcon
  // recreates the element. Selected marker rises above overlapping neighbors.
  function restyleMarker(key) {
    const e = markers.get(key);
    if (!e) return;
    const el = markerEl(e);
    if (!el) return;
    const isSelected = key === selectedKey;
    el.classList.toggle('selected', isSelected);
    e.marker.setZIndexOffset(isSelected ? 1000 : 0);
    const snap = tracks.get(e.uid);
    if (snap) el.classList.toggle('stale', isStale(snap));
  }

  function upsertMarker(key, kind, uid, lat, lon, icon, onClick) {
    const e = markers.get(key);
    if (e) { e.marker.setLatLng([lat, lon]); e.marker.setIcon(icon); e.onClick = onClick; }
    else {
      const marker = L.marker([lat, lon], { icon, keyboard: false });
      const entry = { marker, kind, uid, onClick };
      marker.on('click', () => entry.onClick && entry.onClick());
      markers.set(key, entry);
    }
    restyleMarker(key);
  }

  const trackKeyOf = (snap) => FL.trackKeyOf(snap);

  function applyTrack(snap) {
    if (!snap || !snap.uid) return;
    const tkey = trackKeyOf(snap);
    tracks.set(tkey, snap);
    if (snap.position) {
      upsertMarker('track:' + tkey, 'track', tkey, snap.position.lat, snap.position.lon, trackIcon(snap), () => openTrackDrawer(tkey));
    }
    if (snap.rid && typeof snap.rid.operatorLat === 'number' && typeof snap.rid.operatorLon === 'number') {
      upsertMarker('op:' + tkey, 'op', tkey, snap.rid.operatorLat, snap.rid.operatorLon, operatorIcon(), () => openOperatorDrawer(tkey));
    }
    // Live re-render: an open drawer follows its track's updates (class /
    // confidence / position change per frame — a click-time snapshot would
    // freeze a low-confidence MLAT frame on screen while the map moves on).
    if (selectedKey === 'track:' + tkey) openTrackDrawer(tkey);
    else if (selectedKey === 'op:' + tkey) openOperatorDrawer(tkey);
    if (!focusedOnce && !userMoved && snap.position && !firstTrackTimer) {
      firstTrackTimer = setTimeout(() => { if (!focusedOnce) refocus(false); }, 1000);
    }
    applyFilters();
  }

  // ─── Filters / visibility ─────────────────────────────────────
  function isStale(snap) { return (Date.now() - new Date(snap.lastSeen).getTime()) > WARM_MS; }
  function isFriendlyDemo(snap) { return !!(snap.cot && snap.cot.demoProfile && snap.cot.affiliation === 'friendly'); }

  function markerVisible(snap, kind) {
    if (!snap) return false;
    if (kind === 'track' && !filters.drones) return false;
    if (kind === 'op' && !filters.operators) return false;
    if (!filters.stale && isStale(snap)) return false;
    if (isFriendlyDemo(snap) && !filters.friendly) return false;
    if (snap.agent) {
      if (snap.agent.simulated && !filters.sim) return false;
      if (!snap.agent.simulated && !filters.onchain) return false;
    }
    return true;
  }

  function applyFilters() {
    for (const [, e] of markers) {
      const snap = tracks.get(e.uid);
      const vis = markerVisible(snap, e.kind);
      const on = map.hasLayer(e.marker);
      if (vis && !on) e.marker.addTo(map);
      else if (!vis && on) map.removeLayer(e.marker);
    }
    for (const [, e] of sensorMarkers) {
      const on = map.hasLayer(e.marker);
      if (filters.sensors && !on) e.marker.addTo(map);
      else if (!filters.sensors && on) map.removeLayer(e.marker);
    }
  }

  function initFilters() {
    document.querySelectorAll('#filters input[data-filter]').forEach((cb) => {
      cb.addEventListener('change', () => { filters[cb.dataset.filter] = cb.checked; applyFilters(); });
    });
  }

  // ─── Honest pills ─────────────────────────────────────────────
  function pillsFor(o) {
    // o: { simulated?, friendlyDemo?, wire?, feedSource? }
    const out = [];
    if (o.friendlyDemo) out.push('<span class="pill friendly" title="demo CoT display profile, not an assessment">FRIENDLY · demo</span>');
    if (o.simulated === true) out.push('<span class="pill sim" title="in-memory EIP-8004 registry">EIP-8004 SIM</span>');
    else if (o.simulated === false) out.push('<span class="pill onchain">EIP-8004 ON-CHAIN</span>');
    out.push(`<span class="pill wire" title="canonical wire; this display is a projection">${esc(o.wire || 'SAPIENT protobuf')}</span>`);
    if (o.feedSource) out.push(`<span class="pill feed" data-feed="${esc(o.feedSource)}">feed: ${esc(o.feedSource)}</span>`);
    return out.join('');
  }

  // ─── Drawer ───────────────────────────────────────────────────
  // The drawer sits in the map's top-right corner ABOVE the filter panel
  // (z-index) and additionally shifts the filters left while open
  // (#view-map.drawer-open) so both stay fully visible and clickable.
  function initDrawer() {
    $('#drawerClose').addEventListener('click', closeDrawer);
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && obMode === 'idle') closeDrawer(); });
  }
  function closeDrawer() {
    $('#drawer').classList.add('hidden');
    $('#view-map').classList.remove('drawer-open');
    const prev = selectedKey;
    selectedKey = null;
    if (prev) restyleMarker(prev);
  }
  function openDrawer(html, key) {
    $('#drawerBody').innerHTML = html;
    $('#drawer').classList.remove('hidden');
    $('#view-map').classList.add('drawer-open');
    const prev = selectedKey;
    selectedKey = key || null;
    if (prev && prev !== selectedKey) restyleMarker(prev);
    if (selectedKey) restyleMarker(selectedKey);
  }

  function openTrackDrawer(uid) {
    const snap = tracks.get(uid);
    if (!snap) return;
    const p = [];
    const title = (snap.adsb && (snap.adsb.callsign || snap.adsb.registration)) || snap.uid;
    p.push(`<div class="pop-head"><span class="id">${esc(title)}</span></div>`);
    p.push(`<div class="pills-strip">${pillsFor({ simulated: snap.agent ? snap.agent.simulated : undefined, friendlyDemo: isFriendlyDemo(snap), wire: snap.wire, feedSource: snap.feedSource })}</div>`);

    // classLine is the shared formatter (logic.js, byte-identical to the
    // display's) — the same payload always renders the same class/confidence
    // text in both UIs; a missing confidence shows "conf not provided".
    const cl = FL.classLine(snap.classification);
    if (cl) p.push(kv('class', esc(cl)));
    if (snap.position) {
      p.push(kv('position', `${snap.position.lat.toFixed(5)}, ${snap.position.lon.toFixed(5)}`));
      p.push(kv('altitude', `${snap.position.alt.toFixed(0)} m`));
    }
    // Honest unknown: omitted velocity renders "—", never a fabricated 0.
    p.push(kv('speed', snap.velocity ? `${(snap.velocity.speedMps * 3.6).toFixed(1)} km/h · track ${snap.velocity.trackDeg.toFixed(1)}°` : '—'));

    p.push('<div class="sect">SAPIENT</div>');
    p.push(kv('object_id', esc(dash(snap.objectId)), true));
    p.push(kv('node_id', esc(short(snap.nodeId, 13, 6)), true));
    if (snap.rid) {
      if (snap.rid.idType) p.push(kv('id type', esc(snap.rid.idType)));
      if (snap.rid.uaType) p.push(kv('ua type', esc(snap.rid.uaType)));
      if (snap.rid.status) p.push(kv('status', esc(snap.rid.status)));
      if (snap.rid.operatorId) p.push(kv('operator', esc(snap.rid.operatorId)));
    }
    if (snap.rf) {
      p.push('<div class="sect">RF SIGNAL</div>');
      p.push(kv('rssi', typeof snap.rf.rssiDbm === 'number' ? `${snap.rf.rssiDbm.toFixed(0)} dBm` : '—'));
      p.push(kv('frequency', typeof snap.rf.frequencyHz === 'number' ? `${(snap.rf.frequencyHz / 1e9).toFixed(3)} GHz` : '—'));
      if (snap.rf.transport) p.push(kv('transport', esc(snap.rf.transport)));
      if (snap.rf.channel) p.push(kv('channel', esc(snap.rf.channel)));
    }
    if (snap.cot) {
      p.push('<div class="sect">CoT PROJECTION</div>');
      p.push(kv('type', esc(dash(snap.cot.type)), true));
      p.push(kv('how', esc(dash(snap.cot.how)), true));
      p.push(kv('affiliation', esc(dash(snap.cot.affiliation)) + (snap.cot.demoProfile ? ' (demo profile)' : '')));
    }
    if (snap.agent) {
      p.push('<div class="sect">AGENT CARD · EIP-8004</div>');
      p.push(kv('agent id', esc(dash(snap.agent.agentId))));
      p.push(kv('seller EVM', esc(short(snap.agent.sellerEVM)), true));
      p.push(kv('registry', snap.agent.simulated ? 'in-memory (SIM)' : 'on-chain'));
    }
    const open = snap.agent && snap.agent.agentId
      ? `<button class="linklike" id="drawerToReg" data-agent="${esc(snap.agent.agentId)}">open in Registry →</button>` : '';
    p.push(`<div class="drawer-foot"><span>frames ${esc(dash(snap.frameCount))}</span><span>seen ${esc(timeOf(snap.lastSeen))}</span></div>`);
    if (open) p.push(`<div style="margin-top:8px">${open}</div>`);
    openDrawer(p.join(''), 'track:' + uid);
    wireDrawerToReg();
  }

  function openOperatorDrawer(uid) {
    const snap = tracks.get(uid);
    if (!snap || !snap.rid) return;
    const p = [];
    p.push(`<div class="pop-head"><span class="id">${esc(snap.rid.operatorId || snap.uid + '-OP')} <span class="muted" style="font-weight:400">(operator)</span></span></div>`);
    p.push(`<div class="pills-strip">${pillsFor({ simulated: snap.agent ? snap.agent.simulated : undefined, friendlyDemo: isFriendlyDemo(snap), wire: snap.wire, feedSource: snap.feedSource })}</div>`);
    p.push(kv('position', `${snap.rid.operatorLat.toFixed(5)}, ${snap.rid.operatorLon.toFixed(5)}`));
    if (typeof snap.rid.operatorAltM === 'number') p.push(kv('altitude', `${snap.rid.operatorAltM.toFixed(0)} m`));
    if (snap.rid.operatorIdType) p.push(kv('id type', esc(snap.rid.operatorIdType)));
    p.push(kv('paired track', esc(snap.uid), true));
    p.push(`<div class="drawer-foot"><span>frames ${esc(dash(snap.frameCount))}</span><span>seen ${esc(timeOf(snap.lastSeen))}</span></div>`);
    openDrawer(p.join(''), 'op:' + uid);
  }

  function wireDrawerToReg() {
    const b = $('#drawerToReg');
    if (b) b.addEventListener('click', () => { selectAgent(b.dataset.agent, true); });
  }

  // ─── Sensor / seller location layer (operator-provided config) ─
  // Off unless the server was started with --sensors. Locations are declared by
  // the operator (the only honest source today); never inferred from tracks/RF.
  function sensorSourcePill(src) {
    if (src === 'estimated') return '<span class="pill est" title="operator-marked approximate position">estimated · approx</span>';
    return `<span class="pill src" title="location provenance">${esc(src || 'configured')}</span>`;
  }
  function sensorKey(v, i) { return 'sensor:' + (v.sensorId || v.agentId || v.peerID || v.nodeId || ('idx' + i)); }
  function sensorIcon(v) {
    const est = v.source === 'estimated' ? ' estimated' : '';
    return L.divIcon({ className: '', iconSize: [22, 22], iconAnchor: [11, 11],
      html: `<div class="marker sapient-sensor${est}" role="img" aria-label="sensor location"><div class="shape"></div></div>` });
  }
  function matchAgentForSensor(v) {
    return agents.find((a) =>
      (v.agentId && a.agentId === v.agentId) ||
      (v.peerID && a.peerID === v.peerID) ||
      (v.nodeId && a.nodeId === v.nodeId)) || null;
  }
  async function loadSensors() {
    try {
      const r = await fetch('/sensors.json');
      if (!r.ok) return;
      const b = await r.json();
      sensors = Array.isArray(b.sensors) ? b.sensors : [];
      if (Array.isArray(b.warnings) && b.warnings.length) console.warn('sensor-locations warnings:', b.warnings);
      if (b.error) console.warn('sensor-locations error:', b.error);
      renderSensors();
    } catch (e) { /* layer simply absent */ }
  }
  function renderSensors() {
    sensors.forEach((v, i) => {
      const key = sensorKey(v, i);
      const e = sensorMarkers.get(key);
      if (e) { e.marker.setLatLng([v.lat, v.lon]); e.marker.setIcon(sensorIcon(v)); e.view = v; }
      else {
        const marker = L.marker([v.lat, v.lon], { icon: sensorIcon(v) });
        const entry = { marker, view: v };
        marker.on('click', () => openSensorDrawer(key));
        sensorMarkers.set(key, entry);
      }
    });
    applyFilters();
  }
  function openSensorDrawer(key) {
    const e = sensorMarkers.get(key);
    if (!e) return;
    const v = e.view;
    const p = [];
    p.push(`<div class="pop-head"><span class="id">${esc(v.label || v.sensorId || 'sensor')} <span class="muted" style="font-weight:400">(sensor)</span></span></div>`);
    p.push(`<div class="pills-strip">${sensorSourcePill(v.source)}${v.confidence ? `<span class="pill meta">${esc(v.confidence)}</span>` : ''}</div>`);
    p.push(kv('position', `${v.lat.toFixed(5)}, ${v.lon.toFixed(5)}`));
    if (typeof v.altM === 'number') p.push(kv('altitude', `${v.altM.toFixed(0)} m`));
    if (v.sensorId) p.push(kv('sensor id', esc(v.sensorId), true));
    if (v.nodeId) p.push(kv('node_id', esc(short(v.nodeId, 13, 6)), true));
    if (v.peerID) p.push(kv('peer id', esc(short(v.peerID)), true));
    if (v.lastUpdated) p.push(kv('updated', esc(v.lastUpdated)));
    const a = matchAgentForSensor(v);
    if (a) p.push(`<div style="margin-top:8px"><button class="linklike" id="drawerToReg" data-agent="${esc(a.agentId)}">open in Registry →</button></div>`);
    else p.push('<p class="honest-note">no matching registered agent</p>');
    openDrawer(p.join(''));
    wireDrawerToReg();
  }

  // ─── Sources (proxied from the display's additive /state.json) ───
  let sources = [];
  function sourceStatusFor(nodeId) {
    const src = sources.find((s) => s.nodeId === nodeId);
    return src ? src.status : '';
  }
  function statusPill(status) {
    if (!status) return '<span class="pill src">—</span>';
    const cls = { live: 'onchain', stale: 'feed', offline: 'sim', unknown: 'src' }[status] || 'src';
    return `<span class="pill ${cls}">${esc(status.toUpperCase())}</span>`;
  }
  function paintSources() {
    const host = $('#sourcesMini');
    if (!host) return;
    if (!sources.length) { host.innerHTML = '<p class="muted" style="font-size:11px">no source data from display</p>'; return; }
    host.innerHTML = sources.map((s) => `
      <div class="source-mini">
        <span>${esc(s.name)}</span>
        ${statusPill(s.status)}
      </div>`).join('');
  }

  // ─── Live counts / freshness ──────────────────────────────────
  function paintLive() {
    let operators = 0, aircraft = 0, maxLastSeen = 0;
    let liveAircraft = 0, liveDrones = 0;
    for (const snap of tracks.values()) {
      const kind = FL.markerKind(snap);
      const stale = isStale(snap);
      if (snap.rid && typeof snap.rid.operatorLat === 'number') operators++;
      if (kind === 'aircraft') { aircraft++; if (!stale) liveAircraft++; }
      else if (!stale) liveDrones++;
      const t = new Date(snap.lastSeen).getTime();
      if (t > maxLastSeen) maxLastSeen = t;
    }
    $('#cTracks').textContent = String(tracks.size);
    const cA = $('#cAircraft'); if (cA) cA.textContent = String(aircraft);
    const cD = $('#cDrones'); if (cD) cD.textContent = String(tracks.size - aircraft);
    $('#cOperators').textContent = String(operators);
    updateRecenterButtons({ liveAircraft, liveDrones });
    const age = maxLastSeen ? Date.now() - maxLastSeen : Infinity;
    const fr = $('#freshness');
    const state = tracks.size === 0 ? 'idle' : age <= FRESH_MS ? 'fresh' : age <= WARM_MS ? 'warm' : 'stale';
    fr.dataset.state = state;
    fr.querySelector('.age').textContent = tracks.size === 0 ? 'no data'
      : age < 1000 ? 'fresh · just now'
      : age < 60_000 ? `${(age / 1000).toFixed(1)} s since last frame`
      : `${Math.floor(age / 60_000)} min since last frame`;
    applyFilters(); // staleness may have changed visibility
    markers.forEach((_, key) => restyleMarker(key)); // stale fade sweep
  }

  // ─── Tracks feed (proxy) ──────────────────────────────────────
  async function pollTracks() {
    try {
      const r = await fetch('/tracks.json');
      if (!r.ok) { showOffline(true); return; }
      const b = await r.json();
      if (b.degraded) { showOffline(true); setFidBadge(false); return; }
      showOffline(false); setFidBadge(true);
      if (Array.isArray(b.tracks)) b.tracks.forEach(applyTrack);
      if (Array.isArray(b.sources)) { sources = b.sources; paintSources(); renderRegistryTable(); }
      if (b.focus) applyFocus(b.focus, false);
    } catch (e) { showOffline(true); }
  }

  function showOffline(on) { $('#offlineBanner').classList.toggle('hidden', !on); }
  function setFidBadge(up) {
    const el = $('#fidBadge');
    el.dataset.state = up ? 'up' : 'down';
    el.textContent = up ? 'feed live' : 'feed offline';
  }

  // ─── SSE ──────────────────────────────────────────────────────
  function setConn(state, label) { const el = $('#connBadge'); el.dataset.state = state; el.textContent = label; }
  function connectSSE() {
    setConn('connecting', 'connecting');
    let es;
    try { es = new EventSource('/events'); } catch (e) { setConn('reconnecting', 'reconnecting'); return; }
    es.addEventListener('open', () => setConn('live', 'live'));
    es.addEventListener('error', () => setConn('reconnecting', 'reconnecting'));
    es.addEventListener('degraded', () => { showOffline(true); setFidBadge(false); });
    const onEvent = (e) => {
      try {
        const ev = JSON.parse(e.data);
        if (ev && ev.kind === 'sapient-track' && ev.track) { showOffline(false); setFidBadge(true); applyTrack(ev.track); }
      } catch (err) { /* ignore malformed frame */ }
    };
    es.addEventListener('snapshot', onEvent);
    es.addEventListener('update', onEvent);
  }

  // ─── Tabs / routing ───────────────────────────────────────────
  function initTabs() {
    document.querySelectorAll('#tabs .tab').forEach((t) => {
      t.addEventListener('click', () => { location.hash = '#/' + t.dataset.view; });
    });
  }
  function applyHashRoute() {
    const v = location.hash === '#/registry' ? 'registry' : 'map';
    switchView(v);
  }
  function switchView(v) {
    currentView = v;
    document.querySelectorAll('#tabs .tab').forEach((t) => {
      const on = t.dataset.view === v;
      t.classList.toggle('active', on);
      t.setAttribute('aria-selected', on ? 'true' : 'false');
    });
    $('#view-map').classList.toggle('active', v === 'map');
    $('#view-registry').classList.toggle('active', v === 'registry');
    if (v === 'map' && map) setTimeout(() => map.invalidateSize(), 0);
  }

  // ─── Sidebar ──────────────────────────────────────────────────
  function initSidebar() {
    $('#sidebarToggle').addEventListener('click', () => {
      const sb = $('#sidebar');
      const collapsed = sb.classList.toggle('collapsed');
      $('#sidebarToggle').textContent = collapsed ? '»' : '«';
      if (map) setTimeout(() => map.invalidateSize(), 160);
    });
    $('#agentSearch').addEventListener('input', (e) => renderAgentList(e.target.value));
  }

  // ─── Registry ─────────────────────────────────────────────────
  async function loadAgents() {
    try {
      const r = await fetch('/agents.json');
      const b = await r.json();
      agents = Array.isArray(b.agents) ? b.agents : [];
      $('#agentCount').textContent = String(agents.length);
      const note = b.error
        ? `evidence: ${esc(b.evidenceDir || '—')} · ⚠ ${esc(b.error)}`
        : `evidence: ${esc(b.evidenceDir || '—')} · ${agents.length} agent(s)`;
      $('#evidenceDirNote').innerHTML = note;
      renderAgentList('');
      renderRegistryTable();
    } catch (e) {
      $('#evidenceDirNote').textContent = 'failed to load evidence';
    }
  }

  function agentMatches(a, q) {
    if (!q) return true;
    q = q.toLowerCase();
    return [a.agentId, a.sellerEVM, a.nodeId, a.peerID].some((x) => (x || '').toLowerCase().includes(q));
  }
  function provPill(a) {
    return a.simulated ? '<span class="pill sim">SIM</span>' : '<span class="pill onchain">ON-CHAIN</span>';
  }

  function renderAgentList(q) {
    const list = $('#agentList');
    const shown = agents.filter((a) => agentMatches(a, q));
    if (!shown.length) { list.innerHTML = '<div class="muted" style="font-size:12px">no agents</div>'; return; }
    list.innerHTML = shown.map((a) => `
      <div class="agent-item${a.agentId === selectedAgentId ? ' active' : ''}" role="listitem" data-agent="${esc(a.agentId)}">
        <div class="ai-top"><span class="ai-id">#${esc(dash(a.agentId))}</span>${provPill(a)}</div>
        <div class="ai-sub">${esc(short(a.sellerEVM))}</div>
      </div>`).join('');
    list.querySelectorAll('.agent-item').forEach((it) => it.addEventListener('click', () => selectAgent(it.dataset.agent, true)));
  }

  function renderRegistryTable() {
    const body = $('#registryBody');
    const empty = $('#registryEmpty');
    if (!agents.length) {
      body.innerHTML = '';
      empty.classList.remove('hidden');
      empty.textContent = 'No agent evidence found. Point --dir at a seller evidence directory.';
      return;
    }
    empty.classList.add('hidden');
    body.innerHTML = agents.map((a) => `
      <tr data-agent="${esc(a.agentId)}"${a.agentId === selectedAgentId ? ' class="active"' : ''}>
        <td>#${esc(dash(a.agentId))}</td>
        <td>${esc(short(a.sellerEVM))}</td>
        <td>${esc(short(a.peerID))}</td>
        <td>${esc(short(a.nodeId, 8, 6))}</td>
        <td>${esc(dash(a.service))}</td>
        <td>${esc(dash(a.protocol))}</td>
        <td>${provPill(a)}</td>
        <td>${statusPill(sourceStatusFor(a.nodeId))}</td>
      </tr>`).join('');
    body.querySelectorAll('tr').forEach((tr) => tr.addEventListener('click', () => selectAgent(tr.dataset.agent, false)));
  }

  // Breadcrumb + back nav for the two-pane registry. The breadcrumb root and the
  // "← Back to agents" button reveal the list pane on narrow screens; the
  // selection persists (stays highlighted) so the user keeps their place.
  function setCrumb(id) {
    const sep = $('#crumbSep'), cur = $('#crumbCurrent');
    if (!sep || !cur) return;
    if (id) { cur.textContent = 'Agent #' + id; sep.classList.remove('hidden'); }
    else { cur.textContent = ''; sep.classList.add('hidden'); }
  }
  function initRegistryNav() {
    const toList = () => {
      const split = $('#registrySplit');
      if (split) split.classList.remove('show-detail');
      const pane = $('#registryListPane');
      if (pane) pane.scrollTop = 0;
    };
    const root = $('#crumbRoot'); if (root) root.addEventListener('click', toList);
    const back = $('#backToAgents'); if (back) back.addEventListener('click', toList);
  }

  async function selectAgent(id, switchToRegistry) {
    selectedAgentId = id;
    if (switchToRegistry) location.hash = '#/registry';
    setCrumb(id);
    const split = $('#registrySplit');
    if (split) split.classList.add('show-detail'); // narrow: reveal the detail pane
    renderAgentList($('#agentSearch').value);
    renderRegistryTable();
    let d = detailCache.get(id);
    if (!d) {
      try {
        const r = await fetch('/agents/' + encodeURIComponent(id) + '.json');
        if (!r.ok) { $('#agentDetail').innerHTML = `<p class="muted">agent ${esc(id)} not found.</p>`; return; }
        d = await r.json();
        detailCache.set(id, d);
      } catch (e) { $('#agentDetail').innerHTML = '<p class="muted">failed to load agent detail.</p>'; return; }
    }
    renderDetail(d);
  }

  function renderDetail(d) {
    const prov = d.provenance || {};
    const p = [];
    p.push(`<div class="pop-head"><span class="id">Agent #${esc(dash(d.agentId))}</span></div>`);
    p.push(`<div class="pills-strip">${pillsFor({ simulated: d.simulated, wire: d.wire, feedSource: d.feedSource })}</div>`);

    p.push('<div class="sect">IDENTITY</div>');
    p.push(kv('seller EVM', esc(dash(d.sellerEVM)), true));
    p.push(kv('peer id', esc(dash(d.peerID)), true));
    p.push(kv('node_id', esc(dash(d.nodeId)), true));
    p.push(kv('service', esc(dash(d.service))));
    p.push(kv('protocol', esc(dash(d.protocol)), true));

    p.push('<div class="sect">PROVENANCE</div>');
    p.push(kv('mode', `<strong style="color:${prov.mode === 'ON-CHAIN' ? 'var(--ok)' : 'var(--sim)'}">${esc(dash(prov.mode))}</strong>`));
    p.push(kv('chainId', esc(String(prov.chainId ?? '—'))));
    p.push(kv('registry', esc(dash(prov.registryAddress)), true));
    p.push(kv('outcome', esc(dash(prov.outcome))));
    p.push(kv('tokenId', esc(dash(prov.tokenId))));
    if (prov.transactionHash) p.push(kv('tx hash', esc(short(prov.transactionHash, 10, 8)), true));
    else if (prov.mode === 'SIM') p.push('<p class="honest-note">tx hash hidden — SIM placeholder, not a real receipt</p>');
    p.push(kv('agentURI sha256', esc(short(d.agentURISha256, 10, 8)), true));
    p.push(`<p class="honest-note">source: ${esc(dash(prov.source))}</p>`);

    p.push(`<div class="sect">SENSOR MODEL · ${esc((d.sensor && d.sensor.extensionId) || 'capability extension')}</div>`);
    if (d.sensor) {
      if (d.sensor.modality) p.push(kv('modality', esc(d.sensor.modality)));
      p.push(kv('wire', esc(dash(d.sensor.wire))));
      p.push(kv('sensors', esc((d.sensor.sensorModels || []).join(', ') || '—')));
      if (d.sensor.capabilities && d.sensor.capabilities.length) {
        p.push(kv('capabilities', d.sensor.capabilities.map((c) => `<span class="pill src">${esc(c)}</span>`).join(' ')));
      }
      p.push(kv('schema', esc(short(d.sensor.schema, 18, 10)), true));
      p.push(kv('schema sha256', esc(short(d.sensor.schemaSha256, 10, 8)), true));
    } else {
      p.push('<p class="muted" style="font-size:11.5px">no neuron.* capability extension in this card</p>');
    }

    // Sensor location — operator-provided (the only honest source today). Never
    // inferred from drone/operator/RF positions.
    const sensorLoc = sensors.find((v) =>
      (v.agentId && v.agentId === d.agentId) ||
      (v.peerID && v.peerID === d.peerID) ||
      (v.nodeId && v.nodeId === d.nodeId));
    p.push('<div class="sect">SENSOR LOCATION</div>');
    if (sensorLoc) {
      p.push(kv('position', `${sensorLoc.lat.toFixed(5)}, ${sensorLoc.lon.toFixed(5)}`));
      p.push(kv('source', sensorSourcePill(sensorLoc.source)));
      if (sensorLoc.label) p.push(kv('label', esc(sensorLoc.label)));
    } else {
      p.push('<p class="muted" style="font-size:11.5px">sensor location not provided</p>');
    }

    // Services from the verbatim card.
    const svcs = (d.card && Array.isArray(d.card.services)) ? d.card.services : [];
    p.push(`<div class="sect">SERVICES (${svcs.length})</div>`);
    p.push(`<div class="svc-chips">${svcs.map((s) => `<span class="svc-chip">${esc(s.name || s.type || '?')}</span>`).join('') || '<span class="muted">—</span>'}</div>`);

    // Full card JSON accordion.
    const cardJSON = d.card ? JSON.stringify(d.card, null, 2) : 'null';
    p.push(`<div class="card-json"><details><summary>FULL AGENT CARD JSON <button class="linklike copy-btn" id="copyCard">copy</button></summary><pre id="cardPre" class="card-json-pre">${esc(cardJSON)}</pre></details></div>`);

    $('#agentDetail').innerHTML = p.join('');
    const cp = $('#copyCard');
    if (cp) cp.addEventListener('click', (e) => {
      e.preventDefault();
      const txt = $('#cardPre').textContent;
      if (navigator.clipboard) navigator.clipboard.writeText(txt).then(() => { cp.textContent = 'copied'; setTimeout(() => { cp.textContent = 'copy'; }, 1200); });
    });
  }

  // ─── Onboarding: welcome card + guided spotlight tour ─────────
  // Teaches the Neuron story in-app (no separate tutorial site). A first-time
  // viewer sees a dismissible welcome card; the "❔ How this works" header button
  // re-launches the 6-step tour anytime. Copy is honest — it explains SIM / FRIENDLY
  // / feed:live / SAPIENT protobuf / "display is a projection" rather than overclaiming.
  // Vanilla JS; dismissal persisted in localStorage. Steps are author-controlled
  // static HTML (no user data interpolated, so no injection surface).
  const OB_LS_KEY = 'neuron-explorer-tour';
  const OB_LS_VAL = 'done@v1';
  let obIdx = 0;
  let obMode = 'idle'; // 'idle' | 'welcome' | 'tour'

  function obRoot() { return $('#onboard'); }
  function feedUp() { const b = $('#fidBadge'); return !!b && b.dataset.state === 'up'; }
  function tourDone() { try { return localStorage.getItem(OB_LS_KEY) === OB_LS_VAL; } catch (e) { return false; } }
  function markTourDone() { try { localStorage.setItem(OB_LS_KEY, OB_LS_VAL); } catch (e) { /* private mode: no-op */ } }

  // Step model: { view?, target?, prep?, title, body, bodyDegraded?, degraded? }.
  const OB_STEPS = [
    {
      view: 'registry', target: '#registryTableWrap',
      title: '1 · The Agent Registry',
      body: `Every sensor that joins the demo registers as an <strong>agent</strong>. This table is the
        directory of registered seller agents — each one's identity (EIP-8004 token + EVM address),
        peer ID, and the service it offers.`,
      bodyDegraded: `Every sensor that joins the demo registers as an <strong>agent</strong> and would
        appear in this table — its identity (EIP-8004 token + EVM address), peer ID, and the service it
        offers. No agents are loaded right now; point the Explorer at a seller's evidence directory to
        populate it.`,
      degraded: () => agents.length === 0,
    },
    {
      view: 'registry', target: '#agentDetail',
      prep: async () => { if (agents.length && !selectedAgentId) await selectAgent(agents[0].agentId, false); },
      title: '2 · The Agent Card',
      body: `Selecting an agent opens its <strong>Agent Card</strong> — the seller's self-describing
        capability and evidence document: who it is, what it offers, the canonical data format, and how
        its identity was registered. It's what a buyer inspects before trusting a feed.`,
      bodyDegraded: `An agent's <strong>Agent Card</strong> is its self-describing capability and
        evidence document — identity, offered service, data format, and registration proof. It's what a
        buyer inspects before trusting a feed. Register an agent to see a live card here.`,
      degraded: () => agents.length === 0,
    },
    {
      view: 'registry', target: '#agentDetail',
      title: '3 · What this seller offers',
      body: `This seller advertises the <strong><code>rid</code></strong> service over libp2p protocol
        <strong><code>/sapient/detection/2.0.0</code></strong> — Remote ID drone detections delivered as
        SAPIENT messages. The capability is declared in the card, so a buyer knows exactly what it's
        connecting to before paying.`,
    },
    {
      view: 'map', target: '#map',
      title: '4 · The tactical map',
      body: `Each marker is a SAPIENT <strong>DetectionReport</strong> streamed from a seller agent:
        <strong>teal</strong> = drone track, <strong>amber</strong> = operator/pilot,
        <strong>fuchsia</strong> = sensor. The map is a <strong>projection</strong> of the underlying
        SAPIENT protobuf — it visualises the feed, it isn't the source of truth.`,
      bodyDegraded: `The tactical map plots each SAPIENT <strong>DetectionReport</strong> from a seller
        agent (teal drone, amber operator, fuchsia sensor). The live feed is offline right now, so no
        tracks are shown — the Agent Registry still works independently.`,
      degraded: () => !feedUp(),
    },
    {
      view: 'map', target: '#honesty',
      title: '5 · Reading the labels honestly',
      body: `The Explorer never overclaims. <strong><code>feed: live</code></strong> = a real sensor
        feed (vs replay/synthetic). <strong>EIP-8004 ON-CHAIN</strong> = identity on a real chain;
        <strong>SIM</strong> = an in-memory registry for the demo. <strong>FRIENDLY</strong> = a demo
        CoT display profile, not a tactical assessment. <strong>SAPIENT protobuf</strong> is the
        canonical wire; the map is a projection. Missing fields always render <code>—</code>, never a guess.`,
    },
    {
      view: 'map', target: '#map',
      title: '6 · From a track back to its agent',
      body: `Click any drone track to open its detail drawer, then <strong>"open in Registry →"</strong>
        to jump to the exact Agent Card that produced it. Live detection ↔ registered, verifiable
        agent — that round-trip is the point of Neuron. You're done! Re-open this tour anytime via
        <strong>❔ How this works</strong> in the header.`,
    },
  ];

  function initOnboarding() {
    const btn = $('#tourBtn');
    if (btn) btn.addEventListener('click', startTour);
    if (!tourDone()) showWelcome();
  }

  function openRoot() {
    const r = obRoot();
    if (!r) return;
    r.classList.remove('hidden');
    r.setAttribute('aria-hidden', 'false');
    requestAnimationFrame(() => r.classList.add('open'));
    document.addEventListener('keydown', onboardKeydown, true);
  }

  function closeOnboard() {
    const r = obRoot();
    if (!r) return;
    r.classList.remove('open', 'modal');
    r.classList.add('hidden');
    r.setAttribute('aria-hidden', 'true');
    r.innerHTML = '';
    obMode = 'idle';
    document.removeEventListener('keydown', onboardKeydown, true);
    window.removeEventListener('resize', positionStep);
  }

  // Esc closes (and marks done so it won't re-pop); arrows page through the tour.
  function onboardKeydown(e) {
    if (e.key === 'Escape') { e.preventDefault(); finishOrSkip(); return; }
    if (obMode !== 'tour') return;
    if (e.key === 'ArrowRight') { e.preventDefault(); stepBy(1); }
    else if (e.key === 'ArrowLeft') { e.preventDefault(); stepBy(-1); }
  }

  function finishOrSkip() { markTourDone(); closeOnboard(); }

  function showWelcome() {
    obMode = 'welcome';
    const r = obRoot();
    if (!r) return;
    openRoot();
    r.classList.add('modal');
    r.innerHTML = `
      <div id="onboardCard" class="glass centered">
        <div class="ob-head"><span class="ob-step">Welcome</span>
          <button class="ob-x" type="button" aria-label="close">×</button></div>
        <h3 class="ob-title">Welcome to the Neuron Agent Explorer</h3>
        <div class="ob-body">
          This console shows the SAPIENT drone-detection demo as a network of verifiable
          <strong>agents</strong>. A sensor's software registers as an agent, advertises what it can
          provide, and its live detections appear on the tactical map. Take a ~60-second tour to see how
          the pieces fit together.
          <p class="ob-note">Demo view — a read-only projection. Labels like SIM and FRIENDLY are explained as you go.</p>
        </div>
        <div class="ob-foot">
          <button class="ob-skip" type="button">Skip for now</button>
          <div class="ob-nav"><button class="ob-btn primary" id="obStart" type="button">Start tour</button></div>
        </div>
      </div>`;
    const card = $('#onboardCard');
    card.querySelector('.ob-x').addEventListener('click', finishOrSkip);
    card.querySelector('.ob-skip').addEventListener('click', finishOrSkip);
    const start = $('#obStart');
    start.addEventListener('click', startTour);
    start.focus();
  }

  function startTour() {
    markTourDone();             // launching counts as "seen" — no auto re-pop later
    obMode = 'tour';
    obIdx = 0;
    if (obRoot() && obRoot().classList.contains('hidden')) openRoot();
    window.addEventListener('resize', positionStep);
    showStep();
  }

  function stepBy(d) {
    const n = obIdx + d;
    if (n < 0) return;
    if (n >= OB_STEPS.length) { finishOrSkip(); return; }
    obIdx = n;
    showStep();
  }

  async function showStep() {
    const step = OB_STEPS[obIdx];
    if (step.view && currentView !== step.view) switchView(step.view);
    if (step.prep) { try { await step.prep(); } catch (e) { /* tolerate a failed prep; copy still applies */ } }
    if (obMode !== 'tour') return; // user closed mid-prep
    renderStepCard(step);
    // Let the view switch / agent render / map.invalidateSize settle, then place.
    requestAnimationFrame(() => requestAnimationFrame(positionStep));
  }

  function renderStepCard(step) {
    const r = obRoot();
    const degraded = typeof step.degraded === 'function' && step.degraded();
    const body = (degraded && step.bodyDegraded) ? step.bodyDegraded : step.body;
    const first = obIdx === 0;
    const last = obIdx === OB_STEPS.length - 1;
    r.innerHTML = `
      <div id="onboardRing" class="hidden"></div>
      <div id="onboardCard" class="glass">
        <div class="ob-head"><span class="ob-step">Step ${obIdx + 1} of ${OB_STEPS.length}</span>
          <button class="ob-x" type="button" aria-label="close tour">×</button></div>
        <h3 class="ob-title">${step.title}</h3>
        <div class="ob-body">${body}</div>
        <div class="ob-foot">
          <button class="ob-skip" type="button">Skip</button>
          <div class="ob-nav">
            ${first ? '' : '<button class="ob-btn" id="obBack" type="button">Back</button>'}
            <button class="ob-btn primary" id="obNext" type="button">${last ? 'Finish' : 'Next'}</button>
          </div>
        </div>
      </div>`;
    const card = $('#onboardCard');
    card.querySelector('.ob-x').addEventListener('click', finishOrSkip);
    card.querySelector('.ob-skip').addEventListener('click', finishOrSkip);
    const back = $('#obBack'); if (back) back.addEventListener('click', () => stepBy(-1));
    const next = $('#obNext'); next.addEventListener('click', () => stepBy(1));
    next.focus();
  }

  // Position the ring + card. Falls back to a centered modal when the target is
  // missing/offscreen or the viewport is small — never fragile callout math there.
  function positionStep() {
    if (obMode !== 'tour') return;
    const r = obRoot();
    const ring = $('#onboardRing');
    const card = $('#onboardCard');
    if (!r || !card) return;
    const step = OB_STEPS[obIdx];
    const small = window.matchMedia('(max-width: 760px)').matches;
    const targetEl = step.target ? document.querySelector(step.target) : null;
    const visible = !!targetEl && targetEl.getClientRects().length > 0;

    if (small || !visible) {
      if (ring) ring.classList.add('hidden');
      r.classList.add('modal');
      card.classList.add('centered');
      card.style.left = ''; card.style.top = '';
      return;
    }

    r.classList.remove('modal');
    card.classList.remove('centered');
    if (step.view === 'registry') targetEl.scrollIntoView({ block: 'center', inline: 'nearest' });

    const rect = targetEl.getBoundingClientRect();
    const pad = 6;
    const rx = Math.max(4, rect.left - pad);
    const ry = Math.max(4, rect.top - pad);
    const rw = Math.min(window.innerWidth - 8, rect.width + pad * 2);
    const rh = rect.height + pad * 2;
    ring.classList.remove('hidden');
    ring.style.left = rx + 'px'; ring.style.top = ry + 'px';
    ring.style.width = rw + 'px'; ring.style.height = rh + 'px';

    // Prefer placing the card below the highlight; flip above if it would overflow;
    // clamp horizontally so it always stays on-screen.
    const cr = card.getBoundingClientRect();
    const gap = 14;
    let top = ry + rh + gap;
    if (top + cr.height > window.innerHeight - 8) {
      const above = ry - gap - cr.height;
      top = above >= 8 ? above : Math.max(8, window.innerHeight - cr.height - 8);
    }
    let left = rect.left + rect.width / 2 - cr.width / 2;
    left = Math.max(12, Math.min(left, window.innerWidth - cr.width - 12));
    card.style.left = left + 'px';
    card.style.top = top + 'px';
  }

  // ─── Go ───────────────────────────────────────────────────────
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
})();
