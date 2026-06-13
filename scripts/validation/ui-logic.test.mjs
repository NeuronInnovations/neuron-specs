// ui-logic.test.mjs — node --test suite for the FID display's pure UI logic
// (impl/golang/cmd/sapient-fid-display/static/logic.js). Run via
// scripts/validation/ui-logic-test.sh (skips cleanly when node is absent).
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const require = createRequire(import.meta.url);
const FL = require(join(dirname(fileURLToPath(import.meta.url)), '../../impl/golang/cmd/sapient-fid-display/static/logic.js'));

const aircraft = (over = {}) => ({ uid: 'AC1', nodeId: 'node-jv', kind: 'adsb', adsb: { source: 'A' }, ...over });
const drone = (over = {}) => ({ uid: 'D1', nodeId: 'node-rid', rid: { operatorLat: 1, operatorLon: 2 }, ...over });

function allFilters() {
  return {
    modality: new Set(['aircraft', 'drones', 'operators', 'sensors']),
    source: new Set(['node-jv', 'node-rid', '']),
    status: new Set(['live', 'stale', 'simulated', 'onchain']),
    adsbSource: new Set(['adsb', 'mlat', 'flarm-ogn', 'uat', 'relayed', 'other']),
  };
}

test('trackKeyOf parity with Go trackKey (composite + legacy bare uid)', () => {
  assert.equal(FL.trackKeyOf({ nodeId: 'n1', uid: 'u1' }), 'n1|u1');
  assert.equal(FL.trackKeyOf({ uid: 'u1' }), 'u1');
  assert.equal(FL.trackKeyOf({ nodeId: '', uid: 'u1' }), 'u1');
});

test('markerKind: aircraft never renders as drone and vice versa', () => {
  assert.equal(FL.markerKind(aircraft()), 'aircraft');
  assert.equal(FL.markerKind(drone()), 'drone');
  assert.equal(FL.markerKind({ uid: 'x' }), 'drone', 'legacy/no-kind = drone path');
});

test('labelForCounts matrix', () => {
  assert.equal(FL.labelForCounts({ aircraft: 3, drones: 2 }), 'live tracks');
  assert.equal(FL.labelForCounts({ aircraft: 3, drones: 0 }), 'live aircraft');
  assert.equal(FL.labelForCounts({ aircraft: 0, drones: 2 }), 'live drones');
  assert.equal(FL.labelForCounts({ aircraft: 0, drones: 0 }), 'live tracks');
  assert.equal(FL.labelForCounts({}), 'live tracks');
});

test('adsbSourceClass letter table + provenance + UAT via frequency', () => {
  assert.equal(FL.adsbSourceClass(aircraft()).cls, 'adsb');
  assert.equal(FL.adsbSourceClass(aircraft({ adsb: { source: 'F' } })).cls, 'adsb');
  assert.equal(FL.adsbSourceClass(aircraft({ adsb: { source: 'M' } })).cls, 'mlat');
  for (const letter of ['L', 'O', 'S', 'D']) {
    assert.equal(FL.adsbSourceClass(aircraft({ adsb: { source: letter } })).cls, 'flarm-ogn');
  }
  assert.equal(FL.adsbSourceClass(aircraft({ adsb: { source: 'O', provenance: 'relayed' } })).cls, 'relayed',
    'provenance=relayed takes precedence');
  assert.equal(FL.adsbSourceClass(aircraft({ adsb: { source: 'A' }, rf: { frequencyHz: 978e6 } })).cls, 'uat',
    'UAT detected via 978 MHz');
  assert.equal(FL.adsbSourceClass(aircraft({ adsb: { source: '?' } })).cls, 'other');
  assert.equal(FL.adsbSourceClass(aircraft({ adsb: { source: '?' } })).raw, '?', 'raw letter always surfaced');
  assert.equal(FL.adsbSourceClass({ uid: 'd' }).cls, 'other', 'no adsb block');
});

test('isStale boundary + skew-corrected age', () => {
  const now = 1_000_000;
  assert.equal(FL.isStale(now - 59_000, now, 60_000), false);
  assert.equal(FL.isStale(now - 61_000, now, 60_000), true);
  assert.equal(FL.isStale(0, now, 60_000), true, 'never-seen is stale');
  // skew: client is 10s ahead of server → age shrinks by 10s.
  assert.equal(FL.trackAgeMs(now - 30_000, now, 10_000), 20_000);
  assert.equal(FL.trackAgeMs(now - 30_000, now, 0), 30_000);
});

