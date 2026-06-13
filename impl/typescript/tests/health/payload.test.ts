/**
 * Tests for HeartbeatPayload.
 *
 * Spec reference: 005 spec.md
 *   - FR-H01: HeartbeatPayload construction
 *   - FR-H02: Mandatory fields (type, version, nextHeartbeatDeadline, role)
 *   - FR-H04: Canonical serialization order
 *   - FR-W02: UnsignedInt64 as JSON strings
 *   - FR-W04: Absent optional fields omitted
 *   - FR-W10: Float64 with .0 for integers
 *
 * Chain 3 conformance vector verified separately in conformance tests.
 */

import { describe, it, expect } from 'vitest';
import { HeartbeatPayload } from '../../src/health/payload.js';

describe('HeartbeatPayload', () => {
  // -------------------------------------------------------------------------
  // Construction
  // -------------------------------------------------------------------------

  describe('build()', () => {
    it('sets type to "heartbeat" automatically (FR-H02)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
      });
      expect(p.type).toBe('heartbeat');
    });

    it('sets version to "1.0.0" automatically (FR-H02)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'buyer',
      });
      expect(p.version).toBe('1.0.0');
    });

    it('stores nextHeartbeatDeadline as bigint', () => {
      const deadline = 1700000060000000000n;
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: deadline,
        role: 'seller',
      });
      expect(p.nextHeartbeatDeadline).toBe(deadline);
    });

    it('stores role', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'relay',
      });
      expect(p.role).toBe('relay');
    });

    it('stores capabilities when provided', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        capabilities: { natReachability: true, protocols: ['/adsb/v1'] },
      });
      expect(p.capabilities).toEqual({ natReachability: true, protocols: ['/adsb/v1'] });
    });

    it('omits capabilities when not provided', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
      });
      expect(p.capabilities).toBeUndefined();
    });

    it('stores location when provided', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        location: { lat: 37.7749, lon: -122.4194 },
      });
      expect(p.location).toEqual({ lat: 37.7749, lon: -122.4194 });
    });

    it('stores peers when provided (defensive copy)', () => {
      const peers = ['ab12', 'cd34'];
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        peers,
      });
      expect(p.peers).toEqual(['ab12', 'cd34']);
      // Defensive copy: modifying original does not affect payload
      peers.push('ef56');
      expect(p.peers).toHaveLength(2);
    });

    it('accepts shutdown sentinel (deadline = 0n)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 0n,
        role: 'seller',
      });
      expect(p.nextHeartbeatDeadline).toBe(0n);
    });
  });

  // -------------------------------------------------------------------------
  // Canonical JSON serialization
  // -------------------------------------------------------------------------

  describe('toCanonicalJson()', () => {
    // FR-H04: type -> version -> nextHeartbeatDeadline -> role -> capabilities -> location -> peers
    it('serializes mandatory fields in canonical order (FR-H04)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
      });
      const json = p.toCanonicalJson();
      expect(json).toBe(
        '{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":"1700000060000000000","role":"seller"}',
      );
    });

    // FR-W02: UnsignedInt64 as JSON string
    it('serializes nextHeartbeatDeadline as a JSON string (FR-W02)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 42n,
        role: 'buyer',
      });
      const json = p.toCanonicalJson();
      expect(json).toContain('"nextHeartbeatDeadline":"42"');
    });

    // FR-W04: Absent optional fields omitted
    it('omits absent optional fields (FR-W04)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
      });
      const json = p.toCanonicalJson();
      expect(json).not.toContain('capabilities');
      expect(json).not.toContain('location');
      expect(json).not.toContain('peers');
    });

    // FR-H12: Shutdown sentinel
    it('serializes shutdown sentinel as "0" (FR-H12)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 0n,
        role: 'seller',
      });
      const json = p.toCanonicalJson();
      expect(json).toContain('"nextHeartbeatDeadline":"0"');
    });

    // Capabilities nested object with correct field order
    it('serializes capabilities with correct nested field order', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        capabilities: {
          natReachability: true,
          natType: 'endpoint-independent',
          protocols: ['/adsb/v1'],
        },
      });
      const json = p.toCanonicalJson();
      expect(json).toContain(
        '"capabilities":{"natReachability":true,"natType":"endpoint-independent","protocols":["/adsb/v1"]}',
      );
    });

    // Capabilities: natType omitted when absent
    it('omits natType in capabilities when absent', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        capabilities: { natReachability: true, protocols: ['/adsb/v1'] },
      });
      const json = p.toCanonicalJson();
      expect(json).toContain(
        '"capabilities":{"natReachability":true,"protocols":["/adsb/v1"]}',
      );
      expect(json).not.toContain('natType');
    });

    // Chain 3 test vector (exact match)
    it('matches Chain 3 test vector payload JSON exactly', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        capabilities: { natReachability: true, protocols: ['/adsb/v1'] },
      });
      expect(p.toCanonicalJson()).toBe(
        '{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":"1700000060000000000","role":"seller","capabilities":{"natReachability":true,"protocols":["/adsb/v1"]}}',
      );
    });

    // Location nested object
    it('serializes location with correct nested field order (FR-W10)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        location: { lat: 37.7749, lon: -122.4194, alt: 15.0, fix: '3D' },
      });
      const json = p.toCanonicalJson();
      expect(json).toContain(
        '"location":{"lat":37.7749,"lon":-122.4194,"alt":15.0,"fix":"3D"}',
      );
    });

    // Location: optional fields omitted
    it('omits alt and fix in location when absent', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        location: { lat: 37.7749, lon: -122.4194 },
      });
      const json = p.toCanonicalJson();
      expect(json).toContain('"location":{"lat":37.7749,"lon":-122.4194}');
      expect(json).not.toContain('alt');
      expect(json).not.toContain('fix');
    });

    // FR-W10: Integer float serialized with .0
    it('serializes integer lat/lon with .0 suffix (FR-W10)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        location: { lat: 37.0, lon: -122.0 },
      });
      const json = p.toCanonicalJson();
      expect(json).toContain('"lat":37.0');
      expect(json).toContain('"lon":-122.0');
    });

    // Peers array
    it('serializes peers as array of strings', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        peers: ['ab12', 'cd34'],
      });
      const json = p.toCanonicalJson();
      expect(json).toContain('"peers":["ab12","cd34"]');
    });

    // All optional fields present
    it('serializes all fields when everything is present', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        capabilities: { natReachability: false },
        location: { lat: 0.0, lon: 0.0 },
        peers: ['ab12'],
      });
      const json = p.toCanonicalJson();
      // Verify field order: type, version, nextHeartbeatDeadline, role, capabilities, location, peers
      const typeIdx = json.indexOf('"type"');
      const versionIdx = json.indexOf('"version"');
      const deadlineIdx = json.indexOf('"nextHeartbeatDeadline"');
      const roleIdx = json.indexOf('"role"');
      const capIdx = json.indexOf('"capabilities"');
      const locIdx = json.indexOf('"location"');
      const peersIdx = json.indexOf('"peers"');

      expect(typeIdx).toBeLessThan(versionIdx);
      expect(versionIdx).toBeLessThan(deadlineIdx);
      expect(deadlineIdx).toBeLessThan(roleIdx);
      expect(roleIdx).toBeLessThan(capIdx);
      expect(capIdx).toBeLessThan(locIdx);
      expect(locIdx).toBeLessThan(peersIdx);
    });

    // FR-W01: Compact (no whitespace)
    it('produces compact JSON with no whitespace (FR-W01)', () => {
      const p = HeartbeatPayload.build({
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller',
        capabilities: { natReachability: true },
        location: { lat: 37.7749, lon: -122.4194 },
        peers: ['ab12'],
      });
      const json = p.toCanonicalJson();
      // No spaces, tabs, or newlines outside of string values
      expect(json).not.toMatch(/(?<!\\)["\s]\s/);
      expect(json.startsWith('{')).toBe(true);
      expect(json.endsWith('}')).toBe(true);
    });

    // Determinism (Constitution X)
    it('produces identical results on repeated serialization', () => {
      const opts = {
        nextHeartbeatDeadline: 1700000060000000000n,
        role: 'seller' as const,
        capabilities: { natReachability: true, protocols: ['/adsb/v1'] },
      };
      const json1 = HeartbeatPayload.build(opts).toCanonicalJson();
      const json2 = HeartbeatPayload.build(opts).toCanonicalJson();
      expect(json1).toBe(json2);
    });
  });
});