test('passesFilters: modality group', () => {
  const f = allFilters();
  assert.equal(FL.passesFilters(aircraft(), f, { stale: false }), true);
  f.modality.delete('aircraft');
  assert.equal(FL.passesFilters(aircraft(), f, { stale: false }), false);
  assert.equal(FL.passesFilters(drone(), f, { stale: false }), true, 'drones unaffected');
});

test('passesFilters: source group', () => {
  const f = allFilters();
  f.source.delete('node-jv');
  assert.equal(FL.passesFilters(aircraft(), f, { stale: false }), false);
  assert.equal(FL.passesFilters(drone(), f, { stale: false }), true);
});

test('passesFilters: status group (live/stale/simulated/onchain)', () => {
  const f = allFilters();
  f.status.delete('stale');
  assert.equal(FL.passesFilters(aircraft(), f, { stale: true }), false);
  assert.equal(FL.passesFilters(aircraft(), f, { stale: false }), true);

  const f2 = allFilters();
  f2.status.delete('onchain');
  assert.equal(FL.passesFilters(aircraft({ agent: { simulated: false } }), f2, { stale: false }), false);
  assert.equal(FL.passesFilters(aircraft({ agent: { simulated: true } }), f2, { stale: false }), true);
  assert.equal(FL.passesFilters(aircraft(), f2, { stale: false }), true, 'no agent block → registry filter not applicable');
});

test('passesFilters: adsbSource group constrains only aircraft', () => {
  const f = allFilters();
  f.adsbSource.delete('adsb');
  assert.equal(FL.passesFilters(aircraft(), f, { stale: false }), false);
  assert.equal(FL.passesFilters(aircraft({ adsb: { source: 'M' } }), f, { stale: false }), true);
  assert.equal(FL.passesFilters(drone(), f, { stale: false }), true, 'drones never hidden by the adsb-source group');
});

test('passesFilters: groups combine with AND', () => {
  const f = allFilters();
  f.modality = new Set(['aircraft']);
  f.source = new Set(['node-jv']);
  f.status = new Set(['live', 'simulated', 'onchain']);
  f.adsbSource = new Set(['adsb']);
  assert.equal(FL.passesFilters(aircraft(), f, { stale: false }), true);
  assert.equal(FL.passesFilters(aircraft({ adsb: { source: 'M' } }), f, { stale: false }), false, 'mlat unchecked');
  assert.equal(FL.passesFilters(aircraft(), f, { stale: true }), false, 'stale unchecked');
  assert.equal(FL.passesFilters(drone(), f, { stale: false }), false, 'drones modality unchecked');
});

test('sourceStatusLabel honest sub-labels', () => {
  assert.deepEqual(FL.sourceStatusLabel({ status: 'live' }), { main: 'LIVE', sub: '' });
  assert.deepEqual(FL.sourceStatusLabel({ status: 'live', awaitingFirstMessage: true }),
    { main: 'LIVE', sub: 'connected · awaiting first message' });
  assert.deepEqual(FL.sourceStatusLabel({ status: 'stale', sessionConnected: true }),
    { main: 'STALE', sub: 'connected · no recent messages' });
  assert.deepEqual(FL.sourceStatusLabel({ status: 'offline' }), { main: 'OFFLINE', sub: 'no buyer session' });
  assert.equal(FL.sourceStatusLabel({ status: 'bogus' }).main, 'UNKNOWN');
});

test('hasOperator', () => {
  assert.equal(FL.hasOperator(drone()), true);
  assert.equal(FL.hasOperator(aircraft()), false);
  assert.equal(FL.hasOperator({ rid: { operatorLat: 1 } }), false, 'both coords required');
});

test('recenterStates matrix: per-modality buttons, honest disabled-at-0', () => {
  // Both modalities live → both enabled, independent counts.
  let st = FL.recenterStates({ liveAircraft: 4, liveDrones: 2 });
  assert.deepEqual(st, [
    { kind: 'aircraft', label: 'live aircraft', count: 4, enabled: true },
    { kind: 'drone', label: 'live drones', count: 2, enabled: true },
  ]);
  // Aircraft-only (the live JetVision/DroneScout-offline state): the drones
  // button is DISABLED with an honest 0 — never pretending drones are live.
  st = FL.recenterStates({ liveAircraft: 4, liveDrones: 0 });
  assert.equal(st[0].enabled, true);
  assert.deepEqual(st[1], { kind: 'drone', label: 'live drones', count: 0, enabled: false });
  // Drones-only mirrors it.
  st = FL.recenterStates({ liveAircraft: 0, liveDrones: 3 });
  assert.deepEqual(st[0], { kind: 'aircraft', label: 'live aircraft', count: 0, enabled: false });
  assert.equal(st[1].enabled, true);
  // Nothing live → both disabled. Missing counts behave as 0.
  for (const counts of [{ liveAircraft: 0, liveDrones: 0 }, {}, undefined]) {
    st = FL.recenterStates(counts);
    assert.equal(st[0].enabled, false);
    assert.equal(st[1].enabled, false);
  }
  // Correct English label, singular collective — never "aircrafts".
  assert.equal(st[0].label, 'live aircraft');
});

test('focusForKind: no cross-modality recentering', () => {
  const now = 1_000_000;
  const fresh = new Date(now - 5_000).toISOString();
  const old = new Date(now - 120_000).toISOString();
  const snaps = [
    aircraft({ uid: 'A1', position: { lat: 50, lon: -5 }, lastSeen: fresh }),
    aircraft({ uid: 'A2', position: { lat: 52, lon: -3 }, lastSeen: fresh }),
    drone({ uid: 'D1', position: { lat: 10, lon: 10 }, lastSeen: fresh }),
    drone({ uid: 'D2', position: { lat: 10.1, lon: 10.1 }, lastSeen: old }),   // stale → excluded
    drone({ uid: 'D3', lastSeen: fresh }),                                      // no position → excluded
  ];
  const fa = FL.focusForKind(snaps, 'aircraft', now, 0, 60_000);
  assert.equal(fa.count, 2);
  assert.deepEqual([fa.minLat, fa.maxLat, fa.minLon, fa.maxLon], [50, 52, -5, -3],
    'aircraft bounds never include drone positions');
  const fd = FL.focusForKind(snaps, 'drone', now, 0, 60_000);
  assert.equal(fd.count, 1, 'stale + position-less drones excluded');
  assert.deepEqual([fd.lat, fd.lon], [10, 10]);
  assert.deepEqual([fd.minLat, fd.maxLat], [10, 10], 'single track → degenerate bounds');
  assert.equal(FL.focusForKind([snaps[2]], 'aircraft', now, 0, 60_000), null,
    'no live aircraft → null, never a drone fallback');
  // Skew correction: client 100s ahead of server → fresh-by-server stays live.
  const skewed = FL.focusForKind(snaps, 'aircraft', now + 100_000, 100_000, 60_000);
  assert.equal(skewed.count, 2);
});

test('classLine: shared class/confidence text for both UIs', () => {
  // ADS-B aircraft fixture — the same payload must render identically in the
  // display and the explorer (both call this exact function).
  assert.equal(FL.classLine({ type: 'Air Vehicle', confidence: 0.95 }), 'Air Vehicle · conf 95%');
  assert.equal(FL.classLine({ type: 'Air Vehicle', confidence: 0.4 }), 'Air Vehicle · conf 40%');
  // Missing/zero confidence is an explicit honest state, not a silent hide
  // and never an invented default.
  assert.equal(FL.classLine({ type: 'Air Vehicle' }), 'Air Vehicle · conf not provided');
  assert.equal(FL.classLine({ type: 'Air Vehicle', confidence: 0 }), 'Air Vehicle · conf not provided');
  assert.equal(FL.classLine({ confidence: 0.5 }), '— · conf 50%');
  assert.equal(FL.classLine(null), null, 'no classification block → no row');
});

test('shortId: bounded middle-ellipsis so long IDs cannot blow layouts', () => {
  const evm = '0x60fF31A2bb1b3D9aBcDeF01234567890aB6826ff';
  assert.equal(FL.shortId(evm, 10, 6), '0x60fF31A2…6826ff');
  assert.equal(FL.shortId(evm, 10, 6).length, 10 + 6 + 1);
  assert.equal(FL.shortId('short', 10, 6), 'short', 'short strings pass through');
  assert.equal(FL.shortId('', 10, 6), '—');
  assert.equal(FL.shortId(null, 10, 6), '—');
  // Defaults match the explorer's historical short() (10, 6).
  assert.equal(FL.shortId(evm), '0x60fF31A2…6826ff');
});
